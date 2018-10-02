package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
	analytics "gopkg.in/segmentio/analytics-go.v3"
)

// Regex's used to validate service and key names
var (
	validKeyFormat         = regexp.MustCompile(`^[\w\-\.]+$`)
	validServiceFormat     = regexp.MustCompile(`^[\w\-\.]+$`)
	validServicePathFormat = regexp.MustCompile(`^[\w\-\.]+(\/[\w\-\.]+)*$`)

	verbose        bool
	numRetries     int
	chamberVersion string
	backend        string

	analyticsEnabled  bool
	analyticsWriteKey string
	analyticsClient   analytics.Client
	username          string
)

const (
	// ShortTimeFormat is a short format for printing timestamps
	ShortTimeFormat = "01-02 15:04:05"

	// DefaultNumRetries is the default for the number of retries we'll use for our SSM client
	DefaultNumRetries = 10
)

const (
	NullBackend = "NULL"
	SSMBackend = "SSM"
	S3Backend  = "S3"

	BackendEnvVar = "CHAMBER_SECRET_BACKEND"
)

var Backends = []string{SSMBackend, S3Backend, NullBackend}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:               "chamber",
	Short:             "CLI for storing secrets",
	SilenceUsage:      true,
	PersistentPreRun:  prerun,
	PersistentPostRun: postrun,
}

func init() {
	RootCmd.PersistentFlags().IntVarP(&numRetries, "retries", "r", DefaultNumRetries, "For SSM, the number of retries we'll make before giving up")
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "", false, "Print more information to STDOUT")
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(vers string, writeKey string) {
	chamberVersion = vers

	analyticsWriteKey = writeKey
	analyticsEnabled = analyticsWriteKey != ""

	if cmd, err := RootCmd.ExecuteC(); err != nil {
		if strings.Contains(err.Error(), "arg(s)") || strings.Contains(err.Error(), "usage") {
			cmd.Usage()
		}
		os.Exit(1)
	}
}

func validateService(service string) error {
	_, noPaths := os.LookupEnv("CHAMBER_NO_PATHS")
	if noPaths {
		if !validServiceFormat.MatchString(service) {
			return fmt.Errorf("Failed to validate service name '%s'.  Only alphanumeric, dashes, fullstops and underscores are allowed for service names", service)
		}
	} else {
		if !validServicePathFormat.MatchString(service) {
			return fmt.Errorf("Failed to validate service name '%s'.  Only alphanumeric, dashes, forwardslashes, fullstops and underscores are allowed for service names", service)
		}
	}

	return nil
}

func validateKey(key string) error {
	if !validKeyFormat.MatchString(key) {
		return fmt.Errorf("Failed to validate key name '%s'.  Only alphanumeric, dashes, fullstops and underscores are allowed for key names", key)
	}
	return nil
}

func getSecretStore() (store.Store, error) {
	backend := strings.ToUpper(os.Getenv(BackendEnvVar))

	var s store.Store
	var err error
	switch backend {
	case NullBackend:
		s = store.NewNullStore()
	case S3Backend:
		s, err = store.NewS3Store(numRetries)
	case SSMBackend:
		fallthrough
	default:
		s, err = store.NewSSMStore(numRetries)
	}
	return s, err
}

func prerun(cmd *cobra.Command, args []string) {
	backend = strings.ToUpper(os.Getenv(BackendEnvVar))

	if analyticsEnabled {
		// set up analytics client
		analyticsClient, _ = analytics.NewWithConfig(analyticsWriteKey, analytics.Config{
			BatchSize: 1,
		})

		username = os.Getenv("USER")
		analyticsClient.Enqueue(analytics.Identify{
			UserId: username,
			Traits: analytics.NewTraits().
				Set("chamber-version", chamberVersion),
		})
	}
}

func postrun(cmd *cobra.Command, args []string) {
	if analyticsEnabled && analyticsClient != nil {
		analyticsClient.Close()
	}
}
