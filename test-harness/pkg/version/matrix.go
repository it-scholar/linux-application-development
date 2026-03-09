// Package version provides version matrix testing capabilities
package version

import (
	"fmt"
	"sync"
)

// Matrix represents a version compatibility matrix
type Matrix struct {
	versions  []Version
	clients   []Client
	compatMap map[string]bool
	results   map[string]TestResult
	mu        sync.RWMutex
}

// Version represents a service version
type Version struct {
	Name   string
	Major  int
	Minor  int
	Patch  int
	Binary string
	Config string
}

func (v Version) String() string {
	return fmt.Sprintf("%s-%d.%d.%d", v.Name, v.Major, v.Minor, v.Patch)
}

// Client represents a client version
type Client struct {
	Name    string
	Version string
	Binary  string
}

func (c Client) String() string {
	return fmt.Sprintf("%s-%s", c.Name, c.Version)
}

// TestResult represents a version compatibility test result
type TestResult struct {
	ServerVersion Version
	ClientVersion Client
	Compatible    bool
	Errors        []string
	TestsPassed   int
	TestsFailed   int
}

// NewMatrix creates a new version matrix
func NewMatrix() *Matrix {
	return &Matrix{
		versions:  make([]Version, 0),
		clients:   make([]Client, 0),
		compatMap: make(map[string]bool),
		results:   make(map[string]TestResult),
	}
}

// AddVersion adds a server version to the matrix
func (m *Matrix) AddVersion(v Version) {
	m.versions = append(m.versions, v)
}

// AddClient adds a client version to the matrix
func (m *Matrix) AddClient(c Client) {
	m.clients = append(m.clients, c)
}

// SetCompatibility sets expected compatibility between versions
func (m *Matrix) SetCompatibility(server Version, client Client, compatible bool) {
	key := fmt.Sprintf("%s:%s", server.String(), client.String())
	m.compatMap[key] = compatible
}

// IsCompatible checks if versions should be compatible
func (m *Matrix) IsCompatible(server Version, client Client) bool {
	key := fmt.Sprintf("%s:%s", server.String(), client.String())
	if compat, ok := m.compatMap[key]; ok {
		return compat
	}
	// Default: same major version is compatible
	return server.Major == extractMajor(client.Version)
}

func extractMajor(version string) int {
	var major int
	fmt.Sscanf(version, "%d", &major)
	return major
}

// TestRunner executes compatibility tests
type TestRunner struct {
	matrix *Matrix
	tests  []CompatibilityTest
}

// CompatibilityTest represents a single compatibility test
type CompatibilityTest interface {
	Name() string
	Run(server Version, client Client) error
}

// NewTestRunner creates a new test runner
func NewTestRunner(matrix *Matrix) *TestRunner {
	return &TestRunner{
		matrix: matrix,
		tests:  make([]CompatibilityTest, 0),
	}
}

// AddTest adds a compatibility test
func (r *TestRunner) AddTest(test CompatibilityTest) {
	r.tests = append(r.tests, test)
}

// Run executes all tests across the version matrix
func (r *TestRunner) Run() ([]TestResult, error) {
	results := make([]TestResult, 0)

	for _, server := range r.matrix.versions {
		for _, client := range r.matrix.clients {
			result := r.runTest(server, client)
			results = append(results, result)

			// Store result
			key := fmt.Sprintf("%s:%s", server.String(), client.String())
			r.matrix.mu.Lock()
			r.matrix.results[key] = result
			r.matrix.mu.Unlock()
		}
	}

	return results, nil
}

func (r *TestRunner) runTest(server Version, client Client) TestResult {
	result := TestResult{
		ServerVersion: server,
		ClientVersion: client,
		Compatible:    true,
		Errors:        make([]string, 0),
	}

	for _, test := range r.tests {
		if err := test.Run(server, client); err != nil {
			result.Compatible = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("%s: %v", test.Name(), err))
			result.TestsFailed++
		} else {
			result.TestsPassed++
		}
	}

	return result
}

// Report generates a version compatibility report
func (m *Matrix) Report(results []TestResult) *CompatibilityReport {
	report := &CompatibilityReport{
		TotalTests:      len(results),
		Passed:          0,
		Failed:          0,
		UnexpectedFails: 0,
		Results:         results,
		Matrix:          make(map[string]map[string]bool),
	}

	for _, result := range results {
		expected := m.IsCompatible(result.ServerVersion, result.ClientVersion)

		if result.Compatible {
			report.Passed++
		} else {
			report.Failed++
			if expected {
				report.UnexpectedFails++
			}
		}

		// Build matrix view
		serverKey := result.ServerVersion.String()
		clientKey := result.ClientVersion.String()

		if report.Matrix[serverKey] == nil {
			report.Matrix[serverKey] = make(map[string]bool)
		}
		report.Matrix[serverKey][clientKey] = result.Compatible
	}

	return report
}

// CompatibilityReport represents a compatibility test report
type CompatibilityReport struct {
	TotalTests      int
	Passed          int
	Failed          int
	UnexpectedFails int
	Results         []TestResult
	Matrix          map[string]map[string]bool
}

// AllPassed returns true if all tests passed
func (r *CompatibilityReport) AllPassed() bool {
	return r.Failed == 0
}

// PassRate returns the pass rate as a percentage
func (r *CompatibilityReport) PassRate() float64 {
	if r.TotalTests == 0 {
		return 0
	}
	return float64(r.Passed) / float64(r.TotalTests) * 100
}

// GetCompatiblePairs returns all compatible version pairs
func (r *CompatibilityReport) GetCompatiblePairs() []TestResult {
	var pairs []TestResult
	for _, result := range r.Results {
		if result.Compatible {
			pairs = append(pairs, result)
		}
	}
	return pairs
}

// GetIncompatiblePairs returns all incompatible version pairs
func (r *CompatibilityReport) GetIncompatiblePairs() []TestResult {
	var pairs []TestResult
	for _, result := range r.Results {
		if !result.Compatible {
			pairs = append(pairs, result)
		}
	}
	return pairs
}

// Example tests

// ProtocolTest tests basic protocol compatibility
type ProtocolTest struct{}

func (t *ProtocolTest) Name() string { return "Protocol Compatibility" }

func (t *ProtocolTest) Run(server Version, client Client) error {
	// Implementation would test if client can connect to server
	return nil
}

// HandshakeTest tests handshake compatibility
type HandshakeTest struct{}

func (t *HandshakeTest) Name() string { return "Handshake" }

func (t *HandshakeTest) Run(server Version, client Client) error {
	// Implementation would test handshake
	return nil
}

// APITest tests API compatibility
type APITest struct{}

func (t *APITest) Name() string { return "API Compatibility" }

func (t *APITest) Run(server Version, client Client) error {
	// Implementation would test API calls
	return nil
}

// DefaultVersions returns default service versions for testing
func DefaultVersions() []Version {
	return []Version{
		{Name: "ingestion", Major: 1, Minor: 0, Patch: 0},
		{Name: "ingestion", Major: 1, Minor: 1, Patch: 0},
		{Name: "ingestion", Major: 2, Minor: 0, Patch: 0},
		{Name: "s2_processor", Major: 1, Minor: 0, Patch: 0},
		{Name: "s2_processor", Major: 1, Minor: 5, Patch: 0},
		{Name: "s3_api", Major: 1, Minor: 0, Patch: 0},
		{Name: "s3_api", Major: 2, Minor: 0, Patch: 0},
		{Name: "s4_cluster", Major: 1, Minor: 0, Patch: 0},
	}
}

// DefaultClients returns default client versions for testing
func DefaultClients() []Client {
	return []Client{
		{Name: "c1_cli", Version: "1.0.0"},
		{Name: "c1_cli", Version: "1.1.0"},
		{Name: "c1_cli", Version: "2.0.0"},
		{Name: "python_client", Version: "1.0.0"},
		{Name: "python_client", Version: "1.5.0"},
	}
}
