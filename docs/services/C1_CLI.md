# c1: cli client

## overview

the cli client (c1) provides an interactive command-line interface for querying weather data from local and remote weather stations. it connects to the query service and uses the discovery service to locate peers.

## responsibilities

- connect to local query service via tcp
- query remote stations via discovery service
- provide interactive repl mode
- format and display query results
- support configuration management
- export results in multiple formats (table, csv, json)

## architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      cli client                                  │
│                                                                  │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │   argument   │    │   config     │    │   repl       │      │
│  │   parser     │───►│   loader     │───►│   engine     │      │
│  └──────────────┘    └──────────────┘    └──────┬───────┘      │
│                                                  │               │
│  ┌──────────────┐    ┌──────────────┐           │               │
│  │   output     │◄───┤   command    │◄──────────┘               │
│  │   formatter  │    │   router     │                           │
│  └──────────────┘    └──────┬───────┘                           │
│                             │                                   │
│          ┌──────────────────┼──────────────────┐                │
│          │                  │                  │                │
│          ▼                  ▼                  ▼                │
│    ┌──────────┐      ┌──────────┐      ┌──────────┐            │
│    │  local   │      │  remote  │      │ config   │            │
│    │  query   │      │  query   │      │ commands │            │
│    └────┬─────┘      └────┬─────┘      └──────────┘            │
│         │                 │                                     │
│         │ tcp             │ discovery + tcp                     │
│         │                 │                                     │
│    ┌────┴─────────────────┴────┐                               │
│    │      network layer        │                               │
│    │   (with mtls support)     │                               │
│    └─────────────┬─────────────┘                               │
│                  │                                              │
│  ┌───────────────┼───────────────┐                             │
│  │               │               │                             │
│  ▼               ▼               ▼                             │
│ ┌────┐      ┌────────┐      ┌────────┐                        │
│ │ s3 │      │   s4   │      │ remote │                        │
│ │query│      │discovery│     │stations│                        │
│ └────┘      └────────┘      └────────┘                        │
│                                                                │
└─────────────────────────────────────────────────────────────────┘
```

## configuration

### configuration file (cli.ini)

```ini
[connection]
query_host = localhost
query_port = 8080
discovery_host = localhost
discovery_port = 5000
timeout_seconds = 30

[output]
default_format = table
max_rows = 1000
pretty_print = true
show_timestamps = true

[display]
date_format = "%y-%m-%d %h:%m:%s"
temperature_unit = celsius
locale = en_us.utf-8

[history]
enabled = true
file = ~/.ws_cli_history
max_entries = 1000
```

### environment variables

```bash
ws_cli_config=~/.config/ws/cli.ini
ws_query_host=localhost
ws_query_port=8080
ws_output_format=csv
```

### command-line arguments

```bash
ws-cli [options] [command]

options:
  -c, --config path        configuration file path
  -h, --host host          query service host
  -p, --port port          query service port
  -f, --format format      output format (table|csv|json)
  -o, --output file        output to file
  -i, --interactive        start repl mode
  -v, --verbose            verbose output
  --help                   show help message

commands:
  query                    execute query
  status                   show station status
  peers                    list peer stations
  config                   manage configuration
  help                     show help

query syntax:
  ws-cli query [options]
    --station id           station id (0=all)
    --from timestamp       start time
    --to timestamp         end time
    --metrics list         comma-separated metrics
    --aggregation type     none|hourly|daily
```

## commands

### query command

```bash
# basic query - last 24 hours
ws-cli query --from "-1 day" --to now

# query specific station
ws-cli query --station 2 --from 2024-01-01 --to 2024-01-31

# query specific metrics
ws-cli query --metrics temperature,humidity --from "-1 hour"

# aggregated data
ws-cli query --aggregation hourly --from "-7 days"

# output to csv
ws-cli query --from 2024-01-01 --to 2024-01-31 --format csv -o output.csv

# json output
ws-cli query --station 3 --format json | jq '.records | length'
```

### status command

```bash
# show local station status
ws-cli status

# output:
station id: 1
hostname: station1.example.com
uptime: 3 days, 14:32:10
role: leader
data range: 2024-01-01 00:00:00 to 2024-01-31 23:59:59
total records: 1,543,200
health: healthy
services:
  - ingestion: running (pid 1234)
  - aggregation: running (pid 1235)
  - query: running (pid 1236)
  - discovery: running (pid 1237)
```

### peers command

```bash
# list all peer stations
ws-cli peers

# output:
id  hostname              ip address      port  status   data range
--  --------              ----------      ----  ------   ----------
2   station2.example.com  192.168.1.102   8080  healthy  2024-01-01 - 2024-01-31
3   station3.example.com  192.168.1.103   8080  healthy  2024-01-01 - 2024-01-31
4   station4.example.com  192.168.1.104   8080  unknown  - -
```

### interactive repl mode

```bash
$ ws-cli -i

weather station cli v1.0.0
connected to localhost:8080
type 'help' for available commands.

ws> help
available commands:
  query [options]     execute query
  status              show station status
  peers               list peer stations
  config              show/set configuration
  history             show command history
  exit, quit          exit cli

ws> query --station 2 --from "-1 hour" --metrics temperature
┌─────────────────────┬─────────────┐
│ timestamp           │ temperature │
├─────────────────────┼─────────────┤
│ 2024-01-31 14:30:00 │ 15.5°c      │
│ 2024-01-31 14:35:00 │ 15.7°c      │
│ 2024-01-31 14:40:00 │ 15.8°c      │
└─────────────────────┴─────────────┘
3 rows in set (0.012 sec)

ws> peers
id  hostname              status
--  --------              ------
2   station2.example.com  healthy
3   station3.example.com  healthy

ws> query --station 3 --aggregation daily --from "-7 days" --format json
{
  "station_id": 3,
  "aggregation": "daily",
  "records": [
    {
      "date": "2024-01-25",
      "temperature_min": 10.2,
      "temperature_max": 18.5,
      "temperature_avg": 14.3
    },
    ...
  ]
}

ws> exit
goodbye!
```

## implementation details

### connection management

```c
struct cli_connection {
        int sockfd;
        ssl *ssl;                       /* mtls after day 4 */
        char host[256];
        int port;
        int connected;
};

int cli_connect(struct cli_connection *conn, const char *host, int port)
{
        conn->sockfd = socket(af_inet, sock_stream, 0);
        if (conn->sockfd < 0)
                return -1;
        
        struct sockaddr_in addr = {
                .sin_family = af_inet,
                .sin_port = htons(port)
        };
        
        if (inet_pton(af_inet, host, &addr.sin_addr) <= 0) {
                /* try dns resolution */
                struct hostent *he = gethostbyname(host);
                if (!he) {
                        close(conn->sockfd);
                        return -1;
                }
                memcpy(&addr.sin_addr, he->h_addr, he->h_length);
        }
        
        if (connect(conn->sockfd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
                close(conn->sockfd);
                return -1;
        }
        
        strncpy(conn->host, host, sizeof(conn->host));
        conn->port = port;
        conn->connected = 1;
        
        return 0;
}

void cli_disconnect(struct cli_connection *conn)
{
        if (conn->connected) {
                close(conn->sockfd);
                conn->connected = 0;
        }
}
```

### protocol client

```c
int send_query_request(struct cli_connection *conn,
        struct ws_query_req *query,
        void (*callback)(struct ws_record *record, void *user_data),
        void *user_data)
{
        struct ws_header header = {
                .magic = htonl(ws_magic),
                .version = htons(ws_version),
                .msg_type = htons(ws_msg_query),
                .payload_len = htonl(sizeof(*query)),
                .sequence_id = htonl(get_next_sequence_id())
        };
        
        /* send header */
        if (send_all(conn->sockfd, &header, sizeof(header)) < 0)
                return -1;
        
        /* send query */
        if (send_all(conn->sockfd, query, sizeof(*query)) < 0)
                return -1;
        
        /* receive response */
        struct ws_header resp_header;
        if (recv_all(conn->sockfd, &resp_header, sizeof(resp_header)) < 0)
                return -1;
        
        /* handle streaming response */
        if (ntohs(resp_header.msg_type) == ws_msg_query_resp) {
                struct ws_query_resp resp;
                recv_all(conn->sockfd, &resp, sizeof(resp));
                
                uint32_t record_count = ntohl(resp.total_records);
                for (uint32_t i = 0; i < record_count; i++) {
                        struct ws_record record;
                        recv_all(conn->sockfd, &record, sizeof(record));
                        callback(&record, user_data);
                }
        }
        
        return 0;
}
```

### output formatters

#### table format

```c
void format_table(struct ws_record *records, int count)
{
        /* calculate column widths */
        int ts_width = 19;  /* yyyy-mm-dd hh:mm:ss */
        int temp_width = 12;
        int hum_width = 10;
        
        /* print header */
        printf("┌%.*s┬%.*s┬%.*s┐\n",
                ts_width + 2, "─────────────────────",
                temp_width + 2, "────────────",
                hum_width + 2, "──────────");
        
        printf("│ %-*s │ %-*s │ %-*s │\n",
                ts_width, "timestamp",
                temp_width, "temperature",
                hum_width, "humidity");
        
        printf("├%.*s┼%.*s┼%.*s┤\n",
                ts_width + 2, "─────────────────────",
                temp_width + 2, "────────────",
                hum_width + 2, "──────────");
        
        /* print rows */
        for (int i = 0; i < count; i++) {
                char ts_str[20];
                format_timestamp(records[i].timestamp, ts_str, sizeof(ts_str));
                
                printf("│ %-*s │ %*.1f°c │ %*.1f%% │\n",
                        ts_width, ts_str,
                        temp_width - 2, records[i].temperature,
                        hum_width - 1, records[i].humidity);
        }
        
        printf("└%.*s┴%.*s┴%.*s┘\n",
                ts_width + 2, "─────────────────────",
                temp_width + 2, "────────────",
                hum_width + 2, "──────────");
        
        printf("\n%d rows in set\n", count);
}
```

#### csv format

```c
void format_csv(struct ws_record *records, int count, file *fp)
{
        fprintf(fp, "timestamp,temperature,humidity,pressure,wind_speed\n");
        
        for (int i = 0; i < count; i++) {
                char ts_str[20];
                format_timestamp_csv(records[i].timestamp, ts_str, sizeof(ts_str));
                
                fprintf(fp, "%s,%.2f,%.2f,%.2f,%.2f\n",
                        ts_str,
                        records[i].temperature,
                        records[i].humidity,
                        records[i].pressure,
                        records[i].wind_speed);
        }
}
```

#### json format

```c
void format_json(struct ws_record *records, int count, file *fp)
{
        fprintf(fp, "{\n");
        fprintf(fp, "  \"count\": %d,\n", count);
        fprintf(fp, "  \"records\": [\n");
        
        for (int i = 0; i < count; i++) {
                char ts_str[20];
                format_timestamp_iso8601(records[i].timestamp, ts_str, sizeof(ts_str));
                
                fprintf(fp, "    {\n");
                fprintf(fp, "      \"timestamp\": \"%s\",\n", ts_str);
                fprintf(fp, "      \"temperature\": %.2f,\n", records[i].temperature);
                fprintf(fp, "      \"humidity\": %.2f,\n", records[i].humidity);
                fprintf(fp, "      \"pressure\": %.2f,\n", records[i].pressure);
                fprintf(fp, "      \"wind_speed\": %.2f\n", records[i].wind_speed);
                fprintf(fp, "    }%s\n", (i < count - 1) ? "," : "");
        }
        
        fprintf(fp, "  ]\n");
        fprintf(fp, "}\n");
}
```

### repl implementation

```c
int repl_loop(struct cli_config *config)
{
        char *line;
        
        printf("weather station cli v1.0.0\n");
        printf("connected to %s:%d\n", config->query_host, config->query_port);
        printf("type 'help' for available commands.\n\n");
        
        while ((line = readline("ws> ")) != null) {
                if (strlen(line) == 0) {
                        free(line);
                        continue;
                }
                
                add_history(line);
                
                /* parse command */
                char *argv[64];
                int argc = tokenize(line, argv, 64);
                
                if (argc == 0) {
                        free(line);
                        continue;
                }
                
                /* execute command */
                if (strcmp(argv[0], "exit") == 0 || strcmp(argv[0], "quit") == 0) {
                        free(line);
                        break;
                } else if (strcmp(argv[0], "help") == 0) {
                        show_help();
                } else if (strcmp(argv[0], "query") == 0) {
                        execute_query(argc, argv, config);
                } else if (strcmp(argv[0], "status") == 0) {
                        show_status(config);
                } else if (strcmp(argv[0], "peers") == 0) {
                        list_peers(config);
                } else {
                        printf("unknown command: %s\n", argv[0]);
                }
                
                free(line);
        }
        
        printf("goodbye!\n");
        return 0;
}
```

## error handling

```c
void handle_cli_error(int error_code, const char *context)
{
        switch (error_code) {
        case ws_error_network:
                fprintf(stderr, "error: unable to connect to %s\n", context);
                fprintf(stderr, "check that the query service is running.\n");
                break;
        case ws_error_timeout:
                fprintf(stderr, "error: connection timed out\n");
                break;
        case ws_error_protocol:
                fprintf(stderr, "error: protocol version mismatch\n");
                break;
        case ws_error_not_found:
                fprintf(stderr, "error: station not found\n");
                break;
        default:
                fprintf(stderr, "error: %s\n", context);
                break;
        }
}
```

## troubleshooting

### common issues

| symptom | cause | solution |
|---------|-------|----------|
| connection refused | query service not running | start ws-query service |
| timeout | network issue or high load | check connectivity, retry |
| permission denied | mtls certificate issue | verify certificates |
| no peers found | discovery not working | check udp port 5000 |
| invalid format | output format typo | use table, csv, or json |

### diagnostic commands

```bash
# test connectivity
curl http://localhost:9090/health

# check configuration
cat ~/.config/ws/cli.ini

# test raw connection
echo "ping" | nc localhost 8080

# list available stations
ws-cli peers
```

---

*next: [binary protocol specification](../protocols/binary_protocol.md)*
