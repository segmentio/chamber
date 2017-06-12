package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
)

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history <service> <key>",
	Short: "View the history of a secret",
	RunE:  history,
}

func init() {
	RootCmd.AddCommand(historyCmd)
}

func history(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return ErrTooFewArguments
	}
	if len(args) > 2 {
		return ErrTooManyArguments
	}

	service := strings.ToLower(args[0])
	if err := validateService(service); err != nil {
		return err
	}

	key := strings.ToLower(args[1])
	if err := validateKey(key); err != nil {
		return err
	}

	secretStore := store.NewSSMStore()
	secretId := store.SecretId{
		Service: service,
		Key:     key,
	}

	events, err := secretStore.History(secretId)
	if err != nil {
		return err
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
