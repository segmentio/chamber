package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
	analytics "gopkg.in/segmentio/analytics-go.v3"
)

var (
	importCmd = &cobra.Command{
		Use:   "import <service> <file|->",
		Short: "import secrets from json",
		Args:  cobra.ExactArgs(2),
		RunE:  importRun,
	}
)

func init() {
	importCmd.Flags().BoolVarP(&caseSensitive, "case-sensitive", "", false, "Enable case sensitive service names and keys. Defaults to lowercase keys and services")
	RootCmd.AddCommand(importCmd)
}

func importRun(cmd *cobra.Command, args []string) error {
	service := strings.ToLower(args[0])
	if caseSensitive {
		service = args[0]
	}
	if err := validateService(service); err != nil {
		return errors.Wrap(err, "Failed to validate service")
	}

	var in io.Reader
	var err error

	file := args[1]
	if file == "-" {
		in = os.Stdin
	} else {
		in, err = os.Open(file)
		if err != nil {
			return errors.Wrap(err, "Failed to open file")
		}
	}

	var toBeImported map[string]string

	decoder := json.NewDecoder(in)
	if err := decoder.Decode(&toBeImported); err != nil {
		return errors.Wrap(err, "Failed to decode input as json")
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

	secretStore, err := getSecretStore()
	if err != nil {
		return errors.Wrap(err, "Failed to get secret store")
	}

	// Check for duplicate secrets based on lowercased key names
	toBeImportedLowercase := make(map[string]string)

	for k, v := range toBeImported {
		toBeImportedLowercase[strings.ToLower(k)] = v
	}

	secretsProvided := len(toBeImported)
	secretsProvidedUnique := len(toBeImportedLowercase)

	if secretsProvided != secretsProvidedUnique {
		if !caseSensitive {
			err := fmt.Errorf("Refusing to import %d of %d provided secrets due to mixed case keys", secretsProvidedUnique, secretsProvided)
			return err
		}
	}

	for key, value := range toBeImported {
		if !caseSensitive {
			key = strings.ToLower(key)
		}
		secretId := store.SecretId{
			Service: service,
			Key:     key,
		}
		if err := secretStore.Write(secretId, value); err != nil {
			return errors.Wrap(err, "Failed to write secret")
		}
	}

	fmt.Fprintf(os.Stdout, "Successfully imported %d secrets\n", len(toBeImported))
	return nil
}
