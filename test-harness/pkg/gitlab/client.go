// Package gitlab provides GitLab API integration
package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client provides GitLab API operations
type Client struct {
	token      string
	project    string
	apiURL     string
	httpClient *http.Client
}

// Config holds GitLab client configuration
type Config struct {
	Token   string
	Project string
	APIURL  string
}

// NewClient creates a new GitLab client
func NewClient(config Config) *Client {
	apiURL := config.APIURL
	if apiURL == "" {
		apiURL = "https://gitlab.com/api/v4"
	}

	return &Client{
		token:      config.Token,
		project:    config.Project,
		apiURL:     apiURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// PipelineStatus represents a pipeline status update
type PipelineStatus struct {
	State       string // success, failed, canceled, running
	Name        string
	Description string
	TargetURL   string
}

// UpdateCommitStatus updates the commit status (equivalent to GitHub checks)
func (c *Client) UpdateCommitStatus(ctx context.Context, sha string, status *PipelineStatus) error {
	url := fmt.Sprintf("%s/projects/%s/statuses/%s", c.apiURL, encodePath(c.project), sha)

	payload := map[string]string{
		"state":       status.State,
		"name":        status.Name,
		"description": status.Description,
	}

	if status.TargetURL != "" {
		payload["target_url"] = status.TargetURL
	}

	_, err := c.doRequest(ctx, "POST", url, payload)
	return err
}

// MRComment represents a merge request comment
type MRComment struct {
	Body string
}

// CreateMRComment creates a comment on a merge request
func (c *Client) CreateMRComment(ctx context.Context, mrIID int, comment *MRComment) error {
	url := fmt.Sprintf("%s/projects/%s/merge_requests/%d/notes", c.apiURL, encodePath(c.project), mrIID)

	payload := map[string]string{
		"body": comment.Body,
	}

	_, err := c.doRequest(ctx, "POST", url, payload)
	return err
}

// GetMRHeadSHA gets the head commit SHA for a merge request
func (c *Client) GetMRHeadSHA(ctx context.Context, mrIID int) (string, error) {
	url := fmt.Sprintf("%s/projects/%s/merge_requests/%d", c.apiURL, encodePath(c.project), mrIID)

	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	var result struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}

	return result.SHA, nil
}

// PostTestResults posts test results as an MR comment
func (c *Client) PostTestResults(ctx context.Context, mrIID int, results *TestResults) error {
	body := formatTestResultsComment(results)
	return c.CreateMRComment(ctx, mrIID, &MRComment{Body: body})
}

// TestResults holds test execution results
type TestResults struct {
	TotalTests   int
	PassedTests  int
	FailedTests  int
	SkippedTests int
	Duration     time.Duration
	Score        int
	MaxScore     int
	Grade        string
	ReportURL    string
}

func formatTestResultsComment(results *TestResults) string {
	status := "✅ PASSED"
	if results.FailedTests > 0 || results.Score < 60 {
		status = "❌ FAILED"
	}

	body := fmt.Sprintf(`## 🧪 Test Harness Results

**Status:** %s

### Summary
- **Tests:** %d passed, %d failed, %d skipped (%d total)
- **Duration:** %s
- **Score:** %d/%d (%s)

### Grade: %s
`, status, results.PassedTests, results.FailedTests, results.SkippedTests,
		results.TotalTests, results.Duration, results.Score, results.MaxScore,
		formatPercentage(results.Score, results.MaxScore), results.Grade)

	if results.ReportURL != "" {
		body += fmt.Sprintf("\n📊 [View Full Report](%s)\n", results.ReportURL)
	}

	return body
}

func formatPercentage(score, max int) string {
	if max == 0 {
		return "0%"
	}
	return fmt.Sprintf("%.1f%%", float64(score)/float64(max)*100)
}

func encodePath(path string) string {
	// Simple URL encoding for project path
	// In production, use proper URL encoding
	return path
}

func (c *Client) doRequest(ctx context.Context, method, url string, payload interface{}) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitLab API error: %s - %s", resp.Status, string(respBody))
	}

	return respBody, nil
}
