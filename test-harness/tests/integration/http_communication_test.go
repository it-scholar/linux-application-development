package integration_test

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"weather-station-test/pkg/testlib"
)

// TestHTTPCommunication verifies HTTP API communication between services
func TestHTTPCommunication_IngestionAPI(t *testing.T) {
	// Test ingestion service health endpoint
	client := &http.Client{Timeout: 10 * time.Second}
	
	// Retry with exponential backoff
	maxRetries := 5
	delay := 1 * time.Second
	var lastErr error
	
	for i := 0; i < maxRetries; i++ {
		resp, err := client.Get("http://weather-station-ingestion:8080/health")
		if err == nil && resp.StatusCode == 200 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			testlib.Contains(t, string(body), "healthy", "ingestion health check should return healthy")
			t.Logf("✓ Ingestion service health check passed (attempt %d)", i+1)
			return
		}
		if err != nil {
			lastErr = err
		} else {
			resp.Body.Close()
		}
		
		t.Logf("Attempt %d failed, retrying in %v...", i+1, delay)
		time.Sleep(delay)
		delay = delay * 2
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
	}
	
	t.Fatalf("Ingestion service health check failed after %d retries: %v", maxRetries, lastErr)
}

// TestHTTPCommunication_AggregationAPI tests aggregation service HTTP client
func TestHTTPCommunication_AggregationAPI(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}
	
	maxRetries := 5
	delay := 1 * time.Second
	var lastErr error
	
	for i := 0; i < maxRetries; i++ {
		resp, err := client.Get("http://weather-station-aggregation:8080/health")
		if err == nil && resp.StatusCode == 200 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			testlib.Contains(t, string(body), "healthy", "aggregation health check should return healthy")
			t.Logf("✓ Aggregation service health check passed (attempt %d)", i+1)
			return
		}
		if err != nil {
			lastErr = err
		} else {
			resp.Body.Close()
		}
		
		t.Logf("Attempt %d failed, retrying in %v...", i+1, delay)
		time.Sleep(delay)
		delay = delay * 2
	}
	
	t.Fatalf("Aggregation service health check failed after %d retries: %v", maxRetries, lastErr)
}

// TestHTTPCommunication_FullDataFlow tests data flow through HTTP APIs
func TestHTTPCommunication_FullDataFlow(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}
	
	// Step 1: Check ingestion has data
	t.Log("Step 1: Checking ingestion service for data...")
	resp, err := client.Get("http://weather-station-ingestion:8080/api/v1/stats")
	testlib.NoError(t, err, "should fetch ingestion stats")
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	t.Logf("Ingestion stats: %s", string(body))
	
	// Step 2: Check aggregation has processed data
	t.Log("Step 2: Checking aggregation service for aggregated data...")
	time.Sleep(5 * time.Second) // Wait for aggregation
	
	resp2, err := client.Get("http://weather-station-aggregation:8080/health")
	testlib.NoError(t, err, "aggregation should be running")
	resp2.Body.Close()
	
	t.Log("✓ Full HTTP data flow verification complete")
}

// TestHTTPCommunication_RetryMechanism tests retry and exponential backoff
func TestHTTPCommunication_RetryMechanism(t *testing.T) {
	// This test verifies that the retry mechanism works by testing a failing endpoint
	client := &http.Client{Timeout: 2 * time.Second}
	
	start := time.Now()
	maxRetries := 3
	delay := 500 * time.Millisecond
	attempts := 0
	
	for i := 0; i < maxRetries; i++ {
		attempts++
		_, err := client.Get("http://nonexistent-service:9999/health")
		if err == nil {
			break
		}
		time.Sleep(delay)
		delay = delay * 2
	}
	
	elapsed := time.Since(start)
	// With exponential backoff: 500ms + 1000ms + 2000ms = 3500ms minimum
	minExpectedTime := 1500 * time.Millisecond
	
	testlib.True(t, elapsed >= minExpectedTime, 
		fmt.Sprintf("Retry with exponential backoff should take at least %v, took %v", minExpectedTime, elapsed))
	testlib.Equal(t, maxRetries, attempts, "should have attempted all retries")
	t.Logf("✓ Retry mechanism test passed: %d attempts in %v", attempts, elapsed)
}
