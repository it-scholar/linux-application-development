package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
)

// manager handles service lifecycle
type Manager struct {
	logger   *log.Logger
	services map[string]*Service
}

// service represents a running service instance
type Service struct {
	Name      string
	Binary    string
	Config    string
	PID       int
	Cmd       *exec.Cmd
	StartTime time.Time
	Health    HealthStatus
	Ports     map[string]int
}

// healthstatus represents service health
type HealthStatus struct {
	Healthy   bool
	LastCheck time.Time
	Message   string
}

// newmanager creates a new service manager
func NewManager(logger *log.Logger) *Manager {
	if logger == nil {
		logger = log.New(os.Stderr)
	}
	return &Manager{
		logger:   logger,
		services: make(map[string]*Service),
	}
}

// start starts a service
func (m *Manager) Start(ctx context.Context, name string, config ServiceConfig) (*Service, error) {
	m.logger.Info("starting service", "service", name)

	// check if already running
	if svc, exists := m.services[name]; exists {
		m.logger.Warn("service already running", "service", name, "pid", svc.PID)
		return svc, fmt.Errorf("service %s already running with pid %d", name, svc.PID)
	}

	// find binary
	binary := config.Binary
	if binary == "" {
		binary = m.findBinary(name)
	}

	if binary == "" {
		return nil, fmt.Errorf("binary not found for service %s", name)
	}

	// prepare command
	args := []string{}
	if config.Config != "" {
		args = append(args, "--config", config.Config)
	}
	if config.Daemon {
		args = append(args, "--daemon")
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// start service
	if err := cmd.Start(); err != nil {
		m.logger.Error("failed to start service", "service", name, "error", err)
		return nil, fmt.Errorf("failed to start %s: %w", name, err)
	}

	service := &Service{
		Name:      name,
		Binary:    binary,
		Config:    config.Config,
		PID:       cmd.Process.Pid,
		Cmd:       cmd,
		StartTime: time.Now(),
		Ports:     config.Ports,
		Health: HealthStatus{
			Healthy:   true,
			LastCheck: time.Now(),
			Message:   "started",
		},
	}

	m.services[name] = service

	m.logger.Info("service started",
		"service", name,
		"pid", service.PID,
		"binary", binary,
	)

	// wait a moment for service to initialize
	time.Sleep(500 * time.Millisecond)

	return service, nil
}

// stop stops a service
func (m *Manager) Stop(name string) error {
	service, exists := m.services[name]
	if !exists {
		return fmt.Errorf("service %s not running", name)
	}

	m.logger.Info("stopping service", "service", name, "pid", service.PID)

	// send sigterm for graceful shutdown
	if err := service.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		m.logger.Error("failed to send sigterm", "service", name, "error", err)
		// force kill
		if err := service.Cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill %s: %w", name, err)
		}
	}

	// wait for process to exit
	done := make(chan error)
	go func() {
		done <- service.Cmd.Wait()
	}()

	select {
	case <-done:
		m.logger.Info("service stopped gracefully", "service", name)
	case <-time.After(10 * time.Second):
		m.logger.Warn("service did not stop gracefully, forcing kill", "service", name)
		service.Cmd.Process.Kill()
	}

	delete(m.services, name)
	return nil
}

// stopall stops all running services
func (m *Manager) StopAll() error {
	m.logger.Info("stopping all services", "count", len(m.services))

	for name := range m.services {
		if err := m.Stop(name); err != nil {
			m.logger.Error("failed to stop service", "service", name, "error", err)
		}
	}

	return nil
}

// healthcheck checks if a service is healthy
func (m *Manager) HealthCheck(name string) (*HealthStatus, error) {
	service, exists := m.services[name]
	if !exists {
		return nil, fmt.Errorf("service %s not found", name)
	}

	// check if process is still running
	process, err := os.FindProcess(service.PID)
	if err != nil {
		service.Health = HealthStatus{
			Healthy:   false,
			LastCheck: time.Now(),
			Message:   "process not found",
		}
		return &service.Health, nil
	}

	// check if process is still alive
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		service.Health = HealthStatus{
			Healthy:   false,
			LastCheck: time.Now(),
			Message:   "process not responding",
		}
		return &service.Health, nil
	}

	service.Health = HealthStatus{
		Healthy:   true,
		LastCheck: time.Now(),
		Message:   "running",
	}

	return &service.Health, nil
}

// getservice returns a service by name
func (m *Manager) GetService(name string) (*Service, bool) {
	svc, exists := m.services[name]
	return svc, exists
}

// getservicenames returns list of running service names
func (m *Manager) GetServiceNames() []string {
	names := make([]string, 0, len(m.services))
	for name := range m.services {
		names = append(names, name)
	}
	return names
}

// findbinary finds the binary for a service
func (m *Manager) findBinary(name string) string {
	// common binary names to try
	names := []string{
		name,
		fmt.Sprintf("ws-%s", name),
		fmt.Sprintf("ws_%s", name),
	}

	// common paths to search
	paths := []string{
		"./services/%s/%s",
		"./services/%s/bin/%s",
		"./bin/%s",
		"./%s",
		"/usr/local/bin/%s",
	}

	for _, n := range names {
		for _, p := range paths {
			path := fmt.Sprintf(p, name, n)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}

	return ""
}

// serviceconfig for starting a service
type ServiceConfig struct {
	Binary string
	Config string
	Daemon bool
	Ports  map[string]int
}

// sendsignal sends a signal to a service
func (m *Manager) SendSignal(name string, sig syscall.Signal) error {
	service, exists := m.services[name]
	if !exists {
		return fmt.Errorf("service %s not running", name)
	}

	m.logger.Debug("sending signal to service",
		"service", name,
		"signal", sig,
		"pid", service.PID,
	)

	return service.Cmd.Process.Signal(sig)
}

// getpidfile returns the pid file path for a service
func GetPIDFile(name string) string {
	return filepath.Join("/tmp", fmt.Sprintf("ws-%s.pid", name))
}

// writepidfile writes the pid to a file
func (m *Manager) WritePIDFile(service *Service) error {
	pidFile := GetPIDFile(service.Name)
	return os.WriteFile(pidFile, []byte(strconv.Itoa(service.PID)), 0644)
}

// readpidfile reads the pid from a file
func ReadPIDFile(name string) (int, error) {
	pidFile := GetPIDFile(name)
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}
