# service specifications

complete specifications for all microservices in the weather station system.

---

## table of contents

1. [s1: ingestion service](s1_ingestion.md)
2. [s2: aggregation service](s2_aggregation.md)
3. [s3: query service](s3_query.md)
4. [s4: discovery service](s4_discovery.md)
5. [c1: cli client](c1_cli.md)

---

## service overview

| service | type | protocols | persistence | scaling |
|---------|------|-----------|-------------|---------|
| **s1** | daemon | files, posix mq | sqlite | vertical |
| **s2** | daemon | posix mq | sqlite | vertical (workers) |
| **s3** | daemon | tcp, http | none (stateless) | horizontal |
| **s4** | daemon | udp, tcp | sqlite | single instance |
| **c1** | cli tool | tcp | none | n/a |

## service interactions

```
                    ┌─────────────────────────────────────┐
                    │           weather station           │
                    │                                     │
┌──────────┐        │  ┌─────────┐    ┌─────────┐        │
│ csv      │───────►│  │   s1    │───►│  sqlite │        │
│ files    │        │  │ingestion│    │   db    │        │
└──────────┘        │  └────┬────┘    └────┬────┘        │
                    │       │              │              │
                    │       │ posix mq     │              │
                    │       ▼              │              │
                    │  ┌─────────┐         │              │
                    │  │   s2    │         │              │
                    │  │aggregate│         │              │
                    │  └────┬────┘         │              │
                    │       │              │              │
                    └───────┼──────────────┼──────────────┘
                            │              │
              ┌─────────────┘              └─────────────┐
              ▼                                          ▼
        ┌──────────┐                              ┌──────────┐
        │   s4     │                              │   s3     │
        │discovery │◄────────────────────────────►│  query   │
        │(udp/tcp) │     peer discovery/fanout    │(tcp/http)│
        └──────────┘                              └────┬─────┘
                                                       │
                                                       ▼
                                                ┌──────────┐
                                                │   c1     │
                                                │   cli    │
                                                └──────────┘
```

## common service characteristics

### process management

all services follow these patterns:

```c
/* standard service main() template */
int main(int argc, char *argv[])
{
        struct service_config cfg;
        
        /* 1. parse command line arguments */
        parse_args(argc, argv, &cfg);
        
        /* 2. setup logging */
        setup_logging(cfg.log_level, cfg.log_dest);
        
        /* 3. load configuration */
        load_config(cfg.config_path, &cfg);
        
        /* 4. daemonize if requested */
        if (cfg.daemon_mode)
                daemonize(cfg.service_name);
        
        /* 5. setup signal handlers */
        setup_signal_handlers();
        
        /* 6. initialize service-specific resources */
        service_init(&cfg);
        
        /* 7. main event loop */
        service_run();
        
        /* 8. cleanup */
        service_cleanup();
        
        return 0;
}
```

### signal handling

all services handle these signals:

| signal | action |
|--------|--------|
| **sigterm** | graceful shutdown: stop accepting work, complete current operations, exit |
| **sigint** | same as sigterm (for interactive use) |
| **sighup** | configuration reload: re-read config without restarting |
| **sigusr1** | debug dump: print internal state to log |
| **sigusr2** | toggle log level: switch between info and debug |

### configuration

all services support three configuration methods (in priority order):

1. **environment variables**: `ws_db_path=/data/weather.db`
2. **configuration file**: `/etc/weather-station/s1_ingestion.ini`
3. **command-line flags**: `--db-path=/data/weather.db`

### error handling

structured error pattern:

```c
/* error codes */
enum ws_error {
        ws_ok = 0,
        ws_error_invalid_arg = -1,
        ws_error_io = -2,
        ws_error_db = -3,
        ws_error_network = -4,
        ws_error_protocol = -5,
        ws_error_auth = -6,
        ws_error_not_found = -7,
        ws_error_timeout = -8,
};

/* error object */
struct ws_error_info {
        int code;
        const char *function;
        const char *file;
        int line;
        char message[256];
};

/* usage pattern */
int ingest_file(const char *path, struct ws_error_info *err)
{
        if (!path) {
                err->code = ws_error_invalid_arg;
                snprintf(err->message, sizeof(err->message),
                        "path cannot be null");
                return -1;
        }
        
        int fd = open(path, o_rdonly);
        if (fd < 0) {
                err->code = ws_error_io;
                snprintf(err->message, sizeof(err->message),
                        "failed to open %s: %m", path);
                return -1;
        }
        
        return 0;
}
```

### logging

all services use syslog with structured format:

```
jan 15 14:32:45 station1 ws-ingestion[1234]: info: starting ingestion of file: data_20240115.csv
jan 15 14:32:45 station1 ws-ingestion[1234]: debug: using mmap for file size: 2147483648 bytes
jan 15 14:33:12 station1 ws-ingestion[1234]: info: completed ingestion: 1543200 records in 27.3s (56520 rec/s)
```

### health reporting

all services report health status:

```c
/* health check response */
struct ws_health_status {
        int healthy;                    /* 1 = healthy, 0 = degraded */
        char status_message[256];       /* human-readable status */
        int64_t uptime_seconds;         /* service uptime */
        int64_t requests_total;         /* total requests processed */
        int64_t errors_total;           /* total errors encountered */
        double cpu_percent;             /* current cpu usage */
        double memory_mb;               /* current memory usage */
};
```

## service lifecycle

### startup sequence

```
1. parse cli args
2. initialize logging
3. load configuration
4. daemonize (if requested)
5. setup signal handlers
6. initialize database connections
7. create network sockets (if applicable)
8. spawn worker processes/threads (if applicable)
9. enter main event loop
```

### shutdown sequence

```
1. set shutdown flag
2. stop accepting new work/connections
3. wait for active operations to complete (with timeout)
4. terminate worker processes/threads
5. close network sockets
6. close database connections
7. flush logs
8. exit
```

---

*next: [s1: ingestion service](s1_ingestion.md)*
