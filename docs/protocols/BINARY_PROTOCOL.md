# binary protocol specification

## overview

the weather station binary protocol is a custom tcp-based protocol for efficient communication between weather stations. it is designed for low overhead, high throughput, and extensibility.

## protocol design

### design goals

1. **efficiency**: minimal overhead compared to http/json
2. **streaming**: support for large result sets without buffering
3. **versioning**: protocol evolution without breaking changes
4. **simplicity**: easy to implement and debug
5. **endianness**: network byte order for portability

### protocol stack

```
┌─────────────────────────────────────┐
│        application layer            │
│   (query, discovery, replication)   │
├─────────────────────────────────────┤
│        binary protocol              │
│   (headers, payloads, messages)     │
├─────────────────────────────────────┤
│        transport layer              │
│   (tcp with optional mtls)          │
├─────────────────────────────────────┤
│        network layer                │
│   (ipv4/ipv6)                       │
└─────────────────────────────────────┘
```

## message format

### header structure

all messages start with a fixed 16-byte header:

```c
struct ws_header {
        uint32_t magic;                 /* 'weat' (0x57454154) */
        uint16_t version;               /* protocol version (1) */
        uint16_t msg_type;              /* message type */
        uint32_t payload_len;           /* length of payload in bytes */
        uint32_t sequence_id;           /* request/response correlation id */
} __attribute__((packed));
```

**total header size**: 16 bytes

**field details:**

| field | type | description |
|-------|------|-------------|
| magic | uint32_t | protocol identifier: 'weat' (0x57454154) |
| version | uint16_t | protocol version (network byte order) |
| msg_type | uint16_t | message type identifier |
| payload_len | uint32_t | length of following payload (0 = no payload) |
| sequence_id | uint32_t | monotonic sequence number for correlation |

### byte ordering

all multi-byte fields are in **network byte order** (big-endian):

```c
/* conversion macros */
#define ws_htons(x) htons(x)
#define ws_htonl(x) htonl(x)
#define ws_ntohs(x) ntohs(x)
#define ws_ntohl(x) ntohl(x)

/* packing */
void ws_header_pack(const struct ws_header *src, void *dst)
{
        uint8_t *p = dst;
        
        *(uint32_t *)(p + 0) = ws_htonl(src->magic);
        *(uint16_t *)(p + 4) = ws_htons(src->version);
        *(uint16_t *)(p + 6) = ws_htons(src->msg_type);
        *(uint32_t *)(p + 8) = ws_htonl(src->payload_len);
        *(uint32_t *)(p + 12) = ws_htonl(src->sequence_id);
}

/* unpacking */
void ws_header_unpack(const void *src, struct ws_header *dst)
{
        const uint8_t *p = src;
        
        dst->magic = ws_ntohl(*(uint32_t *)(p + 0));
        dst->version = ws_ntohs(*(uint16_t *)(p + 4));
        dst->msg_type = ws_ntohs(*(uint16_t *)(p + 6));
        dst->payload_len = ws_ntohl(*(uint32_t *)(p + 8));
        dst->sequence_id = ws_ntohl(*(uint32_t *)(p + 12));
}
```

## message types

### control messages (0x00-0x0f)

```c
#define ws_msg_ping             0x00
#define ws_msg_pong             0x01
#define ws_msg_hello            0x02
#define ws_msg_goodbye          0x03
```

### query messages (0x10-0x1f)

```c
#define ws_msg_query            0x10
#define ws_msg_query_resp       0x11
#define ws_msg_stream_start     0x12
#define ws_msg_stream_chunk     0x13
#define ws_msg_stream_end       0x14
#define ws_msg_cancel           0x15
```

### discovery messages (0x20-0x2f)

```c
#define ws_msg_discovery        0x20
#define ws_msg_discovery_resp   0x21
#define ws_msg_peer_info        0x22
#define ws_msg_capabilities     0x23
```

### replication messages (0x30-0x3f)

```c
#define ws_msg_replicate        0x30
#define ws_msg_replicate_ack    0x31
#define ws_msg_sync_request     0x32
#define ws_msg_sync_response    0x33
```

### error messages (0xf0-0xff)

```c
#define ws_msg_error            0xf0
#define ws_msg_reject           0xf1
#define ws_msg_not_supported    0xf2
```

## message specifications

### ping (0x00) / pong (0x01)

**purpose**: health check and latency measurement

**request (ping):**
```
header:
  magic:        0x57454154 ('weat')
  version:      0x0001
  msg_type:     0x0000
  payload_len:  0
  sequence_id:  <client assigned>
```

**response (pong):**
```
header:
  magic:        0x57454154
  version:      0x0001
  msg_type:     0x0001
  payload_len:  8
  sequence_id:  <same as request>

payload (8 bytes):
  uint64_t timestamp;             /* server timestamp (microseconds since epoch) */
```

### query (0x10)

**purpose**: request weather data

**request:**
```c
struct ws_query_req {
        uint64_t start_time;            /* unix timestamp (seconds) */
        uint64_t end_time;              /* unix timestamp (seconds) */
        uint32_t station_id;            /* 0 = all stations */
        uint16_t metric_mask;           /* bitmask of requested metrics */
        uint8_t  aggregation;           /* 0=none, 1=hourly, 2=daily */
        uint8_t  format;                /* 0=binary, 1=csv, 2=json */
        char     station_filter[64];    /* optional station name pattern */
} __attribute__((packed));
```

**metric mask bits:**
```c
#define metric_temperature      0x0001
#define metric_humidity         0x0002
#define metric_pressure         0x0004
#define metric_wind_speed       0x0008
#define metric_wind_direction   0x0010
#define metric_precipitation    0x0020
#define metric_all              0xffff
```

**response (query_resp 0x11):**
```c
struct ws_query_resp {
        uint32_t total_records;         /* total matching records */
        uint32_t chunk_count;           /* number of chunks to follow */
        uint16_t status_code;           /* http-like status */
        uint16_t reserved;              /* padding */
        char     status_message[128];   /* human-readable message */
} __attribute__((packed));
```

**status codes:**
- 200: ok
- 400: bad request
- 404: no data found
- 500: internal server error
- 503: service unavailable (rate limited)

### stream_chunk (0x13)

**purpose**: stream large result sets in chunks

```c
struct ws_stream_chunk {
        uint32_t chunk_number;          /* sequential chunk number (0-based) */
        uint32_t record_count;          /* records in this chunk */
        uint32_t flags;                 /* 0x01=last chunk */
        uint32_t reserved;              /* padding */
        /* followed by record_count * sizeof(ws_record) bytes */
} __attribute__((packed));
```

**record structure:**
```c
struct ws_record {
        uint64_t timestamp;             /* unix timestamp */
        int32_t  station_id;            /* station identifier */
        float    temperature;           /* temperature (°c) */
        float    humidity;              /* humidity (%) */
        float    pressure;              /* pressure (hpa) */
        float    wind_speed;            /* wind speed (m/s) */
        float    wind_direction;        /* wind direction (degrees) */
        float    precipitation;         /* precipitation (mm) */
} __attribute__((packed));
```

**total record size**: 40 bytes

### discovery (0x20)

**purpose**: request information about a station

**request:**
```c
struct ws_discovery_req {
        uint32_t flags;                 /* request flags */
        char     query[256];            /* optional query string */
} __attribute__((packed));
```

**response (discovery_resp 0x21):**
```c
struct ws_discovery_resp {
        uint32_t station_id;
        uint32_t timestamp;
        uint16_t query_port;
        uint16_t replication_port;
        uint32_t data_start;            /* earliest data timestamp */
        uint32_t data_end;              /* latest data timestamp */
        uint16_t capabilities;          /* capability flags */
        uint8_t  is_leader;
        uint8_t  status;                /* 0=unknown, 1=healthy, 2=degraded */
        char     hostname[64];
        char     version[32];
        uint8_t  tls_fingerprint[32];
} __attribute__((packed));
```

### error (0xf0)

**purpose**: report errors

```c
struct ws_error {
        uint16_t error_code;            /* error code */
        uint16_t reserved;
        char     message[256];          /* human-readable error */
} __attribute__((packed));
```

**error codes:**
```c
#define ws_err_none             0
#define ws_err_invalid_msg      1
#define ws_err_unsupported      2
#define ws_err_auth_failed      3
#define ws_err_rate_limited     4
#define ws_err_timeout          5
#define ws_err_server_error     6
#define ws_err_not_found        7
```

## communication patterns

### simple query pattern

```
client                                          server
  │                                               │
  │────── query (seq=1) ────────────────────────>│
  │                                               │
  │<───── query_resp (seq=1, chunks=1) ─────────│
  │                                               │
  │<───── stream_chunk (seq=1, chunk=0) ────────│
  │  [records...]                                 │
  │                                               │
```

### streaming query pattern

```
client                                          server
  │                                               │
  │────── query (seq=42) ───────────────────────>│
  │                                               │
  │<───── query_resp (seq=42, chunks=100) ──────│
  │                                               │
  │<───── stream_chunk (seq=42, chunk=0) ───────│
  │<───── stream_chunk (seq=42, chunk=1) ───────│
  │<───── stream_chunk (seq=42, chunk=2) ───────│
  │  ...                                          │
  │<───── stream_chunk (seq=42, chunk=99, last) ─│
  │                                               │
```

### health check pattern

```
client                                          server
  │                                               │
  │────── ping (seq=100) ───────────────────────>│
  │                                               │
  │<───── pong (seq=100, timestamp) ────────────│
  │                                               │
```

### error pattern

```
client                                          server
  │                                               │
  │────── query (seq=50, invalid params) ───────>│
  │                                               │
  │<───── error (seq=50, code=400) ─────────────│
  │                                               │
```

## framing and transport

### message framing

messages are length-prefixed for reliable parsing:

```
┌──────────────────┬──────────────────┐
│ header (16 bytes)│ payload (variable)│
└──────────────────┴──────────────────┘
```

### reading a message

```c
int recv_message(int sockfd, struct ws_header *header, void **payload)
{
        /* read header */
        uint8_t header_buf[16];
        if (recv_all(sockfd, header_buf, 16) != 16)
                return -1;
        
        ws_header_unpack(header_buf, header);
        
        /* validate magic */
        if (header->magic != ws_magic) {
                return -1;
        }
        
        /* read payload if present */
        if (header->payload_len > 0) {
                *payload = malloc(header->payload_len);
                if (recv_all(sockfd, *payload, header->payload_len) !=
                    header->payload_len) {
                        free(*payload);
                        return -1;
                }
        } else {
                *payload = null;
        }
        
        return 0;
}
```

### sending a message

```c
int send_message(int sockfd, uint16_t msg_type, uint32_t sequence_id,
        const void *payload, uint32_t payload_len)
{
        struct ws_header header = {
                .magic = ws_magic,
                .version = ws_version,
                .msg_type = msg_type,
                .payload_len = payload_len,
                .sequence_id = sequence_id
        };
        
        /* pack and send header */
        uint8_t header_buf[16];
        ws_header_pack(&header, header_buf);
        
        if (send_all(sockfd, header_buf, 16) != 16)
                return -1;
        
        /* send payload if present */
        if (payload_len > 0) {
                if (send_all(sockfd, payload, payload_len) != payload_len)
                        return -1;
        }
        
        return 0;
}
```

## protocol versioning

### version compatibility

- **major version changes**: breaking changes, incompatible
- **minor version changes**: backward compatible additions

**current version**: 0x0001 (version 1.0)

**version negotiation:**
```c
/* client sends supported version range */
struct ws_hello {
        uint16_t min_version;
        uint16_t max_version;
};

/* server responds with chosen version */
struct ws_hello_resp {
        uint16_t version;
        uint16_t status;        /* 0=ok, 1=no common version */
};
```

## security considerations

### transport security

- use tls 1.3 with mutual authentication (mtls)
- certificate pinning for known peers
- perfect forward secrecy required

### message integrity

- tls provides integrity for tcp transport
- application-level checksums for stored data

### replay protection

- sequence ids prevent simple replay attacks
- timestamps in messages
- tls session resumption limits

## implementation example

```c
/* client: send query */
void send_query_request(int sockfd, uint64_t start, uint64_t end)
{
        struct ws_query_req req = {
                .start_time = ws_htonll(start),
                .end_time = ws_htonll(end),
                .station_id = ws_htonl(0),              /* all stations */
                .metric_mask = ws_htons(metric_all),
                .aggregation = 0,                       /* no aggregation */
                .format = 0,                            /* binary */
        };
        strncpy(req.station_filter, "", sizeof(req.station_filter));
        
        uint32_t seq_id = get_next_sequence_id();
        send_message(sockfd, ws_msg_query, seq_id, &req, sizeof(req));
}

/* server: handle query */
void handle_query_request(int client_fd, struct ws_header *header,
        void *payload)
{
        struct ws_query_req req;
        memcpy(&req, payload, sizeof(req));
        
        /* convert from network byte order */
        uint64_t start = ws_ntohll(req.start_time);
        uint64_t end = ws_ntohll(req.end_time);
        uint32_t station = ws_ntohl(req.station_id);
        
        /* execute query */
        query_result_t *result = execute_query(start, end, station);
        
        /* send response */
        struct ws_query_resp resp = {
                .total_records = ws_htonl(result->count),
                .chunk_count = ws_htonl(1),
                .status_code = ws_htons(200),
        };
        strncpy(resp.status_message, "ok", sizeof(resp.status_message));
        
        send_message(client_fd, ws_msg_query_resp, header->sequence_id,
                &resp, sizeof(resp));
        
        /* send records */
        send_stream_chunk(client_fd, header->sequence_id, 0, result->records,
                result->count, 1);  /* last=1 */
}
```

## debugging tools

### hex dump of message

```bash
# send raw binary message and capture response
printf '\x57\x45\x41\x54\x00\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x01' |
    nc -q 1 localhost 8080 | xxd
```

### protocol analyzer

```c
/* print message details for debugging */
void debug_print_message(const struct ws_header *header, const void *payload)
{
        printf("magic:      0x%08x (%c%c%c%c)\n",
                header->magic,
                (header->magic >> 24) & 0xff,
                (header->magic >> 16) & 0xff,
                (header->magic >> 8) & 0xff,
                header->magic & 0xff);
        printf("version:    %d\n", header->version);
        printf("type:       0x%04x (%s)\n",
                header->msg_type,
                msg_type_to_string(header->msg_type));
        printf("length:     %u bytes\n", header->payload_len);
        printf("sequence:   %u\n", header->sequence_id);
        
        if (payload && header->payload_len > 0) {
                printf("payload:\n");
                hex_dump(payload, header->payload_len);
        }
}
```

---

*next: [udp discovery protocol](udp_protocol.md)*
