package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/pkg/errors"
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
	// one of *Backend consts
	backend             string
	backendFlag         string
	backendS3BucketFlag string
	kmsKeyAliasFlag     string

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
	SSMBackend  = "SSM"
	S3Backend   = "S3"
	S3KMSBackend   = "S3-KMS"

	BackendEnvVar = "CHAMBER_SECRET_BACKEND"
	BucketEnvVar  = "CHAMBER_S3_BUCKET"
	KMSKeyEnvVar  = "CHAMBER_KMS_KEY_ALIAS"
)

var Backends = []string{SSMBackend, S3Backend, NullBackend, S3KMSBackend}

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
	RootCmd.PersistentFlags().StringVarP(&backendFlag, "backend", "b", "ssm",
		`Backend to use; AKA $CHAMBER_SECRET_BACKEND
	null: no-op
	ssm: SSM Parameter Store
	s3: S3; requires --backend-s3-bucket
	s3-kms: S3 using AWS-KMS encryption; requires --backend-s3-bucket and --kms-key-alias set (if you want to write or delete keys).`,
	)
	RootCmd.PersistentFlags().StringVarP(&backendS3BucketFlag, "backend-s3-bucket", "", "", "bucket for S3 backend; AKA $CHAMBER_S3_BUCKET")
	RootCmd.PersistentFlags().StringVarP(&kmsKeyAliasFlag, "kms-key-alias", "", "alias/parameter_store_key", "KMS Key Alias for writing and deleting secrets; AKA $CHAMBER_KMS_KEY_ALIAS. This option is currently only supported for the S3-KMS backend.")
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
	rootPflags := RootCmd.PersistentFlags()
	if backendEnvVarValue := os.Getenv(BackendEnvVar); !rootPflags.Changed("backend") && backendEnvVarValue != "" {
		backend = backendEnvVarValue
	} else {
		backend = backendFlag
	}
	backend = strings.ToUpper(backend)

	var s store.Store
	var err error

	switch backend {
	case NullBackend:
		s = store.NewNullStore()
	case S3Backend:
		var bucket string
		if bucketEnvVarValue := os.Getenv(BucketEnvVar); !rootPflags.Changed("backend-s3-bucket") && bucketEnvVarValue != "" {
			bucket = bucketEnvVarValue
		} else {
			bucket = backendS3BucketFlag
		}
		if bucket == "" {
			return nil, errors.New("Must set bucket for s3 backend")
		}
		s, err = store.NewS3StoreWithBucket(numRetries, bucket)
	case S3KMSBackend:
		var bucket string
		if bucketEnvVarValue := os.Getenv(BucketEnvVar); !rootPflags.Changed("backend-s3-bucket") && bucketEnvVarValue != "" {
			bucket = bucketEnvVarValue
		} else {
			bucket = backendS3BucketFlag
		}
		if bucket == "" {
			return nil, errors.New("Must set bucket for s3 backend")
		}

		var kmsKeyAlias string
		if kmsKeyAliasValue := os.Getenv(KMSKeyEnvVar); !rootPflags.Changed("kms-key-alias") && kmsKeyAliasValue != "" {
			kmsKeyAlias = kmsKeyAliasValue
		} else {
			kmsKeyAlias = kmsKeyAliasFlag
		}

		if kmsKeyAlias == "" {
			return nil, errors.New("Must set kmsKeyAlias for S3 KMS backend")
		}

		s, err = store.NewS3KMSStore(numRetries, bucket, kmsKeyAlias)
	case SSMBackend:
		s, err = store.NewSSMStore(numRetries)
	default:
		return nil, fmt.Errorf("invalid backend `%s`", backend)
	}
	return s, err
}

func prerun(cmd *cobra.Command, args []string) {
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
