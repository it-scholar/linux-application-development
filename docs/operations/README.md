# operations guide

## overview

this guide covers the day-to-day operation, monitoring, and maintenance of the weather station system.

## table of contents

1. [daily operations](#daily-operations)
2. [monitoring](#monitoring)
3. [backup and recovery](#backup-and-recovery)
4. [troubleshooting](#troubleshooting)
5. [maintenance tasks](#maintenance-tasks)
6. [security operations](#security-operations)

---

## daily operations

### health checks

```bash
#!/bin/bash
# daily-health-check.sh

services="ws-ingestion ws-aggregation ws-query ws-discovery"

for service in $services; do
    status=$(systemctl is-active $service)
    if [ "$status" != "active" ]; then
        echo "alert: $service is not running!"
        systemctl restart $service
    fi
done

# check disk space
usage=$(df /var/lib/ws | awk 'nr==2 {print $5}' | sed 's/%//')
if [ $usage -gt 80 ]; then
    echo "warning: disk usage at ${usage}%"
fi

# check database size
db_size=$(stat -c%s /var/lib/ws/weather.db)
if [ $db_size -gt $((100 * 1024 * 1024 * 1024)) ]; then
    echo "warning: database larger than 100gb"
fi

# test query endpoint
if ! curl -sf http://localhost:9090/health > /dev/null; then
    echo "alert: query service health check failed!"
fi
```

### log monitoring

```bash
# view all service logs
journalctl -u 'ws-*' -f

# view errors only
journalctl -u 'ws-*' -p err -f

# search for specific pattern
journalctl -u ws-ingestion | grep -i error

# logs from last hour
journalctl -u ws-query --since "1 hour ago"

# export logs
journalctl -u 'ws-*' --since "24 hours ago" > /tmp/ws-logs-$(date +%y%m%d).txt
```

### common operations

```bash
# ingest new csv file
cp new_data.csv /data/csv/

# check ingestion status
sqlite3 /var/lib/ws/weather.db "select * from ingest_log order by started_at desc limit 5;"

# query recent data
ws-cli query --from "-1 hour" --metrics temperature

# check peer stations
ws-cli peers

# restart all services
systemctl restart ws-ingestion ws-aggregation ws-query ws-discovery
```

---

## monitoring

### prometheus metrics

key metrics to monitor:

```promql
# service availability
up{job="weather-station"}

# query rate
rate(ws_query_requests_total[5m])

# query latency
histogram_quantile(0.99, rate(ws_query_duration_seconds_bucket[5m]))

# error rate
rate(ws_query_requests_total{status="error"}[5m])

# active connections
ws_active_connections

# disk usage
node_filesystem_avail_bytes{mountpoint="/var/lib/ws"}

# memory usage
process_resident_memory_bytes{job="weather-station"}

# leader status
ws_discovery_leader_leader

# peer health
ws_discovery_peers_total{status="healthy"}
```

### alerting rules

```yaml
# prometheus-alerts.yml
groups:
  - name: weather-station
    rules:
      - alert: servicedown
        expr: up{job="weather-station"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "weather station service down"
          
      - alert: higherrorrate
        expr: rate(ws_query_requests_total{status="error"}[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "high error rate in query service"
          
      - alert: highlatency
        expr: histogram_quantile(0.99, rate(ws_query_duration_seconds_bucket[5m])) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "query latency above 100ms"
          
      - alert: diskspacelow
        expr: (node_filesystem_avail_bytes{mountpoint="/var/lib/ws"} / node_filesystem_size_bytes{mountpoint="/var/lib/ws"}) < 0.2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "disk space below 20%"
          
      - alert: leaderelectionfailed
        expr: ws_discovery_leader_leader == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "no leader elected"
```

### dashboard

**grafana dashboard json (simplified):**

```json
{
  "dashboard": {
    "title": "weather station",
    "panels": [
      {
        "title": "query rate",
        "targets": [{
          "expr": "rate(ws_query_requests_total[5m])"
        }]
      },
      {
        "title": "query latency",
        "targets": [{
          "expr": "histogram_quantile(0.99, rate(ws_query_duration_seconds_bucket[5m]))"
        }]
      },
      {
        "title": "active connections",
        "targets": [{
          "expr": "ws_active_connections"
        }]
      },
      {
        "title": "peer health",
        "targets": [{
          "expr": "ws_discovery_peers_total"
        }]
      }
    ]
  }
}
```

---

## backup and recovery

### backup strategy

```bash
#!/bin/bash
# backup.sh

backup_dir="/backup/weather-station/$(date +%y%m%d)"
mkdir -p "$backup_dir"

# 1. backup database (online)
sqlite3 /var/lib/ws/weather.db ".backup '${backup_dir}/weather.db'"

# 2. backup wal file
cp /var/lib/ws/weather.db-wal "$backup_dir/" 2>/dev/null || true

# 3. backup configuration
cp -r /etc/weather-station "$backup_dir/"

# 4. backup certificates
cp -r /etc/ws/certs "$backup_dir/"

# 5. create manifest
cat > "$backup_dir/manifest.txt" <<eof
backup date: $(date)
database size: $(stat -c%s "${backup_dir}/weather.db")
station id: $(cat /etc/weather-station/station.conf | grep id | cut -d= -f2)
eof

# 6. compress
tar czf "${backup_dir}.tar.gz" -c "$(dirname $backup_dir)" "$(basename $backup_dir)"
rm -rf "$backup_dir"

# 7. upload to remote storage (example: s3)
# aws s3 cp "${backup_dir}.tar.gz" s3://weather-station-backups/

# 8. clean old backups (keep 30 days)
find /backup/weather-station -name "*.tar.gz" -mtime +30 -delete

echo "backup complete: ${backup_dir}.tar.gz"
```

### recovery procedure

```bash
#!/bin/bash
# restore.sh

backup_file="$1"

if [ ! -f "$backup_file" ]; then
    echo "error: backup file not found: $backup_file"
    exit 1
fi

# 1. stop all services
systemctl stop ws-ingestion ws-aggregation ws-query ws-discovery

# 2. backup current state (just in case)
mv /var/lib/ws/weather.db /var/lib/ws/weather.db.bak.$(date +%s)

# 3. extract backup
tar xzf "$backup_file" -c /tmp/
backup_dir=$(tar tzf "$backup_file" | head -1 | cut -d/ -f1)

# 4. restore database
cp "/tmp/${backup_dir}/weather.db" /var/lib/ws/
chown weather:weather /var/lib/ws/weather.db
chmod 644 /var/lib/ws/weather.db

# 5. restore configuration (optional)
# cp -r "/tmp/${backup_dir}/weather-station" /etc/

# 6. verify database integrity
sqlite3 /var/lib/ws/weather.db "pragma integrity_check;"

# 7. start services
systemctl start ws-discovery
sleep 2
systemctl start ws-ingestion
sleep 2
systemctl start ws-aggregation
sleep 2
systemctl start ws-query

# 8. verify
sleep 5
curl -f http://localhost:9090/health

echo "restore complete"
```

### point-in-time recovery

```sql
-- 1. identify point to recover to (e.g., before data corruption)
select timestamp, id from weather_data 
where timestamp > strftime('%s', '2024-01-15 14:30:00');

-- 2. delete corrupted data
delete from weather_data 
where timestamp >= strftime('%s', '2024-01-15 14:30:00');

-- 3. re-ingest from that point
-- copy original csv files and re-ingest

-- 4. re-run aggregations
delete from hourly_stats 
where hour >= strftime('%s', '2024-01-15 14:00:00');

-- then trigger aggregation jobs
```

---

## troubleshooting

### service won't start

```bash
# check configuration syntax
/opt/weather-station/services/s3_query/ws-query --config /etc/weather-station/query.ini --help

# check for port conflicts
ss -tlnp | grep 8080

# check permissions
ls -la /var/lib/ws/weather.db
ls -la /var/log/

# check logs
journalctl -u ws-query -n 100 --no-pager

# run in foreground for debugging
/opt/weather-station/services/s3_query/ws-query --config /etc/weather-station/query.ini
```

### slow queries

```sql
-- check query plan
explain query plan 
select * from weather_data 
where station_id = 1 and timestamp > 1704067200;

-- check for missing indexes
select * from sqlite_master where type='index';

-- check table statistics
select * from sqlite_stat1;

-- analyze database
analyze;

-- check for locks
-- (use lsof or fuser to check file locks)
```

### database locked

```bash
# check for long-running transactions
sqlite3 /var/lib/ws/weather.db ".timeout 5000" "select * from weather_data limit 1;"

# check wal mode
sqlite3 /var/lib/ws/weather.db "pragma journal_mode;"

# checkpoint wal
sqlite3 /var/lib/ws/weather.db "pragma wal_checkpoint(truncate);"

# check for zombie processes
ps aux | grep ws-
```

### network issues

```bash
# test connectivity
curl -v http://localhost:9090/health

# check ports
ss -tlnp | grep -e '8080|9090|5000'

# test discovery
sudo tcpdump -i eth0 udp port 5000 -n

# check firewall
sudo iptables -l | grep 8080
sudo ufw status

# test mtls
openssl s_client -connect localhost:8443 \
  -cert /etc/ws/certs/client.crt \
  -key /etc/ws/certs/client.key \
  -cafile /etc/ws/certs/ca.crt
```

### memory issues

```bash
# check memory usage
ps aux --sort=-%mem | head -20

# check for memory leaks
valgrind --leak-check=full /opt/weather-station/services/s1_ingestion/ws-ingest

# monitor over time
watch -n 1 'ps -o pid,vsz,rss,comm -p $(pgrep -d"," ws-)'

# check oom kills
dmesg | grep -i "out of memory"
```

---

## maintenance tasks

### daily

- [ ] check service health
- [ ] review error logs
- [ ] verify backups completed
- [ ] monitor disk space

### weekly

- [ ] database integrity check
- [ ] clean up old processed csv files
- [ ] review performance metrics
- [ ] update peer registry (remove stale entries)

### monthly

- [ ] full database backup test restore
- [ ] certificate expiration check
- [ ] system updates
- [ ] performance baseline review

### quarterly

- [ ] capacity planning review
- [ ] security audit
- [ ] disaster recovery drill
- [ ] documentation update

### automated maintenance script

```bash
#!/bin/bash
# maintenance.sh

echo "=== weather station maintenance ==="

# 1. database maintenance
echo "running database maintenance..."
sqlite3 /var/lib/ws/weather.db <<eof
pragma optimize;
vacuum;
pragma wal_checkpoint(truncate);
analyze;
eof

# 2. clean up old files
echo "cleaning up old csv files..."
find /data/processed -name "*.csv" -mtime +7 -delete
find /data/error -name "*.csv" -mtime +30 -delete

# 3. rotate logs
journalctl --vacuum-time=7d

# 4. clean up old peer entries
echo "cleaning up stale peers..."
sqlite3 /var/lib/ws/weather.db "delete from peer_stations where last_seen < strftime('%s', 'now', '-30 days');"

# 5. check certificates
echo "checking certificate expiration..."
for cert in /etc/ws/certs/*.crt; do
    expiry=$(openssl x509 -in "$cert" -noout -enddate | cut -d= -f2)
    expiry_epoch=$(date -d "$expiry" +%s)
    now_epoch=$(date +%s)
    days_until=$(( (expiry_epoch - now_epoch) / 86400 ))
    
    if [ $days_until -lt 30 ]; then
        echo "warning: $cert expires in $days_until days"
    fi
done

echo "maintenance complete"
```

---

## security operations

### certificate management

```bash
# check certificate expiration
openssl x509 -in /etc/ws/certs/server.crt -noout -dates

# renew certificates
# 1. generate new csr
openssl req -new -key /etc/ws/certs/server.key -out /tmp/server.csr

# 2. submit to ca (or self-sign)
openssl x509 -req -in /tmp/server.csr \
  -ca /etc/ws/certs/ca.crt -cakey /etc/ws/certs/ca.key \
  -out /etc/ws/certs/server.crt -days 365

# 3. restart services
systemctl restart ws-query ws-discovery

# 4. verify
openssl s_client -connect localhost:8443 -cafile /etc/ws/certs/ca.crt
```

### access control

```bash
# review access logs
grep "authentication" /var/log/syslog

# check for unauthorized queries
sqlite3 /var/lib/ws/weather.db "select * from query_log where status='unauthorized';"

# update firewall rules
sudo ufw allow from 192.168.1.0/24 to any port 8080
sudo ufw deny from any to any port 8080
```

### security audit

```bash
#!/bin/bash
# security-audit.sh

echo "=== security audit ==="

# check file permissions
echo "checking file permissions..."
find /etc/ws -type f ! -perm 600 -ls
find /var/lib/ws -type f ! -perm 644 -ls

# check for world-writable files
echo "checking for world-writable files..."
find /opt/weather-station -type f -perm -002 -ls

# check for suid binaries
echo "checking suid binaries..."
find /opt/weather-station -type f -perm -4000 -ls

# verify tls configuration
echo "checking tls configuration..."
openssl s_client -connect localhost:8443 -tls1_3 2>/dev/null | grep "protocol"

# check for outdated dependencies
echo "checking for updates..."
apt list --upgradable 2>/dev/null | grep -e "sqlite|openssl"

echo "audit complete"
```

---

*next: [troubleshooting guide](troubleshooting.md)*
