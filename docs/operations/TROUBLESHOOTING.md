# troubleshooting guide

## overview

common issues and their solutions for the weather station system.

## quick diagnostic commands

```bash
# check all service status
systemctl status ws-ingestion ws-aggregation ws-query ws-discovery

# view recent logs
journalctl -u 'ws-*' --since "1 hour ago" --no-pager

# check ports
ss -tlnp | grep -e '8080|9090|5000|8443'

# test connectivity
curl http://localhost:9090/health

# check database
sqlite3 /var/lib/ws/weather.db "pragma integrity_check;"
```

---

## common issues

### services won't start

#### symptom
```
$ systemctl start ws-query
failed to start ws-query.service: unit not found.
```

#### solution
```bash
# 1. check if service file exists
ls -la /etc/systemd/system/ws-*.service

# 2. reload systemd
systemctl daemon-reload

# 3. enable service
systemctl enable /etc/systemd/system/ws-query.service

# 4. try again
systemctl start ws-query
```

### port already in use

#### symptom
```
[error] failed to bind to port 8080: address already in use
```

#### solution
```bash
# find process using port
sudo lsof -i :8080
# or
sudo ss -tlnp | grep 8080

# kill process
sudo kill -9 <pid>

# or change port in configuration
sed -i 's/bind_port = 8080/bind_port = 8081/' /etc/weather-station/query.ini
```

### database locked

#### symptom
```
[error] database is locked
```

#### causes and solutions

**cause 1: long-running transaction**
```bash
# check for locks
lsof /var/lib/ws/weather.db
fuser /var/lib/ws/weather.db

# kill blocking process
kill -9 <pid>
```

**cause 2: wal mode not enabled**
```bash
# check journal mode
sqlite3 /var/lib/ws/weather.db "pragma journal_mode;"

# enable wal
sqlite3 /var/lib/ws/weather.db "pragma journal_mode = wal;"

# checkpoint wal
sqlite3 /var/lib/ws/weather.db "pragma wal_checkpoint(truncate);"
```

**cause 3: read-only database**
```bash
# check permissions
ls -la /var/lib/ws/weather.db

# fix permissions
sudo chown weather:weather /var/lib/ws/weather.db
sudo chmod 644 /var/lib/ws/weather.db
```

### slow queries

#### symptom
queries taking longer than 10 seconds

#### diagnosis
```bash
# check query plan
sqlite3 /var/lib/ws/weather.db <<eof
explain query plan 
select * from weather_data 
where station_id = 1 
  and timestamp > 1704067200 
order by timestamp;
eof

# check if using index
# look for "using index" in output

# check table sizes
sqlite3 /var/lib/ws/weather.db <<eof
select 
    name,
    sum(pgsize) as size_bytes
from dbstat
where name in ('weather_data', 'hourly_stats', 'daily_stats')
group by name;
eof
```

#### solutions

**missing index:**
```sql
-- check existing indexes
select * from sqlite_master where type='index';

-- create missing index
create index if not exists idx_weather_station_time 
on weather_data(station_id, timestamp);

-- analyze for query optimizer
analyze weather_data;
```

**database fragmentation:**
```bash
# rebuild database
sqlite3 /var/lib/ws/weather.db "vacuum;"

# note: this requires free disk space equal to database size
```

**cache too small:**
```bash
# increase cache size
sqlite3 /var/lib/ws/weather.db "pragma cache_size = -2097152;"  # 2gb
```

### ingestion stuck

#### symptom
csv file in watch directory but not being processed

#### diagnosis
```bash
# check if file is being written
lsof /data/csv/large_file.csv

# check file permissions
ls -la /data/csv/

# check inotify watches
cat /proc/sys/fs/inotify/max_user_watches
find /proc/*/fd -lname anon_inode:inotify | wc -l

# check service logs
journalctl -u ws-ingestion -n 100
```

#### solutions

**file still being written:**
```bash
# wait for file to be closed
# or move file atomically
mv /tmp/complete_file.csv /data/csv/
```

**permission denied:**
```bash
# fix permissions
sudo chown -r weather:weather /data/csv
sudo chmod 755 /data/csv
```

**inotify limit reached:**
```bash
# increase limit
echo 524288 | sudo tee /proc/sys/fs/inotify/max_user_watches

# make permanent
echo "fs.inotify.max_user_watches = 524288" | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

### out of memory

#### symptom
```
[error] cannot allocate memory
killed process 1234 (ws-ingest) total-vm:4194304kb
```

#### diagnosis
```bash
# check memory usage
free -h

# check oom kills
dmesg | grep -i "killed process"

# check process memory
ps aux --sort=-%mem | head -10
```

#### solutions

**reduce batch size:**
```ini
# /etc/weather-station/ingestion.ini
[ingestion]
batch_size = 1000  # reduce from 10000
mmap_threshold_mb = 512  # reduce from 1024
```

**reduce cache size:**
```bash
sqlite3 /var/lib/ws/weather.db "pragma cache_size = -524288;"  # 512mb
sqlite3 /var/lib/ws/weather.db "pragma mmap_size = 134217728;"  # 128mb
```

**add swap:**
```bash
sudo fallocate -l 4g /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

### discovery not finding peers

#### symptom
`ws-cli peers` shows no stations

#### diagnosis
```bash
# check udp traffic
sudo tcpdump -i eth0 udp port 5000 -n

# check if discovery service running
systemctl status ws-discovery

# check network interface
ip addr show

# test broadcast
echo "test" | nc -u -b 255.255.255.255 5000
```

#### solutions

**firewall blocking:**
```bash
# allow udp port 5000
sudo ufw allow 5000/udp
sudo iptables -a input -p udp --dport 5000 -j accept
```

**wrong interface:**
```ini
# /etc/weather-station/discovery.ini
[discovery]
bind_interface = eth0  # change to correct interface
```

**virtual network issues:**
```bash
# for docker/kubernetes, use host network
# docker-compose.yml:
#   network_mode: host

# for kubernetes:
#   hostnetwork: true
```

### mtls connection failed

#### symptom
```
[error] ssl handshake failed
tls: bad certificate
```

#### diagnosis
```bash
# test tls connection
openssl s_client -connect localhost:8443 \
  -cert /etc/ws/certs/client.crt \
  -key /etc/ws/certs/client.key \
  -cafile /etc/ws/certs/ca.crt

# check certificate details
openssl x509 -in /etc/ws/certs/server.crt -noout -text

# verify certificate chain
openssl verify -cafile /etc/ws/certs/ca.crt /etc/ws/certs/server.crt
```

#### solutions

**certificate expired:**
```bash
# check expiration
openssl x509 -in /etc/ws/certs/server.crt -noout -dates

# generate new certificate
openssl req -new -x509 -days 365 \
  -key /etc/ws/certs/server.key \
  -out /etc/ws/certs/server.crt

# restart services
systemctl restart ws-query ws-discovery
```

**wrong ca:**
```bash
# ensure same ca on all stations
scp /etc/ws/certs/ca.crt station2:/etc/ws/certs/
scp /etc/ws/certs/ca.crt station3:/etc/ws/certs/
```

**certificate not readable:**
```bash
# fix permissions
sudo chown -r weather:weather /etc/ws/certs
sudo chmod 600 /etc/ws/certs/*.key
sudo chmod 644 /etc/ws/certs/*.crt
```

### leader election failing

#### symptom
no leader elected, services in degraded mode

#### diagnosis
```bash
# check election log
sqlite3 /var/lib/ws/weather.db "select * from election_log order by timestamp desc limit 5;"

# check peer registry
sqlite3 /var/lib/ws/weather.db "select station_id, hostname, is_leader, is_healthy from peer_stations;"

# check discovery logs
journalctl -u ws-discovery -n 100
```

#### solutions

**stuck election:**
```bash
# force leader on highest id station
sqlite3 /var/lib/ws/weather.db "update peer_stations set is_leader=1 where station_id=3;"
systemctl restart ws-discovery
```

**network partition:**
```bash
# verify all stations can communicate
ping station2
ping station3

# check discovery beacons
sudo tcpdump -i eth0 udp port 5000
```

### high cpu usage

#### diagnosis
```bash
# find cpu hogs
top -p $(pgrep -d',' ws-)

# check specific process
perf top -p $(pgrep ws-query)

# profile with perf
perf record -g -p $(pgrep ws-query) -- sleep 30
perf report
```

#### solutions

**aggregation workers using too much cpu:**
```ini
# /etc/weather-station/aggregation.ini
[workers]
count = 2  # reduce from 4
max_jobs_per_worker = 50  # reduce from 100
```

**query thread pool too large:**
```ini
# /etc/weather-station/query.ini
[thread_pool]
size = 4  # reduce from 8
```

**inefficient queries:**
```sql
-- check slow queries
select * from query_log where duration_ms > 1000 order by timestamp desc;

-- add missing indexes
create index if not exists idx_weather_time_only on weather_data(timestamp);
```

---

## error messages reference

### service startup errors

| error | cause | solution |
|-------|-------|----------|
| "permission denied" | wrong file permissions | check and fix permissions |
| "address already in use" | port conflict | kill conflicting process or change port |
| "no such file or directory" | missing configuration | check config file path |
| "cannot allocate memory" | out of memory | reduce cache sizes or add ram |

### database errors

| error | cause | solution |
|-------|-------|----------|
| "database is locked" | concurrent access / long transaction | enable wal mode, kill blocking process |
| "database disk image is malformed" | corruption | restore from backup |
| "unable to open database file" | wrong path or permissions | check path and permissions |
| "disk i/o error" | disk full or hardware failure | check disk space, smart status |

### network errors

| error | cause | solution |
|-------|-------|----------|
| "connection refused" | service not running or wrong port | start service, check port |
| "connection timeout" | network issue or firewall | check connectivity, firewall rules |
| "tls handshake failed" | certificate issue | check certificates, clock sync |
| "no route to host" | network unreachable | check routing, interface |

### protocol errors

| error | cause | solution |
|-------|-------|----------|
| "invalid magic number" | wrong protocol | check protocol version |
| "version mismatch" | incompatible versions | update to compatible versions |
| "unknown message type" | protocol error | check implementation |
| "sequence id mismatch" | out of order messages | check network latency |

---

## debug mode

### enable debug logging

```bash
# for systemd services
sudo systemctl edit ws-query

# add:
[service]
environment=ws_log_level=debug

# reload and restart
sudo systemctl daemon-reload
sudo systemctl restart ws-query
```

### run in foreground

```bash
# stop service
sudo systemctl stop ws-query

# run in foreground with debug
sudo -u weather /opt/weather-station/services/s3_query/ws-query \
  --config /etc/weather-station/query.ini \
  --log-level debug \
  --log-dest stderr
```

### enable core dumps

```bash
# enable core dumps
ulimit -c unlimited
echo '/var/crash/core.%e.%p' | sudo tee /proc/sys/kernel/core_pattern

# run service
sudo -u weather /opt/weather-station/services/s3_query/ws-query ...

# after crash, analyze
gdb /opt/weather-station/services/s3_query/ws-query /var/crash/core.ws-query.1234
(gdb) bt full
```

### valgrind memory debugging

```bash
# install valgrind
sudo apt-get install valgrind

# run with valgrind
valgrind --leak-check=full \
  --show-leak-kinds=all \
  --track-origins=yes \
  --verbose \
  /opt/weather-station/services/s1_ingestion/ws-ingest \
  --config /etc/weather-station/ingestion.ini
```

---

## getting help

### collect diagnostic information

```bash
#!/bin/bash
# collect-diagnostics.sh

output="/tmp/ws-diagnostics-$(date +%y%m%d-%h%m%s).tar.gz"
workdir=$(mktemp -d)

echo "collecting diagnostic information..."

# system info
uname -a > "$workdir/system-info.txt"
free -h > "$workdir/memory.txt"
df -h > "$workdir/disk.txt"

# service status
systemctl status ws-ingestion ws-aggregation ws-query ws-discovery > "$workdir/service-status.txt" 2>&1

# logs
journalctl -u 'ws-*' --since "24 hours ago" > "$workdir/logs.txt"

# configuration
cp /etc/weather-station/*.ini "$workdir/"

# database info
sqlite3 /var/lib/ws/weather.db ".schema" > "$workdir/db-schema.txt"
sqlite3 /var/lib/ws/weather.db "select count(*) from weather_data;" > "$workdir/db-stats.txt"
sqlite3 /var/lib/ws/weather.db "select * from peer_stations;" >> "$workdir/db-stats.txt"

# network
ss -tlnp > "$workdir/network.txt"
ip addr >> "$workdir/network.txt"

# create archive
tar czf "$output" -c "$workdir" .
rm -rf "$workdir"

echo "diagnostics collected: $output"
echo "please attach this file when reporting issues."
```

### report issues

when reporting issues, include:

1. **diagnostic tarball** (from script above)
2. **steps to reproduce**
3. **expected behavior**
4. **actual behavior**
5. **environment:**
   - os and version
   - hardware specs (cpu, ram, disk)
   - weather station version
   - database size

---

*next: [instructor guide](instructor_guide.md)*
