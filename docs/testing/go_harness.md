# test harness architecture

## overview

the test harness is a comprehensive go-based testing framework for the weather station microservices system. it validates service contracts, runs performance benchmarks, executes chaos tests, and provides detailed grading.

## technology stack

- **language**: go 1.21+
- **cli framework**: cobra + viper
- **testing**: go test + custom framework
- **containers**: testcontainers-go (fresh db per test)
- **protocol testing**: fuzzing + property-based
- **reporting**: console, junit xml, html
- **ci/cd**: github actions and gitlab ci integration

## architecture

```
test-harness/
├── cmd/harness/                 # cobra cli entry
│   └── main.go
├── pkg/
│   ├── cmd/                     # cli commands
│   │   ├── validate.go
│   │   ├── test.go
│   │   ├── benchmark.go
│   │   ├── chaos.go
│   │   ├── grade.go
│   │   ├── mock.go
│   │   ├── ci.go
│   │   └── retrieve.go
│   │
│   ├── data/                    # noaa data retrieval
│   │   ├── noaa.go             # noaa ghcn-daily client
│   │   ├── downloader.go       # parallel downloads
│   │   ├── parser.go           # csv parsing/validation
│   │   └── cache.go            # local caching
│   │
│   ├── config/                  # viper configuration
│   │   ├── loader.go
│   │   └── types.go
│   │
│   ├── contracts/               # yaml contract parsing
│   │   ├── loader.go
│   │   ├── validator.go
│   │   └── types.go
│   │
│   ├── services/                # service lifecycle
│   │   ├── manager.go           # start/stop/health
│   │   ├── health.go
│   │   └── discovery.go
│   │
│   ├── testcontainers/          # docker test env
│   │   ├── database.go          # fresh sqlite per test
│   │   ├── network.go
│   │   └── volume.go
│   │
│   ├── protocol/                # binary protocol testing
│   │   ├── fuzz.go              # random mutation
│   │   ├── property.go          # property-based
│   │   └── versions/            # all protocol versions
│   │       ├── v1/
│   │       │   ├── encoder.go
│   │       │   └── decoder.go
│   │       └── v2/
│   │
│   ├── mocks/                   # mock services (per-test fresh)
│   │   ├── mock_s1.go
│   │   ├── mock_s2.go
│   │   ├── mock_s3.go
│   │   ├── mock_s4.go
│   │   └── server.go
│   │
│   ├── database/                # sqlite validation
│   │   ├── checker.go
│   │   └── fixtures.go
│   │
│   ├── performance/             # benchmarks
│   │   ├── throughput.go
│   │   ├── latency.go
│   │   └── load.go
│   │
│   ├── chaos/                   # chaos engineering
│   │   ├── network.go           # partition/latency
│   │   ├── resources.go         # cpu/memory
│   │   └── failure.go           # kill services
│   │
│   ├── grading/                 # score calculation
│   │   ├── calculator.go
│   │   ├── categories.go
│   │   └── reporter.go
│   │
│   ├── reporters/               # output formats
│   │   ├── console.go
│   │   ├── junit.go
│   │   └── html.go
│   │
│   └── github/                  # github integration
│       ├── client.go
│       └── comments.go
│
├── api/testlib/                 # importable test api
│   └── testlib.go
│
├── contracts/                   # yaml contracts
│   ├── s1_contract.yaml
│   ├── s2_contract.yaml
│   ├── s3_contract.yaml
│   ├── s4_contract.yaml
│   ├── c1_contract.yaml
│   └── protocol_spec.yaml
│
├── testdata/                    # test fixtures
│   ├── csv/
│   ├── config/
│   └── certs/
│
├── custom_tests/                # yaml custom tests
│   └── *.yaml
│
├── dockerfile                   # container image
├── k8s-cronjob.yaml            # kubernetes cronjob
└── go.mod
```

## cli specification

### global flags

```bash
test-harness [command] [flags]

global flags:
  -c, --config string       config file path (default "./config.yaml")
      --log-level string    log level (debug|info|warn|error)
      --parallel int        parallelism level (default 4)
      --output string       output format (console|json|junit|html)
      --verbose             detailed output
      --fail-fast           stop on first failure
```

### commands

#### 1. validate - check service against contract

```bash
test-harness validate --service s1_ingestion --contract contracts/s1_contract.yaml

validates that s1_ingestion binary exists and meets contract requirements:
- signal handling (sigterm, sighup)
- posix mq interface
- database operations
- performance characteristics
```

#### 2. test - run test suites

```bash
test-harness test [flags]

flags:
      --service strings    specific services (default all)
      --suite strings      test suites (unit|integration|performance|chaos)
      --test string        single test by name
      --parallel int       parallelism (default 4)
      --watch              stream results in real-time
      --use-mocks strings  use mocks for services (s3,s4)

examples:
  # run all tests
  test-harness test

  # test only s1 with mocks for dependencies
  test-harness test --service s1_ingestion --use-mocks s3,s4

  # run specific test suite with 8 parallel workers
  test-harness test --suite performance --parallel 8

  # watch tests run in real-time
  test-harness test --watch
```

#### 3. benchmark - performance testing

```bash
test-harness benchmark [flags]

flags:
      --duration duration   test duration (default 5m)
      --load int           concurrent clients (default 100)
      --target string      target service

examples:
  # benchmark s1 ingestion for 10 minutes
  test-harness benchmark --target s1 --duration 10m

  # benchmark with 1000 concurrent query clients
  test-harness benchmark --target s3 --load 1000
```

#### 4. chaos - chaos engineering

```bash
test-harness chaos [flags]

flags:
      --scenario string    predefined scenario
      --duration duration  chaos duration
      --target string      target service

scenarios:
  - leader_failover: kill leader, verify election
  - network_partition: partition network between nodes
  - resource_pressure: cpu/memory pressure
  - cascading_failure: multiple service failures

examples:
  # run leader failover scenario
  test-harness chaos --scenario leader_failover --duration 60s

  # custom chaos with network partition
  test-harness chaos --target s4 --action partition --duration 30s
```

#### 5. grade - calculate final score

```bash
test-harness grade [flags]

flags:
      --student string    student identifier
      --detailed          very detailed output
      --format string     output format (console|json|html)

examples:
  # grade with detailed console output
  test-harness grade --student station-1 --detailed

  # generate html report
  test-harness grade --format html --output ./reports/

  # json output for ci/cd
  test-harness grade --format json > grade.json
```

#### 6. mock - run mock services

```bash
test-harness mock [flags]

flags:
      --services strings  which mocks to start
      --ports ints       port assignments
      --persist          keep running until sigterm

examples:
  # start mocks for s3 and s4
  test-harness mock --services s3,s4

  # start all mocks on specific ports
  test-harness mock --services s1,s2,s3,s4 --ports 9001,9002,9003,9004
```

#### 7. ci - full ci pipeline

```bash
test-harness ci [flags]

flags:
      --github-token string     github token for pr comments
      --fail-threshold int      minimum score to pass (default 80)
      --pr-number int           pull request number
      --repo string             repository (owner/repo)

examples:
  # run full ci pipeline
  test-harness ci --github-token $token --fail-threshold 80

  # post results to specific pr
  test-harness ci --pr-number 42 --repo owner/repo
```

#### 8. retrieve - download weather data

```bash
test-harness retrieve [flags]

description:
  download weather data from noaa ghcn-daily dataset
  https://www.ncei.noaa.gov/data/global-historical-climatology-network-daily/access/

flags:
      --station string     station id (e.g., usw00014739)
      --country string     country code (e.g., us, de, gb)
      --lat float          latitude for radius search
      --lon float          longitude for radius search
      --radius float       search radius in km (default 50)
      --start date         start date (yyyy-mm-dd)
      --end date           end date (yyyy-mm-dd)
      --limit int          max records to download (default 10000)
      --output string      output directory (default ./data/csv)
      --format string      output format (csv|json)
      --parallel int       parallel downloads (default 4)
      --cache              use local cache
      --cache-dir string   cache directory (default ~/.cache/ws-test/noaa)

data sources:
  - noaa ghcn-daily: global historical climatology network daily
  - includes: temperature, precipitation, wind, pressure
  - coverage: 1750-present, 100k+ stations worldwide

examples:
  # retrieve data for specific station
  test-harness retrieve --station usw00014739 --start 2020-01-01 --end 2023-12-31

  # find stations near location and retrieve data
  test-harness retrieve --lat 52.5200 --lon 13.4050 --radius 100 --limit 50000

  # retrieve all german stations
  test-harness retrieve --country de --start 2022-01-01 --limit 100000

  # parallel download with caching
  test-harness retrieve --country us --parallel 8 --cache --output ./training-data/

  # get small dataset for testing (5 stations, 1 year)
  test-harness retrieve --limit 10000 --start 2024-01-01 --end 2024-12-31

output format:
  csv columns: station_id, date, temperature_max, temperature_min, 
               temperature_avg, precipitation, wind_speed, pressure
```

## configuration

### config.yaml structure

```yaml
global:
  timeout: 30s
  retries: 3
  parallel: 4
  
  output:
    format: console
    verbose: true
    colors: true
    timestamp: true

  logging:
    level: info
    file: ./test-harness.log

services:
  s1_ingestion:
    binary: ./services/s1_ingestion/ws-ingest
    config: ./config/s1.ini
    timeout: 60s
    health_check:
      type: process
      endpoint: ""
      interval: 5s
      retries: 3
    resources:
      memory_max: 2g
      cpu_max: 2.0
  
  s2_aggregation:
    binary: ./services/s2_aggregation/ws-aggregate
    config: ./config/s2.ini
    depends_on: [s1_ingestion]
  
  s3_query:
    binary: ./services/s3_query/ws-query
    config: ./config/s3.ini
    ports:
      query: 8080
      metrics: 9090
    health_check:
      type: http
      endpoint: http://localhost:9090/health
  
  s4_discovery:
    binary: ./services/s4_discovery/ws-discovery
    config: ./config/s4.ini
    network_mode: host
    ports:
      beacon: 5000
      health: 5001

testcontainers:
  enabled: true
  provider: docker
  
  database:
    driver: sqlite3
    fresh_per_test: true
    temp_dir: /tmp/ws-test-db

protocol:
  versions: [v1, v2]
  fuzz_iterations: 1000
  property_tests: 100

chaos:
  scenarios:
    leader_failover:
      description: "kill leader and verify election"
      duration: 60s
      steps:
        - action: kill_service
          target: leader
        - action: wait
          duration: 30s
        - action: assert_sql
          query: "select station_id from peer_stations where is_leader=1"
          expected: "not_empty"
    
    network_partition:
      description: "partition network between nodes"
      duration: 30s
      steps:
        - action: partition_network
          target: station-2
        - action: wait
          duration: 15s
        - action: assert_peer_status
          target: station-2
          expected: unhealthy

grading:
  must_pass:
    - compilation
    - basic_functionality
  
  categories:
    compilation:
      weight: 10
      criteria:
        no_warnings:
          points: 5
          check: gcc -wall -wextra -werror clean
        no_errors:
          points: 5
          check: builds successfully
    
    functionality:
      weight: 40
      criteria:
        s1_ingests_csv:
          points: 10
          test: test_s1_basic_ingest
        s2_aggregates:
          points: 10
          test: test_s2_hourly_stats
        s3_responds:
          points: 10
          test: test_s3_query_response
        s4_discovers:
          points: 5
          test: test_s4_peer_discovery
        c1_works:
          points: 5
          test: test_c1_end_to_end
    
    performance:
      weight: 30
      criteria:
        ingest_100mbps:
          points: 10
          target: 100
          unit: mb/s
        query_latency_10ms:
          points: 10
          target: 10
          unit: ms
        concurrent_100:
          points: 10
          target: 100
          unit: clients
    
    reliability:
      weight: 20
      criteria:
        signal_handling:
          points: 5
          test: test_signals
        graceful_shutdown:
          points: 5
          test: test_shutdown
        no_memory_leaks:
          points: 5
          test: test_valgrind_clean
        ha_failover:
          points: 5
          test: test_leader_failover

  thresholds:
    distinction: 90
    merit: 80
    pass: 60

reporters:
  console:
    colors: true
    progress_bar: true
    live_output: true
  
  junit:
    output_file: ./reports/junit.xml
    include_stdout: true
  
  html:
    output_dir: ./reports/html
    include_charts: true
    include_logs: true

github:
  enabled: true
  post_pr_comments: true
  comment_template: |
    ## weather station test results
    
    **student**: {{.student}}
    **score**: {{.score}}/100 ({{.grade}})
    
    ### breakdown
    {{range .categories}}
    - **{{.name}}**: {{.score}}/{{.max_score}} {{if .passed}}✓{{else}}✗{{end}}
    {{end}}
    
    ### detailed feedback
    {{range .failures}}
    - **{{.test}}**: {{.reason}}
      - **fix**: {{.remediation}}
    {{end}}
    
    [full report]({{.report_url}})
```

## test types

### 1. unit tests (go code)

```go
// tests/unit/s1/signal_test.go
package s1_test

import (
    "testing"
    "syscall"
    "time"
    
    "weather-station-test/pkg/services"
    "weather-station-test/pkg/testcontainers"
)

func tests1sigterm(t *testing.t) {
    // fresh container per test
    ctx := context.background()
    db := testcontainers.freshdatabase(ctx, t)
    defer db.terminate()
    
    // start service
    svc, err := services.start("s1_ingestion",
        services.withdatabase(db),
        services.withconfig("minimal"),
    )
    require.noerror(t, err)
    defer svc.cleanup()
    
    // send sigterm
    err = svc.signal(syscall.sigterm)
    require.noerror(t, err)
    
    // wait for graceful shutdown
    select {
    case <-svc.done():
        // success
    case <-time.after(30 * time.second):
        t.fatal("service did not stop within 30s")
    }
    
    // verify exit code
    assert.equal(t, 0, svc.exitcode())
}
```

### 2. integration tests (go code)

```go
// tests/integration/query_after_ingest_test.go
package integration_test

func testendtoendpipeline(t *testing.t) {
    ctx := context.background()
    
    // start full stack with fresh containers
    stack, err := services.startstack(ctx,
        services.withdatabase(testcontainers.freshdatabase(ctx, t)),
    )
    require.noerror(t, err)
    defer stack.terminate()
    
    // 1. ingest data
    fixture.copy("small.csv", stack.csvdir())
    time.sleep(5 * time.second)
    
    // 2. wait for aggregation
    time.sleep(10 * time.second)
    
    // 3. query via http
    resp, err := http.get("http://localhost:8080/query?from=2024-01-01")
    require.noerror(t, err)
    
    // 4. validate
    var result queryresult
    json.newdecoder(resp.body).decode(&result)
    assert.greater(t, len(result.records), 0)
}
```

### 3. custom tests (yaml)

```yaml
# custom_tests/verify_data_integrity.yaml
name: verify data integrity after failover
description: ensure no data loss during leader failover

targets:
  - station-1
  - station-2
  - station-3

setup:
  - action: start_stack
    services: [s1, s2, s3, s4]
  - action: wait
    duration: 10s
  - action: verify_leader_elected

steps:
  - name: ingest initial data
    action: copy_fixture
    file: medium.csv
    
  - name: wait for replication
    action: wait
    duration: 30s
    
  - name: kill leader
    action: kill_service
    target: leader
    
  - name: wait for failover
    action: wait
    duration: 30s
    
  - name: verify new leader
    action: assert_sql
    query: "select count(*) from peer_stations where is_leader=1"
    expected: "1"
    
  - name: verify data integrity
    action: assert_sql_all_nodes
    query: "select count(*) from weather_data"
    expected_consistent: true

assertions:
  - type: record_count
    min: 10000
  - type: data_integrity
    checksum_match: true
```

### 4. performance tests

```go
// tests/performance/throughput_test.go
package performance_test

func benchmarkingestthroughput(b *testing.b) {
    ctx := context.background()
    
    for i := 0; i < b.n; i++ {
        b.stoptimer()
        
        // fresh db for each iteration
        db := testcontainers.freshdatabase(ctx, b)
        svc := muststartservice(ctx, "s1_ingestion", db)
        
        // generate 5gb csv
        csvpath := generatecsv(5 * 1024) // 5gb
        
        b.starttimer()
        
        // copy file and wait
        copyfile(csvpath, svc.csvdir())
        waitforcompletion(svc, 5*time.minute)
        
        b.stoptimer()
        
        // calculate throughput
        duration := b.elapsed()
        throughput := float64(5*1024) / duration.seconds()
        b.reportmetric("throughput_mb/s", throughput)
        
        // cleanup
        svc.cleanup()
        db.terminate()
    }
}

func benchmarkquerylatency(b *testing.b) {
    ctx := context.background()
    
    // setup once
    db := testcontainers.freshdatabase(ctx, b)
    svc := muststartservice(ctx, "s3_query", db)
    loaddata(db, 1000000) // 1m records
    
    b.resettimer()
    
    // run parallel queries
    b.runparallel(func(pb *testing.pb) {
        for pb.next() {
            start := time.now()
            queryrandomtimerange()
            latency := time.since(start)
            b.reportmetric("latency_ms", float64(latency.milliseconds()))
        }
    })
}
```

### 5. chaos tests

```go
// tests/chaos/leader_failover_test.go
package chaos_test

func testleaderfailover(t *testing.t) {
    ctx := context.background()
    
    // start 3 stations
    stations := make([]*services.station, 3)
    for i := 0; i < 3; i++ {
        stations[i] = services.startstation(ctx, i+1)
    }
    
    // wait for election
    time.sleep(10 * time.second)
    
    // find leader
    leader := findleader(stations)
    require.notnil(t, leader)
    oldleaderid := leader.id()
    
    // inject chaos: kill leader
    t.logf("killing leader %d", oldleaderid)
    leader.kill()
    
    // wait for failover
    time.sleep(30 * time.second)
    
    // verify new leader elected
    newleader := findleader(stations)
    require.notnil(t, newleader)
    assert.notequal(t, oldleaderid, newleader.id())
    
    // verify system still functional
    err := ingestsampledata(stations[0])
    require.noerror(t, err)
}

func testnetworkpartition(t *testing.t) {
    ctx := context.background()
    
    stations := start3stations(ctx)
    defer stopall(stations)
    
    // partition station-2
    chaos.partition(stations[1])
    
    // wait
    time.sleep(15 * time.second)
    
    // verify marked unhealthy
    status := stations[0].querypeerstatus(2)
    assert.false(t, status.ishealthy)
    
    // heal partition
    chaos.heal(stations[1])
    
    // wait for recovery
    time.sleep(15 * time.second)
    
    // verify recovered
    status = stations[0].querypeerstatus(2)
    assert.true(t, status.ishealthy)
}
```

## mock services

per-test fresh mocks:

```go
// pkg/mocks/mock_s3.go
package mocks

type mockqueryservice struct {
    t        *testing.t
    port     int
    server   *http.server
    data     map[string][]weatherrecord
    mu       sync.rwmutex
    latency  time.duration
    errorrate float64
}

func newmockqueryservice(t *testing.t) *mockqueryservice {
    m := &mockqueryservice{
        t:        t,
        data:     make(map[string][]weatherrecord),
        port:     getfreeport(),
        latency:  0,
        errorrate: 0,
    }
    
    mux := http.newservemux()
    mux.handlefunc("/query", m.handlequery)
    mux.handlefunc("/health", m.handlehealth)
    
    m.server = &http.server{
        addr:    fmt.sprintf(":%d", m.port),
        handler: mux,
    }
    
    go m.server.listenandserve()
    
    // wait for ready
    waitfortcp(m.port, 5*time.second)
    
    return m
}

func (m *mockqueryservice) reset() {
    m.mu.lock()
    defer m.mu.unlock()
    m.data = make(map[string][]weatherrecord)
}

func (m *mockqueryservice) stop() {
    ctx, cancel := context.withtimeout(context.background(), 5*time.second)
    defer cancel()
    m.server.shutdown(ctx)
}

func (m *mockqueryservice) setlatency(d time.duration) {
    m.latency = d
}

func (m *mockqueryservice) seterrorrate(rate float64) {
    m.errorrate = rate
}

func (m *mockqueryservice) handlequery(w http.responsewriter, r *http.request) {
    // simulate latency
    if m.latency > 0 {
        time.sleep(m.latency)
    }
    
    // simulate errors
    if rand.float64() < m.errorrate {
        http.error(w, "internal error", http.statusinternalservererror)
        return
    }
    
    // return mock data
    m.mu.rlock()
    records := m.data["default"]
    m.mu.runlock()
    
    json.newencoder(w).encode(queryresponse{records: records})
}
```

## grading system

```go
// pkg/grading/calculator.go
package grading

type calculator struct {
    config *config.gradingconfig
}

func (c *calculator) calculate(results testresults) (*grade, error) {
    grade := &grade{
        student:   results.student,
        timestamp: time.now(),
        categories: make([]categoryscore, 0),
    }
    
    // check must-pass criteria
    for _, criteria := range c.config.mustpass {
        if !results.haspassed(criteria) {
            return nil, fmt.errorf("must-pass criteria failed: %s", criteria)
        }
    }
    
    // calculate category scores
    for name, category := range c.config.categories {
        score := c.calculatecategory(category, results)
        grade.categories = append(grade.categories, categoryscore{
            name:   name,
            score:  score,
            max:    category.weight,
            passed: score >= category.weight*0.6,
        })
        grade.totalscore += score
    }
    
    // determine letter grade
    grade.lettergrade = c.determinegrade(grade.totalscore)
    
    // generate detailed feedback
    grade.feedback = c.generatefeedback(results)
    
    return grade, nil
}

func (c *calculator) generatefeedback(results testresults) []feedback {
    var feedbacks []feedback
    
    for _, failure := range results.failures {
        fb := feedback{
            test:        failure.testname,
            category:    failure.category,
            reason:      failure.reason,
            remediation: c.suggestfix(failure),
        }
        feedbacks = append(feedbacks, fb)
    }
    
    return feedbacks
}
```

## ci/cd integration

### github actions workflow

```yaml
# .github/workflows/test.yml
name: weather station tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    
    services:
      docker:
        image: docker:20-dind
        options: >-
          --privileged
          --cgroupns=host
    
    steps:
    - uses: actions/checkout@v3
    
    - name: setup go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: build test harness
      run: |
        cd test-harness
        go build -o bin/test-harness ./cmd/harness
    
    - name: run contract validation
      run: |
        ./test-harness/bin/test-harness validate \
          --config ./test-harness/config.yaml
    
    - name: run all tests
      run: |
        ./test-harness/bin/test-harness ci \
          --github-token ${{ secrets.github_token }} \
          --fail-threshold 80 \
          --pr-number ${{ github.event.pull_request.number }} \
          --repo ${{ github.repository }}
    
    - name: upload test results
      uses: actions/upload-artifact@v3
      with:
        name: test-results
        path: |
          ./reports/junit.xml
          ./reports/html/
```

### gitlab ci

```yaml
# .gitlab-ci.yml
stages:
  - build
  - test
  - report

variables:
  go_version: "1.21"
  fail_threshold: "80"

before_script:
  - apt-get update && apt-get install -y sqlite3 docker.io
  - go version

test-harness:build:
  stage: build
  image: golang:${go_version}
  script:
    - cd test-harness
    - go mod download
    - go build -ldflags="-w -s" -o bin/test-harness ./cmd/harness
  artifacts:
    paths:
      - test-harness/bin/test-harness
    expire_in: 1 hour

test-harness:validate:
  stage: test
  image: golang:${go_version}
  needs: [test-harness:build]
  script:
    - ./test-harness/bin/test-harness validate --config ./test-harness/config.yaml

test-harness:unit:
  stage: test
  image: golang:${go_version}
  needs: [test-harness:build]
  script:
    - ./test-harness/bin/test-harness test --suite unit --parallel 4
  artifacts:
    reports:
      junit: reports/junit.xml
    paths:
      - reports/
    when: always

test-harness:integration:
  stage: test
  image: golang:${go_version}
  services:
    - docker:20-dind
  variables:
    docker_host: tcp://docker:2376
    docker_tls_certdir: "/certs"
  needs: [test-harness:build]
  script:
    - ./test-harness/bin/test-harness test --suite integration --parallel 4
  artifacts:
    reports:
      junit: reports/junit.xml
    paths:
      - reports/
    when: always

test-harness:performance:
  stage: test
  image: golang:${go_version}
  services:
    - docker:20-dind
  needs: [test-harness:build]
  script:
    - ./test-harness/bin/test-harness benchmark --duration 5m
  only:
    - main
    - merge_requests
  artifacts:
    paths:
      - reports/
    when: always

test-harness:grade:
  stage: report
  image: golang:${go_version}
  needs: 
    - test-harness:unit
    - test-harness:integration
  script:
    - |
      ./test-harness/bin/test-harness grade \
        --format json \
        --detailed \
        --student "${ci_commit_author}" > grade.json
      
      score=$(cat grade.json | jq -r '.totalscore')
      grade=$(cat grade.json | jq -r '.lettergrade')
      
      echo "score: ${score}/100 (${grade})"
      
      if [ "${score}" -lt "${fail_threshold}" ]; then
        echo "failed: score below ${fail_threshold}"
        exit 1
      fi
  artifacts:
    reports:
      junit: reports/junit.xml
    paths:
      - grade.json
      - reports/
    when: always
  only:
    - merge_requests

# post results as merge request comment
test-harness:comment:
  stage: report
  image: alpine/curl:latest
  needs: [test-harness:grade]
  variables:
    gitlab_token: ${gitlab_api_token}
  script:
    - |
      score=$(cat grade.json | jq -r '.totalscore')
      grade=$(cat grade.json | jq -r '.lettergrade')
      
      comment="## weather station test results\n\n"
      comment+="**student**: ${ci_commit_author}\n"
      comment+="**score**: ${score}/100 (${grade})\n\n"
      comment+="see full report in job artifacts"
      
      curl --request post \
        --header "private-token: ${gitlab_token}" \
        --form "body=${comment}" \
        "${ci_api_v4_url}/projects/${ci_project_id}/merge_requests/${ci_merge_request_iid}/notes"
  only:
    - merge_requests
  when: always
```

### kubernetes cronjob

```yaml
# k8s-cronjob.yaml
apiversion: batch/v1
kind: cronjob
metadata:
  name: weather-station-test
spec:
  schedule: "0 */6 * * *"  # every 6 hours
  concurrencypolicy: forbid
  jobtemplate:
    spec:
      backofflimit: 2
      template:
        spec:
          containers:
          - name: test-harness
            image: weather-station/test-harness:latest
            imagepullpolicy: always
            command:
            - /usr/local/bin/test-harness
            - ci
            - --config=/config/test-config.yaml
            - --fail-threshold=80
            - --detailed
            env:
            - name: github_token
              valuefrom:
                secretkeyref:
                  name: github-token
                  key: token
            volumeMounts:
            - name: student-code
              mountpath: /code
              readonly: true
            - name: test-config
              mountpath: /config
              readonly: true
            - name: test-results
              mountpath: /results
            resources:
              requests:
                memory: "4gi"
                cpu: "2000m"
              limits:
                memory: "8gi"
                cpu: "4000m"
          volumes:
          - name: student-code
            persistentvolumeclaim:
              claimname: student-code-pvc
          - name: test-config
            configmap:
              name: test-harness-config
          - name: test-results
            emptydir: {}
          restartpolicy: never
```

## usage examples

### development workflow

```bash
# 1. student implements s1
cd services/s1_ingestion
vim main.c

# 2. validate against contract
cd ../../test-harness
go run ./cmd/harness validate --service s1_ingestion

# 3. run specific test
go run ./cmd/harness test --service s1_ingestion --test tests1basicingest

# 4. fix issues, repeat

# 5. run full test suite for s1
go run ./cmd/harness test --service s1_ingestion --parallel 4

# 6. check performance
go run ./cmd/harness benchmark --target s1 --duration 5m
```

### grading workflow

```bash
# grade single student
test-harness grade --student station-42 --detailed

# output:
# ========================================
# weather station - final assessment
# ========================================
# 
# student: station-42
# timestamp: 2024-02-01 14:30:00
# 
# score: 87/100 (merit)
# 
# breakdown:
#   ✓ compilation        10/10
#   ✓ functionality      38/40 (-2: s4 peer discovery intermittent)
#   ✓ performance        18/20 (-2: query latency 15ms avg)
#   ✓ reliability        15/15
#   ✓ code quality       13/15 (-2: missing function docs)
# 
# detailed feedback:
#   - test_s4_health_check: connection timeout after 5s
#     fix: increase health_check timeout_seconds in discovery.ini
#   
#   - test_query_latency_p99: measured 15ms, target 10ms
#     fix: add index on weather_data(timestamp, station_id)
#     run: sqlite3 weather.db "create index idx_perf on weather_data(timestamp, station_id)"
#   
#   - public functions missing documentation
#     fix: add doxygen comments to functions in s3/query.c
# 
# full report: reports/station-42-20240201.html
```

## installation

```bash
# clone repository
git clone <repo-url>
cd weather-station

# install dependencies
go mod download

# build test harness
cd test-harness
go build -o bin/test-harness ./cmd/harness

# run tests
./bin/test-harness --help
```

## docker image

```dockerfile
# dockerfile
golang:1.21-alpine as builder
workdir /build
copy go.mod go.sum ./
run go mod download
copy . .
run go build -ldflags="-w -s" -o test-harness ./cmd/harness

from alpine:3.18
run apk add --no-cache docker-cli sqlite
workdir /app
copy --from=builder /build/test-harness /usr/local/bin/
copy contracts/ ./contracts/
copy testdata/ ./testdata/
copy config.yaml .
entrypoint ["test-harness"]
```

---

*next: see [implementation guide](implementation.md) for building the test harness*.
