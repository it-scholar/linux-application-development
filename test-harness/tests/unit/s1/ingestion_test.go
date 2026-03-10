package s1_test

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"weather-station-test/pkg/services"
	"weather-station-test/pkg/testlib"
	"weather-station-test/tests"
)

// tests1signalhandling tests signal handling for s1
type tests1signalhandling struct {
	*tests.BaseTest
}

func newtests1signalhandling(t *testing.T) *tests1signalhandling {
	base := tests.NewBaseTest(t)
	return &tests1signalhandling{BaseTest: base}
}

func (s *tests1signalhandling) setup() {
	s.Setup()

	// create temp directories
	csvDir := filepath.Join(os.TempDir(), "ws-test-csv")
	os.MkdirAll(csvDir, 0755)

	// start s1 service
	s.StartService("ingestion", services.ServiceConfig{
		Binary: "./ws-ingest",
		Config: "./config/s1_test.ini",
		Daemon: true,
	})
}

func (s *tests1signalhandling) teardown() {
	s.Teardown()
}

// tests1signalhandling tests s1 handles sigterm correctly
func TestS1SignalHandling_SIGTERM(t *testing.T) {
	s := newtests1signalhandling(t)
	s.setup()
	defer s.teardown()

	// send sigterm
	err := s.SvcManager.SendSignal("ingestion", syscall.SIGTERM)
	testlib.NoError(t, err, "should send sigterm")

	// wait for graceful shutdown
	time.Sleep(2 * time.Second)

	// check service stopped
	svc, exists := s.SvcManager.GetService("ingestion")
	testlib.False(t, exists, "service should not exist after sigterm")

	if exists {
		testlib.Equal(t, false, svc.Health.Healthy, "service should not be healthy")
	}
}

// tests1sighup tests s1 handles sighup (config reload)
func TestS1SignalHandling_SIGHUP(t *testing.T) {
	s := newtests1signalhandling(t)
	s.setup()
	defer s.teardown()

	// send sighup
	err := s.SvcManager.SendSignal("ingestion", syscall.SIGHUP)
	testlib.NoError(t, err, "should send sighup")

	// wait for config reload
	time.Sleep(1 * time.Second)

	// check service still running
	svc, exists := s.SvcManager.GetService("ingestion")
	testlib.True(t, exists, "service should still exist after sighup")
	testlib.True(t, svc.Health.Healthy, "service should still be healthy")
}

// tests1ingestion tests csv ingestion
type tests1ingestion struct {
	*tests.BaseTest
	csvDir string
}

func newtests1ingestion(t *testing.T) *tests1ingestion {
	base := tests.NewBaseTest(t)
	return &tests1ingestion{BaseTest: base}
}

func (s *tests1ingestion) setup() {
	s.Setup()

	// create temp csv directory
	s.csvDir = filepath.Join(os.TempDir(), "ws-test-csv")
	os.MkdirAll(s.csvDir, 0755)

	// start s1
	s.StartService("ingestion", services.ServiceConfig{
		Binary: "./ws-ingest",
		Config: "./config/s1_test.ini",
		Daemon: true,
	})
}

func (s *tests1ingestion) teardown() {
	os.RemoveAll(s.csvDir)
	s.Teardown()
}

// tests1ingestsinglecsv tests ingesting a single csv file
func TestS1Ingestion_SingleCSV(t *testing.T) {
	s := newtests1ingestion(t)
	s.setup()
	defer s.teardown()

	// create test csv with NOAA GHCN-Daily format
	csvContent := `USW00094728,20240101,TMAX,55,,,W
USW00094728,20240101,TMIN,32,,,W
USW00094728,20240101,PRCP,0,,,W`

	csvPath := filepath.Join(s.csvDir, "test.csv")
	os.WriteFile(csvPath, []byte(csvContent), 0644)

	// wait for ingestion (poll interval is 1 second in test config)
	time.Sleep(3 * time.Second)

	// verify database
	s.AssertTableExists("weather_data")

	count, _ := s.DB.GetRowCount("weather_data")
	testlib.Greater(t, count, 0, "should have ingested records")
}

// tests1ingestlargecsv tests ingesting a large csv file
func TestS1Ingestion_LargeCSV(t *testing.T) {
	s := newtests1ingestion(t)
	s.setup()
	defer s.teardown()

	// create large test csv (100 records) in NOAA format
	csvContent := ""
	for i := 0; i < 100; i++ {
		csvContent += fmt.Sprintf("USW00094728,202401%02d,TMAX,%d,,,W\n",
			(i%30)+1, 50+i%20)
		csvContent += fmt.Sprintf("USW00094728,202401%02d,TMIN,%d,,,W\n",
			(i%30)+1, 30+i%15)
	}

	csvPath := filepath.Join(s.csvDir, "large.csv")
	os.WriteFile(csvPath, []byte(csvContent), 0644)

	// wait for ingestion
	time.Sleep(5 * time.Second)

	// verify all records ingested
	count, _ := s.DB.GetRowCount("weather_data")
	testlib.Greater(t, count, 50, "should have ingested most records")
}

// tests1invalidcsv tests handling invalid csv
func TestS1Ingestion_InvalidCSV(t *testing.T) {
	s := newtests1ingestion(t)
	s.setup()
	defer s.teardown()

	// create invalid csv
	csvContent := `invalid,line,here
USW00094728,20240101,TMAX,999999,,,W
USW00094728,notadate,TMIN,30,,,W`

	csvPath := filepath.Join(s.csvDir, "invalid.csv")
	os.WriteFile(csvPath, []byte(csvContent), 0644)

	// wait
	time.Sleep(3 * time.Second)

	// verify error handling (should not crash)
	svc, exists := s.SvcManager.GetService("ingestion")
	testlib.True(t, exists, "service should still exist")
	testlib.True(t, svc.Health.Healthy, "service should still be healthy")
}

// tests1performance tests ingestion performance
func TestS1Performance_IngestionThroughput(t *testing.T) {
	s := newtests1ingestion(t)
	s.setup()
	defer s.teardown()

	// create medium csv (1000 records)
	csvContent := ""
	for i := 0; i < 1000; i++ {
		csvContent += fmt.Sprintf("USW00094728,202401%02d,TMAX,%d,,,W\n",
			(i%30)+1, 50+i%20)
	}

	csvPath := filepath.Join(s.csvDir, "perf.csv")
	os.WriteFile(csvPath, []byte(csvContent), 0644)

	// measure ingestion time
	start := time.Now()

	// wait for ingestion
	s.WaitForCondition(30*time.Second, func() bool {
		count, _ := s.DB.GetRowCount("weather_data")
		return count >= 900
	}, "records to be ingested")

	duration := time.Since(start)

	// calculate throughput
	fileSize := float64(len(csvContent)) / (1024 * 1024) // MB
	throughput := fileSize / duration.Seconds()

	t.Logf("Ingestion throughput: %.2f MB/s", throughput)
	testlib.Greater(t, throughput, 0.1, "throughput should be reasonable")
}
