# configuration reference

## overview

complete reference for all configuration options in the weather station system.

## configuration hierarchy

configuration is loaded in this priority order (highest first):

1. command-line arguments
2. environment variables
3. configuration files
4. built-in defaults

## global configuration

### environment variables

all services support these environment variables:

```bash
# core
ws_config=/path/to/config.ini
ws_log_level=debug|info|warn|error
ws_log_dest=syslog|stderr|/path/to/file
ws_station_id=1

# paths
ws_db_path=/var/lib/ws/weather.db
ws_config_dir=/etc/weather-station

# feature flags
ws_mtls_enabled=true|false
ws_replication_enabled=true|false
ws_ha_enabled=true|false
```

### command-line arguments

common arguments for all services:

```bash
-c, --config path        configuration file path
--log-level level        log level (debug|info|warn|error)
--log-dest dest          log destination
--daemon                 run as daemon
--pid-file path          pid file location
-h, --help               show help message
-v, --version            show version
```

## service-specific configuration

### s1: ingestion service

**file**: `s1_ingestion.ini`

```ini
[service]
name = ws-ingestion                    ; service name for logging
log_level = info                       ; log level
daemon = false                         ; run as daemon
pid_file = /var/run/ws-ingestion.pid   ; pid file path

[paths]
csv_directory = /data/csv              ; watch directory for csv files
processed_directory = /data/processed  ; move processed files here
error_directory = /data/error          ; move invalid files here
database_path = /var/lib/ws/weather.db ; sqlite database path

[ingestion]
; processing strategy selection
mmap_threshold_bytes = 1073741824      ; use mmap for files < 1gb (0 = never)
streaming_buffer_size = 65536          ; buffer size for streaming (bytes)

; batch settings
batch_size = 10000                     ; records per database transaction
max_batches_per_transaction = 10       ; max batches before commit

; retry settings
max_retries = 3                        ; max retry attempts for failed files
retry_delay_seconds = 5                ; delay between retries

; file handling
file_pattern = *.csv                   ; glob pattern for csv files
skip_existing = true                   ; skip files already in ingest_log
move_processed = true                  ; move files after processing

[csv]
; column configuration
timestamp_column = 0                   ; column index for timestamp
timestamp_format = "%y-%m-%d %h:%m:%s" ; strptime format
delimiter = ","                        ; field delimiter
has_header = true                      ; first row is header
encoding = utf-8                       ; file encoding

; field mappings (column indices)
temperature = 1
humidity = 2
pressure = 3
wind_speed = 4
wind_direction = 5
precipitation = 6

; optional fields (set to -1 if not present)
location_lat = -1
location_lon = -1
station_id_column = -1                 ; override station_id from file

[validation]
; range validation
min_temperature = -100.0
max_temperature = 100.0
min_humidity = 0.0
max_humidity = 100.0
min_pressure = 800.0
max_pressure = 1100.0
min_wind_speed = 0.0
max_wind_speed = 500.0

; data quality checks
check_timestamp_monotonic = true       ; timestamps must increase
max_timestamp_future_seconds = 3600    ; max future timestamp
reject_invalid_rows = false            ; skip invalid rows vs reject file

[database]
; sqlite pragmas
wal_mode = true                        ; enable wal mode
cache_size_mb = 512                    ; cache size in mb (negative = kib)
mmap_size_mb = 256                     ; memory map size
synchronous = normal                   ; normal, full, or off
temp_store = memory                    ; memory or file
journal_size_limit_mb = 100            ; max wal size
auto_checkpoint_pages = 1000           ; checkpoint frequency

[inotify]
enabled = true                         ; use inotify for file monitoring
events = create,close_write,moved_to   ; events to watch
max_watches = 65536                    ; max inotify watches
recursive = false                      ; watch subdirectories

[mqueue]
enabled = true
queue_name = /ws_ingest_status
max_messages = 100
message_size = 1024

[metrics]
enabled = true
prometheus_port = 9090
prometheus_path = /metrics
report_interval_seconds = 60
```

### s2: aggregation service

**file**: `s2_aggregation.ini`

```ini
[service]
name = ws-aggregation
log_level = info
daemon = false
pid_file = /var/run/ws-aggregation.pid

[paths]
database_path = /var/lib/ws/weather.db

[workers]
count = 4                              ; number of worker processes
max_jobs_per_worker = 100              ; max queued jobs per worker
job_timeout_seconds = 300              ; max job execution time
restart_on_failure = true              ; restart crashed workers
worker_nice = 10                       ; nice level for workers

[scheduling]
; when to run aggregations
hourly_enabled = true
hourly_schedule = "0 * * * *"          ; cron expression (top of hour)
daily_enabled = true
daily_schedule = "0 0 * * *"           ; daily at midnight
weekly_enabled = false
weekly_schedule = "0 0 * * 0"          ; weekly on sunday

; backfill settings
backfill_enabled = true                ; process missing historical data
backfill_lookback_days = 7             ; how far back to backfill
backfill_batch_hours = 24              ; hours per backfill job

[aggregation]
; metrics to aggregate
metrics = temperature,humidity,pressure,wind_speed

; statistics to compute
compute_min = true
compute_max = true
compute_avg = true
compute_count = true
compute_stddev = false

; batch sizes
records_per_batch = 50000              ; process n records at a time
parallel_batches = 4                   ; parallel batch processing

[database]
wal_mode = true
cache_size_mb = 256
synchronous = normal

[mqueue]
enabled = true
command_queue = /ws_agg_cmd
status_queue = /ws_agg_status

[metrics]
enabled = true
prometheus_port = 9090
```

### s3: query service

**file**: `s3_query.ini`

```ini
[service]
name = ws-query
log_level = info
daemon = false
pid_file = /var/run/ws-query.pid

[network]
; tcp server configuration
bind_address = 0.0.0.0                 ; listen address (0.0.0.0 = all)
bind_port = 8080                       ; tcp port
max_connections = 1000                 ; max concurrent connections
connection_timeout_seconds = 300       ; connection idle timeout
keepalive_enabled = true               ; tcp keepalive
keepalive_interval_seconds = 60        ; keepalive probe interval
tcp_nodelay = true                     ; disable nagle algorithm

; buffer sizes
receive_buffer_size = 65536            ; socket receive buffer
send_buffer_size = 65536               ; socket send buffer

[thread_pool]
size = 8                               ; number of worker threads
queue_depth = 1000                     ; max queued requests
thread_stack_size = 8388608            ; 8mb stack per thread

[rate_limit]
enabled = true
algorithm = token_bucket               ; token_bucket or fixed_window
requests_per_minute = 600              ; rate limit
burst_size = 100                       ; burst allowance
per_client = true                      ; limit per client ip

[database]
path = /var/lib/ws/weather.db
read_only = true                       ; read-only mode for safety
max_query_time_seconds = 30            ; query timeout
max_records_per_query = 100000         ; result set limit

; query caching
cache_enabled = true
cache_ttl_seconds = 60                 ; cache time-to-live
cache_max_entries = 1000               ; max cached queries

[http]
enabled = true
port = 9090                            ; http port (metrics)
bind_address = 0.0.0.0

; endpoints
metrics_path = /metrics                ; prometheus metrics
health_path = /health                  ; health check endpoint
query_path = /query                    ; http query endpoint (optional)

; http settings
max_request_size = 1048576             ; 1mb max request
max_response_size = 10485760           ; 10mb max response
timeout_seconds = 30

[protocol]
version = 1                            ; protocol version
max_request_size = 65536               ; max binary request
max_response_chunks = 10000            ; max streaming chunks
stream_threshold_records = 1000        ; stream if > n records
compression = none                     ; none, gzip, lz4

[performance]
epoll_max_events = 1024                ; max epoll events per iteration
use_sendfile = true                    ; use sendfile() for zero-copy

[metrics]
enabled = true
prometheus_port = 9090
report_interval_seconds = 15

; exported metrics
export_query_count = true
export_query_latency = true
export_connection_count = true
export_cache_stats = true
```

### s4: discovery service

**file**: `s4_discovery.ini`

```ini
[service]
name = ws-discovery
log_level = info
daemon = false
pid_file = /var/run/ws-discovery.pid

[station]
id = 1                                   ; unique station id (1-65535)
hostname = auto                          ; auto or explicit hostname
location_lat = 52.5200                   ; station latitude
location_lon = 13.4050                   ; station longitude
capabilities = query,aggregate,replicate ; comma-separated capabilities

[network]
; interface configuration
bind_interface = eth0                    ; network interface
bind_address = auto                      ; auto or explicit ip

; udp broadcast/multicast
broadcast_address = 255.255.255.255      ; broadcast address
multicast_group = 239.255.42.42          ; multicast group (if enabled)
multicast_ttl = 1                        ; multicast ttl
enable_multicast = false                 ; use multicast instead of broadcast

; ports
beacon_port = 5000                       ; udp beacon port
health_check_port = 5001                 ; tcp health check port
replication_port = 8443                  ; replication/mtls port

[beacon]
interval_seconds = 5                     ; beacon broadcast interval
ttl = 1                                  ; beacon ttl
max_age_seconds = 30                     ; peer considered stale after

[health]
check_interval_seconds = 10              ; health check frequency
timeout_seconds = 5                      ; health check timeout
failures_before_down = 3                 ; consecutive failures before marking down
recovery_interval_seconds = 60           ; time before retrying failed peer

[election]
enabled = true                           ; enable leader election
algorithm = bully                        ; bully or raft
coordinator_timeout_seconds = 15         ; election timeout
split_brain_prevention = true            ; detect and prevent split brain

[replication]
enabled = true                           ; enable ha replication
role = auto                              ; auto, leader, follower
sync_mode = async                        ; sync or async
sync_interval_seconds = 60               ; replication interval
batch_size = 1000                        ; records per replication batch
conflict_resolution = timestamp          ; timestamp or manual
lag_threshold_seconds = 300              ; alert if lag > threshold

[mtls]
enabled = true                           ; enable mtls
cert_path = /etc/ws/certs/server.crt
key_path = /etc/ws/certs/server.key
ca_path = /etc/ws/certs/ca.crt
crl_path = /etc/ws/certs/crl.pem         ; optional crl
verify_peer = true                       ; require client certificates
cipher_suites = ecdhe+aes256             ; allowed cipher suites
tls_version = 1.3                        ; minimum tls version

[database]
path = /var/lib/ws/weather.db
wal_mode = true

[metrics]
enabled = true
prometheus_port = 9090
```

### c1: cli client

**file**: `cli.ini`

```ini
[connection]
; local service connection
query_host = localhost
query_port = 8080
discovery_host = localhost
discovery_port = 5000
timeout_seconds = 30
retry_attempts = 3

; security
mtls_enabled = true
cert_path = ~/.ws/certs/client.crt
key_path = ~/.ws/certs/client.key
ca_path = ~/.ws/certs/ca.crt
verify_hostname = true

[output]
default_format = table                 ; table, csv, json
max_rows = 1000                        ; max rows to display
max_width = 120                        ; terminal width
pretty_print = true                    ; pretty formatting
show_timestamps = true
show_units = true                      ; show units (°c, %, etc.)

[display]
date_format = "%y-%m-%d %h:%m:%s"      ; date display format
timezone = local                       ; local or utc
temperature_unit = celsius             ; celsius, fahrenheit, kelvin
pressure_unit = hpa                    ; hpa, mbar, inhg
speed_unit = ms                        ; ms, kmh, mph, knots
locale = en_us.utf-8
color_enabled = auto                   ; auto, true, false

[history]
enabled = true
file = ~/.ws/history
max_entries = 1000
duplicate_policy = ignore              ; ignore, erase_previous

[query]
default_station = 0                    ; 0 = all stations
default_timerange = 24h                ; default query time range
auto_aggregate_threshold = 10000       ; auto-aggregate if > n records

[aliases]
; command shortcuts
t = query --metrics temperature
h = query --metrics humidity
last = query --from "-1 hour"
today = query --from 00:00 --to now
```

## configuration examples

### minimal development config

```ini
[service]
log_level = debug
daemon = false

[paths]
database_path = ./weather.db
csv_directory = ./data/csv

[network]
bind_port = 8080
```

### production config

```ini
[service]
name = ws-production
log_level = warn
daemon = true

[paths]
database_path = /var/lib/ws/weather.db
csv_directory = /data/csv
processed_directory = /data/processed
error_directory = /data/error

[ingestion]
batch_size = 10000
mmap_threshold_bytes = 2147483648

[query]
bind_port = 8080
thread_pool_size = 16
max_connections = 1000
rate_limit_enabled = true

[database]
cache_size_mb = 2048
wal_mode = true

[mtls]
enabled = true
cert_path = /etc/ws/certs/server.crt
key_path = /etc/ws/certs/server.key
ca_path = /etc/ws/certs/ca.crt
```

### high-performance config

```ini
[service]
log_level = error

[ingestion]
batch_size = 50000
mmap_threshold_bytes = 5368709120  ; 5gb

[query]
thread_pool_size = 32
epoll_max_events = 4096
use_sendfile = true

[database]
cache_size_mb = 4096
mmap_size_mb = 1024
synchronous = normal
```

## validation

### check configuration

```bash
# validate syntax
ws-ingest --config /etc/weather-station/ingestion.ini --check-config

# show effective configuration
ws-ingest --config /etc/weather-station/ingestion.ini --show-config

# test configuration
ws-ingest --config /etc/weather-station/ingestion.ini --test
```

### configuration schema

configuration files are validated against schema:

```ini
# required sections marked with !
[!service]
name = string
log_level = enum(debug,info,warn,error)

[!paths]
database_path = path

[?ingestion]  ; optional section
batch_size = int(min=1,max=100000)
```

## environment-specific configuration

### using includes

```ini
; main.ini
[include]
file = common.ini

[include:production]
file = production.ini
condition = env(env) == "production"

[include:development]
file = development.ini
condition = env(env) == "development"
```

### profile-based config

```bash
# set profile
export ws_profile=production

# loads: config/production.ini
# falls back to: config/default.ini
```

## migration guide

### version 1.0 to 2.0

```diff
 ; changed in v2.0
-[ingestion]
-batch_size = 10000
+[ingestion]
+batch_size = 10000
+parallel_parsers = 4  ; new option
 
-[database]
+; deprecated: [database]
+; use [storage] instead
+[storage]
 path = /var/lib/ws/weather.db
```

---

## troubleshooting config issues

### common problems

| issue | solution |
|-------|----------|
| "config file not found" | check path, use absolute path |
| "invalid value for option" | check data type (int vs string) |
| "unknown section" | check spelling, update version |
| "permission denied" | check file permissions |

### debug configuration

```bash
# show config loading
ws-ingest --config test.ini -vvv

# show effective values
ws-ingest --config test.ini --dump-config
```

---

*next: [security operations](security.md)*
