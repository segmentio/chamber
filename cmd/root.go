package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// Regex's used to validate service and key names
var (
	validKeyFormat     = regexp.MustCompile(`^[A-Za-z0-9-_]+$`)
	validServiceFormat = regexp.MustCompile(`^[A-Za-z0-9-_\/.]+$`)

	numRetries     int
	chamberVersion string
)

const (
	// ShortTimeFormat is a short format for printing timestamps
	ShortTimeFormat = "01-02 15:04:05"

	// DefaultNumRetries is the default for the number of retries we'll use for our SSM client
	DefaultNumRetries = 10
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:          "chamber",
	Short:        "CLI for storing secrets",
	SilenceUsage: true,
}

func init() {
	RootCmd.PersistentFlags().IntVarP(&numRetries, "retries", "r", DefaultNumRetries, "For SSM, the number of retries we'll make before giving up")
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(vers string) {
	chamberVersion = vers
	if cmd, err := RootCmd.ExecuteC(); err != nil {
		if strings.Contains(err.Error(), "arg(s)") || strings.Contains(err.Error(), "usage") {
			cmd.Usage()
		}
		os.Exit(1)
	}
}

func validateService(service string) error {
	if !validServiceFormat.MatchString(service) {
		return fmt.Errorf("Failed to validate service name '%s'.  Only alphanumeric, dashes, forwardslases, fullstops and underscores are allowed for service names", service)
	}
	return nil
}

func validateKey(key string) error {
	if !validKeyFormat.MatchString(key) {
		return fmt.Errorf("Failed to validate key name '%s'.  Only alphanumeric, dashes, and underscores are allowed for key names", key)
	}
	return nil
}
