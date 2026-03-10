package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"weather-station-test/pkg/linter"
)

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "run c code linting",
	Long: `runs static analysis and linting on all c code.

linters used:
  - cppcheck: static analysis for bugs and style issues
  - gcc: compiler warnings with strict flags

examples:
  # lint all services
  test-harness lint

  # lint specific service
  test-harness lint --service ingestion

  # verbose output with all issues
  test-harness lint --verbose`,
	RunE: runLint,
}

var lintFlags struct {
	service string
	verbose bool
}

func init() {
	rootCmd.AddCommand(lintCmd)

	lintCmd.Flags().StringVarP(&lintFlags.service, "service", "s", "", "specific service to lint (ingestion, aggregation, query, discovery, cli)")
	lintCmd.Flags().BoolVarP(&lintFlags.verbose, "verbose", "v", false, "show all issues in detail")
}

func runLint(cmd *cobra.Command, args []string) error {
	// Check if cppcheck is available
	if !linter.IsCppcheckAvailable() {
		logger.Error("cppcheck not found", "install", "brew install cppcheck")
		return fmt.Errorf("cppcheck is required for linting")
	}

	// Find services directory
	workDir, _ := os.Getwd()
	servicesPath := filepath.Join(workDir, "..", "services")
	if _, err := os.Stat(servicesPath); os.IsNotExist(err) {
		// Try alternate path
		servicesPath = filepath.Join(workDir, "services")
	}

	sharedPath := filepath.Join(servicesPath, "shared")

	// Create linter
	l := linter.New(servicesPath, sharedPath)

	var summary *linter.Summary
	var err error

	if lintFlags.service != "" {
		// Lint specific service
		logger.Info("linting service", "service", lintFlags.service)
		result, err := l.RunOnService(lintFlags.service)
		if err != nil {
			logger.Error("linting failed", "error", err)
			return err
		}

		summary = &linter.Summary{
			Files:         []linter.Result{*result},
			TotalFiles:    1,
			TotalErrors:   result.ErrorCount,
			TotalWarnings: result.WarningCount,
			TotalStyle:    result.StyleCount,
			TotalIssues:   result.TotalIssues,
		}
		// Calculate score based on single result
		if summary.TotalIssues == 0 {
			summary.Score = 100
			summary.Grade = "A"
			summary.Passed = true
		} else {
			baseScore := 100.0
			deductions := float64(result.ErrorCount)*5.0 +
				float64(result.WarningCount)*2.0 +
				float64(result.StyleCount)*0.5
			score := baseScore - deductions
			if score < 0 {
				score = 0
			}
			summary.Score = int(score)
			// Assign grade
			switch {
			case summary.Score >= 93:
				summary.Grade = "A"
				summary.Passed = true
			case summary.Score >= 90:
				summary.Grade = "A-"
				summary.Passed = true
			case summary.Score >= 83:
				summary.Grade = "B"
				summary.Passed = true
			case summary.Score >= 60:
				summary.Grade = "C"
			default:
				summary.Grade = "F"
				summary.Passed = false
			}
		}
	} else {
		// Lint all services
		logger.Info("linting all c code")
		summary, err = l.RunAll()
		if err != nil {
			logger.Error("linting failed", "error", err)
			return err
		}
	}

	// Print results
	summary.PrintSummary()

	// Exit with error if not passed
	if !summary.Passed {
		os.Exit(1)
	}

	return nil
}
