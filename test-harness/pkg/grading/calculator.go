// Package grading provides automatic grading and scoring capabilities
package grading

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Calculator computes grades based on test results
type Calculator struct {
	submission Submission
	criteria   []Criterion
	results    map[string]CategoryResult
}

// Submission represents a student submission
type Submission struct {
	StudentID   string
	StudentName string
	Timestamp   time.Time
	Repository  string
	CommitHash  string
}

// Criterion defines a grading category
type Criterion struct {
	Name        string
	Weight      float64 // 0.0 - 1.0
	MaxPoints   int
	MustPass    bool // Must be 100% to pass overall
	Description string
}

// CategoryResult holds results for a grading category
type CategoryResult struct {
	Criterion  Criterion
	Score      int
	MaxPoints  int
	Percentage float64
	Feedback   []string
	Details    map[string]interface{}
	Pass       bool
}

// Grade represents the final calculated grade
type Grade struct {
	Submission    Submission
	TotalScore    int
	MaxScore      int
	Percentage    float64
	LetterGrade   string
	Passed        bool
	Categories    []CategoryResult
	MustPassMet   bool
	Feedback      []string
	DetailedNotes []string
	Timestamp     time.Time
}

// DefaultCriteria returns the default grading criteria
func DefaultCriteria() []Criterion {
	return []Criterion{
		{
			Name:        "compilation",
			Weight:      0.10,
			MaxPoints:   10,
			MustPass:    true,
			Description: "All services compile without errors",
		},
		{
			Name:        "functionality",
			Weight:      0.40,
			MaxPoints:   40,
			MustPass:    false,
			Description: "Core functionality tests pass",
		},
		{
			Name:        "performance",
			Weight:      0.30,
			MaxPoints:   30,
			MustPass:    false,
			Description: "Performance benchmarks meet targets",
		},
		{
			Name:        "reliability",
			Weight:      0.20,
			MaxPoints:   20,
			MustPass:    false,
			Description: "Chaos tests and reliability metrics",
		},
	}
}

// NewCalculator creates a new grade calculator
func NewCalculator(submission Submission) *Calculator {
	return &Calculator{
		submission: submission,
		criteria:   DefaultCriteria(),
		results:    make(map[string]CategoryResult),
	}
}

// SetCriteria sets custom grading criteria
func (c *Calculator) SetCriteria(criteria []Criterion) {
	c.criteria = criteria
}

// AddResult adds a result for a category
func (c *Calculator) AddResult(category string, score, maxPoints int, feedback []string, details map[string]interface{}) {
	// Find the criterion
	var criterion Criterion
	for _, crit := range c.criteria {
		if crit.Name == category {
			criterion = crit
			break
		}
	}

	percentage := 0.0
	if maxPoints > 0 {
		percentage = float64(score) / float64(maxPoints) * 100
	}

	pass := percentage >= 60.0
	if criterion.MustPass {
		pass = percentage >= 100.0
	}

	c.results[category] = CategoryResult{
		Criterion:  criterion,
		Score:      score,
		MaxPoints:  maxPoints,
		Percentage: percentage,
		Feedback:   feedback,
		Details:    details,
		Pass:       pass,
	}
}

// Calculate computes the final grade
func (c *Calculator) Calculate() (*Grade, error) {
	grade := &Grade{
		Submission:    c.submission,
		Categories:    make([]CategoryResult, 0, len(c.criteria)),
		Feedback:      make([]string, 0),
		DetailedNotes: make([]string, 0),
		Timestamp:     time.Now(),
	}

	totalScore := 0
	totalMax := 0
	mustPassMet := true

	for _, criterion := range c.criteria {
		result, exists := c.results[criterion.Name]
		if !exists {
			// Category not evaluated - give 0
			result = CategoryResult{
				Criterion:  criterion,
				Score:      0,
				MaxPoints:  criterion.MaxPoints,
				Percentage: 0,
				Feedback:   []string{"Category not evaluated"},
				Pass:       false,
			}
		}

		grade.Categories = append(grade.Categories, result)

		// Calculate weighted score
		weightedScore := float64(result.Score) * criterion.Weight
		weightedMax := float64(criterion.MaxPoints) * criterion.Weight

		totalScore += int(weightedScore)
		totalMax += int(weightedMax)

		// Check must-pass criteria
		if criterion.MustPass && !result.Pass {
			mustPassMet = false
			grade.Feedback = append(grade.Feedback,
				fmt.Sprintf("Must-pass criterion failed: %s", criterion.Name))
		}
	}

	grade.TotalScore = totalScore
	grade.MaxScore = totalMax
	grade.MustPassMet = mustPassMet

	// Calculate percentage
	if totalMax > 0 {
		grade.Percentage = float64(totalScore) / float64(totalMax) * 100
	}

	// Determine letter grade
	grade.LetterGrade = c.calculateLetterGrade(grade.Percentage)

	// Determine pass/fail
	grade.Passed = mustPassMet && grade.Percentage >= 60.0

	// Generate detailed notes
	for _, cat := range grade.Categories {
		status := "✓"
		if !cat.Pass {
			status = "✗"
		}
		grade.DetailedNotes = append(grade.DetailedNotes,
			fmt.Sprintf("%s %s: %d/%d (%.1f%%)",
				status, cat.Criterion.Name, cat.Score, cat.MaxPoints, cat.Percentage))

		for _, fb := range cat.Feedback {
			grade.DetailedNotes = append(grade.DetailedNotes,
				fmt.Sprintf("    - %s", fb))
		}
	}

	return grade, nil
}

// calculateLetterGrade converts percentage to letter grade
func (c *Calculator) calculateLetterGrade(percentage float64) string {
	switch {
	case percentage >= 93:
		return "A"
	case percentage >= 90:
		return "A-"
	case percentage >= 87:
		return "B+"
	case percentage >= 83:
		return "B"
	case percentage >= 80:
		return "B-"
	case percentage >= 77:
		return "C+"
	case percentage >= 73:
		return "C"
	case percentage >= 70:
		return "C-"
	case percentage >= 60:
		return "D"
	default:
		return "F"
	}
}

// Export exports grade to file
func (g *Grade) Export(format, outputPath string) error {
	switch format {
	case "json":
		return g.exportJSON(outputPath)
	case "html":
		return g.exportHTML(outputPath)
	case "text", "txt":
		return g.exportText(outputPath)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// exportJSON exports grade to JSON file
func (g *Grade) exportJSON(outputPath string) error {
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Simple JSON export
	content := fmt.Sprintf(`{
  "student_id": "%s",
  "student_name": "%s",
  "timestamp": "%s",
  "total_score": %d,
  "max_score": %d,
  "percentage": %.2f,
  "letter_grade": "%s",
  "passed": %t,
  "must_pass_met": %t,
  "categories": [
`, g.Submission.StudentID, g.Submission.StudentName,
		g.Timestamp.Format(time.RFC3339), g.TotalScore, g.MaxScore,
		g.Percentage, g.LetterGrade, g.Passed, g.MustPassMet)

	for i, cat := range g.Categories {
		content += fmt.Sprintf(`    {
      "name": "%s",
      "score": %d,
      "max_points": %d,
      "percentage": %.2f,
      "passed": %t,
      "must_pass": %t
    }`, cat.Criterion.Name, cat.Score, cat.MaxPoints, cat.Percentage, cat.Pass, cat.Criterion.MustPass)
		if i < len(g.Categories)-1 {
			content += ","
		}
		content += "\n"
	}

	content += "  ],\n  "

	if len(g.Feedback) > 0 {
		content += `"feedback": [`
		for i, fb := range g.Feedback {
			content += fmt.Sprintf(`"%s"`, fb)
			if i < len(g.Feedback)-1 {
				content += ", "
			}
		}
		content += "],\n  "
	}

	content += `"detailed_notes": [`
	for i, note := range g.DetailedNotes {
		content += fmt.Sprintf(`"%s"`, note)
		if i < len(g.DetailedNotes)-1 {
			content += ", "
		}
	}
	content += "]\n}\n"

	return os.WriteFile(outputPath, []byte(content), 0644)
}

// exportHTML exports grade to HTML file
func (g *Grade) exportHTML(outputPath string) error {
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	passStatus := "Passed"
	passClass := "passed"
	if !g.Passed {
		passStatus = "Failed"
		passClass = "failed"
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Grade Report - %s</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 800px; margin: 0 auto; background: white; padding: 40px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #333; margin-bottom: 5px; }
        .subtitle { color: #666; margin-bottom: 30px; }
        .grade-box { text-align: center; padding: 30px; margin: 30px 0; border-radius: 8px; }
        .grade-box.passed { background: #e8f5e9; }
        .grade-box.failed { background: #ffebee; }
        .grade-letter { font-size: 72px; font-weight: bold; }
        .grade-percent { font-size: 24px; color: #666; margin-top: 10px; }
        .grade-status { font-size: 18px; margin-top: 10px; text-transform: uppercase; }
        .categories { margin-top: 30px; }
        .category { display: flex; justify-content: space-between; padding: 15px; margin-bottom: 10px; border-radius: 6px; }
        .category.passed { background: #e8f5e9; border-left: 4px solid #4caf50; }
        .category.failed { background: #ffebee; border-left: 4px solid #f44336; }
        .category-name { font-weight: 500; }
        .category-score { text-align: right; }
        .category-percent { font-size: 12px; color: #666; }
        .must-pass { color: #f44336; font-size: 12px; font-weight: bold; }
        .feedback { margin-top: 30px; padding: 20px; background: #fff3e0; border-radius: 6px; }
        .feedback h3 { margin-top: 0; }
        .feedback ul { margin: 0; padding-left: 20px; }
        .footer { margin-top: 30px; padding-top: 20px; border-top: 1px solid #e0e0e0; color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Grade Report</h1>
        <div class="subtitle">%s - %s</div>
        
        <div class="grade-box %s">
            <div class="grade-letter">%s</div>
            <div class="grade-percent">%.1f%% (%d/%d points)</div>
            <div class="grade-status">%s</div>
        </div>
        
        <div class="categories">
            <h2>Category Breakdown</h2>
`, g.Submission.StudentName, g.Submission.StudentName, g.Submission.StudentID,
		passClass, g.LetterGrade, g.Percentage, g.TotalScore, g.MaxScore, passStatus)

	for _, cat := range g.Categories {
		catClass := "passed"
		if !cat.Pass {
			catClass = "failed"
		}

		mustPassText := ""
		if cat.Criterion.MustPass {
			mustPassText = `<span class="must-pass">[MUST PASS]</span>`
		}

		html += fmt.Sprintf(`            <div class="category %s">
                <div>
                    <div class="category-name">%s %s</div>
                    <div style="color: #666; font-size: 12px;">%s</div>
                </div>
                <div class="category-score">
                    <div>%d/%d</div>
                    <div class="category-percent">%.1f%%</div>
                </div>
            </div>
`, catClass, cat.Criterion.Name, mustPassText, cat.Criterion.Description,
			cat.Score, cat.MaxPoints, cat.Percentage)
	}

	html += `        </div>
`

	if len(g.Feedback) > 0 {
		html += `        <div class="feedback">
            <h3>Feedback</h3>
            <ul>
`
		for _, fb := range g.Feedback {
			html += fmt.Sprintf("                <li>%s</li>\n", fb)
		}
		html += `            </ul>
        </div>
`
	}

	html += fmt.Sprintf(`        <div class="footer">
            Generated: %s<br>
            Repository: %s<br>
            Commit: %s
        </div>
    </div>
</body>
</html>`, g.Timestamp.Format(time.RFC3339), g.Submission.Repository, g.Submission.CommitHash)

	return os.WriteFile(outputPath, []byte(html), 0644)
}

// exportText exports grade to text file
func (g *Grade) exportText(outputPath string) error {
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	passStatus := "PASSED"
	if !g.Passed {
		passStatus = "FAILED"
	}

	content := fmt.Sprintf(`WEATHER STATION - FINAL ASSESSMENT
=====================================

Student: %s
ID: %s
Date: %s

FINAL GRADE: %s (%.1f%%)
Status: %s

Score: %d/%d points

CATEGORY BREAKDOWN
------------------
`, g.Submission.StudentName, g.Submission.StudentID,
		g.Timestamp.Format("2006-01-02 15:04:05"),
		g.LetterGrade, g.Percentage, passStatus, g.TotalScore, g.MaxScore)

	for _, cat := range g.Categories {
		mustPass := ""
		if cat.Criterion.MustPass {
			mustPass = " [MUST PASS]"
		}

		status := "✓"
		if !cat.Pass {
			status = "✗"
		}

		content += fmt.Sprintf("%s %s%s: %d/%d (%.1f%%)\n",
			status, cat.Criterion.Name, mustPass, cat.Score, cat.MaxPoints, cat.Percentage)
		content += fmt.Sprintf("    %s\n", cat.Criterion.Description)
	}

	if len(g.Feedback) > 0 {
		content += "\nFEEDBACK\n--------\n"
		for _, fb := range g.Feedback {
			content += fmt.Sprintf("- %s\n", fb)
		}
	}

	if !g.MustPassMet {
		content += "\nNOTE: One or more must-pass criteria were not met.\n"
	}

	content += fmt.Sprintf("\nRepository: %s\n", g.Submission.Repository)
	content += fmt.Sprintf("Commit: %s\n", g.Submission.CommitHash)

	return os.WriteFile(outputPath, []byte(content), 0644)
}

// Summary returns a brief summary of the grade
func (g *Grade) Summary() string {
	passStatus := "PASS"
	if !g.Passed {
		passStatus = "FAIL"
	}

	return fmt.Sprintf("%s: %s (%.1f%%) - %d/%d points",
		passStatus, g.LetterGrade, g.Percentage, g.TotalScore, g.MaxScore)
}
