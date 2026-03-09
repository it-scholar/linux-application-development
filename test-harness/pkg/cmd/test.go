package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"weather-station-test/pkg/report"
	"weather-station-test/pkg/testrunner"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "run test suites",
	Long: `runs test suites for the weather station services.

supported test suites:
  - unit: unit tests for individual functions
  - integration: tests for service interactions
  - performance: benchmarks and load tests
  - chaos: resilience and failure tests
  - all: run all test suites

examples:
  # run all tests
  test-harness test

  # test specific service
  test-harness test --service ingestion

  # run with mocks for dependencies
  test-harness test --service s1 --use-mocks s3,s4

  # run specific test suite
  test-harness test --suite performance --parallel 8`,
	RunE: runTest,
}

var testFlags struct {
	service  []string
	suite    []string
	testName string
	parallel int
	watch    bool
	useMocks []string
}

func init() {
	rootCmd.AddCommand(testCmd)

	testCmd.Flags().StringArrayVarP(&testFlags.service, "service", "s", nil, "specific services to test")
	testCmd.Flags().StringArrayVar(&testFlags.suite, "suite", []string{"all"}, "test suites to run")
	testCmd.Flags().StringVar(&testFlags.testName, "test", "", "single test by name")
	testCmd.Flags().IntVarP(&testFlags.parallel, "parallel", "p", 4, "parallelism level")
	testCmd.Flags().BoolVarP(&testFlags.watch, "watch", "w", false, "stream results in real-time")
	testCmd.Flags().StringArrayVar(&testFlags.useMocks, "use-mocks", nil, "use mocks for services")
}

func runTest(cmd *cobra.Command, args []string) error {
	logger.Info("running test suites",
		"suites", testFlags.suite,
		"parallel", testFlags.parallel,
	)

	if len(testFlags.service) > 0 {
		logger.Info("target services", "services", testFlags.service)
	}

	if len(testFlags.useMocks) > 0 {
		logger.Info("using mocks", "services", testFlags.useMocks)
	}

	// Create test runner
	config := testrunner.Config{
		Suites:   testFlags.suite,
		Services: testFlags.service,
		Parallel: testFlags.parallel,
		Verbose:  viper.GetBool("verbose"),
		UseMocks: testFlags.useMocks,
	}

	runner := testrunner.NewRunner(config, logger)

	// Run tests
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	results, err := runner.Run(ctx)
	if err != nil {
		logger.Error("test execution failed", "error", err)
		return err
	}

	// Log results
	logger.Info("=== test results ===")
	allPassed := true
	for _, result := range results {
		status := "✓"
		if !result.Passed {
			status = "✗"
			allPassed = false
		}
		logger.Info(status+" "+result.Suite+": "+
			fmt.Sprintf("%d/%d passed", result.PassCount, result.TestCount),
			"duration", result.Duration)

		if result.Error != nil && viper.GetBool("verbose") {
			logger.Error("test suite error", "suite", result.Suite, "error", result.Error)
		}
	}

	// Generate summary
	summary := testrunner.Summary(results)
	logger.Info(summary)

	// Generate reports if requested
	outputFormat := viper.GetString("output")
	if outputFormat != "console" {
		reporter := runner.GetReporter()
		for _, result := range results {
			status := report.StatusPass
			if !result.Passed {
				status = report.StatusFail
			}
			reporter.AddTest(result.Suite, "test", result.Duration, status, "")
		}

		if outputFormat == "junit" || outputFormat == "all" {
			if err := reporter.ExportJUnit("test-results.xml"); err != nil {
				logger.Error("failed to export junit report", "error", err)
			}
		}
		if outputFormat == "html" || outputFormat == "all" {
			if err := reporter.ExportHTML("test-results.html"); err != nil {
				logger.Error("failed to export html report", "error", err)
			}
		}
		if outputFormat == "json" || outputFormat == "all" {
			if err := reporter.ExportJSON("test-results.json"); err != nil {
				logger.Error("failed to export json report", "error", err)
			}
		}
	}

	if allPassed {
		logger.Info("all tests passed!")
		return nil
	}

	logger.Error("some tests failed")
	os.Exit(1)
	return nil
}
