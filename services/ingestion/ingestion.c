#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <signal.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <dirent.h>
#include <time.h>
#include <errno.h>
#include <sqlite3.h>
#include <ctype.h>

#define VERSION "1.0.0"
#define DEFAULT_CONFIG_FILE "ingestion.ini"
#define DEFAULT_PID_FILE "/tmp/ingestion.pid"
#define BUFFER_SIZE 4096

/* Configuration structure */
typedef struct {
    char database_path[256];
    char csv_directory[256];
    char log_file[256];
    int poll_interval_seconds;
    int log_level;
    int daemon_mode;
} Config;

/* Global state */
typedef struct {
    Config config;
    sqlite3 *db;
    volatile int running;
    volatile int reload_config;
    FILE *log_fp;
} State;

static State g_state = {0};

/* Log levels */
enum {
    LOG_DEBUG = 0,
    LOG_INFO,
    LOG_WARN,
    LOG_ERROR
};

/* Function prototypes */
static void log_message(int level, const char *format, ...);
static int parse_config(const char *filename, Config *config);
static int init_database(const char *db_path);
static int ingest_csv_file(const char *filepath);
static void signal_handler(int sig);
static void cleanup(void);
static int write_pid_file(const char *pid_file);
static void remove_pid_file(const char *pid_file);

/* Logging function */
static void log_message(int level, const char *format, ...) {
    if (level < g_state.config.log_level) return;
    
    const char *level_str[] = {"DEBUG", "INFO", "WARN", "ERROR"};
    time_t now = time(NULL);
    struct tm *tm_info = localtime(&now);
    char timestamp[26];
    strftime(timestamp, 26, "%Y-%m-%d %H:%M:%S", tm_info);
    
    FILE *output = g_state.log_fp ? g_state.log_fp : stdout;
    
    fprintf(output, "[%s] [%s] ", timestamp, level_str[level]);
    
    va_list args;
    va_start(args, format);
    vfprintf(output, format, args);
    va_end(args);
    
    fprintf(output, "\n");
    fflush(output);
}

/* Parse configuration file */
static int parse_config(const char *filename, Config *config) {
    FILE *fp = fopen(filename, "r");
    if (!fp) {
        log_message(LOG_ERROR, "Cannot open config file: %s", filename);
        return -1;
    }
    
    char line[BUFFER_SIZE];
    while (fgets(line, sizeof(line), fp)) {
        /* Remove comments */
        char *comment = strchr(line, '#');
        if (comment) *comment = '\0';
        
        /* Remove trailing whitespace */
        int len = strlen(line);
        while (len > 0 && isspace(line[len-1])) {
            line[--len] = '\0';
        }
        
        /* Skip empty lines */
        if (len == 0) continue;
        
        /* Parse key = value */
        char *key = line;
        char *equals = strchr(line, '=');
        if (!equals) continue;
        
        *equals = '\0';
        char *value = equals + 1;
        
        /* Trim whitespace */
        while (isspace(*key)) key++;
        while (isspace(*value)) value++;
        while (len > 0 && isspace(key[len-1])) key[--len] = '\0';
        
        if (strcmp(key, "database_path") == 0) {
            strncpy(config->database_path, value, sizeof(config->database_path) - 1);
        } else if (strcmp(key, "csv_directory") == 0) {
            strncpy(config->csv_directory, value, sizeof(config->csv_directory) - 1);
        } else if (strcmp(key, "log_file") == 0) {
            strncpy(config->log_file, value, sizeof(config->log_file) - 1);
        } else if (strcmp(key, "poll_interval_seconds") == 0) {
            config->poll_interval_seconds = atoi(value);
        } else if (strcmp(key, "log_level") == 0) {
            if (strcmp(value, "debug") == 0) config->log_level = LOG_DEBUG;
            else if (strcmp(value, "info") == 0) config->log_level = LOG_INFO;
            else if (strcmp(value, "warn") == 0) config->log_level = LOG_WARN;
            else if (strcmp(value, "error") == 0) config->log_level = LOG_ERROR;
        } else if (strcmp(key, "daemon_mode") == 0) {
            config->daemon_mode = (strcmp(value, "true") == 0 || strcmp(value, "1") == 0);
        }
    }
    
    fclose(fp);
    
    /* Set defaults */
    if (config->database_path[0] == '\0') {
        strcpy(config->database_path, "weather.db");
    }
    if (config->csv_directory[0] == '\0') {
        strcpy(config->csv_directory, "./data");
    }
    if (config->poll_interval_seconds == 0) {
        config->poll_interval_seconds = 60;
    }
    
    log_message(LOG_INFO, "Configuration loaded from %s", filename);
    return 0;
}

/* Initialize SQLite database */
static int init_database(const char *db_path) {
    int rc = sqlite3_open(db_path, &g_state.db);
    if (rc != SQLITE_OK) {
        log_message(LOG_ERROR, "Cannot open database: %s", sqlite3_errmsg(g_state.db));
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
        log_message(LOG_ERROR, "SQL error: %s", err_msg);
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
            log_message(LOG_ERROR, "SQL error creating index: %s", err_msg);
            sqlite3_free(err_msg);
        }
    }
    
    log_message(LOG_INFO, "Database initialized: %s", db_path);
    return 0;
}

/* Ingest a single CSV file */
static int ingest_csv_file(const char *filepath) {
    FILE *fp = fopen(filepath, "r");
    if (!fp) {
        log_message(LOG_ERROR, "Cannot open CSV file: %s", filepath);
        return -1;
    }
    
    log_message(LOG_INFO, "Ingesting file: %s", filepath);
    
    /* Prepare insert statement */
    const char *insert_sql = 
        "INSERT INTO weather_data (station_id, date, element, value, mflag, qflag, sflag) "
        "VALUES (?, ?, ?, ?, ?, ?, ?);";
    
    sqlite3_stmt *stmt;
    int rc = sqlite3_prepare_v2(g_state.db, insert_sql, -1, &stmt, NULL);
    if (rc != SQLITE_OK) {
        log_message(LOG_ERROR, "Failed to prepare statement: %s", sqlite3_errmsg(g_state.db));
        fclose(fp);
        return -1;
    }
    
    /* Begin transaction */
    sqlite3_exec(g_state.db, "BEGIN TRANSACTION;", NULL, NULL, NULL);
    
    char line[BUFFER_SIZE];
    int line_count = 0;
    int inserted_count = 0;
    int error_count = 0;
    
    /* Skip header if present */
    fgets(line, sizeof(line), fp);
    
    while (fgets(line, sizeof(line), fp)) {
        line_count++;
        
        /* Parse CSV line */
        /* NOAA GHCN-Daily format: station_id,date,element,value,mflag,qflag,sflag */
        char station_id[32] = {0};
        char date[16] = {0};
        char element[8] = {0};
        double value = 0;
        char mflag[8] = {0};
        char qflag[8] = {0};
        char sflag[8] = {0};
        
        int parsed = sscanf(line, "%11[^,],%8[^,],%5[^,],%lf,%1[^,],%1[^,],%1s",
                           station_id, date, element, &value, mflag, qflag, sflag);
        
        if (parsed < 4) {
            log_message(LOG_WARN, "Skipping malformed line %d in %s", line_count, filepath);
            error_count++;
            continue;
        }
        
        /* Bind parameters */
        sqlite3_bind_text(stmt, 1, station_id, -1, SQLITE_STATIC);
        sqlite3_bind_text(stmt, 2, date, -1, SQLITE_STATIC);
        sqlite3_bind_text(stmt, 3, element, -1, SQLITE_STATIC);
        sqlite3_bind_double(stmt, 4, value);
        sqlite3_bind_text(stmt, 5, mflag[0] ? mflag : NULL, -1, SQLITE_STATIC);
        sqlite3_bind_text(stmt, 6, qflag[0] ? qflag : NULL, -1, SQLITE_STATIC);
        sqlite3_bind_text(stmt, 7, sflag[0] ? sflag : NULL, -1, SQLITE_STATIC);
        
        /* Execute */
        rc = sqlite3_step(stmt);
        if (rc != SQLITE_DONE) {
            log_message(LOG_WARN, "Failed to insert line %d: %s", line_count, sqlite3_errmsg(g_state.db));
            error_count++;
        } else {
            inserted_count++;
        }
        
        /* Reset for next iteration */
        sqlite3_reset(stmt);
        sqlite3_clear_bindings(stmt);
    }
    
    /* Commit transaction */
    sqlite3_exec(g_state.db, "COMMIT;", NULL, NULL, NULL);
    
    sqlite3_finalize(stmt);
    fclose(fp);
    
    log_message(LOG_INFO, "File %s processed: %d lines, %d inserted, %d errors",
                filepath, line_count, inserted_count, error_count);
    
    return (error_count > 0) ? -1 : 0;
}

/* Signal handler */
static void signal_handler(int sig) {
    switch (sig) {
        case SIGTERM:
        case SIGINT:
            log_message(LOG_INFO, "Received signal %d, shutting down gracefully...", sig);
            g_state.running = 0;
            break;
        case SIGHUP:
            log_message(LOG_INFO, "Received SIGHUP, reloading configuration...");
            g_state.reload_config = 1;
            break;
    }
}

/* Cleanup function */
static void cleanup(void) {
    log_message(LOG_INFO, "Cleaning up...");
    
    if (g_state.db) {
        sqlite3_close(g_state.db);
        g_state.db = NULL;
    }
    
    if (g_state.log_fp && g_state.log_fp != stdout) {
        fclose(g_state.log_fp);
        g_state.log_fp = NULL;
    }
    
    remove_pid_file(DEFAULT_PID_FILE);
}

/* Write PID file */
static int write_pid_file(const char *pid_file) {
    FILE *fp = fopen(pid_file, "w");
    if (!fp) {
        log_message(LOG_ERROR, "Cannot write PID file: %s", pid_file);
        return -1;
    }
    
    fprintf(fp, "%d\n", getpid());
    fclose(fp);
    
    return 0;
}

/* Remove PID file */
static void remove_pid_file(const char *pid_file) {
    unlink(pid_file);
}

/* Main function */
int main(int argc, char *argv[]) {
    const char *config_file = DEFAULT_CONFIG_FILE;
    int validate_only = 0;
    
    /* Parse command line arguments */
    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "--config") == 0 && i + 1 < argc) {
            config_file = argv[++i];
        } else if (strcmp(argv[i], "--daemon") == 0) {
            g_state.config.daemon_mode = 1;
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
    
    /* Parse configuration */
    if (parse_config(config_file, &g_state.config) != 0) {
        fprintf(stderr, "Failed to parse configuration\n");
        return 1;
    }
    
    /* Validate only mode */
    if (validate_only) {
        printf("Configuration validated successfully\n");
        printf("  Database: %s\n", g_state.config.database_path);
        printf("  CSV Directory: %s\n", g_state.config.csv_directory);
        printf("  Poll Interval: %d seconds\n", g_state.config.poll_interval_seconds);
        return 0;
    }
    
    /* Open log file if specified */
    if (g_state.config.log_file[0] != '\0') {
        g_state.log_fp = fopen(g_state.config.log_file, "a");
        if (!g_state.log_fp) {
            fprintf(stderr, "Cannot open log file: %s\n", g_state.config.log_file);
            g_state.log_fp = stdout;
        }
    }
    
    /* Daemon mode */
    if (g_state.config.daemon_mode) {
        pid_t pid = fork();
        if (pid < 0) {
            log_message(LOG_ERROR, "Fork failed");
            return 1;
        }
        if (pid > 0) {
            /* Parent process */
            return 0;
        }
        
        /* Child process */
        setsid();
        chdir("/");
        
        /* Redirect standard files to /dev/null */
        freopen("/dev/null", "r", stdin);
        freopen("/dev/null", "w", stdout);
        freopen("/dev/null", "w", stderr);
    }
    
    /* Write PID file */
    if (write_pid_file(DEFAULT_PID_FILE) != 0) {
        return 1;
    }
    
    /* Setup signal handlers */
    signal(SIGTERM, signal_handler);
    signal(SIGINT, signal_handler);
    signal(SIGHUP, signal_handler);
    
    /* Initialize database */
    if (init_database(g_state.config.database_path) != 0) {
        cleanup();
        return 1;
    }
    
    log_message(LOG_INFO, "Ingestion Service v%s started", VERSION);
    log_message(LOG_INFO, "Watching directory: %s", g_state.config.csv_directory);
    log_message(LOG_INFO, "Database: %s", g_state.config.database_path);
    
    /* Main loop */
    g_state.running = 1;
    time_t last_check = 0;
    
    while (g_state.running) {
        /* Check for config reload */
        if (g_state.reload_config) {
            g_state.reload_config = 0;
            log_message(LOG_INFO, "Reloading configuration...");
            parse_config(config_file, &g_state.config);
        }
        
        /* Check for CSV files */
        time_t now = time(NULL);
        if (now - last_check >= g_state.config.poll_interval_seconds) {
            last_check = now;
            
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
                        
                        /* Process file */
                        ingest_csv_file(filepath);
                        
                        /* Optionally move processed file */
                        /* char processed_path[512]; */
                        /* snprintf(processed_path, sizeof(processed_path), "%s/processed/%s", */
                        /*         g_state.config.csv_directory, entry->d_name); */
                        /* rename(filepath, processed_path); */
                    }
                }
                closedir(dir);
            } else {
                log_message(LOG_ERROR, "Cannot open CSV directory: %s", g_state.config.csv_directory);
            }
        }
        
        /* Sleep for a short period */
        sleep(1);
    }
    
    /* Cleanup */
    cleanup();
    
    log_message(LOG_INFO, "Ingestion Service stopped");
    
    return 0;
}
