# Example: Multi-Station Deployment with Peer Endpoints

This example shows how to deploy two weather stations in different Kubernetes namespaces and configure them to discover each other using FQDN-based peer endpoints.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Kubernetes Cluster                          │
│                                                                  │
│  ┌──────────────────────────┐  ┌──────────────────────────┐    │
│  │ Namespace: station1      │  │ Namespace: station2      │    │
│  │                          │  │                          │    │
│  │  ┌──────────────────┐    │  │  ┌──────────────────┐    │    │
│  │  │ Station 1        │    │  │  │ Station 2        │    │    │
│  │  │ All Services     │◄───┼──┼──┼►All Services     │    │    │
│  │  └──────────────────┘    │  │  └──────────────────┘    │    │
│  │                          │  │                          │    │
│  │  Peer Endpoint Config:   │  │  Peer Endpoint Config:   │    │
│  │  station2-query...:8080  │  │  station1-query...:8080  │    │
│  └──────────────────────────┘  └──────────────────────────┘    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Deployment Steps

### 1. Deploy Station 1

```bash
# Build images
docker-compose build

# Tag images
docker tag linux-application-development-ingestion:latest weather-station-ingestion:latest
docker tag linux-application-development-aggregation:latest weather-station-aggregation:latest
docker tag linux-application-development-query:latest weather-station-query:latest
docker tag linux-application-development-discovery:latest weather-station-discovery:latest
docker tag linux-application-development-cli:latest weather-station-cli:latest

# Deploy station 1
helm install station1 charts/weather-station \
  --namespace station1 \
  --create-namespace \
  --set global.imagePullPolicy=Never \
  --set namespace.name=station1
```

### 2. Deploy Station 2 with Peer Configuration

Create a values file for station 2:

```yaml
# station2-values.yaml
namespace:
  name: station2

discovery:
  config:
    node_id: "station2-discovery"
    # Configure peer endpoint to station 1
    peer_endpoints:
      - "weather-station-query.station1.svc.cluster.local:8080:station1"

ingestion:
  config:
    database_path: /data/weather.db

aggregation:
  config:
    output_database: /data/aggregated.db

query:
  config:
    database_path: /data/aggregated.db
```

Deploy station 2:

```bash
helm install station2 charts/weather-station \
  --namespace station2 \
  --create-namespace \
  --set global.imagePullPolicy=Never \
  -f station2-values.yaml
```

### 3. Configure Station 1 to Discover Station 2

Update station 1 to know about station 2:

```bash
# Create patch for station 1
kubectl patch configmap weather-station-config -n station1 --type=merge -p='{
  "data": {
    "discovery.ini": "node_id = station1-discovery\nbind_address = 0.0.0.0\ndiscovery_port = 9000\nelection_timeout_ms = 5000\nheartbeat_interval_ms = 1000\nlog_level = info\npeer_endpoint = weather-station-query.station2.svc.cluster.local:8080:station2\n"
  }
}'

# Restart discovery service to pick up new config
kubectl rollout restart deployment/weather-station-discovery -n station1
```

### 4. Verify Peer Discovery

Check logs on both stations:

```bash
# Station 1 - should show discovery of station 2
kubectl logs -n station1 weather-station-discovery-xxxxx --tail=20

# Station 2 - should show discovery of station 1
kubectl logs -n station2 weather-station-discovery-xxxxx --tail=20
```

Expected log output:
```
[INFO] Added peer endpoint: weather-station-query.station2.svc.cluster.local:8080 (station2)
[INFO] Discovered peer via FQDN: station2 (10.102.xx.xx:8080)
```

### 5. Test Cross-Station Queries

Once peers are discovered, you can query data from either station:

```bash
# Port forward to station 1
kubectl port-forward svc/weather-station-query 8080:8080 -n station1 &

# Query stations from station 1
# This should return stations from BOTH station 1 and station 2
curl http://localhost:8080/api/v1/stations

# Query weather data
# This should return aggregated data from both stations
curl "http://localhost:8080/api/v1/weather/daily?station=USW00014739"
```

## Configuration Reference

### peer_endpoint Format

```
peer_endpoint = fqdn:port[:node_id]
```

**Examples:**

```ini
# Minimal - FQDN and port only (node_id auto-generated)
peer_endpoint = weather-station-query.station2.svc.cluster.local:8080

# With explicit node_id
peer_endpoint = weather-station-query.station2.svc.cluster.local:8080:station2

# External cluster
peer_endpoint = weather-station.example.com:8080:remote-station

# IP address (if DNS not available)
peer_endpoint = 192.168.1.100:8080:station-on-prem
```

### Multiple Peers

You can configure multiple peer endpoints:

```ini
peer_endpoint = weather-station-query.station2.svc.cluster.local:8080:station2
peer_endpoint = weather-station-query.station3.svc.cluster.local:8080:station3
peer_endpoint = weather-station-remote.external.com:8080:remote-station
```

### Helm Values

```yaml
discovery:
  config:
    peer_endpoints:
      - "weather-station-query.other-namespace.svc.cluster.local:8080:station-name"
      - "weather-station-query.another-ns.svc.cluster.local:8080:another-station"
```

## How It Works

1. **FQDN Resolution**: Discovery service resolves the FQDN to an IP address using DNS
2. **TCP Health Checks**: Service periodically checks peer health via TCP connection
3. **Peer Registration**: Healthy peers are added to the cluster node list
4. **Heartbeat Exchange**: TCP heartbeats are sent to maintain peer relationships
5. **Query Federation**: Query service can route requests to peer stations

## Troubleshooting

### Peers Not Discovering Each Other

1. Check DNS resolution:
```bash
kubectl run -it --rm debug --image=busybox:1.36 -n station1 -- nslookup weather-station-query.station2.svc.cluster.local
```

2. Check firewall rules between namespaces

3. Verify peer endpoint configuration:
```bash
kubectl get configmap weather-station-config -n station1 -o yaml | grep peer_endpoint
```

4. Check discovery service logs:
```bash
kubectl logs -n station1 weather-station-discovery-xxxxx --tail=50
```

### DNS Resolution Failures

If FQDN resolution fails, the discovery service will log warnings:
```
[WARN] Failed to resolve weather-station-query.station2.svc.cluster.local: ...
```

Ensure:
- Services are created in both namespaces
- FQDN format is correct: `<service>.<namespace>.svc.cluster.local`
- CoreDNS is running in the cluster

### Cross-Namespace Service Access

For peer discovery to work across namespaces, services must be accessible. The default ClusterIP services work fine within the same cluster.

If using network policies, ensure:
- Port 8080 (query) is accessible between namespaces
- Port 9000 (discovery UDP) is accessible (for local discovery)
- TCP connections are allowed between namespaces

## Use Cases

1. **Multi-Tenant Setup**: Each namespace = one tenant/student
2. **Geographic Distribution**: Different regions/countries
3. **High Availability**: Multiple replicas across zones
4. **Federation**: Connect on-prem and cloud stations
5. **Testing**: Local development with multiple instances
