package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"weather-station-test/pkg/mock"
)

var mockCmd = &cobra.Command{
	Use:   "mock",
	Short: "run mock services",
	Long: `starts mock services for isolated testing.

mock services:
  - s1: ingestion service mock
  - s2: aggregation service mock
  - s3: query service mock
  - s4: discovery service mock

examples:
  # start mocks for s3 and s4
  test-harness mock --services s3,s4

  # start all mocks on specific ports
  test-harness mock --services s1,s2,s3,s4 --ports 9001,9002,9003,9004

  # persistent mode (run until SIGTERM)
  test-harness mock --services s3 --persist`,
	RunE: runMock,
}

var mockFlags struct {
	services []string
	ports    []int
	persist  bool
}

func init() {
	rootCmd.AddCommand(mockCmd)

	mockCmd.Flags().StringArrayVarP(&mockFlags.services, "services", "s", nil, "which mocks to start (required)")
	mockCmd.Flags().IntSliceVarP(&mockFlags.ports, "ports", "p", nil, "port assignments")
	mockCmd.Flags().BoolVar(&mockFlags.persist, "persist", false, "keep running until SIGTERM")
	mockCmd.MarkFlagRequired("services")
}

func runMock(cmd *cobra.Command, args []string) error {
	logger.Info("starting mock services", "services", mockFlags.services)

	if len(mockFlags.ports) > 0 {
		logger.Info("port assignments", "ports", mockFlags.ports)
	}

	// Create and start mock servers
	servers := make([]*mock.Server, 0, len(mockFlags.services))

	for i, service := range mockFlags.services {
		port := 9000 + i
		if i < len(mockFlags.ports) {
			port = mockFlags.ports[i]
		}

		config := mock.Config{
			Address:  "localhost",
			Port:     port,
			Protocol: "tcp",
		}

		server := mock.NewServer(config)

		// Register default handlers
		for msgType, handler := range mock.DefaultHandlers() {
			server.RegisterHandler(msgType, handler)
		}

		if err := server.Start(); err != nil {
			logger.Error("failed to start mock service", "service", service, "error", err)
			// Stop already started servers
			for _, s := range servers {
				s.Stop()
			}
			return err
		}

		servers = append(servers, server)
		logger.Info("mock service started",
			"service", service,
			"address", server.Address(),
			"protocol", config.Protocol)
	}

	logger.Info("all mock services started", "count", len(servers))

	if mockFlags.persist {
		logger.Info("press Ctrl+C to stop...")

		// wait for interrupt
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		logger.Info("stopping mock services...")
		for _, server := range servers {
			server.Stop()
		}
		logger.Info("mock services stopped")
	} else {
		// Non-persistent mode - just report success
		logger.Info("mock services ready (non-persistent mode)")
	}

	return nil
}
