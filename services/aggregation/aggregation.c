#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <signal.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <time.h>
#include <errno.h>
#include <sqlite3.h>
#include <ctype.h>

#define VERSION "1.0.0"
#define DEFAULT_CONFIG_FILE "aggregation.ini"
#define DEFAULT_PID_FILE "/tmp/aggregation.pid"
#define BUFFER_SIZE 4096

/* Configuration structure */
typedef struct {
    char input_database[256];
    char output_database[256];
    char log_file[256];
    int aggregation_interval_seconds;
    int log_level;
    int daemon_mode;
} Config;

/* Global state */
typedef struct {
    Config config;
    sqlite3 *input_db;
    sqlite3 *output_db;
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
static int init_output_database(const char *db_path);
static int perform_aggregation(void);
static int aggregate_daily(void);
static int aggregate_hourly(void);
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
        
        if (strcmp(key, "input_database") == 0) {
            strncpy(config->input_database, value, sizeof(config->input_database) - 1);
        } else if (strcmp(key, "output_database") == 0) {
            strncpy(config->output_database, value, sizeof(config->output_database) - 1);
        } else if (strcmp(key, "log_file") == 0) {
            strncpy(config->log_file, value, sizeof(config->log_file) - 1);
        } else if (strcmp(key, "aggregation_interval_seconds") == 0) {
            config->aggregation_interval_seconds = atoi(value);
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
    if (config->input_database[0] == '\0') {
        strcpy(config->input_database, "weather.db");
    }
    if (config->output_database[0] == '\0') {
        strcpy(config->output_database, "aggregated.db");
    }
    if (config->aggregation_interval_seconds == 0) {
        config->aggregation_interval_seconds = 300; /* 5 minutes */
    }
    
    log_message(LOG_INFO, "Configuration loaded from %s", filename);
    return 0;
}

/* Initialize output SQLite database */
static int init_output_database(const char *db_path) {
    int rc = sqlite3_open(db_path, &g_state.output_db);
    if (rc != SQLITE_OK) {
        log_message(LOG_ERROR, "Cannot open output database: %s", sqlite3_errmsg(g_state.output_db));
        return -1;
    }
    
    /* Create daily aggregates table */
    const char *create_daily_sql = 
        "CREATE TABLE IF NOT EXISTS daily_aggregates ("
        "id INTEGER PRIMARY KEY AUTOINCREMENT,"
        "station_id TEXT NOT NULL,"
        "date TEXT NOT NULL,"
        "metric TEXT NOT NULL,"
        "avg_value REAL,"
        "min_value REAL,"
        "max_value REAL,"
        "count INTEGER,"
        "created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,"
        "UNIQUE(station_id, date, metric)"
        ");";
    
    char *err_msg = NULL;
    rc = sqlite3_exec(g_state.output_db, create_daily_sql, NULL, NULL, &err_msg);
    if (rc != SQLITE_OK) {
        log_message(LOG_ERROR, "SQL error creating daily_aggregates: %s", err_msg);
        sqlite3_free(err_msg);
        return -1;
    }
    
    /* Create hourly aggregates table */
    const char *create_hourly_sql = 
        "CREATE TABLE IF NOT EXISTS hourly_aggregates ("
        "id INTEGER PRIMARY KEY AUTOINCREMENT,"
        "station_id TEXT NOT NULL,"
        "hour TEXT NOT NULL,"
        "metric TEXT NOT NULL,"
        "avg_value REAL,"
        "min_value REAL,"
        "max_value REAL,"
        "count INTEGER,"
        "created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,"
        "UNIQUE(station_id, hour, metric)"
        ");";
    
    rc = sqlite3_exec(g_state.output_db, create_hourly_sql, NULL, NULL, &err_msg);
    if (rc != SQLITE_OK) {
        log_message(LOG_ERROR, "SQL error creating hourly_aggregates: %s", err_msg);
        sqlite3_free(err_msg);
        return -1;
    }
    
    /* Create indexes */
    const char *create_indexes[] = {
        "CREATE INDEX IF NOT EXISTS idx_daily_station ON daily_aggregates(station_id);",
        "CREATE INDEX IF NOT EXISTS idx_daily_date ON daily_aggregates(date);",
        "CREATE INDEX IF NOT EXISTS idx_hourly_station ON hourly_aggregates(station_id);",
        "CREATE INDEX IF NOT EXISTS idx_hourly_hour ON hourly_aggregates(hour);",
        NULL
    };
    
    for (int i = 0; create_indexes[i] != NULL; i++) {
        rc = sqlite3_exec(g_state.output_db, create_indexes[i], NULL, NULL, &err_msg);
        if (rc != SQLITE_OK) {
            log_message(LOG_WARN, "SQL error creating index: %s", err_msg);
            sqlite3_free(err_msg);
        }
    }
    
    log_message(LOG_INFO, "Output database initialized: %s", db_path);
    return 0;
}

/* Open input database */
static int open_input_database(const char *db_path) {
    int rc = sqlite3_open(db_path, &g_state.input_db);
    if (rc != SQLITE_OK) {
        log_message(LOG_ERROR, "Cannot open input database: %s", sqlite3_errmsg(g_state.input_db));
        return -1;
    }
    
    log_message(LOG_DEBUG, "Input database opened: %s", db_path);
    return 0;
}

/* Aggregate daily data */
static int aggregate_daily(void) {
    log_message(LOG_INFO, "Performing daily aggregation...");
    
    /* Query to aggregate by day and station */
    const char *query = 
        "SELECT station_id, substr(date, 1, 8) as day, element,"
        "       AVG(value) as avg_val, MIN(value) as min_val,"
        "       MAX(value) as max_val, COUNT(*) as cnt"
        " FROM weather_data"
        " GROUP BY station_id, day, element"
        " ORDER BY day DESC"
        " LIMIT 1000;";
    
    sqlite3_stmt *stmt;
    int rc = sqlite3_prepare_v2(g_state.input_db, query, -1, &stmt, NULL);
    if (rc != SQLITE_OK) {
        log_message(LOG_ERROR, "Failed to prepare daily aggregation query: %s", sqlite3_errmsg(g_state.input_db));
        return -1;
    }
    
    /* Prepare insert statement for output */
    const char *insert_sql = 
        "INSERT OR REPLACE INTO daily_aggregates"
        " (station_id, date, metric, avg_value, min_value, max_value, count)"
        " VALUES (?, ?, ?, ?, ?, ?, ?);";
    
    sqlite3_stmt *insert_stmt;
    rc = sqlite3_prepare_v2(g_state.output_db, insert_sql, -1, &insert_stmt, NULL);
    if (rc != SQLITE_OK) {
        log_message(LOG_ERROR, "Failed to prepare insert statement: %s", sqlite3_errmsg(g_state.output_db));
        sqlite3_finalize(stmt);
        return -1;
    }
    
    /* Begin transaction */
    sqlite3_exec(g_state.output_db, "BEGIN TRANSACTION;", NULL, NULL, NULL);
    
    int rows_processed = 0;
    
    while ((rc = sqlite3_step(stmt)) == SQLITE_ROW) {
        const char *station_id = (const char *)sqlite3_column_text(stmt, 0);
        const char *day = (const char *)sqlite3_column_text(stmt, 1);
        const char *element = (const char *)sqlite3_column_text(stmt, 2);
        double avg_val = sqlite3_column_double(stmt, 3);
        double min_val = sqlite3_column_double(stmt, 4);
        double max_val = sqlite3_column_double(stmt, 5);
        int count = sqlite3_column_int(stmt, 6);
        
        /* Bind parameters */
        sqlite3_bind_text(insert_stmt, 1, station_id, -1, SQLITE_STATIC);
        sqlite3_bind_text(insert_stmt, 2, day, -1, SQLITE_STATIC);
        sqlite3_bind_text(insert_stmt, 3, element, -1, SQLITE_STATIC);
        sqlite3_bind_double(insert_stmt, 4, avg_val);
        sqlite3_bind_double(insert_stmt, 5, min_val);
        sqlite3_bind_double(insert_stmt, 6, max_val);
        sqlite3_bind_int(insert_stmt, 7, count);
        
        /* Execute insert */
        rc = sqlite3_step(insert_stmt);
        if (rc != SQLITE_DONE) {
            log_message(LOG_WARN, "Failed to insert daily aggregate: %s", sqlite3_errmsg(g_state.output_db));
        }
        
        sqlite3_reset(insert_stmt);
        sqlite3_clear_bindings(insert_stmt);
        
        rows_processed++;
    }
    
    /* Commit transaction */
    sqlite3_exec(g_state.output_db, "COMMIT;", NULL, NULL, NULL);
    
    sqlite3_finalize(stmt);
    sqlite3_finalize(insert_stmt);
    
    log_message(LOG_INFO, "Daily aggregation complete: %d rows processed", rows_processed);
    return 0;
}

/* Aggregate hourly data */
static int aggregate_hourly(void) {
    log_message(LOG_INFO, "Performing hourly aggregation...");
    
    /* Query to aggregate by hour and station */
    /* Note: GHCN-Daily doesn't have hourly data, so we'll create synthetic hours */
    const char *query = 
        "SELECT station_id, substr(date, 1, 10) || '00' as hour, element,"
        "       AVG(value) as avg_val, MIN(value) as min_val,"
        "       MAX(value) as max_val, COUNT(*) as cnt"
        " FROM weather_data"
        " GROUP BY station_id, hour, element"
        " ORDER BY hour DESC"
        " LIMIT 1000;";
    
    sqlite3_stmt *stmt;
    int rc = sqlite3_prepare_v2(g_state.input_db, query, -1, &stmt, NULL);
    if (rc != SQLITE_OK) {
        log_message(LOG_ERROR, "Failed to prepare hourly aggregation query: %s", sqlite3_errmsg(g_state.input_db));
        return -1;
    }
    
    /* Prepare insert statement */
    const char *insert_sql = 
        "INSERT OR REPLACE INTO hourly_aggregates"
        " (station_id, hour, metric, avg_value, min_value, max_value, count)"
        " VALUES (?, ?, ?, ?, ?, ?, ?);";
    
    sqlite3_stmt *insert_stmt;
    rc = sqlite3_prepare_v2(g_state.output_db, insert_sql, -1, &insert_stmt, NULL);
    if (rc != SQLITE_OK) {
        log_message(LOG_ERROR, "Failed to prepare insert statement: %s", sqlite3_errmsg(g_state.output_db));
        sqlite3_finalize(stmt);
        return -1;
    }
    
    sqlite3_exec(g_state.output_db, "BEGIN TRANSACTION;", NULL, NULL, NULL);
    
    int rows_processed = 0;
    
    while ((rc = sqlite3_step(stmt)) == SQLITE_ROW) {
        const char *station_id = (const char *)sqlite3_column_text(stmt, 0);
        const char *hour = (const char *)sqlite3_column_text(stmt, 1);
        const char *element = (const char *)sqlite3_column_text(stmt, 2);
        double avg_val = sqlite3_column_double(stmt, 3);
        double min_val = sqlite3_column_double(stmt, 4);
        double max_val = sqlite3_column_double(stmt, 5);
        int count = sqlite3_column_int(stmt, 6);
        
        sqlite3_bind_text(insert_stmt, 1, station_id, -1, SQLITE_STATIC);
        sqlite3_bind_text(insert_stmt, 2, hour, -1, SQLITE_STATIC);
        sqlite3_bind_text(insert_stmt, 3, element, -1, SQLITE_STATIC);
        sqlite3_bind_double(insert_stmt, 4, avg_val);
        sqlite3_bind_double(insert_stmt, 5, min_val);
        sqlite3_bind_double(insert_stmt, 6, max_val);
        sqlite3_bind_int(insert_stmt, 7, count);
        
        rc = sqlite3_step(insert_stmt);
        if (rc != SQLITE_DONE) {
            log_message(LOG_WARN, "Failed to insert hourly aggregate: %s", sqlite3_errmsg(g_state.output_db));
        }
        
        sqlite3_reset(insert_stmt);
        sqlite3_clear_bindings(insert_stmt);
        
        rows_processed++;
    }
    
    sqlite3_exec(g_state.output_db, "COMMIT;", NULL, NULL, NULL);
    
    sqlite3_finalize(stmt);
    sqlite3_finalize(insert_stmt);
    
    log_message(LOG_INFO, "Hourly aggregation complete: %d rows processed", rows_processed);
    return 0;
}

/* Perform aggregation */
static int perform_aggregation(void) {
    log_message(LOG_INFO, "Starting aggregation cycle...");
    
    /* Open input database if not already open */
    if (!g_state.input_db) {
        if (open_input_database(g_state.config.input_database) != 0) {
            return -1;
        }
    }
    
    /* Perform aggregations */
    aggregate_daily();
    aggregate_hourly();
    
    log_message(LOG_INFO, "Aggregation cycle complete");
    return 0;
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
    
    if (g_state.input_db) {
        sqlite3_close(g_state.input_db);
        g_state.input_db = NULL;
    }
    
    if (g_state.output_db) {
        sqlite3_close(g_state.output_db);
        g_state.output_db = NULL;
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
            printf("Aggregation Service v%s\n", VERSION);
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
        printf("  Input Database: %s\n", g_state.config.input_database);
        printf("  Output Database: %s\n", g_state.config.output_database);
        printf("  Aggregation Interval: %d seconds\n", g_state.config.aggregation_interval_seconds);
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
            return 0;
        }
        
        setsid();
        chdir("/");
        
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
    
    /* Initialize output database */
    if (init_output_database(g_state.config.output_database) != 0) {
        cleanup();
        return 1;
    }
    
    log_message(LOG_INFO, "Aggregation Service v%s started", VERSION);
    log_message(LOG_INFO, "Input Database: %s", g_state.config.input_database);
    log_message(LOG_INFO, "Output Database: %s", g_state.config.output_database);
    
    /* Main loop */
    g_state.running = 1;
    time_t last_aggregation = 0;
    
    /* Perform initial aggregation */
    perform_aggregation();
    last_aggregation = time(NULL);
    
    while (g_state.running) {
        /* Check for config reload */
        if (g_state.reload_config) {
            g_state.reload_config = 0;
            log_message(LOG_INFO, "Reloading configuration...");
            parse_config(config_file, &g_state.config);
        }
        
        /* Check if it's time to aggregate */
        time_t now = time(NULL);
        if (now - last_aggregation >= g_state.config.aggregation_interval_seconds) {
            perform_aggregation();
            last_aggregation = now;
        }
        
        sleep(1);
    }
    
    cleanup();
    
    log_message(LOG_INFO, "Aggregation Service stopped");
    
    return 0;
}
