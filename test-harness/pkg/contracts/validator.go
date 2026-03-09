package contracts

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"gopkg.in/yaml.v3"
)

// validator validates service contracts
type Validator struct {
	logger *log.Logger
}

// NewValidator creates a new contract validator
func NewValidator(logger *log.Logger) *Validator {
	if logger == nil {
		logger = log.New(os.Stderr)
	}
	return &Validator{logger: logger}
}

// ValidationResult contains the results of contract validation
type ValidationResult struct {
	ServiceName  string
	ContractFile string
	Passed       bool
	Errors       []error
	Warnings     []string
	ChecksPassed int
	ChecksFailed int
	ChecksTotal  int
}

// LoadContract loads a contract from YAML file
func (v *Validator) LoadContract(path string) (*Contract, error) {
	v.logger.Debug("loading contract", "path", path)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read contract file: %w", err)
	}

	var contract Contract
	if err := yaml.Unmarshal(data, &contract); err != nil {
		return nil, fmt.Errorf("failed to parse contract yaml: %w", err)
	}

	v.logger.Debug("contract loaded successfully",
		"service", contract.Service,
		"version", contract.Version,
	)

	return &contract, nil
}

// ValidateService validates a service against its contract
func (v *Validator) ValidateService(serviceName string, contractPath string) (*ValidationResult, error) {
	result := &ValidationResult{
		ServiceName:  serviceName,
		ContractFile: contractPath,
		Passed:       true,
		Errors:       make([]error, 0),
		Warnings:     make([]string, 0),
	}

	v.logger.Info("validating service", "service", serviceName)

	// load contract
	contract, err := v.LoadContract(contractPath)
	if err != nil {
		result.Passed = false
		result.Errors = append(result.Errors, err)
		return result, err
	}

	// validate contract matches service name
	if contract.Service != serviceName {
		result.Passed = false
		result.Errors = append(result.Errors,
			fmt.Errorf("contract service name mismatch: expected %s, got %s",
				serviceName, contract.Service))
	}

	// check service binary exists
	if err := v.checkBinary(serviceName, contract); err != nil {
		result.Passed = false
		result.Errors = append(result.Errors, err)
	} else {
		result.ChecksPassed++
		v.logger.Info("✓ service binary found")
	}
	result.ChecksTotal++

	// validate signal handlers
	if err := v.checkSignals(contract); err != nil {
		result.Passed = false
		result.Errors = append(result.Errors, err)
	} else {
		result.ChecksPassed++
		v.logger.Info("✓ signal handlers defined")
	}
	result.ChecksTotal++

	// validate interfaces
	if err := v.checkInterfaces(contract); err != nil {
		result.Passed = false
		result.Errors = append(result.Errors, err)
	} else {
		result.ChecksPassed++
		v.logger.Info("✓ interfaces defined")
	}
	result.ChecksTotal++

	// validate performance requirements
	if err := v.checkPerformance(contract); err != nil {
		result.Warnings = append(result.Warnings, err.Error())
	} else {
		result.ChecksPassed++
		v.logger.Info("✓ performance requirements defined")
	}
	result.ChecksTotal++

	result.ChecksFailed = result.ChecksTotal - result.ChecksPassed

	return result, nil
}

// checkBinary checks if service binary exists
func (v *Validator) checkBinary(serviceName string, contract *Contract) error {
	// look for binary in common locations
	paths := []string{
		fmt.Sprintf("../../services/%s/%s", serviceName, serviceName),
		fmt.Sprintf("../services/%s/%s", serviceName, serviceName),
		fmt.Sprintf("./services/%s/%s", serviceName, serviceName),
		fmt.Sprintf("./services/%s/ws-%s", serviceName, serviceName),
		fmt.Sprintf("./%s", serviceName),
		fmt.Sprintf("/usr/local/bin/%s", serviceName),
	}

	for _, path := range paths {
		v.logger.Debug("searching for binary", "path", path)
		if _, err := os.Stat(path); err == nil {
			v.logger.Info("found binary", "path", path)
			return nil
		}
	}

	return fmt.Errorf("service binary not found for %s", serviceName)
}

// checkSignals validates signal handling configuration
func (v *Validator) checkSignals(contract *Contract) error {
	if contract.Interfaces.Signals.SIGTERM.Behavior == "" {
		return fmt.Errorf("sigterm handler not defined")
	}
	if contract.Interfaces.Signals.SIGHUP.Behavior == "" {
		return fmt.Errorf("sighup handler not defined")
	}
	return nil
}

// checkInterfaces validates interface definitions
func (v *Validator) checkInterfaces(contract *Contract) error {
	// check at least one interface is defined
	hasInterface := contract.Interfaces.POSIXMQ.QueueName != "" ||
		contract.Interfaces.Network.TCP.BindAddress != "" ||
		contract.Interfaces.Filesystem.WatchDirectory.Path != ""

	if !hasInterface {
		return fmt.Errorf("no interfaces defined in contract")
	}
	return nil
}

// checkPerformance validates performance requirements
func (v *Validator) checkPerformance(contract *Contract) error {
	if contract.Performance.ThroughputMBPS.Target == 0 {
		return fmt.Errorf("throughput target not defined")
	}
	return nil
}

// GetContractPath returns the default contract path for a service
func GetContractPath(serviceName string) string {
	// map service names to contract files
	contractMap := map[string]string{
		"ingestion":   "ingestion_contract.yaml",
		"aggregation": "aggregation_contract.yaml",
		"query":       "query_contract.yaml",
		"discovery":   "discovery_contract.yaml",
		"c1_cli":      "c1_contract.yaml",
	}

	filename, ok := contractMap[serviceName]
	if !ok {
		filename = fmt.Sprintf("%s_contract.yaml", serviceName)
	}

	return filepath.Join("contracts", filename)
}
