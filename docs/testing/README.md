# testing & validation

## overview

the weather station system uses a comprehensive **go-based test harness** built with cobra and viper. it validates service contracts, runs performance benchmarks, executes chaos tests, and provides automatic grading with detailed feedback.

## architecture

the test harness is written in **go 1.21+** and provides:

- **contract validation** - verify services meet yaml-defined contracts
- **unit/integration testing** - go test framework with testcontainers
- **performance benchmarks** - concurrent load testing with goroutines
- **chaos engineering** - network partitions, failures, resource pressure
- **automatic grading** - pass/fail scoring with detailed remediation
- **ci/cd integration** - github actions, kubernetes cronjobs
- **multiple output formats** - console, junit xml, html reports

## quick start

```bash
# build test harness
cd test-harness
go build -o bin/test-harness ./cmd/harness

# retrieve real weather data from noaa
./bin/test-harness retrieve --country de --start 2024-01-01 --limit 10000

# validate service against contract
./bin/test-harness validate --service s1_ingestion

# run all tests
./bin/test-harness test

# run with mocks
./bin/test-harness test --service s1_ingestion --use-mocks s3,s4

# benchmark performance
./bin/test-harness benchmark --target s1 --duration 5m

# run chaos scenario
./bin/test-harness chaos --scenario leader_failover

# grade student submission
./bin/test-harness grade --student station-1 --detailed

# full ci pipeline (github)
./bin/test-harness ci --github-token $token --fail-threshold 80

# full ci pipeline (gitlab)
./bin/test-harness ci --gitlab-token $token --fail-threshold 80
```

## documentation

### detailed specification

see [go test harness specification](go_harness.md) for complete architecture:

- **cli commands** - all 7 commands with flags
- **configuration** - viper/yaml config structure
- **test types** - unit, integration, performance, chaos
- **mock services** - per-test fresh mocks
- **grading system** - automatic scoring with feedback
- **ci/cd** - github actions, kubernetes cronjobs
- **docker** - container image for testing

### test categories

| category | description | location |
|----------|-------------|----------|
| **contract** | yaml-defined service contracts | `contracts/` |
| **unit** | go tests for individual functions | `tests/unit/` |
| **integration** | go tests for service interactions | `tests/integration/` |
| **performance** | go benchmarks for throughput/latency | `tests/performance/` |
| **chaos** | resilience testing (failures, partitions) | `tests/chaos/` |
| **custom** | yaml-defined test scenarios | `custom_tests/` |

### cli commands

| command | description | example |
|---------|-------------|---------|
| `validate` | check service contract | `test-harness validate --service s1` |
| `test` | run test suites | `test-harness test --parallel 4` |
| `benchmark` | performance testing | `test-harness benchmark --duration 5m` |
| `chaos` | chaos engineering | `test-harness chaos --scenario leader_failover` |
| `grade` | calculate score | `test-harness grade --detailed` |
| `mock` | run mock services | `test-harness mock --services s3,s4` |
| `ci` | full ci pipeline | `test-harness ci --fail-threshold 80` |
| `retrieve` | download noaa weather data | `test-harness retrieve --country de --limit 10000` |

### key features

1. **noaa data retrieval** - download real weather data from ghcn-daily
2. **testcontainers** - fresh sqlite database per test
3. **parallel execution** - configurable goroutine parallelism
4. **protocol fuzzing** - random byte mutations + property-based
5. **version testing** - all protocol versions (v1, v2, etc.)
6. **detailed feedback** - specific remediation for failures
7. **github/gitlab integration** - pr comments with results
8. **kubernetes** - runs as cronjob for continuous testing

## directory structure

```
test-harness/
├── cmd/harness/              # cobra cli entry
├── pkg/
│   ├── cmd/                  # cli commands (validate, test, etc.)
│   ├── config/               # viper configuration
│   ├── contracts/            # yaml contract parsing
│   ├── services/             # service lifecycle management
│   ├── testcontainers/       # docker test environment
│   ├── protocol/             # binary protocol testing
│   │   └── versions/         # v1, v2, etc.
│   ├── mocks/                # mock services (per-test fresh)
│   ├── database/             # sqlite validation
│   ├── performance/          # benchmarks
│   ├── chaos/                # chaos engineering
│   ├── grading/              # score calculation
│   ├── reporters/            # output formats
│   └── github/               # github api integration
├── api/testlib/              # importable test api
├── contracts/                # yaml contracts
├── tests/                    # go test files
├── custom_tests/             # yaml custom tests
├── testdata/                 # fixtures (csv, certs, config)
├── dockerfile                # container image
├── k8s-cronjob.yaml         # kubernetes cronjob
└── go.mod
```

## configuration

```yaml
# config.yaml
global:
  timeout: 30s
  retries: 3
  parallel: 4
  
  output:
    format: console
    verbose: true
    colors: true

services:
  s1_ingestion:
    binary: ./services/s1_ingestion/ws-ingest
    config: ./config/s1.ini
    timeout: 60s

grading:
  must_pass: [compilation, basic_functionality]
  categories:
    compilation:
      weight: 10
    functionality:
      weight: 40
    performance:
      weight: 30
    reliability:
      weight: 20
  thresholds:
    distinction: 90
    merit: 80
    pass: 60
```

## grading output example

```
========================================
weather station - final assessment
========================================

student: station-42
timestamp: 2024-02-01 14:30:00

score: 87/100 (merit)

breakdown:
  ✓ compilation        10/10
  ✓ functionality      38/40 (-2: s4 peer discovery intermittent)
  ✓ performance        18/20 (-2: query latency 15ms avg)
  ✓ reliability        15/15
  ✓ code quality       13/15 (-2: missing function docs)

detailed feedback:
  - test_s4_health_check: connection timeout after 5s
    fix: increase health_check timeout_seconds in discovery.ini
  
  - test_query_latency_p99: measured 15ms, target 10ms
    fix: add index on weather_data(timestamp, station_id)
  
  - public functions missing documentation
    fix: add doxygen comments to functions in s3/query.c

full report: reports/station-42-20240201.html
```

## data retrieval

download real weather data from noaa global historical climatology network (ghcn-daily):

```bash
# download data for specific station
test-harness retrieve --station usw00014739 --start 2020-01-01 --end 2023-12-31

# find and download stations near location
test-harness retrieve --lat 52.5200 --lon 13.4050 --radius 100 --limit 50000

# download all german stations
test-harness retrieve --country de --start 2022-01-01

# parallel download with caching
test-harness retrieve --country us --parallel 8 --cache --output ./training-data/

# small dataset for testing
test-harness retrieve --limit 10000 --start 2024-01-01 --end 2024-12-31

# overnight loading with rate limiting (recommended for large datasets)
test-harness retrieve --country us --limit 100000 --rate-limit 0.5 --min-free-space 5.0 --cache
```

see [deployment guide](../deployment/readme.md#loading-noaa-weather-data) for detailed instructions on loading data into kubernetes.

**data source**: [noaa ghcn-daily](https://www.ncei.noaa.gov/data/global-historical-climatology-network-daily/access/)

**includes**: temperature, precipitation, wind, pressure
**coverage**: 1750-present, 100k+ stations worldwide

## kubernetes deployment testing

### validating kubernetes deployments

after deploying to kubernetes, use the test-harness to validate the deployment:

```bash
# grade the deployment
test-harness grade --detailed

# run integration tests against kubernetes
test-harness test --suite integration

# test specific services
test-harness test --service ingestion
test-harness test --service aggregation
test-harness test --service query
```

### expected results

**grading output:**
```
=== weather station - final assessment ===
student name: anonymous
score points: 21 total: 24
percentage value: 87.5%
letter_grade value: B+
status passed: true

breakdown:
  ✓ compilation: 10/10 (100.0%)
  ✗ code_quality: 0/10 (0.0%) - not evaluated
  ✓ functionality: 38/40 (95.0%)
  ✓ performance: 18/20 (90.0%)
  ✓ reliability: 15/15 (100.0%)
```

**pass criteria:**
- overall score: 70%+ (C- or higher)
- compilation: must pass (10/10)
- functionality: 35+/40
- reliability: 12+/15

### manual validation

```bash
# setup port forwards
kubectl port-forward svc/weather-station-ingestion 8081:8080 -n weather-station &
kubectl port-forward svc/weather-station-aggregation 8082:8080 -n weather-station &
kubectl port-forward svc/weather-station-query 8080:8080 -n weather-station &

# test health endpoints
curl http://localhost:8081/health  # ingestion
curl http://localhost:8082/health  # aggregation
curl http://localhost:8080/health  # query

# test data ingestion
curl http://localhost:8081/api/v1/stats

# test query api
curl http://localhost:8080/api/v1/stations
curl "http://localhost:8080/api/v1/weather/daily?station=USW00014739"
```

## ci/cd integration

### github actions

```yaml
name: test
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: build
      run: |
        cd test-harness
        go build -o bin/test-harness ./cmd/harness
    
    - name: run tests
      run: |
        ./test-harness/bin/test-harness ci \
          --github-token ${{ secrets.github_token }} \
          --fail-threshold 80 \
          --pr-number ${{ github.event.pull_request.number }}
```

### gitlab ci

```yaml
stages:
  - build
  - test
  - report

test-harness:build:
  stage: build
  image: golang:1.21
  script:
    - cd test-harness && go build -o bin/test-harness ./cmd/harness
  artifacts:
    paths:
      - test-harness/bin/test-harness

test-harness:test:
  stage: test
  image: golang:1.21
  needs: [test-harness:build]
  script:
    - ./test-harness/bin/test-harness ci \
        --gitlab-token $gitlab_api_token \
        --fail-threshold 80
  artifacts:
    reports:
      junit: reports/junit.xml
```

see [go_harness.md](go_harness.md) for full gitlab ci configuration.

### kubernetes cronjob

runs every 6 hours to continuously validate student submissions.

see [go_harness.md](go_harness.md) for full k8s manifest.

## development workflow

```bash
# 1. student implements service
vim services/s1_ingestion/main.c

# 2. validate against contract
test-harness validate --service s1_ingestion

# 3. run specific test
test-harness test --service s1_ingestion --test tests1basicingest

# 4. fix and iterate

# 5. run full suite
test-harness test --service s1_ingestion --parallel 4

# 6. check performance
test-harness benchmark --target s1
```

## mock services

per-test fresh mocks for isolated development:

```bash
# test cli against mock s3
test-harness test --service c1_cli --use-mocks s3,s4

# start mocks manually
test-harness mock --services s3,s4 --ports 9003,9004
```

mocks support:
- latency simulation
- error injection
- state reset between tests
- predefined responses

## custom yaml tests

instructors/students can add tests without recompiling:

```yaml
# custom_tests/verify_failover.yaml
name: verify data integrity after failover
targets: [station-1, station-2, station-3]

setup:
  - action: start_stack
  - action: wait
    duration: 10s

steps:
  - action: copy_fixture
    file: medium.csv
  - action: wait
    duration: 30s
  - action: kill_service
    target: leader
  - action: assert_sql
    query: "select count(*) from peer_stations where is_leader=1"
    expected: "1"
```

## installation

```bash
# clone
git clone <repo-url>
cd weather-station/test-harness

# install dependencies
go mod download

# build
go build -o bin/test-harness ./cmd/harness

# run
./bin/test-harness --help
```

## docker

```bash
# build image
docker build -t weather-station/test-harness .

# run tests in container
docker run --rm \
  -v $(pwd)/services:/services \
  -v $(pwd)/config:/config \
  weather-station/test-harness \
  test --config /config/test.yaml
```

## references

- **[detailed specification](go_harness.md)** - complete architecture, cli, config
- **[contracts](../contracts/)** - service contract definitions
- **[noaa ghcn-daily](https://www.ncei.noaa.gov/data/global-historical-climatology-network-daily/access/)** - weather data source
- **[github actions ci](../.github/workflows/test.yml)** - github actions example
- **[gitlab ci](../.gitlab-ci.yml)** - gitlab ci example

---

*the test harness is built with go, cobra, viper, and testcontainers-go.*
