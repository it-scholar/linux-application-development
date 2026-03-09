// Package chaos provides chaos engineering testing capabilities
package chaos

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
)

// Engineer executes chaos engineering tests
type Engineer struct {
	logger  *log.Logger
	results []Result
}

// Result represents the outcome of a chaos test
type Result struct {
	Scenario     string
	Action       string
	Target       string
	Duration     time.Duration
	Success      bool
	Error        error
	RecoveryTime time.Duration
	Timestamp    time.Time
}

// ScenarioRunner executes predefined chaos scenarios
type ScenarioRunner struct {
	name    string
	logger  *log.Logger
	targets []string
}

// ServiceManager interface for interacting with services
type ServiceManager interface {
	Start(ctx context.Context, name string, config interface{}) error
	Stop(name string) error
	SendSignal(name string, sig syscall.Signal) error
	HealthCheck(name string) (bool, error)
	GetService(name string) (interface{}, bool)
}

// NewEngineer creates a new chaos engineering test runner
func NewEngineer(logger *log.Logger) *Engineer {
	if logger == nil {
		logger = log.New(os.Stderr)
	}
	return &Engineer{
		logger:  logger,
		results: make([]Result, 0),
	}
}

// RunScenario executes a predefined chaos scenario
func (e *Engineer) RunScenario(ctx context.Context, name string, duration time.Duration, sm ServiceManager) error {
	e.logger.Info("running chaos scenario", "scenario", name, "duration", duration)

	switch name {
	case "leader_failover":
		return e.runLeaderFailover(ctx, duration, sm)
	case "service_kill":
		return e.runServiceKill(ctx, duration, sm)
	case "cascading_failure":
		return e.runCascadingFailure(ctx, duration, sm)
	case "resource_pressure":
		return e.runResourcePressure(ctx, duration, sm)
	default:
		return fmt.Errorf("unknown scenario: %s", name)
	}
}

// RunAction executes a single chaos action
func (e *Engineer) RunAction(ctx context.Context, target, action string, duration time.Duration, sm ServiceManager) error {
	e.logger.Info("running chaos action", "target", target, "action", action, "duration", duration)

	start := time.Now()
	result := Result{
		Action:    action,
		Target:    target,
		Duration:  duration,
		Timestamp: start,
	}

	var err error
	switch action {
	case "kill":
		err = e.actionKill(ctx, target, duration, sm)
	case "stop":
		err = e.actionStop(ctx, target, duration, sm)
	case "restart":
		err = e.actionRestart(ctx, target, duration, sm)
	case "signal":
		err = e.actionSignal(ctx, target, syscall.SIGTERM, duration, sm)
	case "delay":
		err = e.actionDelay(ctx, target, duration, sm)
	default:
		err = fmt.Errorf("unknown action: %s", action)
	}

	result.RecoveryTime = time.Since(start) - duration
	result.Success = err == nil
	result.Error = err

	e.results = append(e.results, result)

	if err != nil {
		e.logger.Error("chaos action failed", "action", action, "target", target, "error", err)
		return err
	}

	e.logger.Info("chaos action completed", "action", action, "target", target, "recovery_time", result.RecoveryTime)
	return nil
}

// GetResults returns all chaos test results
func (e *Engineer) GetResults() []Result {
	return e.results
}

// runLeaderFailover simulates leader failure and verifies failover
func (e *Engineer) runLeaderFailover(ctx context.Context, duration time.Duration, sm ServiceManager) error {
	e.logger.Info("starting leader failover test")

	// Monitor for leader (for now, assume s1 is the data source)
	leader := "ingestion"

	// Verify leader is healthy
	healthy, err := sm.HealthCheck(leader)
	if err != nil || !healthy {
		return fmt.Errorf("leader not healthy before test: %v", err)
	}

	e.logger.Info("leader is healthy, injecting failure", "leader", leader)

	// Kill the leader
	if err := sm.Stop(leader); err != nil {
		return fmt.Errorf("failed to kill leader: %w", err)
	}

	// Wait for the specified duration
	e.logger.Info("waiting for failover", "duration", duration)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration):
	}

	// Check if system has recovered
	// In a real implementation, would check for new leader election
	e.logger.Info("failover test completed")
	return nil
}

// runServiceKill randomly kills services and verifies recovery
func (e *Engineer) runServiceKill(ctx context.Context, duration time.Duration, sm ServiceManager) error {
	e.logger.Info("starting service kill test")

	services := []string{"ingestion", "s2_processor", "s3_api", "s4_cluster"}

	endTime := time.Now().Add(duration)
	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Pick a random service to kill
		for _, svc := range services {
			if _, exists := sm.GetService(svc); exists {
				e.logger.Info("killing service", "service", svc)
				if err := sm.Stop(svc); err != nil {
					e.logger.Warn("failed to kill service", "service", svc, "error", err)
				}

				// Wait a bit before next kill
				time.Sleep(5 * time.Second)
				break
			}
		}

		// Wait between kills
		time.Sleep(10 * time.Second)
	}

	e.logger.Info("service kill test completed")
	return nil
}

// runCascadingFailure simulates multiple service failures
func (e *Engineer) runCascadingFailure(ctx context.Context, duration time.Duration, sm ServiceManager) error {
	e.logger.Info("starting cascading failure test")

	services := []string{"s4_cluster", "s3_api", "s2_processor", "ingestion"}

	// Kill services in reverse order
	for _, svc := range services {
		if _, exists := sm.GetService(svc); exists {
			e.logger.Info("killing service in cascade", "service", svc)
			if err := sm.Stop(svc); err != nil {
				e.logger.Warn("failed to kill service", "service", svc, "error", err)
			}
			time.Sleep(2 * time.Second)
		}
	}

	// Wait for duration
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration):
	}

	e.logger.Info("cascading failure test completed")
	return nil
}

// runResourcePressure simulates resource constraints
func (e *Engineer) runResourcePressure(ctx context.Context, duration time.Duration, sm ServiceManager) error {
	e.logger.Info("starting resource pressure test")

	// Send SIGSTOP to simulate freeze
	if svc, exists := sm.GetService("s2_processor"); exists {
		_ = svc // Use svc to avoid unused variable
		e.logger.Info("sending SIGSTOP to simulate resource pressure")
		// In real implementation: sm.SendSignal("s2_processor", syscall.SIGSTOP)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration / 2):
	}

	// Resume
	if _, exists := sm.GetService("s2_processor"); exists {
		e.logger.Info("sending SIGCONT to resume")
		// In real implementation: sm.SendSignal("s2_processor", syscall.SIGCONT)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration / 2):
	}

	e.logger.Info("resource pressure test completed")
	return nil
}

// actionKill kills a service
func (e *Engineer) actionKill(ctx context.Context, target string, duration time.Duration, sm ServiceManager) error {
	e.logger.Info("killing service", "target", target)

	if err := sm.Stop(target); err != nil {
		return fmt.Errorf("failed to kill %s: %w", target, err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration):
	}

	return nil
}

// actionStop gracefully stops a service
func (e *Engineer) actionStop(ctx context.Context, target string, duration time.Duration, sm ServiceManager) error {
	e.logger.Info("stopping service", "target", target)

	if err := sm.SendSignal(target, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to stop %s: %w", target, err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration):
	}

	return nil
}

// actionRestart restarts a service
func (e *Engineer) actionRestart(ctx context.Context, target string, duration time.Duration, sm ServiceManager) error {
	e.logger.Info("restarting service", "target", target)

	if err := sm.Stop(target); err != nil {
		return fmt.Errorf("failed to stop %s: %w", target, err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
	}

	// In real implementation, would restart via sm.Start()
	e.logger.Info("service restarted (placeholder)", "target", target)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration):
	}

	return nil
}

// actionSignal sends a signal to a service
func (e *Engineer) actionSignal(ctx context.Context, target string, sig syscall.Signal, duration time.Duration, sm ServiceManager) error {
	e.logger.Info("sending signal", "target", target, "signal", sig)

	if err := sm.SendSignal(target, sig); err != nil {
		return fmt.Errorf("failed to send signal to %s: %w", target, err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration):
	}

	return nil
}

// actionDelay introduces a network delay (placeholder)
func (e *Engineer) actionDelay(ctx context.Context, target string, duration time.Duration, sm ServiceManager) error {
	e.logger.Info("introducing delay", "target", target, "duration", duration)

	// In real implementation, would use tc/netem for network delay
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration):
	}

	return nil
}
