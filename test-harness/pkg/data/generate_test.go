package data

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseByteSize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		expect  int64
		wantErr bool
	}{
		{name: "bytes", input: "1024", expect: 1024},
		{name: "kb", input: "1KB", expect: 1024},
		{name: "mb", input: "2MB", expect: 2 * 1024 * 1024},
		{name: "gb", input: "1.5GB", expect: int64(1.5 * 1024 * 1024 * 1024)},
		{name: "spaces", input: " 250 mb ", expect: 250 * 1024 * 1024},
		{name: "invalid", input: "abc", wantErr: true},
		{name: "zero", input: "0MB", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseByteSize(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseByteSize failed: %v", err)
			}
			if got != tt.expect {
				t.Fatalf("expected %d, got %d", tt.expect, got)
			}
		})
	}
}

func TestGenerateCSVWithinToleranceAndHeader(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "synthetic.csv")

	generator := NewSyntheticGenerator(42)
	start := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	target := int64(1 * 1024 * 1024)

	result, err := generator.GenerateCSV(GenerateRequest{
		OutputPath:    outPath,
		TargetBytes:   target,
		StartTime:     start,
		Years:         1,
		TolerancePct:  10.0,
		StationPrefix: "TEST",
	})
	if err != nil {
		t.Fatalf("GenerateCSV failed: %v", err)
	}

	if !result.InTolerance {
		t.Fatalf("expected output size in tolerance, target=%d actual=%d", result.TargetBytes, result.ActualBytes)
	}

	expectedRows := int64(365 * 24 * result.StationsUsed)
	if result.RowsWritten != expectedRows {
		t.Fatalf("expected %d rows for one year across %d stations, got %d", expectedRows, result.StationsUsed, result.RowsWritten)
	}

	if result.StationsUsed < 1 {
		t.Fatalf("expected at least one station, got %d", result.StationsUsed)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat output failed: %v", err)
	}
	if info.Size() != result.ActualBytes {
		t.Fatalf("expected file size %d, got %d", result.ActualBytes, info.Size())
	}

	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("open output failed: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		t.Fatal("expected header line")
	}
	header := scanner.Text()
	expectedHeader := "timestamp,station_id,temperature_c,humidity_pct,pressure_hpa,wind_speed_mps,wind_dir_deg,rain_mm"
	if header != expectedHeader {
		t.Fatalf("unexpected header: %q", header)
	}

	if !scanner.Scan() {
		t.Fatal("expected first data row")
	}
	firstRow := scanner.Text()
	parts := strings.Split(firstRow, ",")
	if len(parts) != 8 {
		t.Fatalf("expected 8 columns, got %d", len(parts))
	}

	if _, err := time.Parse(time.RFC3339, parts[0]); err != nil {
		t.Fatalf("timestamp not RFC3339: %v", err)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner failed: %v", err)
	}
}

func TestGenerateCSVUsesMultipleStationsForLargerTargets(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "synthetic-large.csv")

	generator := NewSyntheticGenerator(7)
	start := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

	result, err := generator.GenerateCSV(GenerateRequest{
		OutputPath:    outPath,
		TargetBytes:   4 * 1024 * 1024,
		StartTime:     start,
		Years:         1,
		TolerancePct:  5.0,
		StationPrefix: "TEST",
	})
	if err != nil {
		t.Fatalf("GenerateCSV failed: %v", err)
	}

	if result.StationsUsed <= 1 {
		t.Fatalf("expected multiple stations for larger target, got %d", result.StationsUsed)
	}
}

func TestGenerateCSVRejectsTooSmallTarget(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "tiny.csv")

	generator := NewSyntheticGenerator(11)
	start := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

	_, err := generator.GenerateCSV(GenerateRequest{
		OutputPath:    outPath,
		TargetBytes:   1024,
		StartTime:     start,
		Years:         1,
		TolerancePct:  5.0,
		StationPrefix: "TEST",
	})
	if err == nil {
		t.Fatal("expected error for too small target size")
	}
}
