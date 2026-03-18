#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <signal.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <fcntl.h>
#include <dirent.h>
#include <time.h>
#include <errno.h>
#include <pthread.h>
#include <sqlite3.h>

/* Shared library headers */
#include "common.h"
#include "logging.h"
#include "config.h"
#include "daemon.h"

#define VERSION "1.1.0"
#define DEFAULT_CONFIG_FILE "ingestion.ini"
#define DEFAULT_PID_FILE "/tmp/ingestion.pid"
#define BUFFER_SIZE 8192
#define MAX_REQUEST_SIZE 4096

/* Configuration structure */
typedef struct {
    char database_path[256];
    char csv_directory[256];
    int poll_interval_seconds;
    int api_port;
} IngestionConfig;

typedef struct ProcessedFileInfo {
    char filepath[512];
    time_t mtime;
    off_t size;
    struct ProcessedFileInfo *next;
} ProcessedFileInfo;

/* Global state */
typedef struct {
    IngestionConfig config;
    sqlite3 *db;
    int server_socket;
    Logger logger;
    DaemonState daemon;
    pthread_t api_thread;
    int api_thread_started;
    ProcessedFileInfo *processed_files;
} IngestionState;

static IngestionState g_state = {0};

/* HTTP response structure */
typedef struct {
    int status_code;
    const char *content_type;
    char body[BUFFER_SIZE];
} HttpResponse;

/* Forward declarations */
static int parse_config_handler(const char *key, const char *value, void *user_data);
static int init_database(const char *db_path);
static int ingest_csv_file(const char *filepath);
static void cleanup(void);
static void process_csv_files(void);
static int start_api_server(void);
static void handle_api_request(int client_socket);
static void handle_pending_api_connections(void);
static void *api_server_thread(void *arg);
static void send_http_response(int client_socket, HttpResponse *response);
static void send_http_response_body(int client_socket, int status_code, const char *content_type, const char *body);
static const char *get_status_text(int code);
static ProcessedFileInfo *find_processed_file(const char *filepath);
static int should_process_file(const char *filepath, time_t mtime, off_t size);
static int mark_file_processed(const char *filepath, time_t mtime, off_t size);
static int load_processed_files_from_db(void);
static void free_processed_files(void);

/* Configuration handler callback */
static int parse_config_handler(const char *key, const char *value, void *user_data) {
    IngestionConfig *config = (IngestionConfig *)user_data;
    int log_level;
    char log_file[256];
    int daemon_mode;
    
    /* Handle common configuration keys */
    if (config_handle_common(key, value, &log_level, log_file, sizeof(log_file), &daemon_mode)) {
        if (strcmp(key, "log_level") == 0) {
            g_state.logger.level = (LogLevel)log_level;
        }
        return 0;
    }
    
    /* Handle ingestion-specific keys */
    if (strcmp(key, "database_path") == 0) {
        SAFE_STRCPY(config->database_path, value, sizeof(config->database_path));
    } else if (strcmp(key, "csv_directory") == 0) {
        SAFE_STRCPY(config->csv_directory, value, sizeof(config->csv_directory));
    } else if (strcmp(key, "poll_interval_seconds") == 0) {
        config->poll_interval_seconds = atoi(value);
    } else if (strcmp(key, "api_port") == 0) {
        config->api_port = atoi(value);
    } else {
        LOG_WARN(&g_state.logger, "Unknown config key: %s", key);
    }
    
    return 0;
}

/* Initialize SQLite database */
static int init_database(const char *db_path) {
    int rc = sqlite3_open(db_path, &g_state.db);
    if (rc != SQLITE_OK) {
        LOG_ERROR(&g_state.logger, "Cannot open database: %s", sqlite3_errmsg(g_state.db));
        return -1;
    }
    
    /* Create table if not exists */
    const char *create_table_sql = 
        "CREATE TABLE IF NOT EXISTS weather_data ("
        "id INTEGER PRIMARY KEY AUTOINCREMENT,"
        "station_id TEXT NOT NULL,"
        "date TEXT NOT NULL,"
        "element TEXT NOT NULL,"
        "value REAL,"
        "mflag TEXT,"
        "qflag TEXT,"
        "sflag TEXT,"
        "created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP"
        ");";
    
    char *err_msg = NULL;
    rc = sqlite3_exec(g_state.db, create_table_sql, NULL, NULL, &err_msg);
    if (rc != SQLITE_OK) {
        LOG_ERROR(&g_state.logger, "SQL error: %s", err_msg);
        sqlite3_free(err_msg);
        return -1;
    }
    
    /* Create indexes */
    const char *create_indexes[] = {
        "CREATE INDEX IF NOT EXISTS idx_station_date ON weather_data(station_id, date);",
        "CREATE INDEX IF NOT EXISTS idx_element ON weather_data(element);",
        NULL
    };
    
    for (int i = 0; create_indexes[i] != NULL; i++) {
        rc = sqlite3_exec(g_state.db, create_indexes[i], NULL, NULL, &err_msg);
        if (rc != SQLITE_OK) {
            LOG_ERROR(&g_state.logger, "SQL error creating index: %s", err_msg);
            sqlite3_free(err_msg);
        }
    }

    const char *create_processed_files_sql =
        "CREATE TABLE IF NOT EXISTS processed_files ("
        "filepath TEXT PRIMARY KEY,"
        "mtime INTEGER NOT NULL,"
        "size INTEGER NOT NULL,"
        "updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP"
        ");";

    rc = sqlite3_exec(g_state.db, create_processed_files_sql, NULL, NULL, &err_msg);
    if (rc != SQLITE_OK) {
        LOG_ERROR(&g_state.logger, "SQL error creating processed_files table: %s", err_msg);
        sqlite3_free(err_msg);
        return -1;
    }
    
    LOG_INFO(&g_state.logger, "Database initialized: %s", db_path);
    return 0;
}

/* Ingest a single CSV file */
static int ingest_csv_file(const char *filepath) {
    FILE *fp = fopen(filepath, "r");
    if (!fp) {
        LOG_ERROR(&g_state.logger, "Cannot open CSV file: %s", filepath);
        return -1;
    }
    
    LOG_INFO(&g_state.logger, "Ingesting file: %s", filepath);
    
    /* Prepare insert statement */
    const char *insert_sql =
        "INSERT INTO weather_data (station_id, date, element, value, mflag, qflag, sflag) "
        "SELECT ?, ?, ?, ?, ?, ?, ? "
        "WHERE NOT EXISTS ("
        "SELECT 1 FROM weather_data WHERE station_id = ? AND date = ? AND element = ? AND value = ? "
        "AND COALESCE(mflag, '') = ? AND COALESCE(qflag, '') = ? AND COALESCE(sflag, '') = ?"
        ");";
    
    sqlite3_stmt *stmt;
    int rc = sqlite3_prepare_v2(g_state.db, insert_sql, -1, &stmt, NULL);
    if (rc != SQLITE_OK) {
        LOG_ERROR(&g_state.logger, "Failed to prepare statement: %s", sqlite3_errmsg(g_state.db));
        fclose(fp);
        return -1;
    }
    
    /* Begin transaction */
    sqlite3_exec(g_state.db, "BEGIN TRANSACTION;", NULL, NULL, NULL);
    
    char line[BUFFER_SIZE];
    int line_count = 0;
    int inserted_count = 0;
    int duplicate_count = 0;
    int error_count = 0;
    
    /* Skip header if present */
    fgets(line, sizeof(line), fp);
    
    while (fgets(line, sizeof(line), fp)) {
        line_count++;
        
        /* Parse CSV line */
        /* NOAA GHCN-Daily format: station_id,date,element,value,mflag,qflag,sflag */
        char station_id[12] = {0};
        char date[9] = {0};
        char element[6] = {0};
        double value = 0;
        char mflag[2] = {0};
        char qflag[2] = {0};
        char sflag[2] = {0};
        
        int parsed = sscanf(line, "%11[^,],%8[^,],%5[^,],%lf,%1[^,],%1[^,],%1s",
                           station_id, date, element, &value, mflag, qflag, sflag);
        
        if (parsed < 4) {
            LOG_WARN(&g_state.logger, "Skipping malformed line %d in %s", line_count, filepath);
            error_count++;
            continue;
        }

        const char *mflag_insert = mflag[0] ? mflag : NULL;
        const char *qflag_insert = qflag[0] ? qflag : NULL;
        const char *sflag_insert = sflag[0] ? sflag : NULL;
        const char *mflag_compare = mflag[0] ? mflag : "";
        const char *qflag_compare = qflag[0] ? qflag : "";
        const char *sflag_compare = sflag[0] ? sflag : "";
        
        /* Bind parameters */
        sqlite3_bind_text(stmt, 1, station_id, -1, SQLITE_STATIC);
        sqlite3_bind_text(stmt, 2, date, -1, SQLITE_STATIC);
        sqlite3_bind_text(stmt, 3, element, -1, SQLITE_STATIC);
        sqlite3_bind_double(stmt, 4, value);
        sqlite3_bind_text(stmt, 5, mflag_insert, -1, SQLITE_STATIC);
        sqlite3_bind_text(stmt, 6, qflag_insert, -1, SQLITE_STATIC);
        sqlite3_bind_text(stmt, 7, sflag_insert, -1, SQLITE_STATIC);
        sqlite3_bind_text(stmt, 8, station_id, -1, SQLITE_STATIC);
        sqlite3_bind_text(stmt, 9, date, -1, SQLITE_STATIC);
        sqlite3_bind_text(stmt, 10, element, -1, SQLITE_STATIC);
        sqlite3_bind_double(stmt, 11, value);
        sqlite3_bind_text(stmt, 12, mflag_compare, -1, SQLITE_STATIC);
        sqlite3_bind_text(stmt, 13, qflag_compare, -1, SQLITE_STATIC);
        sqlite3_bind_text(stmt, 14, sflag_compare, -1, SQLITE_STATIC);
        
        /* Execute */
        rc = sqlite3_step(stmt);
        if (rc != SQLITE_DONE) {
            LOG_WARN(&g_state.logger, "Failed to insert line %d: %s", line_count, sqlite3_errmsg(g_state.db));
            error_count++;
        } else {
            if (sqlite3_changes(g_state.db) > 0) {
                inserted_count++;
            } else {
                duplicate_count++;
            }
        }
        
        /* Reset for next iteration */
        sqlite3_reset(stmt);
        sqlite3_clear_bindings(stmt);
    }
    
    /* Commit transaction */
    sqlite3_exec(g_state.db, "COMMIT;", NULL, NULL, NULL);
    
    sqlite3_finalize(stmt);
    fclose(fp);
    
    LOG_INFO(&g_state.logger, "File %s processed: %d lines, %d inserted, %d duplicates skipped, %d errors",
                filepath, line_count, inserted_count, duplicate_count, error_count);
    
    return (error_count > 0) ? -1 : 0;
}

static ProcessedFileInfo *find_processed_file(const char *filepath) {
    ProcessedFileInfo *current = g_state.processed_files;
    while (current) {
        if (strcmp(current->filepath, filepath) == 0) {
            return current;
        }
        current = current->next;
    }
    return NULL;
}

static int should_process_file(const char *filepath, time_t mtime, off_t size) {
    ProcessedFileInfo *info = find_processed_file(filepath);
    if (!info) {
        return 1;
    }

    if (info->mtime != mtime || info->size != size) {
        return 1;
    }

    return 0;
}

static int mark_file_processed(const char *filepath, time_t mtime, off_t size) {
    ProcessedFileInfo *info = find_processed_file(filepath);
    if (!info) {
        info = (ProcessedFileInfo *)malloc(sizeof(ProcessedFileInfo));
        if (!info) {
            LOG_ERROR(&g_state.logger, "Failed to allocate processed file tracker for %s", filepath);
            return -1;
        }
        memset(info, 0, sizeof(ProcessedFileInfo));
        SAFE_STRCPY(info->filepath, filepath, sizeof(info->filepath));
        info->next = g_state.processed_files;
        g_state.processed_files = info;
    }

    info->mtime = mtime;
    info->size = size;

    const char *upsert_sql =
        "INSERT INTO processed_files (filepath, mtime, size, updated_at) "
        "VALUES (?, ?, ?, CURRENT_TIMESTAMP) "
        "ON CONFLICT(filepath) DO UPDATE SET "
        "mtime = excluded.mtime, "
        "size = excluded.size, "
        "updated_at = CURRENT_TIMESTAMP;";

    sqlite3_stmt *stmt = NULL;
    int rc = sqlite3_prepare_v2(g_state.db, upsert_sql, -1, &stmt, NULL);
    if (rc != SQLITE_OK) {
        LOG_ERROR(&g_state.logger, "Failed to prepare processed_files upsert: %s", sqlite3_errmsg(g_state.db));
        return -1;
    }

    sqlite3_bind_text(stmt, 1, filepath, -1, SQLITE_STATIC);
    sqlite3_bind_int64(stmt, 2, (sqlite3_int64)mtime);
    sqlite3_bind_int64(stmt, 3, (sqlite3_int64)size);

    rc = sqlite3_step(stmt);
    sqlite3_finalize(stmt);

    if (rc != SQLITE_DONE) {
        LOG_ERROR(&g_state.logger, "Failed to persist processed file metadata for %s: %s",
                  filepath, sqlite3_errmsg(g_state.db));
        return -1;
    }

    return 0;
}

static int load_processed_files_from_db(void) {
    const char *sql = "SELECT filepath, mtime, size FROM processed_files";
    sqlite3_stmt *stmt = NULL;
    int rc = sqlite3_prepare_v2(g_state.db, sql, -1, &stmt, NULL);
    if (rc != SQLITE_OK) {
        LOG_ERROR(&g_state.logger, "Failed to prepare processed_files load query: %s", sqlite3_errmsg(g_state.db));
        return -1;
    }

    int loaded = 0;
    while ((rc = sqlite3_step(stmt)) == SQLITE_ROW) {
        const char *filepath = (const char *)sqlite3_column_text(stmt, 0);
        time_t mtime = (time_t)sqlite3_column_int64(stmt, 1);
        off_t size = (off_t)sqlite3_column_int64(stmt, 2);

        if (!filepath || filepath[0] == '\0') {
            continue;
        }

        ProcessedFileInfo *info = (ProcessedFileInfo *)malloc(sizeof(ProcessedFileInfo));
        if (!info) {
            LOG_ERROR(&g_state.logger, "Out of memory while loading processed file metadata");
            sqlite3_finalize(stmt);
            return -1;
        }

        memset(info, 0, sizeof(ProcessedFileInfo));
        SAFE_STRCPY(info->filepath, filepath, sizeof(info->filepath));
        info->mtime = mtime;
        info->size = size;
        info->next = g_state.processed_files;
        g_state.processed_files = info;
        loaded++;
    }

    sqlite3_finalize(stmt);

    if (rc != SQLITE_DONE) {
        LOG_ERROR(&g_state.logger, "Failed while reading processed_files rows: %s", sqlite3_errmsg(g_state.db));
        return -1;
    }

    LOG_INFO(&g_state.logger, "Loaded %d processed file metadata entries", loaded);
    return 0;
}

static void free_processed_files(void) {
    ProcessedFileInfo *current = g_state.processed_files;
    while (current) {
        ProcessedFileInfo *next = current->next;
        free(current);
        current = next;
    }
    g_state.processed_files = NULL;
}

/* Process all CSV files in directory */
static void process_csv_files(void) {
    DIR *dir = opendir(g_state.config.csv_directory);
    if (dir) {
        struct dirent *entry;
        while ((entry = readdir(dir)) != NULL) {
            /* Check if file ends with .csv */
            int len = strlen(entry->d_name);
            if (len > 4 && strcmp(entry->d_name + len - 4, ".csv") == 0) {
                char filepath[512];
                snprintf(filepath, sizeof(filepath), "%s/%s",
                        g_state.config.csv_directory, entry->d_name);

                struct stat st;
                if (stat(filepath, &st) != 0) {
                    LOG_WARN(&g_state.logger, "Cannot stat CSV file %s: %s", filepath, strerror(errno));
                    continue;
                }

                if (!should_process_file(filepath, st.st_mtime, st.st_size)) {
                    LOG_DEBUG(&g_state.logger, "Skipping unchanged file: %s", filepath);
                    continue;
                }

                if (ingest_csv_file(filepath) == 0) {
                    if (mark_file_processed(filepath, st.st_mtime, st.st_size) != 0) {
                        LOG_WARN(&g_state.logger, "Ingested %s but failed to persist file tracking metadata", filepath);
                    }
                }
            }
        }
        closedir(dir);
    } else {
        LOG_ERROR(&g_state.logger, "Cannot open CSV directory: %s", g_state.config.csv_directory);
    }
}

/* Start HTTP API server */
static int start_api_server(void) {
    int server_fd = socket(AF_INET, SOCK_STREAM, 0);
    if (server_fd < 0) {
        LOG_ERROR(&g_state.logger, "Failed to create socket: %s", strerror(errno));
        return -1;
    }
    
    /* Allow socket reuse */
    int opt = 1;
    if (setsockopt(server_fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt)) < 0) {
        LOG_ERROR(&g_state.logger, "Failed to set socket options: %s", strerror(errno));
        close(server_fd);
        return -1;
    }
    
    struct sockaddr_in address;
    memset(&address, 0, sizeof(address));
    address.sin_family = AF_INET;
    address.sin_addr.s_addr = INADDR_ANY;
    address.sin_port = htons(g_state.config.api_port);
    
    if (bind(server_fd, (struct sockaddr *)&address, sizeof(address)) < 0) {
        LOG_ERROR(&g_state.logger, "Failed to bind to port %d: %s",
                   g_state.config.api_port, strerror(errno));
        close(server_fd);
        return -1;
    }
    
    if (listen(server_fd, 10) < 0) {
        LOG_ERROR(&g_state.logger, "Failed to listen: %s", strerror(errno));
        close(server_fd);
        return -1;
    }
    
    /* Set non-blocking */
    int flags = fcntl(server_fd, F_GETFL, 0);
    fcntl(server_fd, F_SETFL, flags | O_NONBLOCK);
    
    g_state.server_socket = server_fd;
    
    LOG_INFO(&g_state.logger, "API server listening on port %d", g_state.config.api_port);
    
    return 0;
}

static void handle_pending_api_connections(void) {
    struct sockaddr_in client_addr;
    socklen_t addr_len = sizeof(client_addr);

    while (1) {
        int client_socket = accept(g_state.server_socket, (struct sockaddr *)&client_addr, &addr_len);
        if (client_socket < 0) {
            if (errno == EAGAIN || errno == EWOULDBLOCK) {
                break;
            }

            LOG_WARN(&g_state.logger, "Accept failed: %s", strerror(errno));
            break;
        }

        handle_api_request(client_socket);
        addr_len = sizeof(client_addr);
    }
}

static void *api_server_thread(void *arg __attribute__((unused))) {
    while (!daemon_should_stop(&g_state.daemon) && g_state.server_socket >= 0) {
        handle_pending_api_connections();
        usleep(10000); /* 10ms */
    }

    return NULL;
}

/* Handle API request */
static void handle_api_request(int client_socket) {
    char buffer[MAX_REQUEST_SIZE];
    int bytes_read = recv(client_socket, buffer, sizeof(buffer) - 1, 0);
    
    if (bytes_read <= 0) {
        close(client_socket);
        return;
    }
    
    buffer[bytes_read] = '\0';
    
    /* Parse request line */
    char method[16], path[256], protocol[16];
    if (sscanf(buffer, "%15s %255s %15s", method, path, protocol) != 3) {
        HttpResponse response; 
        response.status_code = 400; 
        response.content_type = "application/json"; 
        strncpy(response.body, "{\"error\": \"Bad Request\"}", BUFFER_SIZE - 1); 
        response.body[BUFFER_SIZE - 1] = '\0';
        send_http_response(client_socket, &response);
        close(client_socket);
        return;
    }
    
    LOG_DEBUG(&g_state.logger, "%s %s", method, path);
    
    /* Only handle GET requests */
    if (strcmp(method, "GET") != 0) {
        HttpResponse response; 
        response.status_code = 405; 
        response.content_type = "application/json"; 
        strncpy(response.body, "{\"error\": \"Method Not Allowed\"}", BUFFER_SIZE - 1); 
        response.body[BUFFER_SIZE - 1] = '\0';
        send_http_response(client_socket, &response);
        close(client_socket);
        return;
    }
    
    /* Route request */
    if (strcmp(path, "/health") == 0) {
        HttpResponse response; 
        response.status_code = 200; 
        response.content_type = "application/json"; 
        snprintf(response.body, BUFFER_SIZE, "{\"status\": \"healthy\", \"version\": \"%s\"}", VERSION);
        send_http_response(client_socket, &response);
    } else if (strncmp(path, "/api/v1/weather/raw", 19) == 0) {
        /* Return raw weather data with pagination support */
        /* Parse offset and limit from query string: ?offset=0&limit=10000 */
        int offset = 0;
        int limit = 10000;  /* Default limit to prevent timeouts */
        
        char *query = strchr(path, '?');
        if (query) {
            query++;
            char *offset_param = strstr(query, "offset=");
            if (offset_param) {
                offset = atoi(offset_param + 7);
                if (offset < 0) offset = 0;
            }
            char *limit_param = strstr(query, "limit=");
            if (limit_param) {
                limit = atoi(limit_param + 6);
                if (limit < 1) limit = 10000;
                if (limit > 50000) limit = 50000;  /* Cap at 50k to prevent timeouts */
            }
        }
        
        /* Build SQL query with pagination */
        char sql[256];
        snprintf(sql, sizeof(sql), 
                 "SELECT station_id, date, element, value, mflag, qflag, sflag "
                 "FROM weather_data ORDER BY date DESC LIMIT %d OFFSET %d",
                 limit, offset);
        
        sqlite3_stmt *stmt;
        int rc = sqlite3_prepare_v2(g_state.db, sql, -1, &stmt, NULL);
        if (rc != SQLITE_OK) {
            send_http_response_body(client_socket, 500, "application/json", "{\"error\": \"Database error\"}");
        } else {
            size_t json_capacity = 16384;
            char *json = (char *)malloc(json_capacity);
            if (json == NULL) {
                sqlite3_finalize(stmt);
                send_http_response_body(client_socket, 500, "application/json", "{\"error\": \"Out of memory\"}");
                close(client_socket);
                return;
            }

            size_t pos = (size_t)snprintf(json, json_capacity, "{\"data\": [");
            int first = 1;
            int record_count = 0;
            
            while ((rc = sqlite3_step(stmt)) == SQLITE_ROW) {
                const char *station_id = (const char *)sqlite3_column_text(stmt, 0);
                const char *date = (const char *)sqlite3_column_text(stmt, 1);
                const char *element = (const char *)sqlite3_column_text(stmt, 2);
                double value = sqlite3_column_double(stmt, 3);
                const char *mflag = (const char *)sqlite3_column_text(stmt, 4);
                const char *qflag = (const char *)sqlite3_column_text(stmt, 5);
                const char *sflag = (const char *)sqlite3_column_text(stmt, 6);

                char row[512];
                int row_len = snprintf(row, sizeof(row),
                                       "%s{\"station_id\":\"%s\",\"date\":\"%s\",\"element\":\"%s\","
                                       "\"value\":%.2f,\"mflag\":\"%s\",\"qflag\":\"%s\",\"sflag\":\"%s\"}",
                                       first ? "" : ",",
                                       station_id ? station_id : "",
                                       date ? date : "",
                                       element ? element : "",
                                       value,
                                       mflag ? mflag : "",
                                       qflag ? qflag : "",
                                       sflag ? sflag : "");

                if (row_len < 0) {
                    sqlite3_finalize(stmt);
                    free(json);
                    send_http_response_body(client_socket, 500, "application/json", "{\"error\": \"Serialization error\"}");
                    close(client_socket);
                    return;
                }

                while (pos + (size_t)row_len + 128 >= json_capacity) {
                    size_t new_capacity = json_capacity * 2;
                    char *grown = (char *)realloc(json, new_capacity);
                    if (grown == NULL) {
                        sqlite3_finalize(stmt);
                        free(json);
                        send_http_response_body(client_socket, 500, "application/json", "{\"error\": \"Out of memory\"}");
                        close(client_socket);
                        return;
                    }
                    json = grown;
                    json_capacity = new_capacity;
                }
                
                memcpy(json + pos, row, (size_t)row_len);
                pos += (size_t)row_len;
                json[pos] = '\0';
                
                first = 0;
                record_count++;
            }
            
            sqlite3_finalize(stmt);
            
            /* Get total count */
            int total_count = 0;
            sqlite3_stmt *count_stmt;
            rc = sqlite3_prepare_v2(g_state.db, "SELECT COUNT(*) FROM weather_data", -1, &count_stmt, NULL);
            if (rc == SQLITE_OK) {
                if (sqlite3_step(count_stmt) == SQLITE_ROW) {
                    total_count = sqlite3_column_int(count_stmt, 0);
                }
                sqlite3_finalize(count_stmt);
            }

            while (pos + 128 >= json_capacity) {
                size_t new_capacity = json_capacity * 2;
                char *grown = (char *)realloc(json, new_capacity);
                if (grown == NULL) {
                    free(json);
                    send_http_response_body(client_socket, 500, "application/json", "{\"error\": \"Out of memory\"}");
                    close(client_socket);
                    return;
                }
                json = grown;
                json_capacity = new_capacity;
            }
            
            snprintf(json + pos, json_capacity - pos, "],\"offset\":%d,\"limit\":%d,\"total\":%d,\"count\":%d}",
                     offset, limit, total_count, record_count);

            send_http_response_body(client_socket, 200, "application/json", json);
            free(json);
        }
    } else if (strcmp(path, "/api/v1/stats") == 0) {
        /* Return statistics */
        int count = 0;
        sqlite3_stmt *stmt;
        int rc = sqlite3_prepare_v2(g_state.db, "SELECT COUNT(*) FROM weather_data", -1, &stmt, NULL);
        if (rc == SQLITE_OK) {
            if (sqlite3_step(stmt) == SQLITE_ROW) {
                count = sqlite3_column_int(stmt, 0);
            }
            sqlite3_finalize(stmt);
        }
        
        HttpResponse response;
        response.status_code = 200;
        response.content_type = "application/json";
        snprintf(response.body, BUFFER_SIZE, "{\"total_records\": %d}", count);
        send_http_response(client_socket, &response);
    } else {
        HttpResponse response; 
        response.status_code = 404; 
        response.content_type = "application/json"; 
        strncpy(response.body, "{\"error\": \"Not Found\"}", BUFFER_SIZE - 1); 
        response.body[BUFFER_SIZE - 1] = '\0';
        send_http_response(client_socket, &response);
    }
    
    close(client_socket);
}

/* Send HTTP response */
static void send_http_response(int client_socket, HttpResponse *response) {
    send_http_response_body(client_socket,
                            response->status_code,
                            response->content_type,
                            response->body);
}

static void send_http_response_body(int client_socket, int status_code, const char *content_type, const char *body) {
    char headers[512];
    int header_len = snprintf(headers, sizeof(headers),
                              "HTTP/1.1 %d %s\r\n"
                              "Content-Type: %s\r\n"
                              "Content-Length: %zu\r\n"
                              "Connection: close\r\n"
                              "\r\n",
                              status_code,
                              get_status_text(status_code),
                              content_type,
                              strlen(body));
    
    send(client_socket, headers, header_len, 0);
    send(client_socket, body, strlen(body), 0);
}

/* Get HTTP status text */
static const char *get_status_text(int code) {
    switch (code) {
        case 200: return "OK";
        case 400: return "Bad Request";
        case 404: return "Not Found";
        case 405: return "Method Not Allowed";
        case 500: return "Internal Server Error";
        default: return "Unknown";
    }
}

/* Cleanup function */
static void cleanup(void) {
    LOG_INFO(&g_state.logger, "Cleaning up...");
    
    if (g_state.server_socket > 0) {
        close(g_state.server_socket);
        g_state.server_socket = -1;
    }

    if (g_state.api_thread_started) {
        pthread_join(g_state.api_thread, NULL);
        g_state.api_thread_started = 0;
    }
    
    if (g_state.db) {
        sqlite3_close(g_state.db);
        g_state.db = NULL;
    }
    
    daemon_cleanup(&g_state.daemon);
    free_processed_files();
    logger_close(&g_state.logger);
}

/* Main function */
int main(int argc, char *const argv[]) {
    const char *config_file = DEFAULT_CONFIG_FILE;
    int validate_only = 0;
    int daemon_mode = 0;
    
    /* Parse command line arguments */
    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "--config") == 0 && i + 1 < argc) {
            config_file = argv[++i];
        } else if (strcmp(argv[i], "--daemon") == 0) {
            daemon_mode = 1;
        } else if (strcmp(argv[i], "--validate") == 0) {
            validate_only = 1;
        } else if (strcmp(argv[i], "--help") == 0) {
            printf("Ingestion Service v%s\n", VERSION);
            printf("Usage: %s [options]\n", argv[0]);
            printf("Options:\n");
            printf("  --config <file>    Configuration file (default: %s)\n", DEFAULT_CONFIG_FILE);
            printf("  --daemon           Run as daemon\n");
            printf("  --validate         Validate config and exit\n");
            printf("  --help             Show this help\n");
            return 0;
        }
    }
    
    /* Initialize logger to stderr initially for early messages */
    logger_init(&g_state.logger, "ingestion", LOG_LEVEL_INFO, NULL);
    
    /* Set default configuration */
    strcpy(g_state.config.database_path, "weather.db");
    strcpy(g_state.config.csv_directory, "./data");
    g_state.config.poll_interval_seconds = 60;
    g_state.config.api_port = 8080;
    
    /* Parse configuration */
    if (config_parse(config_file, parse_config_handler, &g_state.config) < 0) {
        fprintf(stderr, "Failed to parse configuration\n");
        return 1;
    }
    
    /* Validate only mode */
    if (validate_only) {
        printf("Configuration validated successfully\n");
        printf("  Database: %s\n", g_state.config.database_path);
        printf("  CSV Directory: %s\n", g_state.config.csv_directory);
        printf("  Poll Interval: %d seconds\n", g_state.config.poll_interval_seconds);
        printf("  API Port: %d\n", g_state.config.api_port);
        return 0;
    }
    
    /* Daemon mode */
    if (daemon_mode) {
        if (daemon_fork() != 0) {
            /* Parent exits */
            return 0;
        }
    }
    
    /* Initialize daemon state */
    daemon_init(&g_state.daemon, &g_state.logger, DEFAULT_PID_FILE, NULL);
    
    /* Setup signal handlers */
    daemon_setup_signals(&g_state.daemon);
    
    /* Initialize database */
    if (init_database(g_state.config.database_path) != 0) {
        cleanup();
        return 1;
    }

    if (load_processed_files_from_db() != 0) {
        cleanup();
        return 1;
    }
    
    /* Start API server */
    if (start_api_server() != 0) {
        cleanup();
        return 1;
    }

    if (pthread_create(&g_state.api_thread, NULL, api_server_thread, NULL) != 0) {
        LOG_ERROR(&g_state.logger, "Failed to create API thread");
        cleanup();
        return 1;
    }
    g_state.api_thread_started = 1;
    
    LOG_INFO(&g_state.logger, "Ingestion Service v%s started", VERSION);
    LOG_INFO(&g_state.logger, "Watching directory: %s", g_state.config.csv_directory);
    LOG_INFO(&g_state.logger, "Database: %s", g_state.config.database_path);
    LOG_INFO(&g_state.logger, "API Port: %d", g_state.config.api_port);
    
    /* Write PID file for health checks */
    daemon_write_pid_file(DEFAULT_PID_FILE);
    
    /* Main loop */
    time_t last_check = time(NULL);
    
    while (!daemon_should_stop(&g_state.daemon)) {
        /* Check for config reload */
        if (daemon_should_reload(&g_state.daemon)) {
            LOG_INFO(&g_state.logger, "Reloading configuration...");
            config_parse(config_file, parse_config_handler, &g_state.config);
        }
        
        /* Check for CSV files */
        time_t now = time(NULL);
        if (now - last_check >= g_state.config.poll_interval_seconds) {
            last_check = now;
            process_csv_files();
        }
        
        /* Sleep for a short period */
        usleep(10000); /* 10ms */
    }
    
    /* Cleanup */
    cleanup();
    
    LOG_INFO(&g_state.logger, "Ingestion Service stopped");
    
    return 0;
}
