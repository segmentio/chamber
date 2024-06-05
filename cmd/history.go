package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	analytics "github.com/segmentio/analytics-go/v3"
	"github.com/segmentio/chamber/v2/store"
	"github.com/segmentio/chamber/v2/utils"
	"github.com/spf13/cobra"
)

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history <service> <key>",
	Short: "View the history of a secret",
	Args:  cobra.ExactArgs(2),
	RunE:  history,
}

func init() {
	RootCmd.AddCommand(historyCmd)
}

func history(cmd *cobra.Command, args []string) error {
	service := utils.NormalizeService(args[0])
	if err := validateService(service); err != nil {
		return fmt.Errorf("Failed to validate service: %w", err)
	}

	key := utils.NormalizeKey(args[1])
	if err := validateKey(key); err != nil {
		return fmt.Errorf("Failed to validate key: %w", err)
	}

	if analyticsEnabled && analyticsClient != nil {
		analyticsClient.Enqueue(analytics.Track{
			UserId: username,
			Event:  "Ran Command",
			Properties: analytics.NewProperties().
				Set("command", "history").
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

	events, err := secretStore.History(cmd.Context(), secretId)
	if err != nil {
		return fmt.Errorf("Failed to get history: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, '\t', 0)
	fmt.Fprintln(w, "Event\tVersion\tDate\tUser")
	for _, event := range events {
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
			event.Type,
			event.Version,
			event.Time.Local().Format(ShortTimeFormat),
			event.User,
		)
	}
	w.Flush()
	return nil
}
