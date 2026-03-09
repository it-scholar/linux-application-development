package grading

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewCalculator(t *testing.T) {
	submission := Submission{
		StudentID:   "test-student",
		StudentName: "Test Student",
		Timestamp:   time.Now(),
	}

	calc := NewCalculator(submission)
	if calc == nil {
		t.Fatal("NewCalculator returned nil")
	}

	// Should have default criteria
	if len(calc.criteria) == 0 {
		t.Error("calculator should have default criteria")
	}
}

func TestDefaultCriteria(t *testing.T) {
	criteria := DefaultCriteria()

	if len(criteria) != 4 {
		t.Errorf("expected 4 criteria, got %d", len(criteria))
	}

	// Check that weights sum to 1.0
	totalWeight := 0.0
	for _, c := range criteria {
		totalWeight += c.Weight
	}

	if totalWeight != 1.0 {
		t.Errorf("expected total weight 1.0, got %f", totalWeight)
	}

	// Check compilation is must-pass
	foundCompilation := false
	for _, c := range criteria {
		if c.Name == "compilation" {
			foundCompilation = true
			if !c.MustPass {
				t.Error("compilation should be must-pass")
			}
			if c.Weight != 0.10 {
				t.Errorf("expected compilation weight 0.10, got %f", c.Weight)
			}
		}
	}

	if !foundCompilation {
		t.Error("compilation criterion not found")
	}
}

func TestAddResult(t *testing.T) {
	submission := Submission{
		StudentID: "test-student",
	}

	calc := NewCalculator(submission)

	calc.AddResult("compilation", 10, 10, []string{"all good"}, nil)

	if len(calc.results) != 1 {
		t.Errorf("expected 1 result, got %d", len(calc.results))
	}

	result, ok := calc.results["compilation"]
	if !ok {
		t.Fatal("compilation result not found")
	}

	if result.Score != 10 {
		t.Errorf("expected score 10, got %d", result.Score)
	}

	if result.Percentage != 100.0 {
		t.Errorf("expected percentage 100.0, got %f", result.Percentage)
	}

	if !result.Pass {
		t.Error("expected result to pass")
	}
}

func TestCalculateGrade(t *testing.T) {
	submission := Submission{
		StudentID:   "test-student",
		StudentName: "Test Student",
		Timestamp:   time.Now(),
	}

	calc := NewCalculator(submission)

	// Add perfect results for all categories
	calc.AddResult("compilation", 10, 10, []string{}, nil)
	calc.AddResult("functionality", 40, 40, []string{}, nil)
	calc.AddResult("performance", 30, 30, []string{}, nil)
	calc.AddResult("reliability", 20, 20, []string{}, nil)

	grade, err := calc.Calculate()
	if err != nil {
		t.Fatalf("Calculate failed: %v", err)
	}

	if grade == nil {
		t.Fatal("Calculate returned nil grade")
	}

	// Total score is weighted (10*0.1 + 40*0.4 + 30*0.3 + 20*0.2 = 30)
	if grade.TotalScore != 30 {
		t.Errorf("expected total score 30, got %d", grade.TotalScore)
	}

	if grade.Percentage != 100.0 {
		t.Errorf("expected percentage 100.0, got %f", grade.Percentage)
	}

	if grade.LetterGrade != "A" {
		t.Errorf("expected letter grade A, got %s", grade.LetterGrade)
	}

	if !grade.Passed {
		t.Error("expected grade to pass")
	}

	if !grade.MustPassMet {
		t.Error("expected must-pass criteria to be met")
	}
}

func TestCalculateGradeFailingMustPass(t *testing.T) {
	submission := Submission{
		StudentID: "test-student",
	}

	calc := NewCalculator(submission)

	// Fail compilation (must-pass)
	calc.AddResult("compilation", 5, 10, []string{"compilation failed"}, nil)
	calc.AddResult("functionality", 40, 40, []string{}, nil)
	calc.AddResult("performance", 30, 30, []string{}, nil)
	calc.AddResult("reliability", 20, 20, []string{}, nil)

	grade, err := calc.Calculate()
	if err != nil {
		t.Fatalf("Calculate failed: %v", err)
	}

	if grade.Passed {
		t.Error("expected grade to fail when must-pass criterion fails")
	}

	if grade.MustPassMet {
		t.Error("expected must-pass criteria not to be met")
	}
}

func TestCalculateLetterGrade(t *testing.T) {
	tests := []struct {
		percentage float64
		expected   string
	}{
		{95.0, "A"},
		{93.0, "A"},
		{90.0, "A-"},
		{87.0, "B+"},
		{83.0, "B"},
		{80.0, "B-"},
		{77.0, "C+"},
		{73.0, "C"},
		{70.0, "C-"},
		{65.0, "D"},
		{59.0, "F"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			calc := NewCalculator(Submission{})
			grade := calc.calculateLetterGrade(tt.percentage)
			if grade != tt.expected {
				t.Errorf("expected grade %s for %.1f%%, got %s", tt.expected, tt.percentage, grade)
			}
		})
	}
}

func TestGradeExport(t *testing.T) {
	submission := Submission{
		StudentID:   "test-student",
		StudentName: "Test Student",
		Timestamp:   time.Now(),
		Repository:  "test/repo",
		CommitHash:  "abc123",
	}

	calc := NewCalculator(submission)
	calc.AddResult("compilation", 10, 10, []string{}, nil)

	grade, err := calc.Calculate()
	if err != nil {
		t.Fatalf("Calculate failed: %v", err)
	}

	tmpDir := t.TempDir()

	// Test JSON export
	jsonPath := filepath.Join(tmpDir, "grade.json")
	err = grade.Export("json", jsonPath)
	if err != nil {
		t.Fatalf("Export JSON failed: %v", err)
	}

	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Error("JSON file was not created")
	}

	// Test HTML export
	htmlPath := filepath.Join(tmpDir, "grade.html")
	err = grade.Export("html", htmlPath)
	if err != nil {
		t.Fatalf("Export HTML failed: %v", err)
	}

	if _, err := os.Stat(htmlPath); os.IsNotExist(err) {
		t.Error("HTML file was not created")
	}

	// Test text export
	textPath := filepath.Join(tmpDir, "grade.txt")
	err = grade.Export("text", textPath)
	if err != nil {
		t.Fatalf("Export text failed: %v", err)
	}

	if _, err := os.Stat(textPath); os.IsNotExist(err) {
		t.Error("Text file was not created")
	}
}

func TestGradeSummary(t *testing.T) {
	submission := Submission{
		StudentName: "Test Student",
	}

	calc := NewCalculator(submission)
	calc.AddResult("compilation", 10, 10, []string{}, nil)
	calc.AddResult("functionality", 35, 40, []string{}, nil)

	grade, _ := calc.Calculate()

	summary := grade.Summary()
	if summary == "" {
		t.Error("Summary returned empty string")
	}

	if !contains(summary, "PASS") && !contains(summary, "FAIL") {
		t.Error("Summary should contain PASS or FAIL status")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
