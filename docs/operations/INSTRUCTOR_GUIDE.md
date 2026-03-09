# instructor guide

## overview

this guide provides instructions for instructors teaching the lfd401 linux application development course using the weather station project.

## course structure

### day-by-day breakdown

| day | topic | focus | student deliverable |
|-----|-------|-------|---------------------|
| **1** | development tools | build system, libraries, git | project structure, makefile |
| **2** | debugging | gdb, valgrind, signals | signal handling, debug infrastructure |
| **3** | file i/o | mmap, inotify, sqlite | csv ingestion service |
| **4** | processes & networking | fork, sockets, mtls | query service, discovery |
| **5** | threading & containers | pthreads, epoll, k8s | complete system |

---

## teaching approach

### lecture format (50/50)

- **morning (4 hours)**: theory + demonstrations
- **afternoon (4 hours)**: hands-on lab work

### suggested daily schedule

```
09:00 - 09:30  review and q&a from previous day
09:30 - 11:00  lecture: theory and concepts
11:00 - 11:15  break
11:15 - 12:30  lecture: live coding demonstrations
12:30 - 13:30  lunch
13:30 - 15:00  lab: guided exercises
15:00 - 15:15  break
15:15 - 17:00  lab: independent work
17:00 - 17:30  wrap-up, showcase student work
```

---

## day 1: development tools

### learning objectives

by end of day 1, students will:
- understand gcc compilation process
- create makefiles for multi-directory projects
- use static and shared libraries
- implement proper git workflows

### lecture topics (3 hours)

**hour 1: compilers**
- compilation pipeline (preprocessor, compiler, assembler, linker)
- gcc optimization levels (-o0 through -o3)
- warning flags (-wall, -wextra, -werror)
- debug symbols (-g)

**hour 2: libraries**
- static vs dynamic linking
- creating and using .a and .so files
- library versioning and sonames
- link-time vs runtime resolution

**hour 3: build systems & git**
- makefile syntax and rules
- recursive make
- git branching strategies
- commit message conventions

### live coding demo

```bash
# demonstrate build system
mkdir -p weather-station/{lib,src,tests}
cd weather-station

# create library
# lib/csv.{c,h}
gcc -c -fpic lib/csv.c -o lib/csv.o
gcc -shared -o lib/libws_csv.so lib/csv.o

# create makefile
# show recursive make
# show pattern rules

# git workflow
git init
git add .
git commit -m "initial project structure"
git checkout -b feature/csv-parser
```

### lab exercise

**task**: create project structure with build system

**deliverables**:
1. directory structure matching reference
2. working makefile with all targets
3. git repository with initial commit
4. build provided libraries successfully

**success criteria**:
- `make all` completes without errors
- `make test` runs unit tests
- `make clean` removes build artifacts
- git log shows proper commits

### common student issues

| issue | cause | solution |
|-------|-------|----------|
| linker errors | wrong library order | show -l and -l flags |
| "undefined reference" | missing source files | check all objects linked |
| recursive make fails | missing dependencies | explain subdir order |
| git merge conflicts | concurrent edits | teach proper branching |

### assessment rubric

| criteria | points | description |
|----------|--------|-------------|
| structure | 20 | correct directory layout |
| makefile | 30 | all targets work, no warnings |
| libraries | 30 | properly linked and built |
| git | 20 | clean history, good messages |

---

## day 2: debugging

### learning objectives

by end of day 2, students will:
- use gdb for debugging
- analyze core dumps
- detect memory leaks with valgrind
- implement signal handling

### lecture topics (3 hours)

**hour 1: gdb**
- starting gdb
- breakpoints and watchpoints
- stack traces and backtraces
- debugging multi-threaded programs

**hour 2: core dumps & valgrind**
- generating core dumps (ulimit, gcore)
- post-mortem debugging
- memory leak detection
- profiling with cachegrind

**hour 3: signals**
- signal types and handlers
- reentrancy and async-signal-safety
- sigaction() vs signal()
- real-time signals

### live coding demo

```bash
# gdb demonstration
gcc -g -o test_debug test.c
gdb ./test_debug
(gdb) break main
(gdb) run
(gdb) next
(gdb) print variable
(gdb) backtrace
(gdb) continue

# valgrind demonstration
valgrind --leak-check=full ./test_debug

# signal handling demo
# show sigterm, sighup, sigusr1 handlers
```

### lab exercise

**task**: add debugging infrastructure to ingestion service

**deliverables**:
1. signal handlers for sigterm, sighup
2. core dump configuration
3. valgrind-clean code
4. logging to syslog

**success criteria**:
- service handles sigterm gracefully
- sighup reloads configuration
- valgrind reports "no leaks possible"
- logs visible in journalctl

### common student issues

| issue | cause | solution |
|-------|-------|----------|
| core dumps not generated | ulimit too low | show ulimit -c unlimited |
| "no stack" in gdb | compiled without -g | rebuild with -g flag |
| signal handler crashes | non-async-safe function | use only safe functions |
| valgrind false positives | library issues | show suppression files |

---

## day 3: file i/o

### learning objectives

by end of day 3, students will:
- use mmap for efficient file access
- monitor files with inotify
- handle large files correctly
- integrate with sqlite

### lecture topics (3 hours)

**hour 1: file i/o**
- system calls vs library functions
- buffered vs unbuffered i/o
- vector i/o (readv/writev)
- large file support

**hour 2: memory mapping**
- mmap() and munmap()
- madvise() for optimization
- file-backed vs anonymous mappings
- memory-mapped i/o advantages

**hour 3: sqlite integration**
- database connection management
- prepared statements
- transactions
- wal mode

### live coding demo

```bash
# mmap demonstration
# show reading large file with mmap
# compare performance with read()

# inotify demonstration
# show watching directory
# handle in_close_write events

# sqlite demonstration
# show connection, prepared statements
# show wal mode
```

### lab exercise

**task**: implement csv ingestion service

**deliverables**:
1. streaming csv parser
2. inotify file watcher
3. sqlite integration
4. process 5gb test file

**success criteria**:
- files automatically ingested on arrival
- data correctly stored in sqlite
- throughput >100 mb/s
- memory usage <2gb

### common student issues

| issue | cause | solution |
|-------|-------|----------|
| mmap fails on large files | 32-bit system | use lfs or 64-bit |
| inotify not triggering | max watches exceeded | increase /proc/sys/fs/inotify/max_user_watches |
| sqlite "database locked" | concurrent access | enable wal mode |
| slow ingestion | no transactions | use batch commits |

---

## day 4: processes & networking

### learning objectives

by end of day 4, students will:
- use fork() for process creation
- implement tcp socket servers
- handle multiple clients
- add mtls security

### lecture topics (3 hours)

**hour 1: process management**
- fork() and exec() family
- process pools
- ipc with pipes and fifos
- daemonization

**hour 2: socket programming**
- tcp client/server model
- bind(), listen(), accept()
- byte ordering (htonl/ntohl)
- non-blocking i/o

**hour 3: security (mtls)**
- tls 1.3 handshake
- certificate management
- mutual authentication
- openssl programming

### live coding demo

```bash
# process pool demonstration
# show fork(), exec(), waitpid()

# socket server demonstration
# show blocking server
# show select() multiplexing

# mtls demonstration
# show certificate generation
# show openssl client/server
```

### lab exercise

**task**: implement query service and discovery

**deliverables**:
1. tcp query server (blocking)
2. udp discovery beacons
3. mtls integration
4. process pool for aggregation

**success criteria**:
- query server responds to requests
- discovery finds peer stations
- mtls handshake succeeds
- multiple workers process jobs

### common student issues

| issue | cause | solution |
|-------|-------|----------|
| zombie processes | no wait() | show proper signal handling |
| "address already in use" | time_wait | use so_reuseaddr |
| mtls handshake fails | wrong certificates | verify ca and cn |
| child process crashes | memory sharing | review fork() semantics |

---

## day 5: threading & containers

### learning objectives

by end of day 5, students will:
- use pthreads for concurrency
- implement epoll-based servers
- deploy with docker
- run on kubernetes

### lecture topics (3 hours)

**hour 1: threading**
- pthread_create() and pthread_join()
- mutexes and condition variables
- thread pools
- thread safety

**hour 2: advanced i/o**
- epoll vs select/poll
- edge-triggered vs level-triggered
- concurrent server patterns
- zero-copy i/o

**hour 3: containers**
- docker fundamentals
- docker compose
- kubernetes basics
- deployment patterns

### live coding demo

```bash
# thread pool demonstration
# show pthread_create, mutex, condvar

# epoll demonstration
# show epoll_create, epoll_ctl, epoll_wait

# docker demonstration
# build image, run container
# show docker-compose

# kubernetes demonstration
# deploy pods, service, ingress
```

### lab exercise

**task**: complete system and deploy

**deliverables**:
1. epoll-based query server
2. thread pool implementation
3. docker compose deployment
4. working end-to-end system

**success criteria**:
- all 5 services running
- can ingest, query, discover
- docker containers healthy
- kubernetes pods running

### common student issues

| issue | cause | solution |
|-------|-------|----------|
| deadlocks | lock ordering | show consistent ordering |
| epoll not triggering | edge-triggered mode | read until eagain |
| docker container exits | wrong command | check entrypoint |
| k8s pod crashloopbackoff | health check failing | check liveness probe |

---

## assessment

### daily checkpoints

**day 1**: review makefile with student
- can they build project?
- are there compiler warnings?
- is git history clean?

**day 2**: code review
- signal handlers present?
- valgrind clean?
- proper error handling?

**day 3**: performance test
- can they ingest test file?
- what throughput achieved?
- any data corruption?

**day 4**: integration test
- services communicate?
- discovery working?
- mtls configured?

**day 5**: final demonstration
- complete system working?
- docker deployment successful?
- kubernetes running?

### final evaluation

**pass criteria** (60%):
- all 5 services functional
- can ingest and query data
- basic error handling
- code compiles without warnings

**merit criteria** (80%):
- performance targets met
- complete mtls implementation
- docker deployment working
- clean code style

**distinction criteria** (95%):
- exceeds performance targets
- kubernetes deployment
- comprehensive tests
- exceptional code quality

### grading rubric

| component | weight | description |
|-----------|--------|-------------|
| functionality | 40% | services work, integration successful |
| code quality | 25% | style, comments, error handling |
| performance | 15% | meets or exceeds targets |
| documentation | 10% | readme, inline comments |
| deployment | 10% | docker/kubernetes working |

---

## common pitfalls

### student misconceptions

1. **"i'll copy the reference implementation"**
   - emphasize understanding over copying
   - use reference as guide, not source

2. **"error handling isn't important"**
   - show real-world crash scenarios
   - demonstrate security implications

3. **"optimization comes first"**
   - teach "make it work, then make it fast"
   - profile before optimizing

4. **"i don't need tests"**
   - show test-driven benefits
   - use test harness for validation

### instructor tips

1. **be patient with debugging**
   - memory issues take time to solve
   - guide rather than solve

2. **encourage experimentation**
   - let students try different approaches
   - discuss trade-offs

3. **use real data**
   - synthetic data doesn't show real issues
   - scale matters (gb vs mb)

4. **celebrate failures**
   - learning comes from debugging
   - share your own war stories

---

## resources

### recommended reading

- **the linux programming interface** - michael kerrisk
- **advanced programming in the unix environment** - stevens
- **unix network programming** - stevens

### online resources

- linux man pages (man7.org)
- sqlite documentation
- openssl wiki
- docker documentation
- kubernetes documentation

### tools

- gdb cheat sheet
- valgrind quick start
- make manual
- git reference

---

## backup plans

### if students fall behind

**day 3**: provide more complete skeleton
**day 4**: reduce scope (skip mtls)
**day 5**: focus on docker, skip kubernetes

### if students are ahead

**day 3**: add compression challenge
**day 4**: add http/2 or grpc
**day 5**: add monitoring stack

---

## post-course

### follow-up

- provide solutions after course
- create alumni slack/discord
- schedule office hours
- collect feedback for improvements

### continuous improvement

- track common issues
- update documentation
- refine test harness
- add new challenges

---

*good luck with your course!*
