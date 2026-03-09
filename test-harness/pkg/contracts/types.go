package contracts

// contract represents a service contract definition
type Contract struct {
	Service      string      `yaml:"service"`
	Version      string      `yaml:"version"`
	Description  string      `yaml:"description"`
	Interfaces   Interfaces  `yaml:"interfaces"`
	Performance  Performance `yaml:"performance"`
	Requirements []string    `yaml:"requirements"`
}

// interfaces section
type Interfaces struct {
	Signals    SignalConfig     `yaml:"signals"`
	POSIXMQ    POSIXMQConfig    `yaml:"posix_mq"`
	Filesystem FilesystemConfig `yaml:"filesystem"`
	Database   DatabaseConfig   `yaml:"database"`
	Network    NetworkConfig    `yaml:"network,omitempty"`
}

// signal configuration
type SignalConfig struct {
	SIGTERM SignalHandler `yaml:"sigterm"`
	SIGHUP  SignalHandler `yaml:"sighup"`
	SIGUSR1 SignalHandler `yaml:"sigusr1,omitempty"`
	SIGUSR2 SignalHandler `yaml:"sigusr2,omitempty"`
}

type SignalHandler struct {
	Behavior           string `yaml:"behavior"`
	TimeoutSeconds     int    `yaml:"timeout_seconds"`
	ExpectedExitCode   int    `yaml:"expected_exit_code"`
	ExpectedLogMessage string `yaml:"expected_log_message,omitempty"`
	ExpectedAction     string `yaml:"expected_action,omitempty"`
}

// posix mq configuration
type POSIXMQConfig struct {
	QueueName     string                 `yaml:"queue_name"`
	MessageFormat map[string]interface{} `yaml:"message_format,omitempty"`
}

// filesystem configuration
type FilesystemConfig struct {
	WatchDirectory     DirectoryConfig `yaml:"watch_directory"`
	ProcessedDirectory DirectoryConfig `yaml:"processed_directory"`
	ErrorDirectory     DirectoryConfig `yaml:"error_directory"`
}

type DirectoryConfig struct {
	Path   string   `yaml:"path"`
	Events []string `yaml:"events,omitempty"`
}

// database configuration
type DatabaseConfig struct {
	Driver        string   `yaml:"driver"`
	TablesWritten []string `yaml:"tables_written"`
	TablesRead    []string `yaml:"tables_read,omitempty"`
	WALMode       string   `yaml:"wal_mode"`
	Isolation     string   `yaml:"isolation,omitempty"`
	ReadOnly      bool     `yaml:"read_only,omitempty"`
}

// network configuration
type NetworkConfig struct {
	TCP TCPConfig `yaml:"tcp,omitempty"`
	UDP UDPConfig `yaml:"udp,omitempty"`
}

type TCPConfig struct {
	BindAddress string `yaml:"bind_address,omitempty"`
	Port        int    `yaml:"port,omitempty"`
	Protocol    string `yaml:"protocol,omitempty"`
}

type UDPConfig struct {
	Broadcast UDPBroadcastConfig `yaml:"broadcast,omitempty"`
}

type UDPBroadcastConfig struct {
	Enabled         bool `yaml:"enabled,omitempty"`
	Port            int  `yaml:"port,omitempty"`
	IntervalSeconds int  `yaml:"interval_seconds,omitempty"`
}

// performance requirements
type Performance struct {
	ThroughputMBPS            MetricConfig `yaml:"throughput_mbps,omitempty"`
	LatencyFirstRecordSeconds MetricConfig `yaml:"latency_first_record_seconds,omitempty"`
	MemoryMaxMB               MetricConfig `yaml:"memory_max_mb,omitempty"`
	QueryLatencyMS            MetricConfig `yaml:"query_latency_ms,omitempty"`
	ConcurrentConnections     MetricConfig `yaml:"concurrent_connections,omitempty"`
}

type MetricConfig struct {
	Target  float64 `yaml:"target"`
	Minimum float64 `yaml:"minimum,omitempty"`
	Maximum float64 `yaml:"maximum,omitempty"`
}
