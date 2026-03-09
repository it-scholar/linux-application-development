# system architecture

## overview

the weather station microservices system is a distributed application where each participant operates a complete weather station. stations can discover each other, exchange weather data, and provide high availability through automatic leader election.

## design principles

1. **simplicity**: each service has a single, well-defined responsibility
2. **scalability**: services can be scaled independently based on load
3. **resilience**: automatic failover and graceful degradation
4. **observability**: comprehensive logging and metrics throughout
5. **educational value**: demonstrates linux system programming concepts

## service architecture

### service responsibilities

```
┌─────────────────────────────────────────────────────────────────────┐
│                        weather station                              │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐ │
│  │                    data layer (sqlite)                        │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐   │ │
│  │  │weather_data │  │hourly_stats │  │   peer_stations     │   │ │
│  │  │  (raw)      │  │ (aggregated)│  │  (discovery state)  │   │ │
│  │  └─────────────┘  └─────────────┘  └─────────────────────┘   │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                              ▲                                      │
│         ┌────────────────────┼────────────────────┐                 │
│         │                    │                    │                 │
│  ┌──────┴──────┐    ┌────────┴────────┐  ┌──────┴──────┐          │
│  │  ingestion  │    │   aggregation   │  │    query    │          │
│  │   (s1)      │    │    (s2)         │  │   (s3)      │          │
│  │             │    │                 │  │             │          │
│  │• csv parser │    │• fork workers   │  │• epoll loop │          │
│  │• inotify    │    │• posix mq      │  │• thread pool│          │
│  │• streaming  │    │• stats calc    │  │• protocol   │          │
│  │• mmap opt   │    │• materialized  │  │• http/min   │          │
│  │• sqlite     │    │  views         │  │• prometheus │          │
│  └─────────────┘    └─────────────────┘  └──────┬──────┘          │
│                                                  │                  │
│  ┌───────────────────────────────────────────────────────────────┐ │
│  │                    discovery service (s4)                     │ │
│  │                                                               │ │
│  │  • udp broadcast beacons          • health check probes       │ │
│  │  • peer registry                  • bully leader election     │ │
│  │  • mtls coordinator               • data replication          │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐ │
│  │                     cli client (c1)                           │ │
│  │                                                               │ │
│  │  • interactive repl              • query builder              │ │
│  │  • station browser               • results formatter          │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### data flow

#### 1. ingestion flow

```
┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
│ csv file │────►│ inotify  │────►│ s1 parse │────►│ sqlite   │
│  (gbs)   │     │  watch   │     │ stream   │     │  insert  │
└──────────┘     └──────────┘     └──────────┘     └──────────┘
                                                        │
                              ┌─────────────────────────┘
                              ▼
                        ┌──────────┐
                        │ posix mq │
                        │  status  │
                        └──────────┘
```

#### 2. query flow

```
┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
│  client  │────►│   tcp    │────►│  s3      │────►│ sqlite   │
│ request  │     │  socket  │     │ handler  │     │  query   │
└──────────┘     └──────────┘     └──────────┘     └──────────┘
                                         │
                                         ▼
                              ┌──────────────────────┐
                              │   response chunks    │
                              │   (binary protocol)  │
                              └──────────────────────┘
```

#### 3. discovery flow

```
┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
│  timer   │────►│  beacon  │────►│  udp     │────►│  peer    │
│ (5s int) │     │  build   │     │ broadcast│     │  update  │
└──────────┘     └──────────┘     └──────────┘     └──────────┘
                                                          │
           ┌──────────────────────────────────────────────┘
           ▼
    ┌──────────────┐
    │ bully check  │
    │ (leader?)    │
    └──────────────┘
```

## component interactions

### inter-process communication

| from | to | mechanism | purpose |
|------|-----|-----------|---------|
| s1 | s2 | posix mq | notify new data available |
| s2 | sqlite | sql | write aggregated statistics |
| s3 | s4 | unix socket | get peer list for fanout |
| s4 | s4 (remote) | udp broadcast | beacon presence |
| s4 | s4 (remote) | tcp + mtls | health checks, replication |
| c1 | s3 | tcp + mtls | queries and commands |

### database access patterns

```
┌────────────────────────────────────────────────────────────┐
│                     sqlite database                        │
│                                                            │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    │
│  │   s1 write  │    │   s2 write  │    │   s3 read   │    │
│  │  (wal mode) │    │  (wal mode) │    │   (wal)     │    │
│  │             │    │             │    │             │    │
│  │  insert     │    │  insert/    │    │  select     │    │
│  │  (append)   │    │  update     │    │  (indexed)  │    │
│  └─────────────┘    └─────────────┘    └─────────────┘    │
│                                                            │
│  concurrency: readers don't block writers (wal)           │
│  isolation: transaction for batch operations              │
└────────────────────────────────────────────────────────────┘
```

## deployment architectures

### development (docker compose)

```yaml
# single machine, all services as containers
services:
  s1-ingestion:    # csv ingestion
  s2-aggregation:  # background workers
  s3-query:        # api endpoint (exposed port 8080)
  s4-discovery:    # network mode: host for broadcast
  
volumes:
  sqlite-data:     # shared database volume
```

### production (kubernetes)

```
┌────────────────────────────────────────────────────────────┐
│                    kubernetes cluster                      │
│                                                            │
│  ┌──────────────────────────────────────────────────────┐  │
│  │                 namespace: weather-station-1         │  │
│  │                                                      │  │
│  │   ┌──────────┐  ┌──────────┐  ┌──────────────────┐  │  │
│  │   │ingestion │  │aggregate │  │ query (service)  │  │  │
│  │   │  pod     │  │   pod    │  │   (3 replicas)   │  │  │
│  │   └──────────┘  └──────────┘  └──────────────────┘  │  │
│  │                                                      │  │
│  │   ┌──────────┐  ┌──────────┐                        │  │
│  │   │discovery │  │  sqlite  │                        │  │
│  │   │  pod     │  │   pvc    │                        │  │
│  │   └──────────┘  └──────────┘                        │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                            │
│  ┌──────────────────────────────────────────────────────┐  │
│  │                 namespace: weather-station-2         │  │
│  │                                                      │  │
│  │   [same structure as station-1]                     │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                            │
│  external access: ingress controller → loadbalancer       │
│  monitoring: prometheus servicemonitor                    │
└────────────────────────────────────────────────────────────┘
```

## network architecture

### port allocation

| service | protocol | default port | description |
|---------|----------|--------------|-------------|
| s3 query | tcp | 8080 | binary protocol endpoint |
| s3 metrics | tcp | 9090 | prometheus metrics (http) |
| s4 discovery | udp | 5000 | beacon broadcasts |
| s4 health | tcp | 5001 | health check probes |
| s4 mtls | tcp | 8443 | secure replication |

### network flow

```
┌──────────────┐
│   client     │
│    (cli)     │
└──────┬───────┘
       │ tcp:8080 (mtls after day 4)
       ▼
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ local query  │◄────│   sqlite     │     │ aggregation │
│    (s3)      │     │  database    │────►│   (s2)      │
└──────────────┘     └──────────────┘     └──────────────┘
       │
       │ query remote stations
       ▼
┌──────────────┐
│   discovery  │
│    (s4)      │
└──────┬───────┘
       │ udp:5000 broadcast / tcp:8443 mtls
       ▼
┌──────────────────────────────────────────┐
│           remote stations                │
│  ┌────────┐  ┌────────┐  ┌────────┐     │
│  │station2│  │station3│  │stationn│     │
│  └────────┘  └────────┘  └────────┘     │
└──────────────────────────────────────────┘
```

## high availability architecture

### leader election (bully algorithm)

```
timeline:

t0: station 1 (id=1) starts ──────────────┐
    beacons: "id=1, not leader"            │
                                            │
t1: station 2 (id=2) starts ───────────────┤
    sees id=1, sends coordinate message    │
    station 1 responds with ack            │
    station 2 becomes leader               │
                                            │
t2: station 3 (id=3) starts ───────────────┤
    sees ids 1,2, sends coordinate         │
    station 2 acknowledges                 │
    station 3 becomes leader               │
                                            │
t3: station 3 fails ───────────────────────┤
    missing beacons detected by 1,2        │
    after timeout, new election            │
    station 2 becomes leader ◄─────────────┘
```

### data replication (active-passive)

```
normal operation:
┌──────────┐         write          ┌──────────┐
│  leader  │───────────────────────►│ follower │
│   (s3)   │      (ws_msg_replicate)│   (s3)   │
└──────────┘                        └──────────┘
     │                                    │
     │ read                               │ read
     ▼                                    ▼
  client                               client

failover:
┌──────────┐     failure     ┌──────────┐
│  leader  │◄────────────────│ follower │
│  (down)  │    detected     │ promoted │
└──────────┘                 └──────────┘
                                  │
                                  │ redirect
                                  ▼
                               client
```

## scalability considerations

### horizontal scaling

**query service (s3)**: stateless, can run multiple instances behind load balancer

```
                    ┌──────────────┐
                    │  load        │
                    │  balancer    │
                    └──────┬───────┘
                           │
           ┌───────────────┼───────────────┐
           ▼               ▼               ▼
     ┌──────────┐   ┌──────────┐   ┌──────────┐
     │ query-1  │   │ query-2  │   │ query-3  │
     │  (pod)   │   │  (pod)   │   │  (pod)   │
     └────┬─────┘   └────┬─────┘   └────┬─────┘
          └───────────────┼───────────────┘
                          │
                    ┌─────┴─────┐
                    │  sqlite   │
                    │  (pvc)    │
                    └───────────┘
```

**aggregation service (s2)**: single instance (stateful), but uses worker pool

```
┌──────────────┐
│ aggregation  │
│   master     │
└──────┬───────┘
       │ fork()
       ├──────────┬──────────┬──────────┐
       ▼          ▼          ▼          ▼
  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐
  │worker-1│ │worker-2│ │worker-3│ │worker-4│
  └────────┘ └────────┘ └────────┘ └────────┘
```

### vertical scaling

**ingestion service (s1)**: memory-bound by mmap settings

```c
// tuning for large files
pragma cache_size = -1048576;        // 1gb cache
pragma mmap_size = 268435456;        // 256mb mmap
pragma synchronous = normal;          // balance safety/perf
```

## security architecture

### mtls handshake flow

```
client                                    server
   │                                         │
   │────────── client hello ────────────────►│
   │◄───────── server hello + certificate ──│
   │                                         │
   │◄───────── request client certificate ──│
   │                                         │
   │────────── client certificate ──────────►│
   │                                         │
   │────────── key exchange ────────────────►│
   │◄───────── change cipher spec ──────────│
   │                                         │
   │────────── encrypted handshake ─────────►│
   │◄───────── encrypted handshake ─────────│
   │                                         │
   │══════════ encrypted channel ════════════│
```

### certificate hierarchy

```
                         ┌──────────┐
                         │ root ca  │
                         │ (self)   │
                         └────┬─────┘
                              │
              ┌───────────────┴───────────────┐
              │                               │
        ┌─────┴─────┐                   ┌─────┴─────┐
        │ station 1 │                   │ station 2 │
        │  (server) │                   │  (server) │
        │  (client) │                   │  (client) │
        └───────────┘                   └───────────┘
```

## resource requirements

### minimum (development)

| service | cpu | memory | disk |
|---------|-----|--------|------|
| s1 ingestion | 1 core | 512mb | 10gb |
| s2 aggregation | 1 core | 256mb | shared |
| s3 query | 1 core | 256mb | shared |
| s4 discovery | 0.5 core | 128mb | shared |
| sqlite | - | 1gb (cache) | 100gb |
| **total** | **3.5 cores** | **2gb** | **100gb** |

### recommended (production)

| service | cpu | memory | disk |
|---------|-----|--------|------|
| s1 ingestion | 2 cores | 2gb | 1tb (ssd) |
| s2 aggregation | 2 cores | 1gb | shared |
| s3 query (x3) | 6 cores | 3gb | shared |
| s4 discovery | 1 core | 512mb | shared |
| sqlite | - | 4gb (cache) | 1tb |
| **total** | **11 cores** | **10gb** | **1tb** |

## monitoring & observability

### metrics endpoints

**prometheus format (s3:9090/metrics)**:
```
# help ws_ingested_records_total total records ingested
# type ws_ingested_records_total counter
ws_ingested_records_total{station="1"} 1543200

# help ws_query_duration_seconds query latency
# type ws_query_duration_seconds histogram
ws_query_duration_seconds_bucket{le="0.005"} 1024
ws_query_duration_seconds_bucket{le="0.01"} 2048

# help ws_active_connections current tcp connections
# type ws_active_connections gauge
ws_active_connections 42

# help ws_leader_status leader election state
# type ws_leader_status gauge
ws_leader_status 1
```

### log levels

- **debug**: detailed flow, query execution plans
- **info**: service lifecycle, major operations
- **warn**: degraded performance, retry attempts
- **error**: failures, data integrity issues
- **fatal**: unrecoverable errors, exiting

### health checks

```bash
# tcp health check (s3)
echo "health" | nc localhost 8080
# expected: "healthy ok"

# http health check (s3)
curl http://localhost:9090/health
# expected: {"status":"healthy","services":{"ingestion":"up","query":"up"}}
```

---

*next: [database schema](database.md)*
