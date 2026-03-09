package cmd

import (
	"github.com/spf13/cobra"
	"weather-station-test/pkg/contracts"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "validate service against contract",
	Long: `validates that a service binary exists and meets its contract requirements.

examples:
  # validate s1 ingestion service
  test-harness validate --service s1_ingestion

  # validate with specific contract file
  test-harness validate --service s1 --contract ./contracts/custom.yaml`,
	RunE: runValidate,
}

var validateFlags struct {
	service  string
	contract string
}

func init() {
	rootCmd.AddCommand(validateCmd)

	validateCmd.Flags().StringVarP(&validateFlags.service, "service", "s", "", "service to validate (required)")
	validateCmd.Flags().StringVar(&validateFlags.contract, "contract", "", "contract file path (optional)")
	validateCmd.MarkFlagRequired("service")
}

func runValidate(cmd *cobra.Command, args []string) error {
	// determine contract file
	contractPath := validateFlags.contract
	if contractPath == "" {
		contractPath = contracts.GetContractPath(validateFlags.service)
	}

	logger.Info("validating service",
		"service", validateFlags.service,
		"contract", contractPath,
	)

	// create validator
	validator := contracts.NewValidator(logger)

	// validate service
	result, err := validator.ValidateService(validateFlags.service, contractPath)
	if err != nil && result == nil {
		logger.Error("validation failed", "error", err)
		return err
	}

	// print results
	logger.Info("=== validation results ===")
	logger.Info("service", "name", result.ServiceName)
	logger.Info("contract", "file", result.ContractFile)
	logger.Info("checks",
		"passed", result.ChecksPassed,
		"failed", result.ChecksFailed,
		"total", result.ChecksTotal,
	)

	if result.Passed {
		logger.Info("✓ validation passed!")
	} else {
		logger.Error("✗ validation failed")
		for _, err := range result.Errors {
			logger.Error("  error", "details", err)
		}
	}

	if len(result.Warnings) > 0 {
		logger.Warn("warnings")
		for _, warn := range result.Warnings {
			logger.Warn("  warning", "details", warn)
		}
	}

	if !result.Passed {
		return err
	}

	return nil
}
