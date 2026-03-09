// Package report provides test result reporting capabilities
package report

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Reporter generates test reports in various formats
type Reporter struct {
	suite TestSuite
}

// TestSuite represents a collection of test results
type TestSuite struct {
	Name       string
	Tests      []TestCase
	Duration   time.Duration
	Timestamp  time.Time
	Properties map[string]string
}

// TestCase represents a single test result
type TestCase struct {
	Name      string
	ClassName string
	Duration  time.Duration
	Status    TestStatus
	Message   string
	Details   string
	Timestamp time.Time
}

// TestStatus represents test execution status
type TestStatus int

const (
	StatusPass TestStatus = iota
	StatusFail
	StatusSkip
	StatusError
)

func (s TestStatus) String() string {
	switch s {
	case StatusPass:
		return "passed"
	case StatusFail:
		return "failed"
	case StatusSkip:
		return "skipped"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// NewReporter creates a new test reporter
func NewReporter(suiteName string) *Reporter {
	return &Reporter{
		suite: TestSuite{
			Name:       suiteName,
			Tests:      make([]TestCase, 0),
			Timestamp:  time.Now(),
			Properties: make(map[string]string),
		},
	}
}

// AddTest adds a test result to the suite
func (r *Reporter) AddTest(name, className string, duration time.Duration, status TestStatus, message string) {
	r.suite.Tests = append(r.suite.Tests, TestCase{
		Name:      name,
		ClassName: className,
		Duration:  duration,
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
	})
}

// AddPass adds a passing test
func (r *Reporter) AddPass(name, className string, duration time.Duration) {
	r.AddTest(name, className, duration, StatusPass, "")
}

// AddFail adds a failing test
func (r *Reporter) AddFail(name, className string, duration time.Duration, message string) {
	r.AddTest(name, className, duration, StatusFail, message)
}

// AddSkip adds a skipped test
func (r *Reporter) AddSkip(name, className string, message string) {
	r.AddTest(name, className, 0, StatusSkip, message)
}

// AddError adds a test with an error
func (r *Reporter) AddError(name, className string, duration time.Duration, message string) {
	r.AddTest(name, className, duration, StatusError, message)
}

// SetProperty sets a suite property
func (r *Reporter) SetProperty(key, value string) {
	r.suite.Properties[key] = value
}

// SetDuration sets the total suite duration
func (r *Reporter) SetDuration(d time.Duration) {
	r.suite.Duration = d
}

// GetSuite returns the test suite
func (r *Reporter) GetSuite() TestSuite {
	return r.suite
}

// GetStats returns test statistics
func (r *Reporter) GetStats() TestStats {
	stats := TestStats{
		Total:   len(r.suite.Tests),
		Passed:  0,
		Failed:  0,
		Skipped: 0,
		Errors:  0,
	}

	for _, test := range r.suite.Tests {
		switch test.Status {
		case StatusPass:
			stats.Passed++
		case StatusFail:
			stats.Failed++
		case StatusSkip:
			stats.Skipped++
		case StatusError:
			stats.Errors++
		}
	}

	return stats
}

// TestStats holds test result statistics
type TestStats struct {
	Total   int
	Passed  int
	Failed  int
	Skipped int
	Errors  int
}

// SuccessRate returns the percentage of passing tests
func (s TestStats) SuccessRate() float64 {
	if s.Total == 0 {
		return 0
	}
	return float64(s.Passed) / float64(s.Total) * 100
}

// ExportJUnit exports results in JUnit XML format
func (r *Reporter) ExportJUnit(outputPath string) error {
	stats := r.GetStats()

	junit := JUnitTestSuite{
		Name:      r.suite.Name,
		Tests:     stats.Total,
		Failures:  stats.Failed,
		Errors:    stats.Errors,
		Skipped:   stats.Skipped,
		Time:      formatDuration(r.suite.Duration),
		Timestamp: r.suite.Timestamp.Format(time.RFC3339),
		Properties: JUnitProperties{
			Property: make([]JUnitProperty, 0, len(r.suite.Properties)),
		},
		TestCases: make([]JUnitTestCase, 0, len(r.suite.Tests)),
	}

	// Add properties
	for k, v := range r.suite.Properties {
		junit.Properties.Property = append(junit.Properties.Property, JUnitProperty{
			Name:  k,
			Value: v,
		})
	}

	// Add test cases
	for _, test := range r.suite.Tests {
		jtc := JUnitTestCase{
			Name:      test.Name,
			ClassName: test.ClassName,
			Time:      formatDuration(test.Duration),
		}

		switch test.Status {
		case StatusFail:
			jtc.Failure = &JUnitFailure{
				Message: test.Message,
				Type:    "AssertionError",
			}
		case StatusError:
			jtc.Error = &JUnitError{
				Message: test.Message,
				Type:    "Error",
			}
		case StatusSkip:
			jtc.Skipped = &JUnitSkipped{}
		}

		junit.TestCases = append(junit.TestCases, jtc)
	}

	// Create output directory if needed
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Marshal to XML
	output := JUnitTestSuites{
		Name:     r.suite.Name,
		Tests:    stats.Total,
		Failures: stats.Failed,
		Errors:   stats.Errors,
		Suites:   []JUnitTestSuite{junit},
	}

	data, err := xml.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal junit xml: %w", err)
	}

	// Write to file
	header := []byte(xml.Header)
	content := append(header, data...)
	content = append(content, '\n')

	if err := os.WriteFile(outputPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write junit report: %w", err)
	}

	return nil
}

// ExportJSON exports results in JSON format
func (r *Reporter) ExportJSON(outputPath string) error {
	stats := r.GetStats()

	result := JSONReport{
		Suite: JSONSuite{
			Name:       r.suite.Name,
			Timestamp:  r.suite.Timestamp.Format(time.RFC3339),
			Duration:   r.suite.Duration.Seconds(),
			Properties: r.suite.Properties,
		},
		Stats: JSONStats{
			Total:       stats.Total,
			Passed:      stats.Passed,
			Failed:      stats.Failed,
			Skipped:     stats.Skipped,
			Errors:      stats.Errors,
			SuccessRate: stats.SuccessRate(),
		},
		Tests: make([]JSONTest, 0, len(r.suite.Tests)),
	}

	for _, test := range r.suite.Tests {
		result.Tests = append(result.Tests, JSONTest{
			Name:      test.Name,
			ClassName: test.ClassName,
			Status:    test.Status.String(),
			Duration:  test.Duration.Seconds(),
			Message:   test.Message,
			Timestamp: test.Timestamp.Format(time.RFC3339),
		})
	}

	// Create output directory if needed
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}

	if err := os.WriteFile(outputPath, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("failed to write json report: %w", err)
	}

	return nil
}

// ExportHTML exports results in HTML format
func (r *Reporter) ExportHTML(outputPath string) error {
	stats := r.GetStats()

	html := generateHTMLReport(r.suite, stats)

	// Create output directory if needed
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	if err := os.WriteFile(outputPath, []byte(html), 0644); err != nil {
		return fmt.Errorf("failed to write html report: %w", err)
	}

	return nil
}

// JUnit XML structures
type JUnitTestSuites struct {
	XMLName  xml.Name         `xml:"testsuites"`
	Name     string           `xml:"name,attr"`
	Tests    int              `xml:"tests,attr"`
	Failures int              `xml:"failures,attr"`
	Errors   int              `xml:"errors,attr"`
	Suites   []JUnitTestSuite `xml:"testsuite"`
}

type JUnitTestSuite struct {
	Name       string          `xml:"name,attr"`
	Tests      int             `xml:"tests,attr"`
	Failures   int             `xml:"failures,attr"`
	Errors     int             `xml:"errors,attr"`
	Skipped    int             `xml:"skipped,attr"`
	Time       string          `xml:"time,attr"`
	Timestamp  string          `xml:"timestamp,attr"`
	Properties JUnitProperties `xml:"properties"`
	TestCases  []JUnitTestCase `xml:"testcase"`
}

type JUnitProperties struct {
	Property []JUnitProperty `xml:"property"`
}

type JUnitProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type JUnitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *JUnitFailure `xml:"failure,omitempty"`
	Error     *JUnitError   `xml:"error,omitempty"`
	Skipped   *JUnitSkipped `xml:"skipped,omitempty"`
}

type JUnitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

type JUnitError struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

type JUnitSkipped struct{}

// JSON structures
type JSONReport struct {
	Suite JSONSuite  `json:"suite"`
	Stats JSONStats  `json:"statistics"`
	Tests []JSONTest `json:"tests"`
}

type JSONSuite struct {
	Name       string            `json:"name"`
	Timestamp  string            `json:"timestamp"`
	Duration   float64           `json:"duration_seconds"`
	Properties map[string]string `json:"properties"`
}

type JSONStats struct {
	Total       int     `json:"total"`
	Passed      int     `json:"passed"`
	Failed      int     `json:"failed"`
	Skipped     int     `json:"skipped"`
	Errors      int     `json:"errors"`
	SuccessRate float64 `json:"success_rate_percent"`
}

type JSONTest struct {
	Name      string  `json:"name"`
	ClassName string  `json:"class_name"`
	Status    string  `json:"status"`
	Duration  float64 `json:"duration_seconds"`
	Message   string  `json:"message,omitempty"`
	Timestamp string  `json:"timestamp"`
}

// Helper functions
func formatDuration(d time.Duration) string {
	return fmt.Sprintf("%.3f", d.Seconds())
}

func generateHTMLReport(suite TestSuite, stats TestStats) string {
	// Determine overall status
	var statusClass string
	if stats.Failed > 0 || stats.Errors > 0 {
		statusClass = "failed"
	} else if stats.Skipped > 0 {
		statusClass = "skipped"
	} else {
		statusClass = "passed"
	}
	_ = statusClass // Used for future styling

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Test Report: %s</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #333; margin-bottom: 10px; }
        .subtitle { color: #666; margin-bottom: 30px; }
        .summary { display: grid; grid-template-columns: repeat(5, 1fr); gap: 15px; margin-bottom: 30px; }
        .stat-box { padding: 20px; border-radius: 8px; text-align: center; }
        .stat-box.total { background: #e3f2fd; }
        .stat-box.passed { background: #e8f5e9; }
        .stat-box.failed { background: #ffebee; }
        .stat-box.skipped { background: #fff3e0; }
        .stat-box.errors { background: #fce4ec; }
        .stat-number { font-size: 32px; font-weight: bold; display: block; }
        .stat-label { font-size: 14px; color: #666; text-transform: uppercase; }
        .test-list { margin-top: 30px; }
        .test-item { padding: 15px; margin-bottom: 10px; border-radius: 6px; display: flex; justify-content: space-between; align-items: center; }
        .test-item.passed { background: #e8f5e9; border-left: 4px solid #4caf50; }
        .test-item.failed { background: #ffebee; border-left: 4px solid #f44336; }
        .test-item.skipped { background: #fff3e0; border-left: 4px solid #ff9800; }
        .test-item.error { background: #fce4ec; border-left: 4px solid #e91e63; }
        .test-name { font-weight: 500; }
        .test-class { color: #666; font-size: 12px; }
        .test-duration { color: #999; font-size: 12px; }
        .status-badge { padding: 4px 12px; border-radius: 12px; font-size: 12px; font-weight: 500; text-transform: uppercase; }
        .status-badge.passed { background: #4caf50; color: white; }
        .status-badge.failed { background: #f44336; color: white; }
        .status-badge.skipped { background: #ff9800; color: white; }
        .status-badge.error { background: #e91e63; color: white; }
        .test-message { margin-top: 10px; padding: 10px; background: rgba(0,0,0,0.05); border-radius: 4px; font-family: monospace; font-size: 12px; }
        .progress-bar { height: 8px; background: #e0e0e0; border-radius: 4px; margin-top: 20px; overflow: hidden; }
        .progress-fill { height: 100%%; background: #4caf50; border-radius: 4px; }
        .timestamp { color: #999; font-size: 12px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Test Report</h1>
        <div class="subtitle">%s</div>
        
        <div class="summary">
            <div class="stat-box total">
                <span class="stat-number">%d</span>
                <span class="stat-label">Total</span>
            </div>
            <div class="stat-box passed">
                <span class="stat-number">%d</span>
                <span class="stat-label">Passed</span>
            </div>
            <div class="stat-box failed">
                <span class="stat-number">%d</span>
                <span class="stat-label">Failed</span>
            </div>
            <div class="stat-box skipped">
                <span class="stat-number">%d</span>
                <span class="stat-label">Skipped</span>
            </div>
            <div class="stat-box errors">
                <span class="stat-number">%d</span>
                <span class="stat-label">Errors</span>
            </div>
        </div>
        
        <div class="progress-bar">
            <div class="progress-fill" style="width: %.1f%%;"></div>
        </div>
        
        <div class="test-list">
            <h2>Test Results</h2>
`, suite.Name, suite.Name, stats.Total, stats.Passed, stats.Failed, stats.Skipped, stats.Errors, stats.SuccessRate())

	for _, test := range suite.Tests {
		status := test.Status.String()
		html += fmt.Sprintf(`            <div class="test-item %s">
                <div>
                    <div class="test-name">%s</div>
                    <div class="test-class">%s</div>
                </div>
                <div style="text-align: right;">
                    <span class="status-badge %s">%s</span>
                    <div class="test-duration">%.3fs</div>
                </div>
            </div>
`, status, test.Name, test.ClassName, status, status, test.Duration.Seconds())

		if test.Message != "" {
			html += fmt.Sprintf(`            <div class="test-message">%s</div>
`, test.Message)
		}
	}

	html += fmt.Sprintf(`        </div>
        
        <div class="timestamp">
            Generated: %s<br>
            Duration: %.3f seconds
        </div>
    </div>
</body>
</html>`, suite.Timestamp.Format(time.RFC3339), suite.Duration.Seconds())

	return html
}
