package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"weather-station-test/pkg/services"
	"weather-station-test/pkg/testlib"
	"weather-station-test/tests"
)

// testendtoendpipeline tests the full data flow
type testendtoendpipeline struct {
	*tests.BaseTest
	csvDir string
}

func newtestendtoendpipeline(t *testing.T) *testendtoendpipeline {
	base := tests.NewBaseTest(t)
	return &testendtoendpipeline{BaseTest: base}
}

func (s *testendtoendpipeline) setup() {
	s.Setup()

	// create temp csv directory
	s.csvDir = filepath.Join(os.TempDir(), "ws-test-e2e")
	os.MkdirAll(s.csvDir, 0755)

	// start all services
	s.StartService("s1_ingestion", services.ServiceConfig{
		Binary: "./ws-ingest",
		Config: "./config/s1.ini",
		Daemon: true,
	})

	s.StartService("s2_aggregation", services.ServiceConfig{
		Binary: "./ws-aggregate",
		Config: "./config/s2.ini",
		Daemon: true,
	})

	s.StartService("s3_query", services.ServiceConfig{
		Binary: "./ws-query",
		Config: "./config/s3.ini",
		Ports: map[string]int{
			"query":   8080,
			"metrics": 9090,
		},
	})
}

func (s *testendtoendpipeline) teardown() {
	os.RemoveAll(s.csvDir)
	s.Teardown()
}

// testendtoendpipeline tests data flows through all services
func TestEndToEnd_Pipeline(t *testing.T) {
	s := newtestendtoendpipeline(t)
	s.setup()
	defer s.teardown()

	// create test csv
	csvContent := `timestamp,temperature,humidity,pressure
2024-01-01 00:00:00,15.5,65,1013.2
2024-01-01 00:01:00,15.7,66,1013.4
2024-01-01 00:02:00,15.8,67,1013.1`

	csvPath := filepath.Join(s.csvDir, "e2e.csv")
	os.WriteFile(csvPath, []byte(csvContent), 0644)

	// wait for ingestion
	time.Sleep(3 * time.Second)

	// verify data in database
	s.AssertTableExists("weather_data")
	count, _ := s.DB.GetRowCount("weather_data")
	testlib.Greater(t, count, 0, "data should be ingested")

	// wait for aggregation
	time.Sleep(5 * time.Second)

	// verify aggregation
	s.AssertTableExists("hourly_stats")

	// query via http
	resp, err := http.Get("http://localhost:8080/health")
	testlib.NoError(t, err, "should connect to query service")
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	testlib.Contains(t, string(body), "healthy", "health check should return healthy")
}

// testqueryapi tests the query api
func TestIntegration_QueryAPI(t *testing.T) {
	s := newtestendtoendpipeline(t)
	s.setup()
	defer s.teardown()

	// ingest test data
	csvContent := `timestamp,temperature,humidity
2024-01-01 00:00:00,15.5,65
2024-01-01 00:01:00,15.7,66
2024-01-02 00:00:00,16.0,70`

	csvPath := filepath.Join(s.csvDir, "query.csv")
	os.WriteFile(csvPath, []byte(csvContent), 0644)

	time.Sleep(3 * time.Second)

	// test query endpoint
	queryURL := "http://localhost:8080/query?from=2024-01-01&to=2024-01-02"
	resp, err := http.Get(queryURL)
	testlib.NoError(t, err, "should execute query")
	defer resp.Body.Close()

	testlib.Equal(t, 200, resp.StatusCode, "should return 200")

	body, _ := io.ReadAll(resp.Body)
	testlib.Greater(t, len(body), 0, "should return data")
}

// testdiscovery tests peer discovery
func TestIntegration_Discovery(t *testing.T) {
	s := newtestendtoendpipeline(t)
	s.setup()
	defer s.teardown()

	// start discovery service
	s.StartService("s4_discovery", services.ServiceConfig{
		Binary: "./ws-discovery",
		Config: "./config/s4.ini",
	})

	time.Sleep(5 * time.Second)

	// check discovery endpoint
	resp, err := http.Get("http://localhost:9090/health")
	testlib.NoError(t, err, "should connect to discovery")
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	testlib.Contains(t, string(body), "healthy")
}

// testconcurrentqueries tests concurrent query handling
func TestIntegration_ConcurrentQueries(t *testing.T) {
	s := newtestendtoendpipeline(t)
	s.setup()
	defer s.teardown()

	// ingest test data
	csvContent := `timestamp,temperature,humidity
2024-01-01 00:00:00,15.5,65
2024-01-01 00:01:00,15.7,66
2024-01-01 00:02:00,15.8,67`

	csvPath := filepath.Join(s.csvDir, "concurrent.csv")
	os.WriteFile(csvPath, []byte(csvContent), 0644)

	time.Sleep(3 * time.Second)

	// run concurrent queries
	concurrency := 10
	results := make(chan int, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			resp, err := http.Get("http://localhost:8080/query")
			if err != nil {
				results <- 0
				return
			}
			defer resp.Body.Close()
			results <- resp.StatusCode
		}()
	}

	// collect results
	successCount := 0
	for i := 0; i < concurrency; i++ {
		status := <-results
		if status == 200 {
			successCount++
		}
	}

	testlib.Equal(t, concurrency, successCount, "all concurrent queries should succeed")
}

// testmetricsendpoint tests prometheus metrics
func TestIntegration_MetricsEndpoint(t *testing.T) {
	s := newtestendtoendpipeline(t)
	s.setup()
	defer s.teardown()

	// get metrics
	resp, err := http.Get("http://localhost:9090/metrics")
	testlib.NoError(t, err, "should connect to metrics endpoint")
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// check for expected metrics
	testlib.Contains(t, bodyStr, "# HELP", "should have help text")
	testlib.Contains(t, bodyStr, "# TYPE", "should have type text")
}
