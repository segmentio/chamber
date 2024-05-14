package cmd

import (
	"fmt"
	"io"
	"os"

	analytics "github.com/segmentio/analytics-go/v3"
	"github.com/segmentio/chamber/v2/store"
	"github.com/segmentio/chamber/v2/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	importCmd = &cobra.Command{
		Use:   "import <service> <file|->",
		Short: "import secrets from json or yaml",
		Args:  cobra.ExactArgs(2),
		RunE:  importRun,
	}
	normalizeKeys bool
)

func init() {
	importCmd.Flags().BoolVar(&normalizeKeys, "normalize-keys", false, "Normalize keys to match how `chamber write` would handle them. If not specified, keys will be written exactly how they are defined in the import source.")
	RootCmd.AddCommand(importCmd)
}

func importRun(cmd *cobra.Command, args []string) error {
	service := utils.NormalizeService(args[0])
	if err := validateService(service); err != nil {
		return fmt.Errorf("Failed to validate service: %w", err)
	}

	var in io.Reader
	var err error

	file := args[1]
	if file == "-" {
		in = os.Stdin
	} else {
		in, err = os.Open(file)
		if err != nil {
			return fmt.Errorf("Failed to open file: %w", err)
		}
	}

	var toBeImported map[string]string

	decoder := yaml.NewDecoder(in)
	if err := decoder.Decode(&toBeImported); err != nil {
		return fmt.Errorf("Failed to decode input as json: %w", err)
	}

	if analyticsEnabled && analyticsClient != nil {
		analyticsClient.Enqueue(analytics.Track{
			UserId: username,
			Event:  "Ran Command",
			Properties: analytics.NewProperties().
				Set("command", "import").
				Set("chamber-version", chamberVersion).
				Set("service", service).
				Set("backend", backend),
		})
	}

	secretStore, err := getSecretStore(cmd.Context())
	if err != nil {
		return fmt.Errorf("Failed to get secret store: %w", err)
	}

	for key, value := range toBeImported {
		if normalizeKeys {
			key = utils.NormalizeKey(key)
		}
		secretId := store.SecretId{
			Service: service,
			Key:     key,
		}
		if err := secretStore.Write(cmd.Context(), secretId, value); err != nil {
			return fmt.Errorf("Failed to write secret: %w", err)
		}
	}

	fmt.Fprintf(os.Stdout, "Successfully imported %d secrets\n", len(toBeImported))
	return nil
}
