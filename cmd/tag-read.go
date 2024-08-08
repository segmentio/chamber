package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	analytics "github.com/segmentio/analytics-go/v3"
	"github.com/segmentio/chamber/v3/store"
	"github.com/segmentio/chamber/v3/utils"
	"github.com/spf13/cobra"
)

var (
	// tagReadCmd represents the tag read command
	tagReadCmd = &cobra.Command{
		Use:   "read <service> <key>",
		Short: "Read tags for a specific secret",
		Args:  cobra.ExactArgs(2),
		RunE:  tagRead,
	}
)

func init() {
	tagCmd.AddCommand(tagReadCmd)
}

func tagRead(cmd *cobra.Command, args []string) error {
	service := utils.NormalizeService(args[0])
	if err := validateService(service); err != nil {
		return fmt.Errorf("Failed to validate service: %w", err)
	}

	key := utils.NormalizeKey(args[1])
	if err := validateKey(key); err != nil {
		return fmt.Errorf("Failed to validate key: %w", err)
	}

	if analyticsEnabled && analyticsClient != nil {
		_ = analyticsClient.Enqueue(analytics.Track{
			UserId: username,
			Event:  "Ran Command",
			Properties: analytics.NewProperties().
				Set("command", "tag read").
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

	tags, err := secretStore.ReadTags(cmd.Context(), secretId)
	if err != nil {
		return fmt.Errorf("Failed to read tags: %w", err)
	}

	if quiet {
		fmt.Fprintf(os.Stdout, "%s\n", tags)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, '\t', 0)
	fmt.Fprintln(w, "Key\tValue")
	for k, v := range tags {
		fmt.Fprintf(w, "%s\t%s\n", k, v)
	}
	w.Flush()
	return nil
}
