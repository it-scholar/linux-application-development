package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"weather-station-test/pkg/services"
	"weather-station-test/pkg/testcontainers"
	"weather-station-test/pkg/testlib"
)

// basetest provides common test functionality
type BaseTest struct {
	T          *testing.T
	Ctx        context.Context
	Cancel     context.CancelFunc
	Logger     *log.Logger
	DB         *testcontainers.Database
	SvcManager *services.Manager
}

// newbasetest creates a new base test
func NewBaseTest(t *testing.T) *BaseTest {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

	logger := log.New(os.Stderr)
	if testing.Verbose() {
		logger.SetLevel(log.DebugLevel)
	} else {
		logger.SetLevel(log.InfoLevel)
	}

	return &BaseTest{
		T:      t,
		Ctx:    ctx,
		Cancel: cancel,
		Logger: logger,
	}
}

// setup initializes the test environment
func (bt *BaseTest) Setup() {
	bt.T.Helper()

	// create fresh database
	db, err := testcontainers.CreateTestDatabase(bt.Ctx, bt.Logger)
	if err != nil {
		bt.T.Fatalf("failed to create test database: %v", err)
	}
	bt.DB = db

	// create service manager
	bt.SvcManager = services.NewManager(bt.Logger)

	bt.Logger.Info("test setup complete")
}

// teardown cleans up the test environment
func (bt *BaseTest) Teardown() {
	bt.T.Helper()

	bt.Cancel()

	if bt.SvcManager != nil {
		bt.SvcManager.StopAll()
	}

	if bt.DB != nil {
		bt.DB.Close()
	}

	bt.Logger.Info("test teardown complete")
}

// startservice starts a service for testing
func (bt *BaseTest) StartService(name string, config services.ServiceConfig) *services.Service {
	bt.T.Helper()

	svc, err := bt.SvcManager.Start(bt.Ctx, name, config)
	if err != nil {
		bt.T.Fatalf("failed to start service %s: %v", name, err)
	}

	// wait for service to be ready
	time.Sleep(500 * time.Millisecond)

	return svc
}

// stopservice stops a service
func (bt *BaseTest) StopService(name string) {
	bt.T.Helper()

	if err := bt.SvcManager.Stop(name); err != nil {
		bt.T.Logf("warning: failed to stop service %s: %v", name, err)
	}
}

// assertrowcount asserts the row count in a table
func (bt *BaseTest) AssertRowCount(table string, expected int) {
	bt.T.Helper()

	count, err := bt.DB.GetRowCount(table)
	if err != nil {
		bt.T.Fatalf("failed to get row count: %v", err)
	}

	testlib.Equal(bt.T, expected, count, fmt.Sprintf("row count in %s", table))
}

// asserttableexists asserts a table exists
func (bt *BaseTest) AssertTableExists(table string) {
	bt.T.Helper()

	exists, err := bt.DB.TableExists(table)
	if err != nil {
		bt.T.Fatalf("failed to check table existence: %v", err)
	}

	testlib.True(bt.T, exists, fmt.Sprintf("table %s should exist", table))
}

// loadfixture loads a test fixture file
func (bt *BaseTest) LoadFixture(filename string) string {
	bt.T.Helper()

	path := filepath.Join("testdata", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		bt.T.Fatalf("failed to load fixture %s: %v", filename, err)
	}

	return string(data)
}

// copycsvfixture copies a csv fixture to the watch directory
func (bt *BaseTest) CopyCSVFixture(filename string, destDir string) string {
	bt.T.Helper()

	srcPath := filepath.Join("testdata", "csv", filename)
	destPath := filepath.Join(destDir, filename)

	data, err := os.ReadFile(srcPath)
	if err != nil {
		bt.T.Fatalf("failed to read csv fixture %s: %v", filename, err)
	}

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		bt.T.Fatalf("failed to write csv to %s: %v", destPath, err)
	}

	return destPath
}

// waitforcondition waits for a condition to be true
func (bt *BaseTest) WaitForCondition(timeout time.Duration, condition func() bool, msg string) {
	bt.T.Helper()

	start := time.Now()
	for {
		if condition() {
			return
		}

		if time.Since(start) > timeout {
			bt.T.Fatalf("timeout waiting for condition: %s", msg)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// logtest logs a test message
func (bt *BaseTest) LogTest(msg string, args ...interface{}) {
	bt.T.Helper()
	bt.T.Logf(msg, args...)
}
