package cmd

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
)

// Regex's used to validate service and key names
var (
	validKeyFormat     = regexp.MustCompile(`^[A-Za-z0-9-_]+$`)
	validServiceFormat = regexp.MustCompile(`^[A-Za-z0-9-_]+$`)

	config         *store.Config
	base           string
	skipBaseConfig bool
	numRetries     int

	mandatoryBaseConfigPreRunE = func(cmd *cobra.Command, args []string) error {
		present, err := store.MergeConfigFromSSM(config)
		if err != nil {
			return err
		}
		if !present {
			return errors.New("chamber configuration not found at current base path. Consider using --skip-base-config if that is desirable")
		}
		return nil
	}

	optionalBaseConfigPreRunE = func(cmd *cobra.Command, args []string) error {
		present, err := store.MergeConfigFromSSM(config)
		if err != nil {
			return err
		}
		if !present {
			fmt.Fprintf(os.Stderr, "WARNING: chamber configuration not found at current base path but proceeding anyway")
		}
		return nil
	}
)

const (
	// ShortTimeFormat is a short format for printing timestamps
	ShortTimeFormat = "01-02 15:04:05"

	// DefaultNumRetries is the default for the number of retries we'll use for our SSM client
	DefaultNumRetries = 10
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:               "chamber",
	Short:             "CLI for storing secrets",
	SilenceUsage:      true,
	PersistentPreRunE: mandatoryBaseConfigPreRunE,
}

func init() {
	config = store.NewConfig()
	RootCmd.PersistentFlags().IntVarP(&numRetries, "retries", "r", DefaultNumRetries, "For SSM, the number of retries we'll make before giving up")
	config.BindPFlag(store.ConfigRetries, RootCmd.PersistentFlags().Lookup("retries"))

	RootCmd.PersistentFlags().StringVarP(&base, "base", "b", "", "Base path. If not specified, chamber operates at SSM root level")
	config.BindPFlag(store.ConfigBase, RootCmd.PersistentFlags().Lookup("base"))

	RootCmd.PersistentFlags().BoolVar(&skipBaseConfig, "skip-base-config", false, "Skip loading configuration from base path")
	config.BindPFlag(store.ConfigSkipBaseConfig, RootCmd.PersistentFlags().Lookup("skip-base-config"))
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if cmd, err := RootCmd.ExecuteC(); err != nil {
		if strings.Contains(err.Error(), "arg(s)") || strings.Contains(err.Error(), "usage") {
			cmd.Usage()
		}
		os.Exit(1)
	}
}

func validateService(service string) error {
	if !validServiceFormat.MatchString(service) {
		return fmt.Errorf("Failed to validate service name '%s'.  Only alphanumeric, dashes, and underscores are allowed for service names", service)
	}
	return nil
}

func validateKey(key string) error {
	if !validKeyFormat.MatchString(key) {
		return fmt.Errorf("Failed to validate key name '%s'.  Only alphanumeric, dashes, and underscores are allowed for key names", key)
	}
	return nil
}
