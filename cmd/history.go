package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/pkg/errors"
	"github.com/segmentio/chamber/v2/store"
	"github.com/spf13/cobra"
	analytics "gopkg.in/segmentio/analytics-go.v3"
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
				Set("command", "history").
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

	events, err := secretStore.History(secretId)
	if err != nil {
		return errors.Wrap(err, "Failed to get history")
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
