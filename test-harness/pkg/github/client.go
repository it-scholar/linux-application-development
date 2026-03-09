// Package github provides GitHub API integration
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client provides GitHub API operations
type Client struct {
	token      string
	repository string
	apiURL     string
	httpClient *http.Client
}

// Config holds GitHub client configuration
type Config struct {
	Token      string
	Repository string
	APIURL     string
}

// NewClient creates a new GitHub client
func NewClient(config Config) *Client {
	apiURL := config.APIURL
	if apiURL == "" {
		apiURL = "https://api.github.com"
	}

	return &Client{
		token:      config.Token,
		repository: config.Repository,
		apiURL:     apiURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// CheckRun represents a GitHub check run
type CheckRun struct {
	Name        string
	Status      string // queued, in_progress, completed
	Conclusion  string // success, failure, neutral, cancelled, timed_out, action_required
	Title       string
	Summary     string
	Text        string
	StartedAt   time.Time
	CompletedAt time.Time
}

// CreateCheckRun creates a new check run
func (c *Client) CreateCheckRun(ctx context.Context, sha string, check *CheckRun) (int64, error) {
	url := fmt.Sprintf("%s/repos/%s/check-runs", c.apiURL, c.repository)

	payload := map[string]interface{}{
		"name":     check.Name,
		"head_sha": sha,
		"status":   check.Status,
	}

	if check.Status == "completed" {
		payload["conclusion"] = check.Conclusion
		payload["completed_at"] = check.CompletedAt.Format(time.RFC3339)
	}

	if check.StartedAt.IsZero() {
		payload["started_at"] = check.StartedAt.Format(time.RFC3339)
	}

	if check.Title != "" {
		payload["output"] = map[string]interface{}{
			"title":   check.Title,
			"summary": check.Summary,
			"text":    check.Text,
		}
	}

	resp, err := c.doRequest(ctx, "POST", url, payload)
	if err != nil {
		return 0, err
	}

	var result struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return 0, err
	}

	return result.ID, nil
}

// UpdateCheckRun updates an existing check run
func (c *Client) UpdateCheckRun(ctx context.Context, checkRunID int64, check *CheckRun) error {
	url := fmt.Sprintf("%s/repos/%s/check-runs/%d", c.apiURL, c.repository, checkRunID)

	payload := map[string]interface{}{
		"name":   check.Name,
		"status": check.Status,
	}

	if check.Status == "completed" {
		payload["conclusion"] = check.Conclusion
		payload["completed_at"] = check.CompletedAt.Format(time.RFC3339)
	}

	if check.Title != "" {
		payload["output"] = map[string]interface{}{
			"title":   check.Title,
			"summary": check.Summary,
			"text":    check.Text,
		}
	}

	_, err := c.doRequest(ctx, "PATCH", url, payload)
	return err
}

// PRComment represents a pull request comment
type PRComment struct {
	Body string
}

// CreatePRComment creates a comment on a pull request
func (c *Client) CreatePRComment(ctx context.Context, prNumber int, comment *PRComment) error {
	url := fmt.Sprintf("%s/repos/%s/issues/%d/comments", c.apiURL, c.repository, prNumber)

	payload := map[string]string{
		"body": comment.Body,
	}

	_, err := c.doRequest(ctx, "POST", url, payload)
	return err
}

// CreateCheckComment creates a comment via the checks API (better formatting)
func (c *Client) CreateCheckComment(ctx context.Context, prNumber int, check *CheckRun) error {
	// Get the PR's head SHA
	sha, err := c.GetPRHeadSHA(ctx, prNumber)
	if err != nil {
		return err
	}

	_, err = c.CreateCheckRun(ctx, sha, check)
	return err
}

// GetPRHeadSHA gets the head commit SHA for a pull request
func (c *Client) GetPRHeadSHA(ctx context.Context, prNumber int) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/pulls/%d", c.apiURL, c.repository, prNumber)

	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	var result struct {
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}

	return result.Head.SHA, nil
}

// PostTestResults posts test results as a PR comment
func (c *Client) PostTestResults(ctx context.Context, prNumber int, results *TestResults) error {
	body := formatTestResultsComment(results)
	return c.CreatePRComment(ctx, prNumber, &PRComment{Body: body})
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

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
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
		return nil, fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(respBody))
	}

	return respBody, nil
}
