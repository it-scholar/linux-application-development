package main

import (
	"os"

	"github.com/charmbracelet/log"
	"weather-station-test/pkg/cmd"
)

func main() {
	// setup default logger
	logger := log.New(os.Stderr)
	logger.SetLevel(log.InfoLevel)

	if err := cmd.Execute(); err != nil {
		logger.Error("test harness failed", "error", err)
		os.Exit(1)
	}
}
