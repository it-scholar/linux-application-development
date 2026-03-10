# weather station microservices system

## lfd401 linux application development - reference implementation

---

## table of contents

1. [system overview](architecture/overview.md)
2. [architecture details](architecture/architecture.md)
3. [services specification](services/)
4. [communication protocols](protocols/)
5. [database schema](architecture/database.md)
6. [configuration reference](operations/configuration.md)
7. [security model](operations/security.md)
8. [deployment guides](deployment/)
9. [testing & validation](testing/)
10. [api reference](api/)
11. [operations guide](operations/)
12. [troubleshooting](operations/troubleshooting.md)

---

## quick start

### for students

1. **day 1**: set up build environment, create project structure
2. **day 2**: implement signal handling, debugging infrastructure
3. **day 3**: build csv ingestion with sqlite storage
4. **day 4**: add networking, process management, mtls
5. **day 5**: complete with threading, epoll, kubernetes

### for kubernetes deployment

**deploy to kubernetes:**
```bash
# build and deploy
helm install weather-station charts/weather-station \
  --namespace weather-station --create-namespace

# verify
kubectl get pods -n weather-station
```

**load noaa weather data:**
```bash
# download data
test-harness retrieve --station USW00014739 --output ./data/csv

# copy to kubernetes
kubectl cp ./data/csv/ weather-station/<pod>:/data/csv/

# validate deployment
test-harness grade --detailed
```

see [deployment guide](deployment/readme.md) for complete instructions.

### for instructors

see [instructor guide](operations/instructor_guide.md) for:
- day-by-day teaching plan
- common student pitfalls
- test harness usage
- assessment rubrics

---

## system capabilities

### core features

- **high-performance data ingestion**: stream-process multi-gb csv files with mmap optimization
- **distributed querying**: query data across multiple weather stations via custom binary protocol
- **real-time aggregation**: background statistical analysis with configurable time windows
- **service discovery**: automatic peer discovery via udp broadcast with health monitoring
- **high availability**: leader election and automatic failover using bully algorithm
- **security**: mutual tls authentication and encrypted communication
- **observability**: prometheus metrics and structured logging

### technical specifications

| capability | specification |
|------------|---------------|
| csv processing | >100 mb/s sustained throughput |
| concurrent queries | 100+ simultaneous clients |
| query latency | <10ms p99 for indexed queries |
| failover time | <30 seconds automatic detection |
| data format | sqlite with wal mode |
| protocol | custom binary tcp + posix mq + http |
| security | mtls with certificate pinning |
| deployment | docker compose + kubernetes |

---

## project structure

```
weather-station/
├── docs/                          # this documentation
├── lib/                          # provided libraries (30%)
│   ├── libws_csv/               # streaming csv parser
│   ├── libws_protocol/          # binary protocol codec
│   └── libws_common/            # common utilities
├── services/                     # student implementation (70%)
│   ├── s1_ingestion/
│   ├── s2_aggregation/
│   ├── s3_query/
│   ├── s4_discovery/
│   └── c1_cli/
├── docker/                       # container definitions
├── k8s/                          # kubernetes manifests
├── test-harness/                 # comprehensive test suite
├── certs/                        # certificate generation
└── data/                         # weather datasets (not in repo)
```

---

## communication flow

```
┌─────────────┐     posix mq      ┌─────────────┐
│  ingestion  │◄─────────────────►│ aggregation │
│   (s1)      │                   │   (s2)      │
└──────┬──────┘                   └─────────────┘
       │
       │ sqlite wal
       ▼
┌─────────────┐     epoll/tcp      ┌─────────────┐
│   sqlite    │◄──────────────────►│    query    │
│   (wal)     │                    │   (s3)      │
└─────────────┘                    └──────┬──────┘
                                          │
                     ┌────────────────────┼────────────────────┐
                     │                    │                    │
                     ▼                    ▼                    ▼
              ┌─────────────┐      ┌─────────────┐      ┌─────────────┐
              │  discovery  │      │     cli     │      │   remote    │
              │   (s4)      │      │   (c1)      │      │  stations   │
              └──────┬──────┘      └─────────────┘      └─────────────┘
                     │
                     │ udp broadcast / mtls
                     ▼
              ┌─────────────┐
              │  peer mesh  │
              │ (all stns)  │
              └─────────────┘
```

---

## key design decisions

### why custom binary protocol?

- **educational**: demonstrates byte-ordering, network programming
- **performance**: minimal overhead vs http/json
- **control**: full visibility into wire format
- **extensibility**: version field allows evolution

### why sqlite?

- **simplicity**: single file, no external dependencies
- **performance**: wal mode enables concurrent readers/writers
- **portability**: works everywhere including containers
- **learning**: sql skills transferable to postgresql

### why bully algorithm?

- **simplicity**: easy to implement and understand
- **deterministic**: highest id always wins
- **no external dependencies**: pure p2p, no consensus library
- **educational**: teaches distributed systems basics

### why mtls?

- **mutual authentication**: both client and server verify identities
- **certificate pinning**: prevents man-in-the-middle attacks
- **industry standard**: real-world security practice
- **openssl**: teaches widely-used crypto library

---

## development standards

### code style

- **linux kernel style**: tabs (8-space), 80-column lines, k&r braces
- **naming**: `snake_case` for functions/variables, `struct_name_s` for structs
- **comments**: c89 `/* */` style, doxygen-compatible
- **error handling**: structured error objects, never silent failures

### git workflow

```bash
# feature branch workflow
git checkout -b feature/ingestion-signals
git add .
git commit -m "s1_ingestion: add signal handlers for graceful shutdown"
git push origin feature/ingestion-signals
# create pull request, code review, merge to main
```

### commit message format

```
service: brief description (50 chars)

detailed explanation of what changed and why.
can span multiple lines for complex changes.

- bullet points for multiple changes
- reference issues: fixes #123
```

---

## performance targets

### ingestion service (s1)

- **throughput**: process 5gb csv in <5 minutes (>100 mb/s)
- **memory**: peak usage <2gb regardless of file size
- **latency**: first record committed within 5 seconds

### query service (s3)

- **latency**: p50 <5ms, p99 <10ms for indexed queries
- **throughput**: 1000+ queries/second per core
- **concurrency**: support 100+ simultaneous connections

### discovery service (s4)

- **detection**: peer failure detected within 15 seconds
- **election**: new leader elected within 30 seconds
- **overhead**: <1% cpu for beaconing at 5s intervals

---

## success criteria

students must demonstrate:

1. **functionality**: all services start, ingest data, respond to queries
2. **correctness**: data integrity maintained across all operations
3. **performance**: meets targets above (within 20% tolerance)
4. **reliability**: graceful degradation, no crashes under load
5. **security**: mtls handshake succeeds, encrypted traffic
6. **operations**: docker/kubernetes deployment functional
7. **code quality**: passes all tests, follows style guide
8. **documentation**: services documented, readme complete

---

## license

this reference implementation is provided for educational purposes as part of the lfd401 linux application development course.

---

*document version: 1.0*
*last updated: 2024*
