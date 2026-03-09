#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <signal.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <fcntl.h>
#include <time.h>
#include <errno.h>
#include <ctype.h>
#include <stdarg.h>

#define VERSION "1.0.0"
#define DEFAULT_CONFIG_FILE "discovery.ini"
#define DEFAULT_PID_FILE "/tmp/discovery.pid"
#define BUFFER_SIZE 4096
#define MAX_NODES 32
#define MAX_NODE_ID_LEN 64

/* Node states */
typedef enum {
    NODE_STATE_FOLLOWER = 0,
    NODE_STATE_CANDIDATE,
    NODE_STATE_LEADER
} NodeState;

/* Node information */
typedef struct {
    char id[MAX_NODE_ID_LEN];
    char address[64];
    int port;
    time_t last_seen;
    int is_healthy;
    int is_leader;
} NodeInfo;

/* Configuration structure */
typedef struct {
    char node_id[MAX_NODE_ID_LEN];
    char bind_address[64];
    int discovery_port;
    char log_file[256];
    int log_level;
    int daemon_mode;
    int election_timeout_ms;
    int heartbeat_interval_ms;
    NodeInfo known_nodes[MAX_NODES];
    int known_node_count;
} Config;

/* Global state */
typedef struct {
    Config config;
    volatile int running;
    volatile int reload_config;
    int udp_socket;
    NodeState state;
    time_t last_election;
    time_t last_heartbeat;
    NodeInfo cluster_nodes[MAX_NODES];
    int node_count;
    FILE *log_fp;
} State;

static State g_state = {0};

/* Log levels */
enum {
    LOG_DEBUG = 0,
    LOG_INFO,
    LOG_WARN,
    LOG_ERROR
};

/* Message types */
#define MSG_DISCOVER "DISCOVER"
#define MSG_HEARTBEAT "HEARTBEAT"
#define MSG_ELECTION "ELECTION"
#define MSG_COORDINATOR "COORDINATOR"

/* Function prototypes */
static void log_message(int level, const char *format, ...);
static int parse_config(const char *filename, Config *config);
static int init_udp_socket(void);
static void send_discovery_message(void);
static void send_heartbeat(void);
static void handle_discovery_message(const char *msg, struct sockaddr_in *from);
static void start_election(void);
static void handle_election_message(const char *sender_id);
static void handle_coordinator_message(const char *leader_id);
static void check_leader_health(void);
static void signal_handler(int sig);
static void cleanup(void);
static int write_pid_file(const char *pid_file);
static void remove_pid_file(const char *pid_file);

/* Logging function */
static void log_message(int level, const char *format, ...) {
    if (level < g_state.config.log_level) return;
    
    const char *level_str[] = {"DEBUG", "INFO", "WARN", "ERROR"};
    time_t now = time(NULL);
    struct tm *tm_info = localtime(&now);
    char timestamp[26];
    strftime(timestamp, 26, "%Y-%m-%d %H:%M:%S", tm_info);
    
    FILE *output = g_state.log_fp ? g_state.log_fp : stdout;
    
    fprintf(output, "[%s] [%s] ", timestamp, level_str[level]);
    
    va_list args;
    va_start(args, format);
    vfprintf(output, format, args);
    va_end(args);
    
    fprintf(output, "\n");
    fflush(output);
}

/* Parse configuration file */
static int parse_config(const char *filename, Config *config) {
    FILE *fp = fopen(filename, "r");
    if (!fp) {
        log_message(LOG_ERROR, "Cannot open config file: %s", filename);
        return -1;
    }
    
    char line[BUFFER_SIZE];
    while (fgets(line, sizeof(line), fp)) {
        char *comment = strchr(line, '#');
        if (comment) *comment = '\0';
        
        int len = strlen(line);
        while (len > 0 && isspace(line[len-1])) {
            line[--len] = '\0';
        }
        
        if (len == 0) continue;
        
        char *key = line;
        char *equals = strchr(line, '=');
        if (!equals) continue;
        
        *equals = '\0';
        char *value = equals + 1;
        
        while (isspace(*key)) key++;
        while (isspace(*value)) value++;
        while (len > 0 && isspace(key[len-1])) key[--len] = '\0';
        
        if (strcmp(key, "node_id") == 0) {
            strncpy(config->node_id, value, sizeof(config->node_id) - 1);
        } else if (strcmp(key, "bind_address") == 0) {
            strncpy(config->bind_address, value, sizeof(config->bind_address) - 1);
        } else if (strcmp(key, "discovery_port") == 0) {
            config->discovery_port = atoi(value);
        } else if (strcmp(key, "log_file") == 0) {
            strncpy(config->log_file, value, sizeof(config->log_file) - 1);
        } else if (strcmp(key, "log_level") == 0) {
            if (strcmp(value, "debug") == 0) config->log_level = LOG_DEBUG;
            else if (strcmp(value, "info") == 0) config->log_level = LOG_INFO;
            else if (strcmp(value, "warn") == 0) config->log_level = LOG_WARN;
            else if (strcmp(value, "error") == 0) config->log_level = LOG_ERROR;
        } else if (strcmp(key, "daemon_mode") == 0) {
            config->daemon_mode = (strcmp(value, "true") == 0 || strcmp(value, "1") == 0);
        } else if (strcmp(key, "election_timeout_ms") == 0) {
            config->election_timeout_ms = atoi(value);
        } else if (strcmp(key, "heartbeat_interval_ms") == 0) {
            config->heartbeat_interval_ms = atoi(value);
        } else if (strcmp(key, "cluster_node") == 0 && config->known_node_count < MAX_NODES) {
            /* Parse node format: id:address:port */
            char *colon1 = strchr(value, ':');
            char *colon2 = colon1 ? strchr(colon1 + 1, ':') : NULL;
            if (colon1 && colon2) {
                *colon1 = '\0';
                *colon2 = '\0';
                strncpy(config->known_nodes[config->known_node_count].id, value, MAX_NODE_ID_LEN - 1);
                strncpy(config->known_nodes[config->known_node_count].address, colon1 + 1, 63);
                config->known_nodes[config->known_node_count].port = atoi(colon2 + 1);
                config->known_node_count++;
            }
        }
    }
    
    fclose(fp);
    
    /* Set defaults */
    if (config->node_id[0] == '\0') {
        snprintf(config->node_id, sizeof(config->node_id), "node-%d", getpid());
    }
    if (config->bind_address[0] == '\0') {
        strcpy(config->bind_address, "0.0.0.0");
    }
    if (config->discovery_port == 0) {
        config->discovery_port = 9000;
    }
    if (config->election_timeout_ms == 0) {
        config->election_timeout_ms = 5000;
    }
    if (config->heartbeat_interval_ms == 0) {
        config->heartbeat_interval_ms = 1000;
    }
    
    log_message(LOG_INFO, "Configuration loaded from %s", filename);
    return 0;
}

/* Initialize UDP socket */
static int init_udp_socket(void) {
    int sock = socket(AF_INET, SOCK_DGRAM, 0);
    if (sock < 0) {
        log_message(LOG_ERROR, "Failed to create UDP socket: %s", strerror(errno));
        return -1;
    }
    
    /* Allow socket reuse */
    int opt = 1;
    if (setsockopt(sock, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt)) < 0) {
        log_message(LOG_ERROR, "Failed to set socket options: %s", strerror(errno));
        close(sock);
        return -1;
    }
    
    struct sockaddr_in address;
    memset(&address, 0, sizeof(address));
    address.sin_family = AF_INET;
    address.sin_addr.s_addr = inet_addr(g_state.config.bind_address);
    address.sin_port = htons(g_state.config.discovery_port);
    
    if (bind(sock, (struct sockaddr *)&address, sizeof(address)) < 0) {
        log_message(LOG_ERROR, "Failed to bind to %s:%d: %s",
                   g_state.config.bind_address, g_state.config.discovery_port, strerror(errno));
        close(sock);
        return -1;
    }
    
    /* Set non-blocking */
    int flags = fcntl(sock, F_GETFL, 0);
    fcntl(sock, F_SETFL, flags | O_NONBLOCK);
    
    g_state.udp_socket = sock;
    
    log_message(LOG_INFO, "UDP socket listening on %s:%d",
               g_state.config.bind_address, g_state.config.discovery_port);
    
    return 0;
}

/* Send discovery message */
static void send_discovery_message(void) {
    char msg[256];
    snprintf(msg, sizeof(msg), "%s:%s", MSG_DISCOVER, g_state.config.node_id);
    
    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_port = htons(g_state.config.discovery_port);
    
    /* Send to broadcast address */
    addr.sin_addr.s_addr = inet_addr("255.255.255.255");
    sendto(g_state.udp_socket, msg, strlen(msg), 0, 
           (struct sockaddr *)&addr, sizeof(addr));
    
    /* Send to known nodes */
    for (int i = 0; i < g_state.config.known_node_count; i++) {
        addr.sin_addr.s_addr = inet_addr(g_state.config.known_nodes[i].address);
        addr.sin_port = htons(g_state.config.known_nodes[i].port);
        sendto(g_state.udp_socket, msg, strlen(msg), 0,
               (struct sockaddr *)&addr, sizeof(addr));
    }
    
    log_message(LOG_DEBUG, "Sent discovery message");
}

/* Send heartbeat */
static void send_heartbeat(void) {
    char msg[256];
    int is_leader = (g_state.state == NODE_STATE_LEADER) ? 1 : 0;
    snprintf(msg, sizeof(msg), "%s:%s:%d", MSG_HEARTBEAT, g_state.config.node_id, is_leader);
    
    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_port = htons(g_state.config.discovery_port);
    
    /* Send to all known cluster nodes */
    for (int i = 0; i < g_state.node_count; i++) {
        if (strcmp(g_state.cluster_nodes[i].id, g_state.config.node_id) != 0) {
            addr.sin_addr.s_addr = inet_addr(g_state.cluster_nodes[i].address);
            addr.sin_port = htons(g_state.cluster_nodes[i].port);
            sendto(g_state.udp_socket, msg, strlen(msg), 0,
                   (struct sockaddr *)&addr, sizeof(addr));
        }
    }
    
    g_state.last_heartbeat = time(NULL);
}

/* Handle discovery message */
static void handle_discovery_message(const char *msg, struct sockaddr_in *from) {
    char node_id[MAX_NODE_ID_LEN];
    if (sscanf(msg, "DISCOVER:%63s", node_id) != 1) return;
    
    /* Don't process our own messages */
    if (strcmp(node_id, g_state.config.node_id) == 0) return;
    
    log_message(LOG_DEBUG, "Received discovery from %s", node_id);
    
    /* Add or update node */
    int found = 0;
    for (int i = 0; i < g_state.node_count; i++) {
        if (strcmp(g_state.cluster_nodes[i].id, node_id) == 0) {
            g_state.cluster_nodes[i].last_seen = time(NULL);
            g_state.cluster_nodes[i].is_healthy = 1;
            found = 1;
            break;
        }
    }
    
    if (!found && g_state.node_count < MAX_NODES) {
        strncpy(g_state.cluster_nodes[g_state.node_count].id, node_id, MAX_NODE_ID_LEN - 1);
        strncpy(g_state.cluster_nodes[g_state.node_count].address, 
                inet_ntoa(from->sin_addr), 63);
        g_state.cluster_nodes[g_state.node_count].port = ntohs(from->sin_port);
        g_state.cluster_nodes[g_state.node_count].last_seen = time(NULL);
        g_state.cluster_nodes[g_state.node_count].is_healthy = 1;
        g_state.node_count++;
        log_message(LOG_INFO, "New node discovered: %s", node_id);
    }
}

/* Start leader election (Bully algorithm) */
static void start_election(void) {
    log_message(LOG_INFO, "Starting leader election");
    
    g_state.state = NODE_STATE_CANDIDATE;
    g_state.last_election = time(NULL);
    
    /* Send election message to nodes with higher IDs */
    int higher_nodes = 0;
    char msg[256];
    snprintf(msg, sizeof(msg), "%s:%s", MSG_ELECTION, g_state.config.node_id);
    
    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_port = htons(g_state.config.discovery_port);
    
    for (int i = 0; i < g_state.node_count; i++) {
        if (strcmp(g_state.cluster_nodes[i].id, g_state.config.node_id) > 0) {
            addr.sin_addr.s_addr = inet_addr(g_state.cluster_nodes[i].address);
            addr.sin_port = htons(g_state.cluster_nodes[i].port);
            sendto(g_state.udp_socket, msg, strlen(msg), 0,
                   (struct sockaddr *)&addr, sizeof(addr));
            higher_nodes++;
        }
    }
    
    /* If no higher nodes, become leader */
    if (higher_nodes == 0) {
        g_state.state = NODE_STATE_LEADER;
        log_message(LOG_INFO, "No higher nodes found, becoming leader");
        
        /* Announce leadership */
        snprintf(msg, sizeof(msg), "%s:%s", MSG_COORDINATOR, g_state.config.node_id);
        addr.sin_addr.s_addr = inet_addr("255.255.255.255");
        sendto(g_state.udp_socket, msg, strlen(msg), 0,
               (struct sockaddr *)&addr, sizeof(addr));
    }
}

/* Handle election message */
static void handle_election_message(const char *sender_id) {
    log_message(LOG_DEBUG, "Received election message from %s", sender_id);
    
    /* If we have higher ID, respond and start our own election */
    if (strcmp(g_state.config.node_id, sender_id) > 0) {
        log_message(LOG_INFO, "Higher ID than %s, starting election", sender_id);
        start_election();
    }
}

/* Handle coordinator message */
static void handle_coordinator_message(const char *leader_id) {
    log_message(LOG_INFO, "New leader elected: %s", leader_id);
    
    if (strcmp(leader_id, g_state.config.node_id) == 0) {
        g_state.state = NODE_STATE_LEADER;
    } else {
        g_state.state = NODE_STATE_FOLLOWER;
        
        /* Update leader in node list */
        for (int i = 0; i < g_state.node_count; i++) {
            if (strcmp(g_state.cluster_nodes[i].id, leader_id) == 0) {
                g_state.cluster_nodes[i].is_leader = 1;
            } else {
                g_state.cluster_nodes[i].is_leader = 0;
            }
        }
    }
}

/* Check leader health */
static void check_leader_health(void) {
    if (g_state.state == NODE_STATE_LEADER) return;
    
    /* Find current leader */
    time_t now = time(NULL);
    int leader_found = 0;
    
    for (int i = 0; i < g_state.node_count; i++) {
        if (g_state.cluster_nodes[i].is_leader) {
            leader_found = 1;
            if (now - g_state.cluster_nodes[i].last_seen > 
                g_state.config.election_timeout_ms / 1000) {
                log_message(LOG_WARN, "Leader %s is unresponsive, starting election",
                          g_state.cluster_nodes[i].id);
                start_election();
            }
            break;
        }
    }
    
    /* If no leader found, start election */
    if (!leader_found && g_state.node_count > 0) {
        log_message(LOG_INFO, "No leader found, starting election");
        start_election();
    }
}

/* Signal handler */
static void signal_handler(int sig) {
    switch (sig) {
        case SIGTERM:
        case SIGINT:
            log_message(LOG_INFO, "Received signal %d, shutting down...", sig);
            g_state.running = 0;
            break;
        case SIGHUP:
            log_message(LOG_INFO, "Received SIGHUP, reloading configuration...");
            g_state.reload_config = 1;
            break;
    }
}

/* Cleanup function */
static void cleanup(void) {
    log_message(LOG_INFO, "Cleaning up...");
    
    if (g_state.udp_socket > 0) {
        close(g_state.udp_socket);
        g_state.udp_socket = -1;
    }
    
    if (g_state.log_fp && g_state.log_fp != stdout) {
        fclose(g_state.log_fp);
        g_state.log_fp = NULL;
    }
    
    remove_pid_file(DEFAULT_PID_FILE);
}

/* Write PID file */
static int write_pid_file(const char *pid_file) {
    FILE *fp = fopen(pid_file, "w");
    if (!fp) {
        log_message(LOG_ERROR, "Cannot write PID file: %s", pid_file);
        return -1;
    }
    
    fprintf(fp, "%d\n", getpid());
    fclose(fp);
    
    return 0;
}

/* Remove PID file */
static void remove_pid_file(const char *pid_file) {
    unlink(pid_file);
}

/* Main function */
int main(int argc, char *argv[]) {
    const char *config_file = DEFAULT_CONFIG_FILE;
    int validate_only = 0;
    
    /* Parse command line arguments */
    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "--config") == 0 && i + 1 < argc) {
            config_file = argv[++i];
        } else if (strcmp(argv[i], "--daemon") == 0) {
            g_state.config.daemon_mode = 1;
        } else if (strcmp(argv[i], "--validate") == 0) {
            validate_only = 1;
        } else if (strcmp(argv[i], "--help") == 0) {
            printf("Discovery Service v%s\n", VERSION);
            printf("Usage: %s [options]\n", argv[0]);
            printf("Options:\n");
            printf("  --config <file>    Configuration file (default: %s)\n", DEFAULT_CONFIG_FILE);
            printf("  --daemon           Run as daemon\n");
            printf("  --validate         Validate config and exit\n");
            printf("  --help             Show this help\n");
            return 0;
        }
    }
    
    /* Parse configuration */
    if (parse_config(config_file, &g_state.config) != 0) {
        fprintf(stderr, "Failed to parse configuration\n");
        return 1;
    }
    
    /* Validate only mode */
    if (validate_only) {
        printf("Configuration validated successfully\n");
        printf("  Node ID: %s\n", g_state.config.node_id);
        printf("  Bind Address: %s\n", g_state.config.bind_address);
        printf("  Discovery Port: %d\n", g_state.config.discovery_port);
        printf("  Known Nodes: %d\n", g_state.config.known_node_count);
        return 0;
    }
    
    /* Open log file if specified */
    if (g_state.config.log_file[0] != '\0') {
        g_state.log_fp = fopen(g_state.config.log_file, "a");
        if (!g_state.log_fp) {
            fprintf(stderr, "Cannot open log file: %s\n", g_state.config.log_file);
            g_state.log_fp = stdout;
        }
    }
    
    /* Daemon mode */
    if (g_state.config.daemon_mode) {
        pid_t pid = fork();
        if (pid < 0) {
            log_message(LOG_ERROR, "Fork failed");
            return 1;
        }
        if (pid > 0) {
            return 0;
        }
        
        setsid();
        chdir("/");
        
        freopen("/dev/null", "r", stdin);
        freopen("/dev/null", "w", stdout);
        freopen("/dev/null", "w", stderr);
    }
    
    /* Write PID file */
    if (write_pid_file(DEFAULT_PID_FILE) != 0) {
        return 1;
    }
    
    /* Setup signal handlers */
    signal(SIGTERM, signal_handler);
    signal(SIGINT, signal_handler);
    signal(SIGHUP, signal_handler);
    
    /* Initialize UDP socket */
    if (init_udp_socket() != 0) {
        cleanup();
        return 1;
    }
    
    /* Initialize state */
    g_state.state = NODE_STATE_FOLLOWER;
    g_state.running = 1;
    
    log_message(LOG_INFO, "Discovery Service v%s started", VERSION);
    log_message(LOG_INFO, "Node ID: %s", g_state.config.node_id);
    
    /* Send initial discovery */
    send_discovery_message();
    
    /* Main loop */
    time_t last_discovery = time(NULL);
    time_t last_health_check = time(NULL);
    
    while (g_state.running) {
        /* Check for config reload */
        if (g_state.reload_config) {
            g_state.reload_config = 0;
            log_message(LOG_INFO, "Reloading configuration...");
            parse_config(config_file, &g_state.config);
        }
        
        /* Receive and process messages */
        char buffer[BUFFER_SIZE];
        struct sockaddr_in from;
        socklen_t from_len = sizeof(from);
        
        int len = recvfrom(g_state.udp_socket, buffer, sizeof(buffer) - 1, 0,
                          (struct sockaddr *)&from, &from_len);
        
        if (len > 0) {
            buffer[len] = '\0';
            
            if (strncmp(buffer, MSG_DISCOVER, strlen(MSG_DISCOVER)) == 0) {
                handle_discovery_message(buffer, &from);
            } else if (strncmp(buffer, MSG_HEARTBEAT, strlen(MSG_HEARTBEAT)) == 0) {
                /* Update node health */
                char node_id[MAX_NODE_ID_LEN];
                int is_leader;
                if (sscanf(buffer, "HEARTBEAT:%63s:%d", node_id, &is_leader) == 2) {
                    for (int i = 0; i < g_state.node_count; i++) {
                        if (strcmp(g_state.cluster_nodes[i].id, node_id) == 0) {
                            g_state.cluster_nodes[i].last_seen = time(NULL);
                            g_state.cluster_nodes[i].is_healthy = 1;
                            g_state.cluster_nodes[i].is_leader = is_leader;
                            break;
                        }
                    }
                }
            } else if (strncmp(buffer, MSG_ELECTION, strlen(MSG_ELECTION)) == 0) {
                char sender_id[MAX_NODE_ID_LEN];
                if (sscanf(buffer, "ELECTION:%63s", sender_id) == 1) {
                    handle_election_message(sender_id);
                }
            } else if (strncmp(buffer, MSG_COORDINATOR, strlen(MSG_COORDINATOR)) == 0) {
                char leader_id[MAX_NODE_ID_LEN];
                if (sscanf(buffer, "COORDINATOR:%63s", leader_id) == 1) {
                    handle_coordinator_message(leader_id);
                }
            }
        }
        
        /* Periodic tasks */
        time_t now = time(NULL);
        
        /* Send discovery periodically */
        if (now - last_discovery >= 5) {
            send_discovery_message();
            last_discovery = now;
        }
        
        /* Send heartbeat */
        if (now - g_state.last_heartbeat >= g_state.config.heartbeat_interval_ms / 1000) {
            send_heartbeat();
        }
        
        /* Check leader health */
        if (now - last_health_check >= 2) {
            check_leader_health();
            last_health_check = now;
        }
        
        usleep(10000); /* 10ms */
    }
    
    cleanup();
    
    log_message(LOG_INFO, "Discovery Service stopped");
    
    return 0;
}
