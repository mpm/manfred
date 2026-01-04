package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version   = "dev"
	cfgFile   string
	dataDir   string
	rootCmd   *cobra.Command
)

func SetVersion(v string) {
	version = v
}

func init() {
	rootCmd = &cobra.Command{
		Use:   "manfred",
		Short: "Claude Code agent runner",
		Long: `MANFRED orchestrates Claude Code to work on coding tasks.

It manages tickets (task prompts), runs Claude Code in Docker containers,
and collects results including commit messages and code changes.`,
		SilenceUsage: true,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: $HOME/.manfred/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", "data directory (default: $HOME/.manfred)")

	// Bind flags to viper
	viper.BindPFlag("data_dir", rootCmd.PersistentFlags().Lookup("data-dir"))

	// Add subcommands
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newJobCmd())
	rootCmd.AddCommand(newTicketCmd())
	rootCmd.AddCommand(newProjectCmd())
	rootCmd.AddCommand(newServeCmd())

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Warning: could not find home directory:", err)
			return
		}

		// Search for config in ~/.manfred/
		viper.AddConfigPath(home + "/.manfred")
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// Environment variables
	viper.SetEnvPrefix("MANFRED")
	viper.AutomaticEnv()

	// Read config file (ignore if not found)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintln(os.Stderr, "Warning: error reading config:", err)
		}
	}
}

func Execute() error {
	return rootCmd.Execute()
}
