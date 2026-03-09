# deployment guides

## overview

this directory contains deployment configurations and guides for running the weather station system in various environments.

## deployment options

1. [docker compose](#docker-compose) - development and small deployments
2. [kubernetes](#kubernetes) - production and scale-out deployments
3. [systemd](#systemd) - traditional linux service deployment
4. [manual](#manual) - custom bare-metal deployment

---

## docker compose

### quick start

```bash
# clone repository
git clone <repository-url>
cd weather-station

# copy example configuration
cp config/example.env .env

# edit configuration
vim .env

# start services
docker-compose up -d

# view logs
docker-compose logs -f

# check status
docker-compose ps
```

### configuration

**.env file:**
```bash
# station identification
station_id=1
station_name=station1

# network configuration
host_ip=192.168.1.100
broadcast_interface=eth0
query_port=8080
metrics_port=9090

# paths
data_dir=/data/weather
config_dir=./config

# resource limits
ingest_memory=2g
query_memory=1g
```

### docker-compose.yml

```yaml
version: '3.8'

services:
  s1-ingestion:
    build:
      context: .
      dockerfile: docker/dockerfile.ingestion
    container_name: ws-ingestion
    hostname: ws-ingestion
    restart: unless-stopped
    
    environment:
      - ws_config=/config/ingestion.ini
      - ws_station_id=${station_id}
      - ws_log_level=info
    
    volumes:
      - ${data_dir}/csv:/data/csv:ro
      - ${config_dir}:/config:ro
      - sqlite-data:/var/lib/ws
      - ${data_dir}/processed:/data/processed
      - ${data_dir}/error:/data/error
    
    networks:
      - ws-internal
    
    mem_limit: ${ingest_memory}
    cpus: 2.0
    
    healthcheck:
      test: ["cmd", "test", "-f", "/var/lib/ws/weather.db"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  s2-aggregation:
    build:
      context: .
      dockerfile: docker/dockerfile.aggregation
    container_name: ws-aggregation
    hostname: ws-aggregation
    restart: unless-stopped
    
    environment:
      - ws_config=/config/aggregation.ini
      - ws_db_path=/var/lib/ws/weather.db
      - ws_worker_count=4
    
    volumes:
      - sqlite-data:/var/lib/ws
      - ${config_dir}:/config:ro
    
    networks:
      - ws-internal
    
    mem_limit: ${aggregate_memory}
    cpus: 2.0
    
    depends_on:
      - s1-ingestion

  s3-query:
    build:
      context: .
      dockerfile: docker/dockerfile.query
    container_name: ws-query
    hostname: ws-query
    restart: unless-stopped
    
    environment:
      - ws_config=/config/query.ini
      - ws_db_path=/var/lib/ws/weather.db
      - ws_bind_address=0.0.0.0
      - ws_bind_port=8080
    
    ports:
      - "${query_port}:8080"
      - "${metrics_port}:9090"
    
    volumes:
      - sqlite-data:/var/lib/ws:ro
      - ${config_dir}:/config:ro
      - ./certs:/certs:ro
    
    networks:
      - ws-internal
      - ws-external
    
    mem_limit: ${query_memory}
    cpus: 2.0
    
    healthcheck:
      test: ["cmd", "curl", "-f", "http://localhost:9090/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    
    depends_on:
      - s1-ingestion

  s4-discovery:
    build:
      context: .
      dockerfile: docker/dockerfile.discovery
    container_name: ws-discovery
    hostname: ws-discovery
    restart: unless-stopped
    
    environment:
      - ws_config=/config/discovery.ini
      - ws_station_id=${station_id}
      - ws_beacon_interface=${broadcast_interface}
    
    network_mode: host  # required for broadcast/multicast
    
    volumes:
      - sqlite-data:/var/lib/ws
      - ${config_dir}:/config:ro
      - ./certs:/certs:ro
    
    mem_limit: 256m
    cpus: 0.5
    
    depends_on:
      - s3-query

volumes:
  sqlite-data:
    driver: local

networks:
  ws-internal:
    driver: bridge
    internal: true
  
  ws-external:
    driver: bridge
```

### operations

```bash
# scale query service
docker-compose up -d --scale s3-query=3

# view specific service logs
docker-compose logs -f s3-query

# restart service
docker-compose restart s1-ingestion

# update configuration
docker-compose down
docker-compose up -d

# backup database
docker exec ws-query sqlite3 /var/lib/ws/weather.db ".backup /backup/weather.db"

# access database shell
docker exec -it ws-query sqlite3 /var/lib/ws/weather.db
```

---

## kubernetes

### architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     kubernetes cluster                          │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              namespace: weather-station-1                │   │
│  │                                                          │   │
│  │  configmap: ws-config                                    │   │
│  │  secret: ws-tls-certs                                    │   │
│  │  pvc: ws-sqlite-data                                     │   │
│  │                                                          │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐   │   │
│  │  │ ingestion   │  │ aggregation │  │ query service   │   │   │
│  │  │ deployment  │  │ deployment  │  │ deployment (x3) │   │   │
│  │  │  (1 pod)    │  │  (1 pod)    │  │  (3 replicas)   │   │   │
│  │  └─────────────┘  └─────────────┘  └────────┬────────┘   │   │
│  │                                             │             │   │
│  │                                  ┌──────────┴──────────┐  │   │
│  │                                  │    service: ws-query │  │   │
│  │                                  │    port: 8080        │  │   │
│  │                                  └──────────┬──────────┘  │   │
│  │                                             │             │   │
│  │                                  ┌──────────┴──────────┐  │   │
│  │                                  │   ingress: ws-api   │  │   │
│  │                                  │   tls termination   │  │   │
│  │                                  └─────────────────────┘  │   │
│  │                                                          │   │
│  │  daemonset: ws-discovery (hostnetwork)                   │   │
│  │                                                          │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  servicemonitor: ws-metrics (prometheus)                        │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### installation

**prerequisites:**
- kubernetes 1.20+
- kubectl configured
- helm 3 (optional)

**deploy with kubectl:**

```bash
# create namespace
kubectl create namespace weather-station

# apply configmaps
kubectl apply -f k8s/configmap.yaml -n weather-station

# apply secrets (tls certificates)
kubectl create secret tls ws-tls-cert \
  --cert=certs/server.crt \
  --key=certs/server.key \
  -n weather-station

# apply persistentvolumeclaim
kubectl apply -f k8s/pvc.yaml -n weather-station

# deploy services
kubectl apply -f k8s/deployment-ingestion.yaml -n weather-station
kubectl apply -f k8s/deployment-aggregation.yaml -n weather-station
kubectl apply -f k8s/deployment-query.yaml -n weather-station
kubectl apply -f k8s/daemonset-discovery.yaml -n weather-station

# expose service
kubectl apply -f k8s/service-query.yaml -n weather-station
kubectl apply -f k8s/ingress.yaml -n weather-station

# verify deployment
kubectl get pods -n weather-station
kubectl logs -n weather-station -l app=ws-query
```

### kubernetes manifests

**k8s/configmap.yaml:**
```yaml
apiversion: v1
kind: configmap
metadata:
  name: ws-config
data:
  station.conf: |
    [station]
    id = 1
    location_lat = 52.5200
    location_lon = 13.4050
    
    [ingestion]
    csv_directory = /data/csv
    batch_size = 10000
    
    [query]
    bind_address = 0.0.0.0
    port = 8080
    thread_pool_size = 8
    
    [discovery]
    beacon_interval = 5
    enable_ha = true
```

**k8s/pvc.yaml:**
```yaml
apiversion: v1
kind: persistentvolumeclaim
metadata:
  name: ws-sqlite-data
spec:
  accessmodes:
    - readwriteonce
  resources:
    requests:
      storage: 100gi
  storageclassname: fast-ssd  # use ssd storage class
```

**k8s/deployment-query.yaml:**
```yaml
apiversion: apps/v1
kind: deployment
metadata:
  name: ws-query
  labels:
    app: ws-query
spec:
  replicas: 3
  selector:
    matchlabels:
      app: ws-query
  strategy:
    type: rollingupdate
    rollingupdate:
      maxsurge: 1
      maxunavailable: 0
  template:
    metadata:
      labels:
        app: ws-query
    spec:
      containers:
      - name: query
        image: weather-station/query:latest
        imagepullpolicy: always
        ports:
        - containerport: 8080
          name: query
          protocol: tcp
        - containerport: 9090
          name: metrics
          protocol: tcp
        env:
        - name: ws_config
          value: /config/station.conf
        - name: ws_db_path
          value: /data/weather.db
        volumemounts:
        - name: config
          mountpath: /config
          readonly: true
        - name: data
          mountpath: /data
          readonly: true
        - name: tls-certs
          mountpath: /certs
          readonly: true
        resources:
          requests:
            memory: "512mi"
            cpu: "500m"
          limits:
            memory: "2gi"
            cpu: "2000m"
        livenessprobe:
          httpget:
            path: /health
            port: 9090
          initialdelayseconds: 10
          periodseconds: 10
        readinessprobe:
          httpget:
            path: /health
            port: 9090
          initialdelayseconds: 5
          periodseconds: 5
      volumes:
      - name: config
        configmap:
          name: ws-config
      - name: data
        persistentvolumeclaim:
          claimname: ws-sqlite-data
      - name: tls-certs
        secret:
          secretname: ws-tls-cert
```

**k8s/service-query.yaml:**
```yaml
apiversion: v1
kind: service
metadata:
  name: ws-query
  labels:
    app: ws-query
spec:
  type: clusterip
  selector:
    app: ws-query
  ports:
  - name: query
    port: 8080
    targetport: 8080
    protocol: tcp
  - name: metrics
    port: 9090
    targetport: 9090
    protocol: tcp
```

**k8s/ingress.yaml:**
```yaml
apiversion: networking.k8s.io/v1
kind: ingress
metadata:
  name: ws-ingress
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/proxy-body-size: "10m"
spec:
  tls:
  - hosts:
    - weather.example.com
    secretname: ws-tls-cert
  rules:
  - host: weather.example.com
    http:
      paths:
      - path: /
        pathtype: prefix
        backend:
          service:
            name: ws-query
            port:
              number: 8080
```

**k8s/servicemonitor.yaml:**
```yaml
apiversion: monitoring.coreos.com/v1
kind: servicemonitor
metadata:
  name: ws-metrics
  labels:
    app: ws-query
spec:
  selector:
    matchlabels:
      app: ws-query
  endpoints:
  - port: metrics
    interval: 15s
    path: /metrics
    honorlabels: true
```

### operations

```bash
# view pods
kubectl get pods -n weather-station

# view logs
kubectl logs -n weather-station -l app=ws-query --tail=100 -f

# port forward for local testing
kubectl port-forward -n weather-station svc/ws-query 8080:8080

# scale query service
kubectl scale deployment ws-query --replicas=5 -n weather-station

# rolling update
kubectl set image deployment/ws-query query=weather-station/query:v2.0 -n weather-station

# rollback
kubectl rollout undo deployment/ws-query -n weather-station

# check rollout status
kubectl rollout status deployment/ws-query -n weather-station

# exec into pod
kubectl exec -it -n weather-station deployment/ws-query -- /bin/sh

# debug pod
kubectl run debug --rm -i --tty --image=nicolaka/netshoot -- /bin/bash
```

---

## systemd

### service files

**/etc/systemd/system/ws-ingestion.service:**
```ini
[unit]
description=weather station ingestion service
after=network.target

[service]
type=simple
user=weather
group=weather
workingdirectory=/opt/weather-station
execstart=/opt/weather-station/services/s1_ingestion/ws-ingest \
    --config /etc/weather-station/ingestion.ini \
    --daemon
execreload=/bin/kill -hup $mainpid
restart=always
restartsec=5

# resource limits
limitnofile=65536
limitnproc=4096

# logging
standardoutput=journal
standarderror=journal
syslogidentifier=ws-ingestion

[install]
wantedby=multi-user.target
```

**/etc/systemd/system/ws-query.service:**
```ini
[unit]
description=weather station query service
after=network.target ws-ingestion.service

[service]
type=simple
user=weather
group=weather
workingdirectory=/opt/weather-station
execstart=/opt/weather-station/services/s3_query/ws-query \
    --config /etc/weather-station/query.ini
restart=always
restartsec=5

[install]
wantedby=multi-user.target
```

### operations

```bash
# enable services
sudo systemctl enable ws-ingestion
sudo systemctl enable ws-aggregation
sudo systemctl enable ws-query
sudo systemctl enable ws-discovery

# start services
sudo systemctl start ws-ingestion
sudo systemctl start ws-aggregation
sudo systemctl start ws-query
sudo systemctl start ws-discovery

# check status
sudo systemctl status ws-query

# view logs
sudo journalctl -u ws-query -f

# reload configuration
sudo systemctl reload ws-ingestion

# restart service
sudo systemctl restart ws-query
```

---

## manual deployment

### installation

```bash
# create user
sudo useradd -r -s /bin/false weather

# create directories
sudo mkdir -p /opt/weather-station
sudo mkdir -p /etc/weather-station
sudo mkdir -p /var/lib/ws
sudo mkdir -p /var/log/ws
sudo mkdir -p /data/csv

# copy binaries
sudo cp -r services /opt/weather-station/
sudo cp -r lib /opt/weather-station/
sudo cp config/*.ini /etc/weather-station/

# set permissions
sudo chown -r weather:weather /opt/weather-station
sudo chown -r weather:weather /var/lib/ws
sudo chown -r weather:weather /var/log/ws
sudo chown -r weather:weather /data/csv

# install systemd services
sudo cp systemd/*.service /etc/systemd/system/
sudo systemctl daemon-reload
```

### running manually

```bash
# terminal 1: ingestion
./services/s1_ingestion/ws-ingest --config config/ingestion.ini

# terminal 2: aggregation
./services/s2_aggregation/ws-aggregate --config config/aggregation.ini

# terminal 3: query
./services/s3_query/ws-query --config config/query.ini

# terminal 4: discovery
./services/s4_discovery/ws-discovery --config config/discovery.ini

# terminal 5: cli
./services/c1_cli/ws-cli -i
```

---

## configuration templates

### minimal production config

**config/production.ini:**
```ini
[global]
station_id = 1
environment = production

[database]
path = /var/lib/ws/weather.db
wal_mode = true
cache_size_mb = 1024
mmap_size_mb = 512

[ingestion]
csv_directory = /data/csv
processed_directory = /data/processed
error_directory = /data/error
mmap_threshold_mb = 1024
batch_size = 10000

[query]
bind_address = 0.0.0.0
bind_port = 8080
thread_pool_size = 16
max_connections = 1000

[discovery]
beacon_interval_seconds = 5
health_check_interval_seconds = 10
election_enabled = true

[mtls]
enabled = true
cert_path = /etc/ws/certs/server.crt
key_path = /etc/ws/certs/server.key
ca_path = /etc/ws/certs/ca.crt

[logging]
level = info
destination = syslog
```

---

*next: [operations guide](../operations/readme.md)*
