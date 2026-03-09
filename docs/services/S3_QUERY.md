# s3: query service

## overview

the query service (s3) provides the external api for retrieving weather data. it implements an epoll-based concurrent tcp server with a custom binary protocol, minimal http support, and prometheus metrics export.

## responsibilities

- accept incoming tcp connections (custom binary protocol)
- parse and execute queries against sqlite database
- handle concurrent requests via thread pool
- support streaming responses for large result sets
- provide prometheus metrics endpoint (http)
- implement rate limiting and connection management

## architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     query service                                │
│                                                                  │
│  ┌──────────────┐                                               │
│  │   tcp socket │◄──────────────────────┐                       │
│  │   bind       │                       │                       │
│  └──────┬───────┘                       │ client connections    │
│         │                               │                       │
│         ▼                               │                       │
│  ┌──────────────┐    ┌──────────────┐   │                       │
│  │   epoll      │    │  connection  │   │                       │
│  │   loop       │───►│  manager     │◄──┘                       │
│  └──────┬───────┘    └──────┬───────┘                           │
│         │                   │                                   │
│         │ new connection    │                                   │
│         ▼                   ▼                                   │
│  ┌──────────────┐    ┌──────────────┐                          │
│  │   thread     │◄───┤  request     │                          │
│  │   pool       │    │  queue       │                          │
│  └──────┬───────┘    └──────────────┘                          │
│         │                                                       │
│    ┌────┴────┬──────────┬──────────┬──────────┐                │
│    ▼         ▼          ▼          ▼          ▼                │
│ ┌──────┐ ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐               │
│ │thr 1 │ │thr 2 │  │thr 3 │  │thr 4 │  │thr n │               │
│ └──┬───┘ └──┬───┘  └──┬───┘  └──┬───┘  └──┬───┘               │
│    │        │         │         │         │                    │
│    └────────┴────┬────┴────┬────┴─────────┘                    │
│                  │         │                                   │
│                  ▼         ▼                                   │
│           ┌──────────┐ ┌──────────┐                           │
│           │ sqlite   │ │ sqlite   │                           │
│           │ (read)   │ │ (read)   │                           │
│           └──────────┘ └──────────┘                           │
│                                                                │
│  ┌──────────────┐    ┌──────────────┐                          │
│  │  http server │    │  prometheus  │                          │
│  │  (minimal)   │───►│  metrics     │                          │
│  │  port 9090   │    │  endpoint    │                          │
│  └──────────────┘    └──────────────┘                          │
│                                                                │
└─────────────────────────────────────────────────────────────────┘
```

## configuration

### configuration file (s3_query.ini)

```ini
[service]
name = ws-query
log_level = info
log_destination = syslog

[network]
bind_address = 0.0.0.0
bind_port = 8080
max_connections = 1000
connection_timeout_seconds = 300
keepalive_enabled = true

[thread_pool]
size = 8
queue_depth = 1000

[rate_limit]
enabled = true
requests_per_minute = 600
burst_size = 100

[database]
path = /var/lib/ws/weather.db
read_only = true
max_query_time_seconds = 30

[http]
enabled = true
port = 9090
metrics_path = /metrics
health_path = /health

[protocol]
version = 1
max_request_size = 65536
max_response_chunks = 10000
stream_threshold_records = 1000

[performance]
epoll_max_events = 1024
query_cache_enabled = true
query_cache_ttl_seconds = 60
```

### environment variables

```bash
ws_config=/etc/weather-station/s3_query.ini
ws_bind_address=0.0.0.0
ws_bind_port=8080
ws_db_path=/var/lib/ws/weather.db
ws_thread_pool_size=8
ws_http_port=9090
```

### command-line arguments

```bash
ws-query [options]

options:
  -c, --config path        configuration file path
  -a, --bind-address addr  bind address (default: 0.0.0.0)
  -p, --bind-port port     bind port (default: 8080)
  -b, --db-path path       database file path
  -t, --threads n          thread pool size
  --http-port port         http/metrics port
  --daemon                 run as daemon
  --log-level level        log level
  -h, --help               show help message
```

## api / interface

### binary protocol (primary)

see [binary protocol specification](../protocols/binary_protocol.md) for complete details.

**request types:**
- `ws_msg_query` - execute query
- `ws_msg_ping` - health check
- `ws_msg_discovery` - get station info

**response types:**
- `ws_msg_query_resp` - query results
- `ws_msg_pong` - ping response
- `ws_msg_error` - error response

### http endpoints (port 9090)

#### health check

```bash
get /health

response 200:
{
  "status": "healthy",
  "services": {
    "ingestion": "up",
    "aggregation": "up",
    "query": "up"
  },
  "database": "connected",
  "uptime_seconds": 86400
}
```

#### prometheus metrics

```bash
get /metrics

response 200 (text/plain):
# help ws_query_requests_total total queries
ws_query_requests_total{status="success"} 154320

# help ws_query_duration_seconds query latency
ws_query_duration_seconds_bucket{le="0.01"} 150000

# help ws_active_connections current connections
ws_active_connections 42
```

#### simple query (optional)

```bash
get /data?station=1&from=1704067200&to=1706745600&metrics=temperature,humidity

response 200:
{
  "station_id": 1,
  "records": [
    {"timestamp": 1704067200, "temperature": 15.5, "humidity": 65},
    ...
  ]
}
```

## implementation details

### epoll event loop

```c
struct server_context {
        int listen_fd;
        int epoll_fd;
        int max_events;
        struct epoll_event *events;
        struct thread_pool *workers;
        volatile int shutdown;
};

int server_init(struct server_context *ctx, const char *bind_addr, int port)
{
        /* create listening socket */
        ctx->listen_fd = socket(af_inet, sock_stream | sock_nonblock, 0);
        if (ctx->listen_fd < 0)
                return -1;
        
        /* socket options */
        int reuse = 1;
        setsockopt(ctx->listen_fd, sol_socket, so_reuseaddr, &reuse, sizeof(reuse));
        
        /* bind and listen */
        struct sockaddr_in addr = {
                .sin_family = af_inet,
                .sin_port = htons(port),
                .sin_addr.s_addr = inet_addr(bind_addr)
        };
        
        if (bind(ctx->listen_fd, (struct sockaddr *)&addr, sizeof(addr)) < 0)
                return -1;
        
        if (listen(ctx->listen_fd, somaxconn) < 0)
                return -1;
        
        /* create epoll instance */
        ctx->epoll_fd = epoll_create1(epoll_cloexec);
        if (ctx->epoll_fd < 0)
                return -1;
        
        /* add listen socket to epoll */
        struct epoll_event ev = {
                .events = epollin,
                .data.fd = ctx->listen_fd
        };
        epoll_ctl(ctx->epoll_fd, epoll_ctl_add, ctx->listen_fd, &ev);
        
        return 0;
}

void server_run(struct server_context *ctx)
{
        while (!ctx->shutdown) {
                int nfds = epoll_wait(ctx->epoll_fd, ctx->events,
                        ctx->max_events, -1);
                
                for (int i = 0; i < nfds; i++) {
                        int fd = ctx->events[i].data.fd;
                        uint32_t events = ctx->events[i].events;
                        
                        if (fd == ctx->listen_fd) {
                                /* new connection */
                                accept_connection(ctx);
                        } else if (events & epollin) {
                                /* data available */
                                handle_readable(ctx, fd);
                        } else if (events & epollout) {
                                /* ready to write */
                                handle_writable(ctx, fd);
                        } else if (events & (epollerr | epollhup)) {
                                /* error or disconnect */
                                close_connection(ctx, fd);
                        }
                }
        }
}
```

### thread pool

```c
struct thread_pool {
        pthread_t *threads;
        int num_threads;
        struct request_queue *queue;
        pthread_mutex_t queue_mutex;
        pthread_cond_t queue_cond;
        volatile int shutdown;
};

void* worker_thread(void *arg)
{
        struct thread_pool *pool = arg;
        
        while (!pool->shutdown) {
                pthread_mutex_lock(&pool->queue_mutex);
                
                /* wait for work */
                while (queue_empty(pool->queue) && !pool->shutdown) {
                        pthread_cond_wait(&pool->queue_cond, &pool->queue_mutex);
                }
                
                if (pool->shutdown) {
                        pthread_mutex_unlock(&pool->queue_mutex);
                        break;
                }
                
                struct client_request *req = queue_pop(pool->queue);
                pthread_mutex_unlock(&pool->queue_mutex);
                
                /* process request */
                process_request(req);
                
                /* cleanup */
                free_request(req);
        }
        
        return null;
}
```

### request handler

```c
void process_request(struct client_request *req)
{
        struct ws_header header;
        
        /* parse request header */
        if (parse_header(req->buffer, &header) != 0) {
                send_error_response(req->fd, ws_error_protocol, "invalid header");
                return;
        }
        
        /* validate version */
        if (ntohs(header.version) != ws_protocol_version) {
                send_error_response(req->fd, ws_error_protocol, "version mismatch");
                return;
        }
        
        /* route to handler */
        switch (ntohs(header.msg_type)) {
        case ws_msg_query:
                handle_query_request(req, &header);
                break;
        case ws_msg_ping:
                handle_ping_request(req, &header);
                break;
        case ws_msg_discovery:
                handle_discovery_request(req, &header);
                break;
        default:
                send_error_response(req->fd, ws_error_protocol, "unknown message type");
                break;
        }
}
```

### query execution

```c
void handle_query_request(struct client_request *req, struct ws_header *header)
{
        struct ws_query_req query;
        
        /* parse query parameters */
        if (parse_query_request(req->buffer + sizeof(*header), &query) != 0) {
                send_error_response(req->fd, ws_error_invalid_arg, "invalid query");
                return;
        }
        
        /* build sql */
        char sql[4096];
        build_query_sql(&query, sql, sizeof(sql));
        
        /* execute with timeout */
        sqlite3_stmt *stmt;
        if (sqlite3_prepare_v2(db, sql, -1, &stmt, null) != sqlite_ok) {
                send_error_response(req->fd, ws_error_db, sqlite3_errmsg(db));
                return;
        }
        
        /* bind parameters */
        sqlite3_bind_int64(stmt, 1, query.start_time);
        sqlite3_bind_int64(stmt, 2, query.end_time);
        if (query.station_id != 0) {
                sqlite3_bind_int(stmt, 3, query.station_id);
        }
        
        /* stream results */
        send_query_response_header(req->fd, header->sequence_id);
        
        int record_count = 0;
        while (sqlite3_step(stmt) == sqlite_row) {
                struct ws_record record;
                extract_record(stmt, &record);
                send_record_chunk(req->fd, &record);
                record_count++;
                
                /* yield periodically */
                if (record_count % 1000 == 0) {
                        sched_yield();
                }
        }
        
        sqlite3_finalize(stmt);
        
        send_query_response_trailer(req->fd, record_count);
}
```

## performance characteristics

| metric | target | notes |
|--------|--------|-------|
| throughput | 1000+ queries/sec | with thread pool |
| latency p50 | <5ms | simple indexed queries |
| latency p99 | <10ms | including complex aggregations |
| concurrent connections | 1000 | configurable |
| memory per connection | ~10kb | including buffers |

## monitoring metrics

```
# help ws_query_requests_total total query requests
# type ws_query_requests_total counter
ws_query_requests_total{status="success"} 1543200
ws_query_requests_total{status="error"} 42
ws_query_requests_total{status="timeout"} 5

# help ws_query_duration_seconds query execution time
# type ws_query_duration_seconds histogram
ws_query_duration_seconds_bucket{le="0.001"} 500000
ws_query_duration_seconds_bucket{le="0.005"} 1400000
ws_query_duration_seconds_bucket{le="0.01"} 1543000
ws_query_duration_seconds_bucket{le="0.1"} 1543195

# help ws_query_records_returned total records returned
# type ws_query_records_returned counter
ws_query_records_returned 154320000

# help ws_active_connections current tcp connections
# type ws_active_connections gauge
ws_active_connections 42

# help ws_thread_pool_active_threads active worker threads
# type ws_thread_pool_active_threads gauge
ws_thread_pool_active_threads 8

# help ws_thread_pool_queue_depth pending requests
# type ws_thread_pool_queue_depth gauge
ws_thread_pool_queue_depth 3

# help ws_rate_limit_hits rate limit violations
# type ws_rate_limit_hits counter
ws_rate_limit_hits 15
```

## troubleshooting

### common issues

| symptom | cause | solution |
|---------|-------|----------|
| slow queries | missing indexes | check explain query plan |
| connection refused | port conflict | check `netstat -tlnp` |
| high memory | too many connections | reduce max_connections |
| thread pool exhausted | burst traffic | increase pool size or queue depth |
| timeouts | slow queries | add query timeout, optimize indexes |

### diagnostic commands

```bash
# test connectivity
echo "ping" | nc localhost 8080

# check metrics
curl http://localhost:9090/metrics

# monitor connections
ss -tuln | grep 8080
watch 'ss -tan | grep -c estab'

# query performance
sqlite3 /var/lib/ws/weather.db "explain query plan select * from weather_data where station_id=1;"
```

---

*next: [s4: discovery service](s4_discovery.md)*
