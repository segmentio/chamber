package cmd

import (
	"fmt"

	analytics "github.com/segmentio/analytics-go/v3"
	"github.com/segmentio/chamber/v2/store"
	"github.com/segmentio/chamber/v2/utils"
	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete <service> <key>",
	Short: "Delete a secret, including all versions",
	Args:  cobra.ExactArgs(2),
	RunE:  delete,
}

var exactKey bool

func init() {
	deleteCmd.Flags().BoolVar(&exactKey, "exact-key", false, "Prevent normalization of the provided key in order to delete any keys that match the exact provided casing.")
	RootCmd.AddCommand(deleteCmd)
}

func delete(cmd *cobra.Command, args []string) error {
	service := utils.NormalizeService(args[0])
	if err := validateService(service); err != nil {
		return fmt.Errorf("Failed to validate service: %w", err)
	}

	key := args[1]
	if !exactKey {
		key = utils.NormalizeKey(key)
	}

	if err := validateKey(key); err != nil {
		return fmt.Errorf("Failed to validate key: %w", err)
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
	secretStore, err := getSecretStore(cmd.Context())
	if err != nil {
		return fmt.Errorf("Failed to get secret store: %w", err)
	}
	secretId := store.SecretId{
		Service: service,
		Key:     key,
	}

	return secretStore.Delete(cmd.Context(), secretId)
}
