package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"weather-station-test/pkg/benchmark"
)

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "run performance benchmarks",
	Long: `runs performance benchmarks for the weather station services.

benchmarks:
  - ingest: csv ingestion throughput
  - query: query latency and throughput
  - load: concurrent client load testing

examples:
  # benchmark s1 ingestion
  test-harness benchmark --target s1 --duration 10m

  # benchmark with 1000 concurrent clients
  test-harness benchmark --target s3 --load 1000 --duration 5m`,
	RunE: runBenchmark,
}

var benchmarkFlags struct {
	duration time.Duration
	load     int
	target   string
}

func init() {
	rootCmd.AddCommand(benchmarkCmd)

	benchmarkCmd.Flags().DurationVarP(&benchmarkFlags.duration, "duration", "d", 5*time.Minute, "test duration")
	benchmarkCmd.Flags().IntVarP(&benchmarkFlags.load, "load", "l", 100, "concurrent clients")
	benchmarkCmd.Flags().StringVarP(&benchmarkFlags.target, "target", "t", "", "target service (required)")
	benchmarkCmd.MarkFlagRequired("target")
}

func runBenchmark(cmd *cobra.Command, args []string) error {
	logger.Info("running benchmarks",
		"target", benchmarkFlags.target,
		"duration", benchmarkFlags.duration,
		"load", benchmarkFlags.load,
	)

	// Create benchmark runner
	runner := benchmark.NewRunner(logger)

	// Configure benchmark target
	target := benchmark.ServiceTarget{
		Name:      benchmarkFlags.target,
		Endpoint:  "http://localhost",
		Port:      8080,
		Operation: benchmarkFlags.target,
	}

	// Configure benchmark
	cfg := benchmark.Config{
		Duration:    benchmarkFlags.duration,
		Concurrency: benchmarkFlags.load,
		WarmUp:      10 * time.Second,
		Target:      benchmarkFlags.target,
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration+cfg.WarmUp+30*time.Second)
	defer cancel()

	// Run appropriate benchmark based on target
	var result *benchmark.Result
	var err error

	switch benchmarkFlags.target {
	case "s1", "s1_ingestion", "ingest":
		result, err = runner.RunIngestBenchmark(ctx, target, cfg)
	case "s3", "s3_api", "query":
		result, err = runner.RunQueryBenchmark(ctx, target, cfg)
	default:
		// Default to load benchmark
		result, err = runner.RunLoadBenchmark(ctx, target, cfg)
	}

	if err != nil {
		logger.Error("benchmark failed", "target", benchmarkFlags.target, "error", err)
		return err
	}

	logger.Info("=== benchmark summary ===")
	logger.Info("benchmark completed", "name", result.Name, "total_ops", result.TotalOps)
	logger.Info("throughput", "ops/sec", result.Throughput)
	logger.Info("latency_percentiles", "p50", result.P50(), "p95", result.P95(), "p99", result.P99())

	return nil
}
