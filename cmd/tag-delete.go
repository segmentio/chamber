package cmd

import (
	"fmt"

	analytics "github.com/segmentio/analytics-go/v3"
	"github.com/segmentio/chamber/v3/store"
	"github.com/segmentio/chamber/v3/utils"
	"github.com/spf13/cobra"
)

var (
	// tagWriteCmd represents the tag read command
	tagDeleteCmd = &cobra.Command{
		Use:   "delete <service> <key> <tag key>...",
		Short: "Delete tags for a specific secret",
		Args:  cobra.MinimumNArgs(3),
		RunE:  tagDelete,
	}
)

func init() {
	tagCmd.AddCommand(tagDeleteCmd)
}

func tagDelete(cmd *cobra.Command, args []string) error {
	service := utils.NormalizeService(args[0])
	if err := validateService(service); err != nil {
		return fmt.Errorf("Failed to validate service: %w", err)
	}

	key := utils.NormalizeKey(args[1])
	if err := validateKey(key); err != nil {
		return fmt.Errorf("Failed to validate key: %w", err)
	}

	tagKeys := make([]string, len(args)-2)
	for i, tagArg := range args[2:] {
		if err := validateTag(tagArg, "dummy"); err != nil {
			return fmt.Errorf("Failed to validate tag key %s: %w", tagArg, err)
		}
		tagKeys[i] = tagArg
	}

	if analyticsEnabled && analyticsClient != nil {
		_ = analyticsClient.Enqueue(analytics.Track{
			UserId: username,
			Event:  "Ran Command",
			Properties: analytics.NewProperties().
				Set("command", "tag delete").
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

	err = secretStore.DeleteTags(cmd.Context(), secretId, tagKeys)
	if err != nil {
		return fmt.Errorf("Failed to delete tags: %w", err)
	}

	return nil
}
