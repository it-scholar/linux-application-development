package report

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewReporter(t *testing.T) {
	reporter := NewReporter("Test Suite")

	if reporter == nil {
		t.Fatal("NewReporter returned nil")
	}

	suite := reporter.GetSuite()
	if suite.Name != "Test Suite" {
		t.Errorf("expected suite name 'Test Suite', got '%s'", suite.Name)
	}

	if len(suite.Tests) != 0 {
		t.Errorf("expected 0 tests, got %d", len(suite.Tests))
	}
}

func TestAddPass(t *testing.T) {
	reporter := NewReporter("Test Suite")
	reporter.AddPass("test1", "class1", 1*time.Second)

	stats := reporter.GetStats()
	if stats.Total != 1 {
		t.Errorf("expected 1 total test, got %d", stats.Total)
	}
	if stats.Passed != 1 {
		t.Errorf("expected 1 passed test, got %d", stats.Passed)
	}
}

func TestAddFail(t *testing.T) {
	reporter := NewReporter("Test Suite")
	reporter.AddFail("test1", "class1", 1*time.Second, "error message")

	stats := reporter.GetStats()
	if stats.Total != 1 {
		t.Errorf("expected 1 total test, got %d", stats.Total)
	}
	if stats.Failed != 1 {
		t.Errorf("expected 1 failed test, got %d", stats.Failed)
	}
}

func TestAddSkip(t *testing.T) {
	reporter := NewReporter("Test Suite")
	reporter.AddSkip("test1", "class1", "skipped reason")

	stats := reporter.GetStats()
	if stats.Total != 1 {
		t.Errorf("expected 1 total test, got %d", stats.Total)
	}
	if stats.Skipped != 1 {
		t.Errorf("expected 1 skipped test, got %d", stats.Skipped)
	}
}

func TestAddError(t *testing.T) {
	reporter := NewReporter("Test Suite")
	reporter.AddError("test1", "class1", 1*time.Second, "error occurred")

	stats := reporter.GetStats()
	if stats.Total != 1 {
		t.Errorf("expected 1 total test, got %d", stats.Total)
	}
	if stats.Errors != 1 {
		t.Errorf("expected 1 error test, got %d", stats.Errors)
	}
}

func TestGetStats(t *testing.T) {
	reporter := NewReporter("Test Suite")

	// Add various test results
	reporter.AddPass("test1", "class1", 1*time.Second)
	reporter.AddPass("test2", "class1", 1*time.Second)
	reporter.AddFail("test3", "class1", 1*time.Second, "fail message")
	reporter.AddSkip("test4", "class1", "skip reason")
	reporter.AddError("test5", "class1", 1*time.Second, "error")

	stats := reporter.GetStats()

	if stats.Total != 5 {
		t.Errorf("expected 5 total tests, got %d", stats.Total)
	}
	if stats.Passed != 2 {
		t.Errorf("expected 2 passed tests, got %d", stats.Passed)
	}
	if stats.Failed != 1 {
		t.Errorf("expected 1 failed test, got %d", stats.Failed)
	}
	if stats.Skipped != 1 {
		t.Errorf("expected 1 skipped test, got %d", stats.Skipped)
	}
	if stats.Errors != 1 {
		t.Errorf("expected 1 error test, got %d", stats.Errors)
	}
}

func TestSuccessRate(t *testing.T) {
	tests := []struct {
		name         string
		total        int
		passed       int
		expectedRate float64
	}{
		{"all pass", 10, 10, 100.0},
		{"half pass", 10, 5, 50.0},
		{"none pass", 10, 0, 0.0},
		{"empty", 0, 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := TestStats{
				Total:  tt.total,
				Passed: tt.passed,
			}

			rate := stats.SuccessRate()
			if rate != tt.expectedRate {
				t.Errorf("expected success rate %.1f%%, got %.1f%%", tt.expectedRate, rate)
			}
		})
	}
}

func TestExportJUnit(t *testing.T) {
	reporter := NewReporter("Test Suite")
	reporter.SetDuration(10 * time.Second)
	reporter.SetProperty("author", "test")

	reporter.AddPass("test1", "class1", 1*time.Second)
	reporter.AddFail("test2", "class1", 2*time.Second, "failure message")
	reporter.AddSkip("test3", "class1", "skip reason")

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test-results.xml")

	err := reporter.ExportJUnit(outputPath)
	if err != nil {
		t.Fatalf("ExportJUnit failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("JUnit file was not created")
	}

	// Verify file is not empty
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read JUnit file: %v", err)
	}
	if len(content) == 0 {
		t.Error("JUnit file is empty")
	}
}

func TestExportJSON(t *testing.T) {
	reporter := NewReporter("Test Suite")
	reporter.SetDuration(10 * time.Second)
	reporter.SetProperty("version", "1.0")

	reporter.AddPass("test1", "class1", 1*time.Second)

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test-results.json")

	err := reporter.ExportJSON(outputPath)
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("JSON file was not created")
	}
}

func TestExportHTML(t *testing.T) {
	reporter := NewReporter("Test Suite")
	reporter.SetDuration(10 * time.Second)

	reporter.AddPass("test1", "class1", 1*time.Second)
	reporter.AddFail("test2", "class1", 2*time.Second, "failure")

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test-results.html")

	err := reporter.ExportHTML(outputPath)
	if err != nil {
		t.Fatalf("ExportHTML failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("HTML file was not created")
	}

	// Verify it contains HTML
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read HTML file: %v", err)
	}
	if !contains(string(content), "<!DOCTYPE html>") {
		t.Error("HTML file does not contain DOCTYPE declaration")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
