# database schema

## overview

the weather station system uses sqlite as its primary data store. the database is designed for high-performance time-series data with support for concurrent reads and writes through write-ahead logging (wal) mode.

## design principles

1. **time-series optimized**: indexed by timestamp and station
2. **minimal normalization**: denormalized for read performance
3. **flexible schema**: support for variable csv columns
4. **audit trail**: track all data ingestion
5. **aggregation support**: materialized views for statistics

## schema diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        sqlite database                           │
│                                                                  │
│  ┌──────────────────────┐    ┌──────────────────────┐          │
│  │   weather_data       │    │    hourly_stats      │          │
│  │   (raw data)         │    │   (aggregated)       │          │
│  ├──────────────────────┤    ├──────────────────────┤          │
│  │ id (pk)              │    │ id (pk)              │          │
│  │ station_id (idx)     │    │ station_id (idx)     │          │
│  │ timestamp (idx)      │───►│ hour (idx)           │          │
│  │ temperature          │    │ metric_name          │          │
│  │ humidity             │    │ min_val              │          │
│  │ pressure             │    │ max_val              │          │
│  │ wind_speed           │    │ avg_val              │          │
│  │ wind_direction       │    │ count                │          │
│  │ precipitation        │    └──────────────────────┘          │
│  │ location_lat         │                                      │
│  │ location_lon         │    ┌──────────────────────┐          │
│  │ data_quality         │    │    daily_stats       │          │
│  │ source_file          │    │   (aggregated)       │          │
│  │ imported_at          │    ├──────────────────────┤          │
│  └──────────────────────┘    │ id (pk)              │          │
│                              │ station_id (idx)     │          │
│  ┌──────────────────────┐    │ day (idx)            │          │
│  │   peer_stations      │    │ metric_name          │          │
│  │  (discovery state)   │    │ min_val              │          │
│  ├──────────────────────┤    │ max_val              │          │
│  │ station_id (pk)      │    │ avg_val              │          │
│  │ hostname             │    │ count                │          │
│  │ ip_address           │    └──────────────────────┘          │
│  │ query_port           │                                      │
│  │ last_seen            │    ┌──────────────────────┐          │
│  │ is_leader            │    │   replication_peers  │          │
│  │ is_healthy           │    │  (sync status)       │          │
│  └──────────────────────┘    ├──────────────────────┤          │
│                              │ peer_id (pk, fk)     │          │
│  ┌──────────────────────┐    │ last_sync_timestamp  │          │
│  │   ingest_log         │    │ sync_status          │          │
│  │   (audit trail)      │    │ lag_seconds          │          │
│  ├──────────────────────┤    └──────────────────────┘          │
│  │ id (pk)              │                                      │
│  │ filename             │    ┌──────────────────────┐          │
│  │ started_at           │    │  aggregation_jobs    │          │
│  │ completed_at         │    │   (job tracking)     │          │
│  │ records_processed    │    ├──────────────────────┤          │
│  │ status               │    │ id (pk)              │          │
│  └──────────────────────┘    │ job_type             │          │
│                              │ time_start           │          │
│  ┌──────────────────────┐    │ status               │          │
│  │   election_log       │    └──────────────────────┘          │
│  │  (ha tracking)       │                                      │
│  ├──────────────────────┤                                      │
│  │ id (pk)              │                                      │
│  │ old_leader_id        │                                      │
│  │ new_leader_id        │                                      │
│  │ reason               │                                      │
│  └──────────────────────┘                                      │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## table definitions

### weather_data

primary storage for raw weather observations.

```sql
create table if not exists weather_data (
        id integer primary key autoincrement,
        station_id integer not null,
        timestamp integer not null,              /* unix epoch seconds */
        temperature real,
        humidity real,
        pressure real,
        wind_speed real,
        wind_direction real,
        precipitation real,
        location_lat real,
        location_lon real,
        data_quality integer default 0,          /* 0=raw, 1=validated, 2=aggregated */
        source_file text,                        /* original csv filename */
        imported_at integer default (strftime('%s', 'now')),
        
        /* constraints */
        unique(station_id, timestamp)
);

/* primary query indexes */
create index idx_weather_station_time 
on weather_data(station_id, timestamp);

create index idx_weather_time_only 
on weather_data(timestamp);

/* quality filtering index */
create index idx_weather_quality 
on weather_data(data_quality) 
where data_quality >= 1;
```

**access patterns:**
- **write**: s1 (ingestion) - append-only, batch inserts
- **read**: s3 (query) - range queries by time and station
- **read**: s2 (aggregation) - full table scans for statistics

### hourly_stats

materialized aggregations by hour for fast querying.

```sql
create table if not exists hourly_stats (
        id integer primary key autoincrement,
        station_id integer not null,
        hour integer not null,                   /* unix epoch, truncated to hour */
        metric_name text not null,               /* 'temperature', 'humidity', etc. */
        min_val real,
        max_val real,
        avg_val real,
        count integer,
        computed_at integer default (strftime('%s', 'now')),
        
        /* constraints */
        unique(station_id, hour, metric_name)
);

/* query indexes */
create index idx_hourly_station_time 
on hourly_stats(station_id, hour);

create index idx_hourly_metric_time 
on hourly_stats(metric_name, hour);

/* daily aggregation source index */
create index idx_hourly_daily_source 
on hourly_stats(station_id, hour, metric_name);
```

**access patterns:**
- **write**: s2 (aggregation) - batch inserts after computation
- **read**: s3 (query) - fast statistical queries

### daily_stats

materialized aggregations by day.

```sql
create table if not exists daily_stats (
        id integer primary key autoincrement,
        station_id integer not null,
        day integer not null,                    /* unix epoch, truncated to day */
        metric_name text not null,
        min_val real,
        max_val real,
        avg_val real,
        count integer,
        min_time integer,                        /* hour of minimum value */
        max_time integer,                        /* hour of maximum value */
        computed_at integer default (strftime('%s', 'now')),
        
        unique(station_id, day, metric_name)
);

create index idx_daily_station_time 
on daily_stats(station_id, day);

create index idx_daily_metric_time 
on daily_stats(metric_name, day);
```

### peer_stations

registry of known peer weather stations.

```sql
create table if not exists peer_stations (
        station_id integer primary key,
        hostname text not null,
        ip_address text,
        query_port integer,
        replication_port integer,
        first_seen integer,
        last_seen integer,
        last_beacon integer,
        is_leader boolean default 0,
        is_healthy boolean default 0,
        capabilities integer default 0,          /* bitmask */
        tls_fingerprint blob,                    /* sha256 of certificate */
        consecutive_failures integer default 0,
        
        unique(hostname, query_port)
);

/* leader lookup index */
create index idx_peer_leader 
on peer_stations(is_leader) 
where is_leader = 1;

/* healthy peers index */
create index idx_peer_healthy 
on peer_stations(is_healthy, last_seen) 
where is_healthy = 1;
```

**access patterns:**
- **write**: s4 (discovery) - insert on first beacon, update on subsequent
- **read**: s4 (discovery) - leader lookup, peer enumeration
- **read**: s3 (query) - fan-out query routing

### ingest_log

audit trail of csv ingestion operations.

```sql
create table if not exists ingest_log (
        id integer primary key autoincrement,
        filename text not null,
        started_at integer not null,
        completed_at integer,
        records_processed integer default 0,
        records_failed integer default 0,
        status text not null,                    /* 'running', 'completed', 'failed' */
        error_message text,
        processing_time_ms integer
);

create index idx_ingest_status 
on ingest_log(status, started_at);

create index idx_ingest_file 
on ingest_log(filename);
```

**access patterns:**
- **write**: s1 (ingestion) - start and completion logging
- **read**: s1 (ingestion) - resume interrupted files
- **read**: operations - monitoring and debugging

### aggregation_jobs

track aggregation job execution.

```sql
create table if not exists aggregation_jobs (
        id integer primary key autoincrement,
        job_type text not null,                  /* 'hourly', 'daily' */
        time_start integer not null,             /* window start */
        time_end integer not null,               /* window end */
        status text not null,                    /* 'pending', 'running', 'completed', 'failed' */
        worker_id integer,
        started_at integer,
        completed_at integer,
        records_processed integer,
        error_message text
);

create index idx_agg_jobs_status 
on aggregation_jobs(status, job_type, time_start);

create index idx_agg_jobs_time 
on aggregation_jobs(time_start, time_end);
```

### election_log

track leader election events for debugging ha issues.

```sql
create table if not exists election_log (
        id integer primary key autoincrement,
        timestamp integer default (strftime('%s', 'now')),
        old_leader_id integer,
        new_leader_id integer,
        reason text                              /* 'timeout', 'higher_id', 'manual' */
);

create index idx_election_time 
on election_log(timestamp);
```

### replication_peers

track replication status for ha.

```sql
create table if not exists replication_peers (
        peer_id integer primary key,
        last_sync_timestamp integer,
        last_sync_records integer,
        sync_status text,                        /* 'idle', 'syncing', 'failed' */
        lag_seconds integer,
        foreign key (peer_id) references peer_stations(station_id)
);
```

## database configuration

### recommended pragma settings

```sql
-- enable write-ahead logging for concurrent reads/writes
pragma journal_mode = wal;

-- balance durability and performance
pragma synchronous = normal;

-- large cache for better performance
pragma cache_size = -1048576;        /* 1gb in pages (negative = kibibytes) */

-- memory for temporary tables
pragma temp_store = memory;

-- memory map the database file
pragma mmap_size = 268435456;        /* 256mb */

-- optimize query planner
pragma optimize;

-- foreign key constraints
pragma foreign_keys = on;
```

### connection settings

```c
/* open database with recommended settings */
sqlite3 *db;
int rc = sqlite3_open_v2(db_path, &db,
        sqlite_open_readwrite | sqlite_open_create | sqlite_open_wal,
        null);

if (rc != sqlite_ok) {
        fprintf(stderr, "cannot open database: %s\n", sqlite3_errmsg(db));
        return -1;
}

/* set pragmas */
sqlite3_exec(db, "pragma journal_mode = wal", null, null, null);
sqlite3_exec(db, "pragma synchronous = normal", null, null, null);
sqlite3_exec(db, "pragma cache_size = -1048576", null, null, null);
sqlite3_exec(db, "pragma temp_store = memory", null, null, null);
sqlite3_exec(db, "pragma mmap_size = 268435456", null, null, null);
```

## common queries

### insert weather record

```sql
insert into weather_data 
(station_id, timestamp, temperature, humidity, pressure, wind_speed, 
 wind_direction, precipitation, location_lat, location_lon, source_file)
values 
(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
on conflict(station_id, timestamp) 
do update set
    temperature = excluded.temperature,
    humidity = excluded.humidity,
    pressure = excluded.pressure,
    wind_speed = excluded.wind_speed,
    wind_direction = excluded.wind_direction,
    precipitation = excluded.precipitation;
```

### query by time range

```sql
select * from weather_data
where station_id = ?
  and timestamp >= ?
  and timestamp <= ?
order by timestamp;
```

### hourly aggregation

```sql
insert or replace into hourly_stats
(station_id, hour, metric_name, min_val, max_val, avg_val, count)
select 
    station_id,
    (timestamp / 3600) * 3600 as hour,
    'temperature',
    min(temperature),
    max(temperature),
    avg(temperature),
    count(temperature)
from weather_data
where timestamp >= ?
  and timestamp < ?
  and temperature is not null
group by station_id, hour;
```

### get latest data timestamp

```sql
select max(timestamp) as latest
from weather_data
where station_id = ?;
```

### find missing hourly stats

```sql
select distinct (timestamp / 3600) * 3600 as hour
from weather_data wd
where wd.timestamp >= ?
  and wd.timestamp < ?
  and not exists (
      select 1 from hourly_stats hs
      where hs.station_id = wd.station_id
        and hs.hour = (wd.timestamp / 3600) * 3600
        and hs.metric_name = 'temperature'
  );
```

## data retention

### archival strategy

```sql
-- archive old raw data (keep aggregates)
insert into weather_data_archive
select * from weather_data
where timestamp < strftime('%s', 'now', '-90 days');

delete from weather_data
where timestamp < strftime('%s', 'now', '-90 days');

-- vacuum to reclaim space
vacuum;
```

### partitioning (future)

for very large datasets, consider:
- separate database per month/year
- attached databases for queries
- wal archiving

## performance tuning

### index optimization

```sql
-- analyze query patterns
explain query plan
select * from weather_data 
where station_id = 1 
  and timestamp > 1704067200;

-- check index usage
select * from sqlite_stat1;

-- force index usage
select * from weather_data 
indexed by idx_weather_station_time
where station_id = 1 
  and timestamp > 1704067200;
```

### query optimization

```sql
-- use covering indexes
select station_id, timestamp, temperature
from weather_data
where station_id = 1
  and timestamp > 1704067200;
-- index: (station_id, timestamp, temperature) covers query

-- batch operations in transactions
begin transaction;
-- 10000 inserts
commit;

-- use prepared statements for repeated queries
```

## backup and recovery

### online backup

```bash
# sqlite backup while database is in use
sqlite3 weather.db ".backup to backup.db"

# incremental backup via wal archiving
cp weather.db-wal backup/
```

### point-in-time recovery

```sql
-- enable wal archiving (in sqlite3 cli)
pragma wal_checkpoint;

-- restore from backup
-- 1. stop all services
-- 2. replace weather.db with backup
-- 3. replay wal files
-- 4. start services
```

## monitoring

### database metrics

```sql
-- database size
select page_count * page_size as size_bytes
from pragma_page_count(), pragma_page_size();

-- table sizes
select 
    name,
    sum(pgsize) as size_bytes,
    count(*) as pages
from dbstat
where name in ('weather_data', 'hourly_stats', 'daily_stats')
group by name;

-- wal size
select * from pragma_wal_checkpoint();

-- index usage
select * from sqlite_stat1;
```

---

*next: [testing documentation](../testing/readme.md)*
