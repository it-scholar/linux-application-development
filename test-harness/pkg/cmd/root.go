package cmd

import (
	"os"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	logger  *log.Logger

	rootCmd = &cobra.Command{
		Use:   "test-harness",
		Short: "weather station microservices test harness",
		Long: `a comprehensive go-based testing framework for the weather station
microservices system. validates service contracts, runs performance benchmarks,
executes chaos tests, and provides detailed grading.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// setup logger with configured level
			logger = log.New(os.Stderr)

			logLevel := viper.GetString("log-level")
			switch logLevel {
			case "debug":
				logger.SetLevel(log.DebugLevel)
			case "info":
				logger.SetLevel(log.InfoLevel)
			case "warn":
				logger.SetLevel(log.WarnLevel)
			case "error":
				logger.SetLevel(log.ErrorLevel)
			default:
				logger.SetLevel(log.InfoLevel)
			}

			// set report caller for verbose mode
			if viper.GetBool("verbose") {
				logger.SetReportCaller(true)
			}
		},
	}
)

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().String("log-level", "info", "log level (debug|info|warn|error)")
	rootCmd.PersistentFlags().Int("parallel", 4, "parallelism level")
	rootCmd.PersistentFlags().String("output", "console", "output format (console|json|junit|html)")
	rootCmd.PersistentFlags().Bool("verbose", false, "detailed output")
	rootCmd.PersistentFlags().Bool("fail-fast", false, "stop on first failure")

	viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("parallel", rootCmd.PersistentFlags().Lookup("parallel"))
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("fail-fast", rootCmd.PersistentFlags().Lookup("fail-fast"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.config/ws-test")
		viper.AddConfigPath("/etc/ws-test")
	}

	viper.SetEnvPrefix("ws_test")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		if logger != nil {
			logger.Info("using config file", "path", viper.ConfigFileUsed())
		}
	}
}
