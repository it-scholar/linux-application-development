package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"weather-station-test/pkg/grading"
	"weather-station-test/pkg/report"
)

var gradeCmd = &cobra.Command{
	Use:   "grade",
	Short: "calculate final score",
	Long: `calculates the final grade for a student submission based on test results.

grading categories:
  - compilation (10%)
  - functionality (40%)
  - performance (30%)
  - reliability (20%)

examples:
  # grade with detailed output
  test-harness grade --student station-1 --detailed

  # generate html report
  test-harness grade --format html --output ./reports/

  # json output for ci/cd
  test-harness grade --format json > grade.json`,
	RunE: runGrade,
}

var gradeFlags struct {
	student  string
	detailed bool
	format   string
	output   string
}

func init() {
	rootCmd.AddCommand(gradeCmd)

	gradeCmd.Flags().StringVarP(&gradeFlags.student, "student", "s", "", "student identifier")
	gradeCmd.Flags().BoolVarP(&gradeFlags.detailed, "detailed", "d", false, "very detailed output")
	gradeCmd.Flags().StringVarP(&gradeFlags.format, "format", "f", "console", "output format (console|json|html)")
	gradeCmd.Flags().StringVarP(&gradeFlags.output, "output", "o", "./reports", "output directory for reports")
}

func runGrade(cmd *cobra.Command, args []string) error {
	if gradeFlags.student == "" {
		gradeFlags.student = "anonymous"
	}

	logger.Info("grading student", "student", gradeFlags.student)

	// Create submission
	submission := grading.Submission{
		StudentID:   gradeFlags.student,
		StudentName: gradeFlags.student,
		Timestamp:   time.Now(),
		Repository:  "local",
		CommitHash:  "unknown",
	}

	// Create calculator with default criteria
	calc := grading.NewCalculator(submission)

	// Simulate test results (in real implementation, these would come from actual test runs)
	calc.AddResult("compilation", 10, 10,
		[]string{"All services compiled successfully"},
		map[string]interface{}{"warnings": 0})

	calc.AddResult("functionality", 38, 40,
		[]string{"S4 peer discovery intermittent - connection timeout after 5s"},
		map[string]interface{}{"tests_passed": 42, "tests_total": 44})

	calc.AddResult("performance", 18, 20,
		[]string{"Query latency measured 15ms, target 10ms"},
		map[string]interface{}{"p50_latency_ms": 8, "p99_latency_ms": 15})

	calc.AddResult("reliability", 15, 15,
		[]string{"All chaos tests passed"},
		map[string]interface{}{"recovery_time_ms": 250})

	// Calculate grade
	grade, err := calc.Calculate()
	if err != nil {
		logger.Error("failed to calculate grade", "error", err)
		return err
	}

	// Output results
	logger.Info("=== weather station - final assessment ===")
	logger.Info("student", "name", grade.Submission.StudentName)
	logger.Info("score", "points", grade.TotalScore, "total", grade.MaxScore)
	logger.Info("percentage", "value", fmt.Sprintf("%.1f%%", grade.Percentage))
	logger.Info("letter_grade", "value", grade.LetterGrade)
	logger.Info("status", "passed", grade.Passed)

	logger.Info("breakdown")
	for _, cat := range grade.Categories {
		status := "✓"
		if !cat.Pass {
			status = "✗"
		}
		mustPass := ""
		if cat.Criterion.MustPass {
			mustPass = " [MUST PASS]"
		}
		logger.Info("  " + status + " " + cat.Criterion.Name + ": " +
			fmt.Sprintf("%d/%d", cat.Score, cat.MaxPoints) + mustPass)
	}

	if gradeFlags.detailed {
		logger.Info("detailed feedback")
		for _, note := range grade.DetailedNotes {
			logger.Info(note)
		}
		for _, fb := range grade.Feedback {
			logger.Info("feedback", "message", fb)
		}
	}

	// Export to file if output path specified
	if gradeFlags.output != "" {
		outputFile := filepath.Join(gradeFlags.output, gradeFlags.student+"-grade."+gradeFlags.format)
		if err := grade.Export(gradeFlags.format, outputFile); err != nil {
			logger.Error("failed to export grade", "error", err)
			return err
		}
		logger.Info("grade exported", "path", outputFile)
	}

	// Generate test report if tests were run
	if gradeFlags.detailed {
		reporter := report.NewReporter("Weather Station Tests")
		reporter.AddPass("test_s1_compilation", "compilation", 2*time.Second)
		reporter.AddPass("test_s2_compilation", "compilation", 2*time.Second)
		reporter.AddPass("test_s3_compilation", "compilation", 2*time.Second)
		reporter.AddPass("test_s4_compilation", "compilation", 2*time.Second)
		reporter.AddPass("test_csv_ingestion", "functionality", 5*time.Second)
		reporter.AddPass("test_query_api", "functionality", 3*time.Second)
		reporter.AddFail("test_s4_discovery", "functionality", 5*time.Second, "connection timeout after 5s")

		// Export reports
		if gradeFlags.output != "" {
			junitFile := filepath.Join(gradeFlags.output, "test-results.xml")
			if err := reporter.ExportJUnit(junitFile); err != nil {
				logger.Error("failed to export junit report", "error", err)
			} else {
				logger.Info("junit report exported", "path", junitFile)
			}

			htmlFile := filepath.Join(gradeFlags.output, "test-results.html")
			if err := reporter.ExportHTML(htmlFile); err != nil {
				logger.Error("failed to export html report", "error", err)
			} else {
				logger.Info("html report exported", "path", htmlFile)
			}
		}
	}

	return nil
}
