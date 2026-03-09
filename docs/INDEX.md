# documentation index

## complete documentation guide

welcome to the weather station microservices documentation. this index will help you navigate all available documentation.

---

## quick navigation

### getting started

- **[main readme](readme.md)** - system overview and quick start
- **[architecture overview](architecture/architecture.md)** - system design and components
- **[database schema](architecture/database.md)** - sqlite schema and optimization

### services

- **[services overview](services/readme.md)** - all 5 services overview
- **[s1: ingestion service](services/s1_ingestion.md)** - csv ingestion and processing
- **[s2: aggregation service](services/s2_aggregation.md)** - background statistics
- **[s3: query service](services/s3_query.md)** - api and querying
- **[s4: discovery service](services/s4_discovery.md)** - peer discovery and ha
- **[c1: cli client](services/c1_cli.md)** - command-line interface

### protocols

- **[binary protocol](protocols/binary_protocol.md)** - tcp communication protocol

### operations

- **[configuration reference](operations/configuration.md)** - all config options
- **[operations guide](operations/readme.md)** - day-to-day operations
- **[troubleshooting](operations/troubleshooting.md)** - common issues and fixes
- **[instructor guide](operations/instructor_guide.md)** - teaching the course

### deployment

- **[deployment guides](deployment/readme.md)** - docker, kubernetes, systemd

### testing

- **[testing guide](testing/readme.md)** - go test harness overview and quick start
- **[go test harness](testing/go_harness.md)** - detailed architecture and specification
- **[data retrieval](testing/readme.md#data-retrieval)** - download real noaa weather data
- **[contract validation](testing/go_harness.md#1-validate)** - verify service contracts
- **[performance testing](testing/go_harness.md#3-benchmark)** - throughput and latency benchmarks
- **[chaos testing](testing/go_harness.md#4-chaos)** - resilience and failure testing
- **[automatic grading](testing/go_harness.md#5-grade)** - scoring and detailed feedback
- **[github actions ci](testing/go_harness.md#github-actions-workflow)** - github actions workflow
- **[gitlab ci](testing/go_harness.md#gitlab-ci)** - gitlab ci/cd configuration

---

## by role

### for students

1. start with [main readme](readme.md)
2. study [architecture overview](architecture/architecture.md)
3. read service specifications:
   - [s1](services/s1_ingestion.md) - day 2-3
   - [s2](services/s2_aggregation.md) - day 4
   - [s3](services/s3_query.md) - day 4
   - [s4](services/s4_discovery.md) - day 4-5
   - [c1](services/c1_cli.md) - day 4
4. review [binary protocol](protocols/binary_protocol.md)
5. check [troubleshooting](operations/troubleshooting.md) when stuck

### for instructors

1. read [main readme](readme.md) for context
2. study [instructor guide](operations/instructor_guide.md)
3. review [architecture overview](architecture/architecture.md)
4. prepare using [deployment guides](deployment/readme.md)
5. use [testing guide](testing/readme.md) for validation

### for operators

1. start with [operations guide](operations/readme.md)
2. reference [configuration](operations/configuration.md)
3. use [troubleshooting](operations/troubleshooting.md)
4. deploy using [deployment guides](deployment/readme.md)

---

## by topic

### system architecture

- [architecture overview](architecture/architecture.md) - complete system design
- [database schema](architecture/database.md) - data model
- [binary protocol](protocols/binary_protocol.md) - communication

### implementation

- [services](services/) - all 5 microservices
- [configuration](operations/configuration.md) - all settings
- [binary protocol](protocols/binary_protocol.md) - wire format

### deployment

- [docker compose](deployment/readme.md#docker-compose)
- [kubernetes](deployment/readme.md#kubernetes)
- [systemd](deployment/readme.md#systemd)
- [manual](deployment/readme.md#manual)

### operations

- [daily operations](operations/readme.md#daily-operations)
- [monitoring](operations/readme.md#monitoring)
- [backup/recovery](operations/readme.md#backup-and-recovery)
- [troubleshooting](operations/troubleshooting.md)
- [security](operations/readme.md#security-operations)

### testing

- [test harness overview](testing/readme.md)
- [go harness architecture](testing/go_harness.md)
- [unit tests](testing/go_harness.md#1-unit-tests-go-code)
- [integration tests](testing/go_harness.md#2-integration-tests-go-code)
- [performance tests](testing/go_harness.md#4-performance-tests)
- [chaos tests](testing/go_harness.md#5-chaos-tests)

---

## by day (course structure)

### day 1: development tools

- [main readme](readme.md) - system overview
- [architecture overview](architecture/architecture.md) - high-level design
- [services overview](services/readme.md) - what we're building
- [instructor guide](operations/instructor_guide.md#day-1-development-tools)

### day 2: debugging

- [s1: ingestion service](services/s1_ingestion.md) - service structure
- [troubleshooting](operations/troubleshooting.md) - debugging techniques
- [instructor guide](operations/instructor_guide.md#day-2-debugging)

### day 3: file i/o

- [s1: ingestion service](services/s1_ingestion.md) - complete spec
- [database schema](architecture/database.md) - sqlite integration
- [instructor guide](operations/instructor_guide.md#day-3-file-io)

### day 4: processes & networking

- [s2: aggregation service](services/s2_aggregation.md)
- [s3: query service](services/s3_query.md)
- [s4: discovery service](services/s4_discovery.md)
- [c1: cli client](services/c1_cli.md)
- [binary protocol](protocols/binary_protocol.md)
- [instructor guide](operations/instructor_guide.md#day-4-processes--networking)

### day 5: threading & containers

- [s3: query service](services/s3_query.md) - epoll and threads
- [deployment guides](deployment/readme.md)
- [testing guide](testing/readme.md) - go test harness
- [go harness spec](testing/go_harness.md) - full testing framework
- [instructor guide](operations/instructor_guide.md#day-5-threading--containers)

---

## reference tables

### all services

| service | doc | protocols | database | day |
|---------|-----|-----------|----------|-----|
| s1 ingestion | [link](services/s1_ingestion.md) | files, posix mq | write | 2-3 |
| s2 aggregation | [link](services/s2_aggregation.md) | posix mq | write | 4 |
| s3 query | [link](services/s3_query.md) | tcp, http | read | 4-5 |
| s4 discovery | [link](services/s4_discovery.md) | udp, tcp | read/write | 4-5 |
| c1 cli | [link](services/c1_cli.md) | tcp | none | 4 |

### all configuration files

| file | service | doc |
|------|---------|-----|
| s1_ingestion.ini | s1 | [link](operations/configuration.md#s1-ingestion-service) |
| s2_aggregation.ini | s2 | [link](operations/configuration.md#s2-aggregation-service) |
| s3_query.ini | s3 | [link](operations/configuration.md#s3-query-service) |
| s4_discovery.ini | s4 | [link](operations/configuration.md#s4-discovery-service) |
| cli.ini | c1 | [link](operations/configuration.md#c1-cli-client) |

### key topics

| topic | documentation |
|-------|---------------|
| build system | [day 1 guide](operations/instructor_guide.md#day-1-development-tools) |
| signal handling | [day 2 guide](operations/instructor_guide.md#day-2-debugging) |
| mmap | [s1 spec](services/s1_ingestion.md#mmap-implementation) |
| sqlite | [database doc](architecture/database.md) |
| fork() | [s2 spec](services/s2_aggregation.md#worker-pool-architecture) |
| sockets | [s3 spec](services/s3_query.md#epoll-event-loop) |
| mtls | [s4 spec](services/s4_discovery.md#mtls-handshake-flow) |
| pthreads | [s3 spec](services/s3_query.md#thread-pool) |
| epoll | [s3 spec](services/s3_query.md#epoll-event-loop) |
| docker | [deployment](deployment/readme.md#docker-compose) |
| kubernetes | [deployment](deployment/readme.md#kubernetes) |

---

## search by keywords

### performance
- [ingestion throughput](services/s1_ingestion.md#performance-characteristics)
- [query latency](services/s3_query.md#performance-characteristics)
- [performance tests](testing/readme.md#performance-tests)
- [performance tuning](architecture/database.md#performance-tuning)

### security
- [mtls](services/s4_discovery.md#security-architecture)
- [certificate management](operations/readme.md#certificate-management)
- [security audit](operations/readme.md#security-audit)

### reliability
- [high availability](architecture/architecture.md#high-availability-architecture)
- [leader election](services/s4_discovery.md#bully-leader-election)
- [chaos testing](testing/readme.md#6-chaos-tests)
- [backup/recovery](operations/readme.md#backup-and-recovery)

### debugging
- [troubleshooting](operations/troubleshooting.md)
- [gdb](operations/troubleshooting.md#debug-mode)
- [valgrind](operations/troubleshooting.md#valgrind-memory-debugging)
- [core dumps](operations/troubleshooting.md#enable-core-dumps)

### testing
- [go test harness](testing/go_harness.md)
- [contract validation](testing/go_harness.md#contract-validation)
- [mock services](testing/go_harness.md#mock-services)
- [ci/cd integration](testing/go_harness.md#cicd-integration)

---

## document statistics

- **total documents**: 18
- **architecture docs**: 3
- **service specs**: 6
- **protocol docs**: 1
- **operations docs**: 4
- **deployment docs**: 1
- **testing docs**: 2
- **estimated reading time**: ~10 hours

---

## contributing

when adding documentation:

1. update this index
2. follow existing structure
3. use code blocks for examples
4. include diagrams where helpful
5. link to related docs

---

*last updated: 2024*
*version: 1.0*
