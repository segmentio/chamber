package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/pkg/errors"
	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list <service>",
	Short: "List the secrets set for a service",
	RunE:  list,
}

func init() {
	RootCmd.AddCommand(listCmd)
}

func list(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return ErrTooFewArguments
	}
	if len(args) > 1 {
		return ErrTooManyArguments
	}

	service := strings.ToLower(args[0])
	if err := validateService(service); err != nil {
		return errors.Wrap(err, "Failed to validate service")
	}

	secretStore := store.NewSSMStore(numRetries)
	secrets, err := secretStore.List(service, listIncludeValues)
	if err != nil {
		return errors.Wrap(err, "Failed to list store contents")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, '\t', 0)

	if listIncludeValues {
		fmt.Fprintln(w, "Key\tValue\tVersion\tLastModified\tUser")
		for _, secret := range secrets {
			fmt.Println(secret)
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
				key(secret.Meta.Key),
				*secret.Value,
				secret.Meta.Version,
				secret.Meta.Created.Local().Format(ShortTimeFormat),
				secret.Meta.CreatedBy)
		}
	} else {
		fmt.Fprintln(w, "Key\tVersion\tLastModified\tUser")
		for _, secret := range secrets {
			fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
				key(secret.Meta.Key),
				secret.Meta.Version,
				secret.Meta.Created.Local().Format(ShortTimeFormat),
				secret.Meta.CreatedBy)
		}
	}
	w.Flush()
	return nil
}

func key(s string) string {
	tokens := strings.Split(s, "/")
	secretKey := tokens[2]
	return secretKey
}
