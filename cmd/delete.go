package cmd

import (
	"github.com/pkg/errors"
	"github.com/segmentio/chamber/v2/store"
	"github.com/segmentio/chamber/v2/utils"
	"github.com/spf13/cobra"
	analytics "gopkg.in/segmentio/analytics-go.v3"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete <service> <key>",
	Short: "Delete a secret, including all versions",
	Args:  cobra.ExactArgs(2),
	RunE:  delete,
}

var preserveKey bool

func init() {
	deleteCmd.Flags().BoolVar(&preserveKey, "preserve-key", false, "Prevent normalization of the provided key in order to delete any keys that may not have been previously normalized on write.")
	RootCmd.AddCommand(deleteCmd)
}

func delete(cmd *cobra.Command, args []string) error {
	service := utils.NormalizeService(args[0])
	if err := validateService(service); err != nil {
		return errors.Wrap(err, "Failed to validate service")
	}

	key := args[1]
	if !preserveKey {
		key = utils.NormalizeKey(key)
	}

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
