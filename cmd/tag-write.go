package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	analytics "github.com/segmentio/analytics-go/v3"
	"github.com/segmentio/chamber/v3/store"
	"github.com/segmentio/chamber/v3/utils"
	"github.com/spf13/cobra"
)

var (
	deleteOtherTags bool

	// tagWriteCmd represents the tag read command
	tagWriteCmd = &cobra.Command{
		Use:   "write <service> <key> <tag>...",
		Short: "Write tags for a specific secret",
		Args:  cobra.MinimumNArgs(3),
		RunE:  tagWrite,
	}
)

func init() {
	tagWriteCmd.Flags().BoolVar(&deleteOtherTags, "delete-other-tags", false, "Delete tags not specified in the command")
	tagCmd.AddCommand(tagWriteCmd)
}

func tagWrite(cmd *cobra.Command, args []string) error {
	service := utils.NormalizeService(args[0])
	if err := validateService(service); err != nil {
		return fmt.Errorf("Failed to validate service: %w", err) //nolint:staticcheck // ST1005 pre-existing
	}

	key := utils.NormalizeKey(args[1])
	if err := validateKey(key); err != nil {
		return fmt.Errorf("Failed to validate key: %w", err) //nolint:staticcheck // ST1005 pre-existing
	}

	tags := make(map[string]string, len(args)-2)
	for _, tagArg := range args[2:] {
		tagKey, tagValue, found := strings.Cut(tagArg, "=")
		if !found {
			return fmt.Errorf("Failed to parse tag %s: tag must be in the form key=value", tagArg) //nolint:staticcheck // ST1005 pre-existing
		}
		if err := validateTag(tagKey, tagValue); err != nil {
			return fmt.Errorf("Failed to validate tag with key %s: %w", tagKey, err) //nolint:staticcheck // ST1005 pre-existing
		}
		tags[tagKey] = tagValue
	}

	if analyticsEnabled && analyticsClient != nil {
		_ = analyticsClient.Enqueue(analytics.Track{
			UserId: username,
			Event:  "Ran Command",
			Properties: analytics.NewProperties().
				Set("command", "tag write").
				Set("chamber-version", chamberVersion).
				Set("service", service).
				Set("key", key).
				Set("backend", backend),
		})
	}

	secretStore, err := getSecretStore(cmd.Context())
	if err != nil {
		return fmt.Errorf("Failed to get secret store: %w", err) //nolint:staticcheck // ST1005 pre-existing
	}

	secretId := store.SecretId{
		Service: service,
		Key:     key,
	}

	err = secretStore.WriteTags(cmd.Context(), secretId, tags, deleteOtherTags)
	if err != nil {
		return fmt.Errorf("Failed to write tags: %w", err) //nolint:staticcheck // ST1005 pre-existing
	}

	if quiet {
		fmt.Fprintf(os.Stdout, "%s\n", tags) //nolint:errcheck // pre-existing
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, '\t', 0)
	fmt.Fprintln(w, "Key\tValue") //nolint:errcheck // pre-existing
	for k, v := range tags {
		fmt.Fprintf(w, "%s\t%s\n", k, v) //nolint:errcheck // pre-existing
	}
	w.Flush() //nolint:errcheck // pre-existing
	return nil
}
