# s2: aggregation service

## overview

the aggregation service (s2) performs background statistical analysis on ingested weather data, computing hourly and daily aggregations using a multi-process worker pool architecture.

## responsibilities

- listen for new data notifications via posix mq
- spawn worker processes for parallel computation
- calculate statistical aggregations (min, max, avg, count)
- maintain materialized views in database
- handle schema migrations for aggregation tables
- report progress and health status

## architecture

```
┌───────────────────────────────────────────────────────────────┐
│                  aggregation service                           │
│                                                                │
│  ┌──────────────┐                                             │
│  │   posix mq   │◄──────────────────┐                         │
│  │   listener   │                   │                         │
│  └──────┬───────┘                   │ new data notifications  │
│         │                           │ from s1                 │
│         ▼                           │                         │
│  ┌──────────────┐                   │                         │
│  │   job        │                   │                         │
│  │   queue      │───────────────────┘                         │
│  └──────┬───────┘                                             │
│         │                                                      │
│         ▼                                                      │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐     │
│  │  dispatcher  │───►│ worker pool  │    │   health     │     │
│  │              │    │  manager     │◄──►│   monitor    │     │
│  └──────┬───────┘    └──────┬───────┘    └──────────────┘     │
│         │                   │                                  │
│         │ fork()            │                                  │
│         │                   │                                  │
│    ┌────┴────┬──────────────┼──────────────┬──────────┐       │
│    ▼         ▼              ▼              ▼          ▼       │
│ ┌──────┐ ┌──────┐      ┌──────┐      ┌──────┐  ┌──────┐      │
│ │wrkr 1│ │wrkr 2│      │wrkr 3│      │wrkr 4│  │wrkr n│      │
│ └──┬───┘ └──┬───┘      └──┬───┘      └──┬───┘  └──┬───┘      │
│    │        │             │             │         │          │
│    └────────┴──────┬──────┴──────┬──────┴─────────┘          │
│                    │             │                            │
│                    ▼             ▼                            │
│              ┌──────────┐ ┌──────────┐                       │
│              │ sqlite   │ │ sqlite   │                       │
│              │ (writes) │ │ (writes) │                       │
│              └──────────┘ └──────────┘                       │
│                                                                │
└───────────────────────────────────────────────────────────────┘
```

## configuration

### configuration file (s2_aggregation.ini)

```ini
[service]
name = ws-aggregation
log_level = info
log_destination = syslog

[paths]
database_path = /var/lib/ws/weather.db

[workers]
count = 4
max_jobs_per_worker = 100
job_timeout_seconds = 300
restart_on_failure = true

[aggregation]
; time windows for aggregation
hourly_enabled = true
daily_enabled = true
weekly_enabled = false

; aggregation schedule (cron-like)
hourly_schedule = "0 * * * *"      ; every hour
daily_schedule = "0 0 * * *"       ; daily at midnight

; batch sizes
records_per_batch = 50000

[database]
wal_mode = true
cache_size_mb = 256
synchronous = normal
```

### environment variables

```bash
ws_config=/etc/weather-station/s2_aggregation.ini
ws_db_path=/var/lib/ws/weather.db
ws_worker_count=4
ws_log_level=info
```

### command-line arguments

```bash
ws-aggregate [options]

options:
  -c, --config path        configuration file path
  -b, --db-path path       database file path
  -w, --workers n          number of worker processes
  --daemon                 run as daemon
  --log-level level        log level
  --trigger-now            trigger aggregation immediately
  -h, --help               show help message
```

## api / interface

### posix message queue interface

**queue name**: `/ws_agg_cmd`

**commands**:

```c
enum agg_cmd_type {
        agg_cmd_new_data = 1,           /* new data available */
        agg_cmd_run_hourly = 2,         /* trigger hourly aggregation */
        agg_cmd_run_daily = 3,          /* trigger daily aggregation */
        agg_cmd_status = 4,             /* request status */
        agg_cmd_shutdown = 5,           /* graceful shutdown */
};

struct agg_mq_command {
        uint32_t cmd_type;              /* enum agg_cmd_type */
        uint64_t timestamp_start;       /* data range start */
        uint64_t timestamp_end;         /* data range end */
        uint32_t priority;              /* job priority (0=low, 9=high) */
};
```

**queue name**: `/ws_agg_status` (responses)

```c
struct agg_mq_response {
        uint32_t response_type;
        uint32_t worker_id;
        uint64_t jobs_completed;
        uint64_t jobs_failed;
        double cpu_percent;
        double memory_mb;
        char status_message[256];
};
```

## implementation details

### worker pool architecture

```c
struct worker_pool {
        pid_t workers[max_workers];
        int worker_status[max_workers];     /* 0=idle, 1=busy, 2=failed */
        int pipe_to_worker[max_workers][2]; /* parent -> child */
        int pipe_from_worker[max_workers][2]; /* child -> parent */
        pthread_mutex_t lock;
        int shutdown;
};

int spawn_workers(struct worker_pool *pool, int count)
{
        for (int i = 0; i < count; i++) {
                /* create communication pipes */
                if (pipe(pool->pipe_to_worker[i]) != 0)
                        return -1;
                if (pipe(pool->pipe_from_worker[i]) != 0)
                        return -1;
                
                pid_t pid = fork();
                if (pid < 0) {
                        return -1;
                } else if (pid == 0) {
                        /* child process */
                        close(pool->pipe_to_worker[i][1]);
                        close(pool->pipe_from_worker[i][0]);
                        
                        worker_loop(i,
                                pool->pipe_to_worker[i][0],
                                pool->pipe_from_worker[i][1]);
                        
                        exit(0);
                }
                
                /* parent process */
                close(pool->pipe_to_worker[i][0]);
                close(pool->pipe_from_worker[i][1]);
                
                pool->workers[i] = pid;
                pool->worker_status[i] = 0;
        }
        
        return 0;
}

void worker_loop(int worker_id, int read_fd, int write_fd)
{
        struct agg_job job;
        struct agg_result result;
        
        while (!shutdown) {
                /* wait for job from parent */
                ssize_t n = read(read_fd, &job, sizeof(job));
                if (n != sizeof(job))
                        continue;
                
                /* execute job */
                result.worker_id = worker_id;
                result.success = execute_job(&job, &result);
                
                /* send result to parent */
                write(write_fd, &result, sizeof(result));
        }
}
```

### aggregation algorithms

#### hourly aggregation

```c
int aggregate_hourly(sqlite3 *db, time_t hour_start, struct ws_error_info *err)
{
        time_t hour_end = hour_start + 3600;
        
        const char *sql = 
                "insert or replace into hourly_stats "
                "(station_id, hour, metric_name, min_val, max_val, avg_val, count) "
                "select "
                "  station_id, "
                "  ?1 as hour, "
                "  'temperature', "
                "  min(temperature), "
                "  max(temperature), "
                "  avg(temperature), "
                "  count(temperature) "
                "from weather_data "
                "where timestamp >= ?2 and timestamp < ?3 "
                "group by station_id";
        
        sqlite3_stmt *stmt;
        if (sqlite3_prepare_v2(db, sql, -1, &stmt, null) != sqlite_ok) {
                ws_error_set(err, ws_error_db, "prepare failed: %s",
                        sqlite3_errmsg(db));
                return -1;
        }
        
        sqlite3_bind_int64(stmt, 1, hour_start);
        sqlite3_bind_int64(stmt, 2, hour_start);
        sqlite3_bind_int64(stmt, 3, hour_end);
        
        int rc = sqlite3_step(stmt);
        sqlite3_finalize(stmt);
        
        if (rc != sqlite_done) {
                ws_error_set(err, ws_error_db, "step failed: %s",
                        sqlite3_errmsg(db));
                return -1;
        }
        
        return 0;
}
```

#### daily aggregation

```c
int aggregate_daily(sqlite3 *db, time_t day_start, struct ws_error_info *err)
{
        time_t day_end = day_start + 86400;
        
        const char *sql = 
                "insert or replace into daily_stats "
                "(station_id, day, metric_name, min_val, max_val, avg_val, count, "
                " min_time, max_time) "
                "select "
                "  station_id, "
                "  ?1 as day, "
                "  metric_name, "
                "  min(min_val), "
                "  max(max_val), "
                "  avg(avg_val), "
                "  sum(count), "
                "  min(hour) as min_time, "
                "  max(hour) as max_time "
                "from hourly_stats "
                "where hour >= ?2 and hour < ?3 "
                "group by station_id, metric_name";
        
        /* similar implementation to hourly */
        return execute_aggregation(db, sql, day_start, day_end, err);
}
```

### job distribution

```c
void dispatch_job(struct worker_pool *pool, struct agg_job *job)
{
        pthread_mutex_lock(&pool->lock);
        
        /* find idle worker */
        int selected = -1;
        for (int i = 0; i < max_workers; i++) {
                if (pool->worker_status[i] == 0) {  /* idle */
                        selected = i;
                        pool->worker_status[i] = 1;   /* mark busy */
                        break;
                }
        }
        
        pthread_mutex_unlock(&pool->lock);
        
        if (selected >= 0) {
                /* send job to selected worker */
                write(pool->pipe_to_worker[selected][1], job, sizeof(*job));
        } else {
                /* queue job for later */
                queue_job_for_later(job);
        }
}

void handle_worker_response(struct worker_pool *pool, int worker_id)
{
        struct agg_result result;
        
        read(pool->pipe_from_worker[worker_id][0], &result, sizeof(result));
        
        pthread_mutex_lock(&pool->lock);
        pool->worker_status[worker_id] = 0;  /* mark idle */
        pthread_mutex_unlock(&pool->lock);
        
        if (result.success) {
                ws_log_info("worker %d completed job: %lu records processed",
                        worker_id, result.records_processed);
        } else {
                ws_log_error("worker %d failed job: %s",
                        worker_id, result.error_message);
        }
        
        /* check for queued jobs */
        struct agg_job next_job;
        if (dequeue_job(&next_job)) {
                dispatch_job(pool, &next_job);
        }
}
```

## database schema

### aggregation tables

```sql
-- hourly statistics (materialized view)
create table if not exists hourly_stats (
        id integer primary key autoincrement,
        station_id integer not null,
        hour integer not null,              -- unix timestamp, truncated to hour
        metric_name text not null,          -- 'temperature', 'humidity', etc.
        min_val real,
        max_val real,
        avg_val real,
        count integer,
        computed_at integer default (strftime('%s', 'now')),
        
        unique(station_id, hour, metric_name)
);

create index idx_hourly_station_time 
on hourly_stats(station_id, hour);

create index idx_hourly_metric 
on hourly_stats(metric_name, hour);

-- daily statistics
create table if not exists daily_stats (
        id integer primary key autoincrement,
        station_id integer not null,
        day integer not null,               -- unix timestamp, truncated to day
        metric_name text not null,
        min_val real,
        max_val real,
        avg_val real,
        count integer,
        min_time integer,                   -- hour of min value
        max_time integer,                   -- hour of max value
        computed_at integer default (strftime('%s', 'now')),
        
        unique(station_id, day, metric_name)
);

create index idx_daily_station_time 
on daily_stats(station_id, day);

-- aggregation job log
create table if not exists aggregation_jobs (
        id integer primary key autoincrement,
        job_type text not null,             -- 'hourly', 'daily'
        time_start integer not null,        -- time window start
        time_end integer not null,          -- time window end
        status text not null,               -- 'pending', 'running', 'completed', 'failed'
        worker_id integer,
        started_at integer,
        completed_at integer,
        records_processed integer,
        error_message text
);

create index idx_agg_jobs_status 
on aggregation_jobs(status, job_type);
```

## performance characteristics

| metric | target | notes |
|--------|--------|-------|
| aggregation speed | 100k records/sec | per worker process |
| memory per worker | 100mb | peak during computation |
| latency (hourly) | <1 minute | from trigger to completion |
| latency (daily) | <5 minutes | from trigger to completion |
| cpu usage | 4 cores | with 4 workers at 100% |
| database writes | batched | 1000 records per transaction |

## monitoring metrics

### prometheus metrics

```
# help ws_agg_jobs_total total aggregation jobs
# type ws_agg_jobs_total counter
ws_agg_jobs_total{type="hourly",status="success"} 1543
ws_agg_jobs_total{type="hourly",status="failed"} 2
ws_agg_jobs_total{type="daily",status="success"} 65

# help ws_agg_job_duration_seconds job execution time
# type ws_agg_job_duration_seconds histogram
ws_agg_job_duration_seconds_bucket{type="hourly",le="10"} 1400
ws_agg_job_duration_seconds_bucket{type="hourly",le="30"} 1543

# help ws_agg_records_processed_total records processed
# type ws_agg_records_processed_total counter
ws_agg_records_processed_total{type="hourly"} 154320000

# help ws_agg_workers_active active worker processes
# type ws_agg_workers_active gauge
ws_agg_workers_active 4

# help ws_agg_workers_busy busy workers
# type ws_agg_workers_busy gauge
ws_agg_workers_busy 2

# help ws_agg_queue_depth pending jobs
# type ws_agg_queue_depth gauge
ws_agg_queue_depth 5
```

## troubleshooting

### common issues

| symptom | cause | solution |
|---------|-------|----------|
| workers not processing | database locked | enable wal mode, reduce batch size |
| high memory usage | large result sets | reduce records_per_batch |
| slow aggregation | missing indexes | run analyze, check indexes |
| worker crashes | memory corruption | enable core dumps, check with valgrind |
| queue growing | not enough workers | increase worker count |

### diagnostic commands

```bash
# check worker processes
ps aux | grep ws-aggregate

# view aggregation job status
sqlite3 /var/lib/ws/weather.db "select * from aggregation_jobs order by started_at desc limit 10;"

# check for missing aggregations
sqlite3 /var/lib/ws/weather.db "select hour from hourly_stats where hour < (select max(timestamp)/3600*3600 from weather_data) order by hour desc limit 5;"

# monitor queue depth
cat /proc/sys/fs/mqueue/msg_max
cat /proc/sys/fs/mqueue/msg_default
```

---

*next: [s3: query service](s3_query.md)*
