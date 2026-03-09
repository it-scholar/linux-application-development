package cmd

import (
	"context"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"weather-station-test/pkg/chaos"
	"weather-station-test/pkg/services"
)

var chaosCmd = &cobra.Command{
	Use:   "chaos",
	Short: "run chaos engineering tests",
	Long: `executes chaos engineering tests to validate system resilience.

scenarios:
  - leader_failover: kill leader and verify election
  - network_partition: partition network between nodes
  - resource_pressure: cpu/memory pressure testing
  - cascading_failure: multiple service failures

examples:
  # run leader failover scenario
  test-harness chaos --scenario leader_failover --duration 60s

  # custom chaos test
  test-harness chaos --target s4 --action partition --duration 30s`,
	RunE: runChaos,
}

var chaosFlags struct {
	scenario string
	duration time.Duration
	target   string
	action   string
}

func init() {
	rootCmd.AddCommand(chaosCmd)

	chaosCmd.Flags().StringVarP(&chaosFlags.scenario, "scenario", "s", "", "predefined scenario")
	chaosCmd.Flags().DurationVarP(&chaosFlags.duration, "duration", "d", 60*time.Second, "chaos duration")
	chaosCmd.Flags().StringVarP(&chaosFlags.target, "target", "t", "", "target service")
	chaosCmd.Flags().StringVarP(&chaosFlags.action, "action", "a", "", "chaos action")
}

func runChaos(cmd *cobra.Command, args []string) error {
	if chaosFlags.scenario == "" && chaosFlags.action == "" {
		logger.Error("must specify either --scenario or --action")
		return nil
	}

	// Create chaos engineer
	engineer := chaos.NewEngineer(logger)

	// Create service manager wrapper
	sm := services.NewManager(logger)

	ctx, cancel := context.WithTimeout(context.Background(), chaosFlags.duration+30*time.Second)
	defer cancel()

	if chaosFlags.scenario != "" {
		logger.Info("running chaos scenario", "scenario", chaosFlags.scenario)
		if err := engineer.RunScenario(ctx, chaosFlags.scenario, chaosFlags.duration, &chaosServiceManager{sm}); err != nil {
			logger.Error("chaos scenario failed", "scenario", chaosFlags.scenario, "error", err)
			return err
		}
	} else {
		if chaosFlags.target == "" {
			logger.Error("--target is required when using --action")
			return nil
		}
		logger.Info("running chaos action", "action", chaosFlags.action, "target", chaosFlags.target)
		if err := engineer.RunAction(ctx, chaosFlags.target, chaosFlags.action, chaosFlags.duration, &chaosServiceManager{sm}); err != nil {
			logger.Error("chaos action failed", "action", chaosFlags.action, "target", chaosFlags.target, "error", err)
			return err
		}
	}

	logger.Info("=== chaos test results ===")
	for _, result := range engineer.GetResults() {
		if result.Scenario != "" {
			logger.Info("scenario completed",
				"name", result.Scenario,
				"success", result.Success,
				"recovery_time", result.RecoveryTime,
			)
		} else {
			logger.Info("action completed",
				"action", result.Action,
				"target", result.Target,
				"success", result.Success,
				"recovery_time", result.RecoveryTime,
			)
		}
	}
	logger.Info("chaos test completed")

	return nil
}

// chaosServiceManager adapts services.Manager to chaos.ServiceManager interface
type chaosServiceManager struct {
	mgr *services.Manager
}

func (c *chaosServiceManager) Start(ctx context.Context, name string, config interface{}) error {
	// Not used in chaos testing
	return nil
}

func (c *chaosServiceManager) Stop(name string) error {
	return c.mgr.Stop(name)
}

func (c *chaosServiceManager) SendSignal(name string, sig syscall.Signal) error {
	return c.mgr.SendSignal(name, sig)
}

func (c *chaosServiceManager) HealthCheck(name string) (bool, error) {
	status, err := c.mgr.HealthCheck(name)
	if err != nil {
		return false, err
	}
	return status.Healthy, nil
}

func (c *chaosServiceManager) GetService(name string) (interface{}, bool) {
	return c.mgr.GetService(name)
}
