package integration_test

import (
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

	// start ingestion service
	s.StartService("ingestion", services.ServiceConfig{
		Binary: "./ws-ingest",
		Config: "./config/s1.ini",
		Daemon: true,
	})

	// start aggregation service
	s.StartService("aggregation", services.ServiceConfig{
		Binary: "./ws-aggregate",
		Config: "./config/s2.ini",
		Daemon: true,
	})

	// start query service
	s.StartService("query", services.ServiceConfig{
		Binary: "./ws-query",
		Config: "./config/s3.ini",
		Ports: map[string]int{
			"http": 8080,
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

	// create test csv in NOAA format
	csvContent := `USW00094728,20240101,TMAX,55,,,W
USW00094728,20240101,TMIN,32,,,W
USW00094728,20240101,PRCP,0,,,W`

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
	s.AssertTableExists("daily_aggregates")

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

	// ingest test data in NOAA format
	csvContent := `USW00094728,20240101,TMAX,55,,,W
USW00094728,20240102,TMAX,60,,,W
USW00094728,20240103,TMAX,58,,,W`

	csvPath := filepath.Join(s.csvDir, "query.csv")
	os.WriteFile(csvPath, []byte(csvContent), 0644)

	time.Sleep(3 * time.Second)

	// test health endpoint
	resp, err := http.Get("http://localhost:8080/health")
	testlib.NoError(t, err, "should connect to query service")
	testlib.Equal(t, 200, resp.StatusCode, "should return 200")
	resp.Body.Close()

	// test stations endpoint
	resp, err = http.Get("http://localhost:8080/api/v1/stations")
	testlib.NoError(t, err, "should get stations")
	testlib.Equal(t, 200, resp.StatusCode, "should return 200")
	resp.Body.Close()
}

// testdiscovery tests peer discovery
func TestIntegration_Discovery(t *testing.T) {
	s := newtestendtoendpipeline(t)
	s.setup()
	defer s.teardown()

	// start discovery service
	s.StartService("discovery", services.ServiceConfig{
		Binary: "./ws-discovery",
		Config: "./config/s4.ini",
	})

	time.Sleep(3 * time.Second)

	// discovery service should be running
	svc, exists := s.SvcManager.GetService("discovery")
	testlib.True(t, exists, "discovery service should exist")
	testlib.True(t, svc.Health.Healthy, "discovery service should be healthy")
}

// testconcurrentqueries tests concurrent query handling
func TestIntegration_ConcurrentQueries(t *testing.T) {
	s := newtestendtoendpipeline(t)
	s.setup()
	defer s.teardown()

	// ingest test data
	csvContent := `USW00094728,20240101,TMAX,55,,,W
USW00094728,20240101,TMIN,32,,,W
USW00094728,20240101,PRCP,0,,,W`

	csvPath := filepath.Join(s.csvDir, "concurrent.csv")
	os.WriteFile(csvPath, []byte(csvContent), 0644)

	time.Sleep(3 * time.Second)

	// run concurrent queries to health endpoint
	concurrency := 5
	results := make(chan int, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			resp, err := http.Get("http://localhost:8080/health")
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
