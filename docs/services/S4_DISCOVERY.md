# s4: discovery service

## overview

the discovery service (s4) manages the peer-to-peer mesh network of weather stations. it handles automatic service discovery via udp broadcast, health monitoring, leader election using the bully algorithm, and coordinates secure communication with mtls.

## responsibilities

- broadcast presence beacons via udp multicast/broadcast
- maintain registry of peer stations
- monitor peer health via tcp probes
- implement bully algorithm for leader election
- coordinate mtls certificate exchange
- manage data replication for high availability

## architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    discovery service                             │
│                                                                  │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │   beacon     │    │   peer       │    │   leader     │      │
│  │   timer      │───►│   registry   │◄──►│   election   │      │
│  │   (5s)       │    │              │    │   (bully)    │      │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘      │
│         │                   │                   │              │
│         │ udp               │                   │              │
│         ▼                   ▼                   ▼              │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │  udp socket  │    │   sqlite     │    │   mtls       │      │
│  │  broadcast   │    │   storage    │    │   coordinator│      │
│  └──────────────┘    └──────────────┘    └──────────────┘      │
│         ▲                                              │       │
│         │                                              │       │
│         │ peer beacons                                 │       │
│         │                                              │       │
│  ┌──────┴──────────────────────────────────────────────┴───┐   │
│  │                     remote stations                      │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │   │
│  │  │station 2 │  │station 3 │  │station 4 │  │station n │ │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘ │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌──────────────┐    ┌──────────────┐                          │
│  │  health      │    │  replication │                          │
│  │  monitor     │    │  manager     │                          │
│  │  (tcp)       │    │  (mtls)      │                          │
│  └──────────────┘    └──────────────┘                          │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## configuration

### configuration file (s4_discovery.ini)

```ini
[service]
name = ws-discovery
log_level = info
log_destination = syslog

[network]
bind_interface = eth0
broadcast_address = 255.255.255.255
multicast_group = 239.255.42.42
beacon_port = 5000
query_port = 8080
enable_multicast = true

[station]
id = 1
hostname = station1
location_lat = 52.5200
location_lon = 13.4050
capabilities = query,aggregate,replicate

[beacon]
interval_seconds = 5
max_age_seconds = 30
ttl = 1

[health]
check_interval_seconds = 10
timeout_seconds = 5
failures_before_down = 3

[election]
enabled = true
algorithm = bully
coordinator_timeout_seconds = 15

[replication]
enabled = true
listen_port = 8443
sync_interval_seconds = 60
batch_size = 1000

[mtls]
enabled = true
cert_path = /etc/ws/certs/server.crt
key_path = /etc/ws/certs/server.key
ca_path = /etc/ws/certs/ca.crt
crl_path = /etc/ws/certs/crl.pem
```

### environment variables

```bash
ws_config=/etc/weather-station/s4_discovery.ini
ws_station_id=1
ws_beacon_interface=eth0
ws_mtls_enabled=true
ws_replication_enabled=true
```

### command-line arguments

```bash
ws-discovery [options]

options:
  -c, --config path        configuration file path
  -i, --id id              station id
  --interface iface        network interface
  --beacon-port port       udp beacon port
  --daemon                 run as daemon
  --force-leader           force this station as leader
  --log-level level        log level
  -h, --help               show help message
```

## api / interface

### udp beacon protocol

**broadcast/multicast**: port 5000

```c
struct ws_beacon {
        uint32_t magic;                 /* 'wsbc' (0x57534243) */
        uint16_t version;               /* protocol version */
        uint16_t flags;                 /* capability flags */
        uint32_t station_id;            /* unique station id */
        uint32_t timestamp;             /* unix timestamp */
        uint16_t query_port;            /* tcp port for queries */
        uint16_t replication_port;      /* tcp port for replication */
        uint32_t data_start;            /* earliest data timestamp */
        uint32_t data_end;              /* latest data timestamp */
        uint8_t is_leader;              /* 1 if leader, 0 otherwise */
        uint8_t health_status;          /* 0=unknown, 1=healthy, 2=degraded */
        char hostname[64];              /* station hostname */
        char capabilities[32];          /* comma-separated list */
        uint8_t tls_fingerprint[32];    /* certificate fingerprint */
} __attribute__((packed));
```

**capability flags:**
- `cap_query` - can answer queries
- `cap_aggregate` - can perform aggregations
- `cap_replicate` - participates in replication
- `cap_ha` - high availability enabled

### tcp health check protocol

port 5001 (or query_port for comprehensive check)

```bash
# simple health probe
$ echo "health" | nc station2 5001
healthy uptime=86400 leader=no peers=5

# detailed status
$ echo "status" | nc station2 5001
station_id: 2
hostname: station2.example.com
leader: no
uptime: 86400
peers: 5
data_range: 1704067200-1706745600
capabilities: query,aggregate,replicate
tls_fingerprint: a1b2c3d4e5f6...
```

## implementation details

### beacon broadcast

```c
int send_beacon(int sockfd, struct sockaddr_in *broadcast_addr)
{
        struct ws_beacon beacon = {
                .magic = htonl(0x57534243),     /* 'wsbc' */
                .version = htons(1),
                .flags = htons(get_capabilities()),
                .station_id = htonl(my_station_id),
                .timestamp = htonl(time(null)),
                .query_port = htons(config.query_port),
                .replication_port = htons(config.replication_port),
                .data_start = htonl(get_earliest_data()),
                .data_end = htonl(get_latest_data()),
                .is_leader = (current_role == role_leader) ? 1 : 0,
                .health_status = get_health_status(),
        };
        
        gethostname(beacon.hostname, sizeof(beacon.hostname));
        snprintf(beacon.capabilities, sizeof(beacon.capabilities),
                "query,aggregate,replicate");
        
        /* get tls fingerprint */
        get_tls_fingerprint(beacon.tls_fingerprint);
        
        ssize_t sent = sendto(sockfd, &beacon, sizeof(beacon), 0,
                (struct sockaddr *)broadcast_addr, sizeof(*broadcast_addr));
        
        return (sent == sizeof(beacon)) ? 0 : -1;
}
```

### beacon reception

```c
void handle_beacon(int sockfd)
{
        struct ws_beacon beacon;
        struct sockaddr_in from_addr;
        socklen_t addr_len = sizeof(from_addr);
        
        ssize_t received = recvfrom(sockfd, &beacon, sizeof(beacon), 0,
                (struct sockaddr *)&from_addr, &addr_len);
        
        if (received != sizeof(beacon))
                return;
        
        /* validate magic number */
        if (ntohl(beacon.magic) != 0x57534243)
                return;
        
        /* validate version */
        if (ntohs(beacon.version) != 1)
                return;
        
        /* update peer registry */
        update_peer_info(&beacon, &from_addr);
        
        /* check if leader changed */
        if (beacon.is_leader && beacon.station_id != current_leader_id) {
                handle_new_leader(ntohl(beacon.station_id));
        }
        
        /* trigger bully election check if needed */
        if (beacon.station_id > my_station_id && beacon.is_leader) {
                /* higher id station is leader - correct state */
        } else if (beacon.station_id > my_station_id && !beacon.is_leader) {
                /* higher id station not leader - election needed? */
                check_election_needed();
        }
}
```

### bully leader election

```c
enum election_state {
        election_none,
        election_running,
        election_coordinator,
};

void check_election_needed(void)
{
        /* check if any higher-id peer is alive */
        int higher_alive = 0;
        for (int i = 0; i < peer_count; i++) {
                if (peers[i].station_id > my_station_id &&
                    peers[i].is_healthy &&
                    peers[i].last_seen > time(null) - config.max_age) {
                        higher_alive = 1;
                        break;
                }
        }
        
        if (!higher_alive && current_role != role_leader) {
                /* no higher station is alive - i should be leader */
                start_election();
        }
}

void start_election(void)
{
        ws_log_info("starting leader election (bully algorithm)");
        
        /* send election message to all higher-id peers */
        for (int i = 0; i < peer_count; i++) {
                if (peers[i].station_id > my_station_id) {
                        send_election_message(peers[i].address);
                }
        }
        
        /* wait for responses */
        struct timespec timeout;
        clock_gettime(clock_realtime, &timeout);
        timeout.tv_sec += config.coordinator_timeout_seconds;
        
        int higher_responded = 0;
        pthread_mutex_lock(&election_mutex);
        while (election_state == election_running) {
                if (pthread_cond_timedwait(&election_cond, &election_mutex, &timeout) == etimedout) {
                        break;
                }
                if (election_higher_responded) {
                        higher_responded = 1;
                        break;
                }
        }
        pthread_mutex_unlock(&election_mutex);
        
        if (!higher_responded) {
                /* no higher station responded - i am leader */
                become_leader();
        }
}

void become_leader(void)
{
        ws_log_info("becoming leader (station_id=%d)", my_station_id);
        
        current_role = role_leader;
        current_leader_id = my_station_id;
        
        /* update database */
        sqlite3_exec(db, "update peer_stations set is_leader=0", null, null, null);
        sqlite3_exec(db, "update peer_stations set is_leader=1 where station_id=?",
                null, null, my_station_id);
        
        /* broadcast coordinator message */
        broadcast_coordinator_message();
        
        /* start leader duties */
        start_replication_coordinator();
}
```

### peer registry

```c
struct peer_info {
        uint32_t station_id;
        char hostname[64];
        struct in_addr ip_address;
        uint16_t query_port;
        uint16_t replication_port;
        time_t first_seen;
        time_t last_seen;
        time_t last_beacon;
        int is_leader;
        int is_healthy;
        uint16_t capabilities;
        uint8_t tls_fingerprint[32];
};

void update_peer_info(const struct ws_beacon *beacon,
        const struct sockaddr_in *addr)
{
        uint32_t station_id = ntohl(beacon->station_id);
        
        /* check if peer exists */
        struct peer_info *peer = find_peer_by_id(station_id);
        
        if (!peer) {
                /* new peer */
                peer = add_new_peer(station_id);
                peer->first_seen = time(null);
                ws_log_info("discovered new peer: %s (id=%d)",
                        beacon->hostname, station_id);
        }
        
        /* update peer info */
        strncpy(peer->hostname, beacon->hostname, sizeof(peer->hostname));
        peer->ip_address = addr->sin_addr;
        peer->query_port = ntohs(beacon->query_port);
        peer->replication_port = ntohs(beacon->replication_port);
        peer->last_seen = time(null);
        peer->last_beacon = ntohl(beacon->timestamp);
        peer->is_leader = beacon->is_leader;
        peer->capabilities = ntohs(beacon->flags);
        memcpy(peer->tls_fingerprint, beacon->tls_fingerprint, 32);
        
        /* persist to database */
        persist_peer_to_db(peer);
}
```

### health monitoring

```c
void* health_monitor_thread(void *arg)
{
        while (!shutdown) {
                for (int i = 0; i < peer_count; i++) {
                        struct peer_info *peer = &peers[i];
                        
                        /* skip self */
                        if (peer->station_id == my_station_id)
                                continue;
                        
                        /* check if health check needed */
                        if (time(null) - peer->last_seen > config.check_interval) {
                                check_peer_health(peer);
                        }
                        
                        /* mark stale peers */
                        if (time(null) - peer->last_seen > config.max_age) {
                                mark_peer_down(peer);
                        }
                }
                
                sleep(config.check_interval);
        }
        
        return null;
}

int check_peer_health(struct peer_info *peer)
{
        int sockfd = socket(af_inet, sock_stream, 0);
        if (sockfd < 0)
                return -1;
        
        struct sockaddr_in addr = {
                .sin_family = af_inet,
                .sin_port = htons(peer->query_port),
                .sin_addr = peer->ip_address
        };
        
        /* set timeout */
        struct timeval tv = {
                .tv_sec = config.health_timeout,
                .tv_usec = 0
        };
        setsockopt(sockfd, sol_socket, so_rcvtimeo, &tv, sizeof(tv));
        setsockopt(sockfd, sol_socket, so_sndtimeo, &tv, sizeof(tv));
        
        /* attempt connection */
        int result = connect(sockfd, (struct sockaddr *)&addr, sizeof(addr));
        close(sockfd);
        
        if (result == 0) {
                peer->consecutive_failures = 0;
                if (!peer->is_healthy) {
                        peer->is_healthy = 1;
                        ws_log_info("peer %d is healthy", peer->station_id);
                }
                return 0;
        } else {
                peer->consecutive_failures++;
                if (peer->consecutive_failures >= config.failures_before_down &&
                    peer->is_healthy) {
                        peer->is_healthy = 0;
                        ws_log_warn("peer %d marked down after %d failures",
                                peer->station_id, peer->consecutive_failures);
                        
                        /* trigger election if leader went down */
                        if (peer->is_leader) {
                                check_election_needed();
                        }
                }
                return -1;
        }
}
```

## database schema

```sql
-- peer stations registry
create table if not exists peer_stations (
        station_id integer primary key,
        hostname text not null,
        ip_address text,
        query_port integer,
        replication_port integer,
        first_seen integer,
        last_seen integer,
        last_beacon integer,
        is_leader boolean default 0,
        is_healthy boolean default 0,
        capabilities integer default 0,
        tls_fingerprint blob,
        consecutive_failures integer default 0,
        
        unique(hostname, query_port)
);

create index idx_peer_leader on peer_stations(is_leader) where is_leader=1;
create index idx_peer_healthy on peer_stations(is_healthy, last_seen);

-- leader election log
create table if not exists election_log (
        id integer primary key autoincrement,
        timestamp integer default (strftime('%s', 'now')),
        old_leader_id integer,
        new_leader_id integer,
        reason text  -- 'timeout', 'higher_id', 'manual'
);

-- replication status
create table if not exists replication_peers (
        peer_id integer primary key,
        last_sync_timestamp integer,
        last_sync_records integer,
        sync_status text,  -- 'idle', 'syncing', 'failed'
        lag_seconds integer,
        foreign key (peer_id) references peer_stations(station_id)
);
```

## performance characteristics

| metric | target | notes |
|--------|--------|-------|
| beacon latency | <1ms | udp broadcast overhead |
| election time | <30s | from leader failure to new leader |
| health check | <5s | tcp connection timeout |
| peer capacity | 100+ | stations in mesh |
| memory per peer | ~1kb | registry storage |

## monitoring metrics

```
# help ws_discovery_peers_total total peers known
# type ws_discovery_peers_total gauge
ws_discovery_peers_total{status="healthy"} 5
ws_discovery_peers_total{status="unhealthy"} 1

# help ws_discovery_beacons_sent total beacons broadcast
# type ws_discovery_beacons_sent counter
ws_discovery_beacons_sent 172800

# help ws_discovery_beacons_received total beacons received
# type ws_discovery_beacons_received counter
ws_discovery_beacons_received 864000

# help ws_discovery_leader_leader am i leader (1=yes, 0=no)
# type ws_discovery_leader_leader gauge
ws_discovery_leader_leader 0

# help ws_discovery_leader_id current leader station id
# type ws_discovery_leader_id gauge
ws_discovery_leader_id 3

# help ws_discovery_elections_total total elections
# type ws_discovery_elections_total counter
ws_discovery_elections_total{result="won"} 2
ws_discovery_elections_total{result="lost"} 5

# help ws_discovery_health_checks_total health checks performed
# type ws_discovery_health_checks_total counter
ws_discovery_health_checks_total{result="success"} 10000
ws_discovery_health_checks_total{result="failure"} 50
```

## troubleshooting

### common issues

| symptom | cause | solution |
|---------|-------|----------|
| no peers discovered | firewall blocking udp | check port 5000/udp |
| frequent elections | network instability | increase beacon timeout |
| split brain | network partition | enable stonith or quorum |
| mtls failures | clock skew | synchronize ntp |
| high cpu | too many peers | limit mesh size, use gossip |

### diagnostic commands

```bash
# check beacon reception
sudo tcpdump -i eth0 udp port 5000 -n

# list known peers
sqlite3 /var/lib/ws/weather.db "select station_id, hostname, is_leader, is_healthy from peer_stations;"

# test health check
echo "health" | nc station2 5001

# check election log
sqlite3 /var/lib/ws/weather.db "select * from election_log order by timestamp desc limit 5;"
```

---

*next: [c1: cli client](c1_cli.md)*
