package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/segmentio/chamber/v2/store"
	"github.com/spf13/cobra"
	analytics "gopkg.in/segmentio/analytics-go.v3"
	"gopkg.in/yaml.v3"
)

var (
	importCmd = &cobra.Command{
		Use:   "import <service> <file|->",
		Short: "import secrets from json or yaml",
		Args:  cobra.ExactArgs(2),
		RunE:  importRun,
	}
)

func init() {
	AddCommandWithTagging(RootCmd, importCmd)
}

func importRun(cmd *cobra.Command, args []string) error {
	service := strings.ToLower(args[0])
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

	decoder := yaml.NewDecoder(in)
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

	for key, value := range toBeImported {
		secretId := store.SecretId{
			Service: service,
			Key:     key,
		}
		if err := secretStore.Write(secretId, value, tags); err != nil {
			return errors.Wrap(err, "Failed to write secret")
		}
	}

	fmt.Fprintf(os.Stdout, "Successfully imported %d secrets\n", len(toBeImported))
	return nil
}
