package cmd

import (
	"strings"

	"github.com/pkg/errors"
	analytics "github.com/segmentio/analytics-go"
	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete <service> <key>",
	Short: "Delete a secret, including all versions",
	Args:  cobra.ExactArgs(2),
	RunE:  delete,
}

func init() {
	RootCmd.AddCommand(deleteCmd)
}

func delete(cmd *cobra.Command, args []string) error {
	service := strings.ToLower(args[0])
	if err := validateService(service); err != nil {
		return errors.Wrap(err, "Failed to validate service")
	}

	key := strings.ToLower(args[1])
	if err := validateKey(key); err != nil {
		return errors.Wrap(err, "Failed to validate key")
	}

	if analyticsEnabled && analyticsClient != nil {
		analyticsClient.Enqueue(analytics.Track{
			UserId: username,
			Event:  "Ran Command",
			Properties: analytics.NewProperties().
				Set("command", "delete").
				Set("chamber-version", chamberVersion).
				Set("service", service).
				Set("key", key).
				Set("backend", backend),
		})
	}
	secretStore, err := getSecretStore()
	if err != nil {
		return errors.Wrap(err, "Failed to get secret store")
	}
	secretId := store.SecretId{
		Service: service,
		Key:     key,
	}

	return secretStore.Delete(secretId)
}
