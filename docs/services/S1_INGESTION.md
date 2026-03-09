# s1: ingestion service

## overview

the ingestion service (s1) is responsible for high-performance csv file processing, streaming multi-gigabyte weather data files into sqlite with minimal memory footprint.

## responsibilities

- monitor csv directory for new files (inotify)
- parse csv files using streaming or mmap approach
- validate and transform data according to configuration
- insert records into sqlite database
- report progress and status via posix mq
- handle partial failures and retry logic

## architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    ingestion service                         │
│                                                              │
│  ┌──────────────┐                                           │
│  │   inotify    │◄───────────┐                              │
│  │   watch      │            │                              │
│  └──────┬───────┘            │ csv files                    │
│         │                    │                              │
│         ▼                    │                              │
│  ┌──────────────┐    ┌───────┴───────┐                     │
│  │   file       │    │   directory   │                     │
│  │   handler    │    │   monitor     │                     │
│  └──────┬───────┘    └───────────────┘                     │
│         │                                                  │
│         ▼                                                  │
│  ┌──────────────┐    ┌──────────────┐                     │
│  │   parser     │───►│  libws_csv   │                     │
│  │   router     │    │  (provided)  │                     │
│  └──────┬───────┘    └──────────────┘                     │
│         │                                                  │
│         ▼                                                  │
│  ┌──────────────┐    ┌──────────────┐                     │
│  │  processing  │    │   strategy   │                     │
│  │   strategy   │───►│   selector   │                     │
│  └──────┬───────┘    └──────────────┘                     │
│         │                                                  │
│    ┌────┴────┐                                             │
│    ▼         ▼                                             │
│ ┌──────┐  ┌──────┐                                         │
│ │ mmap │  │stream│                                         │
│ └────┬─┘  └───┬──┘                                         │
│      │        │                                            │
│      └────┬───┘                                            │
│           ▼                                                │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐ │
│  │  validation  │───►│  transform   │───►│   sqlite     │ │
│  └──────────────┘    └──────────────┘    │  (wal mode)  │ │
│                                          └──────────────┘ │
│                                                 │          │
│  ┌──────────────┐                              │          │
│  │  posix mq    │◄─────────────────────────────┘          │
│  │  (status)    │  progress & completion status            │
│  └──────────────┘                                         │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## configuration

### configuration file (s1_ingestion.ini)

```ini
[service]
name = ws-ingestion
log_level = info
log_destination = syslog

[paths]
csv_directory = /data/csv
processed_directory = /data/processed
error_directory = /data/error
database_path = /var/lib/ws/weather.db

[ingestion]
; strategy: mmap for files < 1gb, streaming for larger
mmap_threshold_bytes = 1073741824
batch_size = 10000
max_retries = 3
retry_delay_seconds = 5

[csv]
; column mapping: csv_column_index = database_column
timestamp_column = 0
timestamp_format = "%y-%m-%d %h:%m:%s"
delimiter = ","
has_header = true

; field mappings (column index)
temperature = 1
humidity = 2
pressure = 3
wind_speed = 4
wind_direction = 5
precipitation = 6

[validation]
; range validation
min_temperature = -100.0
max_temperature = 100.0
min_humidity = 0.0
max_humidity = 100.0

[database]
wal_mode = true
cache_size_mb = 512
mmap_size_mb = 256
synchronous = normal
```

### environment variables

```bash
ws_config=/etc/weather-station/s1_ingestion.ini
ws_csv_directory=/data/csv
ws_db_path=/var/lib/ws/weather.db
ws_log_level=debug
ws_station_id=1
```

### command-line arguments

```bash
ws-ingest [options]

options:
  -c, --config path        configuration file path
  -d, --csv-dir dir        csv watch directory
  -b, --db-path path       database file path
  --daemon                 run as daemon
  --log-level level        log level (debug|info|warn|error)
  --log-dest dest          log destination (syslog|stderr|path)
  -h, --help               show help message
```

## api / interface

### signal interface

| signal | action |
|--------|--------|
| **sigterm/sigint** | graceful shutdown: finish current file, exit |
| **sighup** | reload configuration: re-read ini file, update settings |
| **sigusr1** | status dump: log current processing state |
| **sigusr2** | toggle log level between info and debug |

### posix message queue interface

**queue name**: `/ws_ingest_status`

**message format**:

```c
enum ingest_status_type {
        ingest_status_started = 1,
        ingest_status_progress = 2,
        ingest_status_completed = 3,
        ingest_status_error = 4,
};

struct ingest_mq_message {
        uint32_t msg_type;              /* enum ingest_status_type */
        uint32_t sequence_id;           /* monotonic counter */
        char filename[256];             /* csv filename */
        uint64_t records_processed;     /* total records */
        uint64_t records_failed;        /* failed records */
        double progress_percent;        /* 0.0 - 100.0 */
        char error_message[512];        /* if status == error */
};
```

### database interface

**writes to**:
- `weather_data` table: raw weather records
- `ingest_log` table: ingestion audit trail

**reads from**:
- `ingest_log` table: resume interrupted ingestions

## implementation details

### file processing strategy

```c
/* select optimal processing strategy based on file size */
enum processing_strategy {
        strategy_mmap,
        strategy_streaming,
};

enum processing_strategy select_strategy(const char *filepath)
{
        struct stat st;
        
        if (stat(filepath, &st) != 0)
                return strategy_streaming;  /* fallback */
        
        /* use mmap for files under threshold */
        if (st.st_size < config.mmap_threshold)
                return strategy_mmap;
        
        /* streaming for large files */
        return strategy_streaming;
}
```

### mmap implementation

```c
int ingest_mmap(const char *filepath, struct ws_error_info *err)
{
        int fd = open(filepath, o_rdonly | o_cloexec);
        if (fd < 0) {
                ws_error_set(err, ws_error_io, "open failed: %m");
                return -1;
        }
        
        struct stat st;
        if (fstat(fd, &st) < 0) {
                close(fd);
                ws_error_set(err, ws_error_io, "fstat failed: %m");
                return -1;
        }
        
        /* memory map entire file */
        char *data = mmap(null, st.st_size, prot_read, map_private, fd, 0);
        if (data == map_failed) {
                close(fd);
                ws_error_set(err, ws_error_io, "mmap failed: %m");
                return -1;
        }
        
        /* advise sequential access pattern */
        madvise(data, st.st_size, madv_sequential);
        
        /* parse using library */
        csv_parser_t *parser = csv_parser_create(config.delimiter);
        csv_parser_set_buffer(parser, data, st.st_size);
        
        /* process with database transaction */
        sqlite3_exec(db, "begin transaction", null, null, null);
        
        csv_record_t record;
        while (csv_parser_next(parser, &record) == 0) {
                if (validate_record(&record, err) != 0) {
                        log_invalid_record(filepath, csv_line_number(parser), err);
                        continue;
                }
                
                insert_weather_record(db, &record, config.column_map);
        }
        
        sqlite3_exec(db, "commit", null, null, null);
        
        /* cleanup */
        csv_parser_destroy(parser);
        munmap(data, st.st_size);
        close(fd);
        
        return 0;
}
```

### streaming implementation

```c
int ingest_streaming(const char *filepath, struct ws_error_info *err)
{
        file *fp = fopen(filepath, "r");
        if (!fp) {
                ws_error_set(err, ws_error_io, "fopen failed: %m");
                return -1;
        }
        
        csv_parser_t *parser = csv_parser_create(config.delimiter);
        csv_parser_set_file(parser, fp);
        
        /* process in batches to commit periodically */
        int batch_count = 0;
        csv_record_t record;
        
        sqlite3_exec(db, "begin transaction", null, null, null);
        
        while (csv_parser_next(parser, &record) == 0) {
                if (validate_record(&record, err) != 0) {
                        log_invalid_record(filepath, csv_line_number(parser), err);
                        continue;
                }
                
                insert_weather_record(db, &record, config.column_map);
                
                /* commit every n records */
                if (++batch_count >= config.batch_size) {
                        sqlite3_exec(db, "commit", null, null, null);
                        sqlite3_exec(db, "begin transaction", null, null, null);
                        batch_count = 0;
                        
                        /* report progress */
                        send_progress_update(filepath, parser);
                }
        }
        
        /* final commit */
        sqlite3_exec(db, "commit", null, null, null);
        
        csv_parser_destroy(parser);
        fclose(fp);
        
        return 0;
}
```

### inotify watch implementation

```c
int setup_inotify_watch(const char *directory)
{
        int inotify_fd = inotify_init1(in_nonblock | in_cloexec);
        if (inotify_fd < 0) {
                ws_log_error("inotify_init failed: %m");
                return -1;
        }
        
        /* watch for new files and close writes */
        int watch_fd = inotify_add_watch(inotify_fd, directory,
                in_create | in_close_write | in_moved_to);
        
        if (watch_fd < 0) {
                ws_log_error("inotify_add_watch failed: %m");
                close(inotify_fd);
                return -1;
        }
        
        return inotify_fd;
}

void handle_inotify_events(int inotify_fd)
{
        char buffer[4096];
        ssize_t len;
        
        while ((len = read(inotify_fd, buffer, sizeof(buffer))) > 0) {
                for (char *ptr = buffer; ptr < buffer + len; ) {
                        struct inotify_event *event = (struct inotify_event *)ptr;
                        
                        if (event->mask & (in_close_write | in_moved_to)) {
                                /* file completed writing */
                                if (is_csv_file(event->name)) {
                                        queue_file_for_processing(event->name);
                                }
                        }
                        
                        ptr += sizeof(struct inotify_event) + event->len;
                }
        }
}
```

## data validation

### validation rules

```c
struct validation_rule {
        const char *column_name;
        double min_value;
        double max_value;
        int required;
};

struct validation_rule validation_rules[] = {
        {"temperature", -100.0, 100.0, 1},
        {"humidity", 0.0, 100.0, 1},
        {"pressure", 800.0, 1100.0, 0},
        {"wind_speed", 0.0, 500.0, 0},
        {null, 0, 0, 0}
};

int validate_record(csv_record_t *record, struct ws_error_info *err)
{
        for (int i = 0; validation_rules[i].column_name != null; i++) {
                struct validation_rule *rule = &validation_rules[i];
                double value = get_column_value(record, rule->column_name);
                
                if (rule->required && isnan(value)) {
                        ws_error_set(err, ws_error_invalid_arg,
                                "required field %s is missing",
                                rule->column_name);
                        return -1;
                }
                
                if (value < rule->min_value || value > rule->max_value) {
                        ws_error_set(err, ws_error_invalid_arg,
                                "%s value %.2f out of range [%.2f, %.2f]",
                                rule->column_name, value,
                                rule->min_value, rule->max_value);
                        return -1;
                }
        }
        
        return 0;
}
```

## error handling & recovery

### retry logic

```c
int ingest_with_retry(const char *filepath, struct ws_error_info *err)
{
        for (int attempt = 1; attempt <= config.max_retries; attempt++) {
                if (attempt > 1) {
                        ws_log_warn("retry attempt %d/%d for %s",
                                attempt, config.max_retries, filepath);
                        sleep(config.retry_delay_seconds);
                }
                
                if (ingest_file(filepath, err) == 0)
                        return 0;
                
                /* don't retry on permanent errors */
                if (err->code == ws_error_invalid_arg)
                        return -1;
        }
        
        ws_error_set(err, ws_error_io,
                "failed after %d attempts", config.max_retries);
        return -1;
}
```

### partial failure handling

```c
void handle_partial_failure(const char *filepath, int records_processed,
        int records_failed)
{
        if (records_failed > 0) {
                ws_log_warn("partial failure: %d/%d records failed for %s",
                        records_failed, records_processed + records_failed,
                        filepath);
                
                /* move to error directory for inspection */
                move_to_error_directory(filepath);
                
                /* log details for debugging */
                log_failed_records(filepath);
        }
}
```

## performance characteristics

| metric | target | notes |
|--------|--------|-------|
| throughput | >100 mb/s | depends on storage and cpu |
| latency (first record) | <5s | time to first database commit |
| memory | <2gb | regardless of file size |
| cpu usage | 1-2 cores | can saturate single core |
| i/o pattern | sequential | optimized for hdd/ssd |

## monitoring metrics

### prometheus metrics

```
# help ws_ingest_files_total total files processed
# type ws_ingest_files_total counter
ws_ingest_files_total{status="success"} 1543
ws_ingest_files_total{status="error"} 12

# help ws_ingest_records_total total records ingested
# type ws_ingest_records_total counter
ws_ingest_records_total 1543200000

# help ws_ingest_duration_seconds file ingestion duration
# type ws_ingest_duration_seconds histogram
ws_ingest_duration_seconds_bucket{le="10"} 523
ws_ingest_duration_seconds_bucket{le="30"} 1024
ws_ingest_duration_seconds_bucket{le="60"} 1543

# help ws_ingest_bytes_total total bytes processed
# type ws_ingest_bytes_total counter
ws_ingest_bytes_total 1073741824000

# help ws_ingest_active_files currently processing files
# type ws_ingest_active_files gauge
ws_ingest_active_files 2
```

## troubleshooting

### common issues

| symptom | cause | solution |
|---------|-------|----------|
| slow ingestion | database not in wal mode | check `pragma journal_mode` |
| memory usage high | too many files queued | reduce batch size, add backpressure |
| validation errors | csv format mismatch | check column mapping configuration |
| sqlite busy errors | concurrent writers | enable wal mode, reduce batch commits |
| inotify not working | max watches exceeded | `echo 524288 > /proc/sys/fs/inotify/max_user_watches` |

### debug commands

```bash
# check service status
systemctl status ws-ingestion

# view logs
journalctl -u ws-ingestion -f

# monitor database
sqlite3 /var/lib/ws/weather.db "select count(*) from weather_data;"
sqlite3 /var/lib/ws/weather.db "select * from ingest_log order by started_at desc limit 5;"

# check inotify watches
find /proc/*/fd -lname anon_inode:inotify |
    cut -d/ -f3 |
    xargs -i '{}' -- ps --no-headers -o '%p %u %c' -p '{}' |
    uniq -c |
    sort -nr
```

---

*next: [s2: aggregation service](s2_aggregation.md)*
