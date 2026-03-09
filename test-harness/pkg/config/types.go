package config

import (
	"time"
)

// global configuration
type globalconfig struct {
	timeout  duration      `mapstructure:"timeout"`
	retries  int           `mapstructure:"retries"`
	parallel int           `mapstructure:"parallel"`
	output   outputconfig  `mapstructure:"output"`
	logging  loggingconfig `mapstructure:"logging"`
}

type outputconfig struct {
	format    string `mapstructure:"format"`
	verbose   bool   `mapstructure:"verbose"`
	colors    bool   `mapstructure:"colors"`
	timestamp bool   `mapstructure:"timestamp"`
}

type loggingconfig struct {
	level string `mapstructure:"level"`
	file  string `mapstructure:"file"`
}

// service configuration
type serviceconfig struct {
	name        string            `mapstructure:"name"`
	binary      string            `mapstructure:"binary"`
	config      string            `mapstructure:"config"`
	timeout     duration          `mapstructure:"timeout"`
	dependson   []string          `mapstructure:"depends_on"`
	healthcheck healthcheckconfig `mapstructure:"health_check"`
	ports       portconfig        `mapstructure:"ports"`
	resources   resourceconfig    `mapstructure:"resources"`
	networkmode string            `mapstructure:"network_mode"`
}

type healthcheckconfig struct {
	typ      string   `mapstructure:"type"`
	endpoint string   `mapstructure:"endpoint"`
	interval duration `mapstructure:"interval"`
	retries  int      `mapstructure:"retries"`
}

type portconfig struct {
	query       int `mapstructure:"query"`
	metrics     int `mapstructure:"metrics"`
	beacon      int `mapstructure:"beacon"`
	health      int `mapstructure:"health"`
	replication int `mapstructure:"replication"`
}

type resourceconfig struct {
	memorymax string  `mapstructure:"memory_max"`
	cpumax    float64 `mapstructure:"cpu_max"`
}

// testcontainers configuration
type testcontainersconfig struct {
	enabled  bool           `mapstructure:"enabled"`
	provider string         `mapstructure:"provider"`
	database databaseconfig `mapstructure:"database"`
}

type databaseconfig struct {
	driver       string `mapstructure:"driver"`
	freshpertest bool   `mapstructure:"fresh_per_test"`
	tempdir      string `mapstructure:"temp_dir"`
}

// grading configuration
type gradingconfig struct {
	mustpass   []string                  `mapstructure:"must_pass"`
	categories map[string]categoryconfig `mapstructure:"categories"`
	thresholds thresholdconfig           `mapstructure:"thresholds"`
}

type categoryconfig struct {
	weight   int                        `mapstructure:"weight"`
	criteria map[string]criterionconfig `mapstructure:"criteria"`
}

type criterionconfig struct {
	points int    `mapstructure:"points"`
	check  string `mapstructure:"check"`
	test   string `mapstructure:"test,omitempty"`
	target int    `mapstructure:"target,omitempty"`
	unit   string `mapstructure:"unit,omitempty"`
}

type thresholdconfig struct {
	distinction int `mapstructure:"distinction"`
	merit       int `mapstructure:"merit"`
	pass        int `mapstructure:"pass"`
}

// chaos configuration
type chaosconfig struct {
	scenarios map[string]scenarioconfig `mapstructure:"scenarios"`
}

type scenarioconfig struct {
	description string       `mapstructure:"description"`
	duration    duration     `mapstructure:"duration"`
	steps       []stepconfig `mapstructure:"steps"`
}

type stepconfig struct {
	action   string            `mapstructure:"action"`
	target   string            `mapstructure:"target,omitempty"`
	duration duration          `mapstructure:"duration,omitempty"`
	query    string            `mapstructure:"query,omitempty"`
	expected string            `mapstructure:"expected,omitempty"`
	params   map[string]string `mapstructure:"params,omitempty"`
}

// protocol configuration
type protocolconfig struct {
	versions       []string `mapstructure:"versions"`
	fuzziterations int      `mapstructure:"fuzz_iterations"`
	propertytests  int      `mapstructure:"property_tests"`
}

// ci/cd configuration
type ciconfig struct {
	github githubconfig `mapstructure:"github"`
	gitlab gitlabconfig `mapstructure:"gitlab"`
}

type githubconfig struct {
	enabled         bool   `mapstructure:"enabled"`
	postprcomments  bool   `mapstructure:"post_pr_comments"`
	commenttemplate string `mapstructure:"comment_template"`
}

type gitlabconfig struct {
	enabled         bool   `mapstructure:"enabled"`
	postmrcomments  bool   `mapstructure:"post_mr_comments"`
	commenttemplate string `mapstructure:"comment_template"`
}

// noaa data configuration
type noaaconfig struct {
	baseurl      string   `mapstructure:"base_url"`
	cachedir     string   `mapstructure:"cache_dir"`
	cacheenabled bool     `mapstructure:"cache_enabled"`
	timeout      duration `mapstructure:"timeout"`
	parallel     int      `mapstructure:"parallel_downloads"`
}

// duration is a wrapper for time.Duration that supports yaml unmarshaling
type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

func (d duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}
