package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"weather-station-test/pkg/data"

	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "generate synthetic weather CSV files",
	Long: `generate synthetic CSV weather data for fake weather stations.

each generated file contains 100 years of hourly UTC records per station.
for large size targets, multiple station IDs are included automatically.

examples:
  # single file target size
  test-harness generate --size 1GB --out ./testdata/fake_1gb.csv

  # batch generation
  test-harness generate --sizes 100MB,1GB,5GB --out-dir ./testdata/generated

  # deterministic generation with custom start date
  test-harness generate --size 500MB --out ./fake.csv --start 1950-01-01 --seed 42`,
	RunE: runGenerate,
}

var generateFlags struct {
	size          string
	sizes         []string
	out           string
	outDir        string
	start         string
	seed          int64
	tolerance     float64
	stationPrefix string
	years         int
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().StringVar(&generateFlags.size, "size", "", "target file size (e.g., 100MB, 1GB)")
	generateCmd.Flags().StringSliceVar(&generateFlags.sizes, "sizes", nil, "comma-separated target sizes for batch generation")
	generateCmd.Flags().StringVar(&generateFlags.out, "out", "", "output CSV file path for single generation")
	generateCmd.Flags().StringVar(&generateFlags.outDir, "out-dir", "", "output directory for batch generation")
	generateCmd.Flags().StringVar(&generateFlags.start, "start", "1926-01-01", "start date in UTC (yyyy-mm-dd)")
	generateCmd.Flags().Int64Var(&generateFlags.seed, "seed", 0, "random seed (0 = time-based)")
	generateCmd.Flags().Float64Var(&generateFlags.tolerance, "tolerance", 5.0, "allowed size variance percent")
	generateCmd.Flags().StringVar(&generateFlags.stationPrefix, "station-prefix", "FAKE", "station id prefix")
	generateCmd.Flags().IntVar(&generateFlags.years, "years", data.DefaultGenerateYears, "years per station in each file")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	if generateFlags.size == "" && len(generateFlags.sizes) == 0 {
		return fmt.Errorf("must provide --size for single generation or --sizes for batch generation")
	}
	if generateFlags.size != "" && len(generateFlags.sizes) > 0 {
		return fmt.Errorf("use either --size or --sizes, not both")
	}
	if generateFlags.years != data.DefaultGenerateYears {
		logger.Warn("non-default year span selected", "years", generateFlags.years)
	}

	startTime, err := time.Parse("2006-01-02", generateFlags.start)
	if err != nil {
		return fmt.Errorf("invalid --start date %q (expected yyyy-mm-dd): %w", generateFlags.start, err)
	}
	startTime = time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, time.UTC)

	generator := data.NewSyntheticGenerator(generateFlags.seed)
	logger.Info("starting synthetic weather data generation",
		"seed", generator.Seed(),
		"start", startTime.Format("2006-01-02"),
		"years", generateFlags.years,
		"tolerance_pct", fmt.Sprintf("%.2f", generateFlags.tolerance),
	)

	if generateFlags.size != "" {
		if generateFlags.out == "" {
			return fmt.Errorf("--out is required when using --size")
		}

		targetBytes, err := data.ParseByteSize(generateFlags.size)
		if err != nil {
			return err
		}

		result, err := generator.GenerateCSV(data.GenerateRequest{
			OutputPath:    generateFlags.out,
			TargetBytes:   targetBytes,
			StartTime:     startTime,
			Years:         generateFlags.years,
			TolerancePct:  generateFlags.tolerance,
			StationPrefix: generateFlags.stationPrefix,
		})
		if err != nil {
			return err
		}

		logGenerateResult(result)
		if !result.InTolerance {
			logger.Warn("generated file is outside requested tolerance",
				"target", data.HumanByteSize(result.TargetBytes),
				"actual", data.HumanByteSize(result.ActualBytes),
			)
		}

		return nil
	}

	if generateFlags.outDir == "" {
		return fmt.Errorf("--out-dir is required when using --sizes")
	}

	for i, sizeText := range generateFlags.sizes {
		targetBytes, err := data.ParseByteSize(sizeText)
		if err != nil {
			return fmt.Errorf("invalid size at --sizes[%d]=%q: %w", i, sizeText, err)
		}

		safeName := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(sizeText), " ", ""))
		outputPath := filepath.Join(generateFlags.outDir, fmt.Sprintf("fake-weather-%s.csv", safeName))

		result, err := generator.GenerateCSV(data.GenerateRequest{
			OutputPath:    outputPath,
			TargetBytes:   targetBytes,
			StartTime:     startTime,
			Years:         generateFlags.years,
			TolerancePct:  generateFlags.tolerance,
			StationPrefix: generateFlags.stationPrefix,
		})
		if err != nil {
			return fmt.Errorf("failed generating %q: %w", outputPath, err)
		}

		logGenerateResult(result)
		if !result.InTolerance {
			logger.Warn("generated file is outside requested tolerance",
				"file", result.OutputPath,
				"target", data.HumanByteSize(result.TargetBytes),
				"actual", data.HumanByteSize(result.ActualBytes),
			)
		}
	}

	return nil
}

func logGenerateResult(result data.GenerateResult) {
	logger.Info("generated synthetic weather CSV",
		"file", result.OutputPath,
		"target_size", data.HumanByteSize(result.TargetBytes),
		"actual_size", data.HumanByteSize(result.ActualBytes),
		"rows", result.RowsWritten,
		"stations", result.StationsUsed,
		"in_tolerance", result.InTolerance,
	)
}
