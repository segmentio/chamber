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
	Args:  cobra.ExactArgs(1),
	RunE:  list,
}

var (
	withValues bool
)

func init() {
	listCmd.Flags().BoolVarP(&withValues, "expand", "e", false, "Expand parameter list with values")
	RootCmd.AddCommand(listCmd)
}

func list(cmd *cobra.Command, args []string) error {
	service := strings.ToLower(args[0])
	if err := validateService(service); err != nil {
		return errors.Wrap(err, "Failed to validate service")
	}

	secretStore := store.NewSSMStore(numRetries)
	secrets, err := secretStore.List(service, withValues)
	if err != nil {
		return errors.Wrap(err, "Failed to list store contents")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, '\t', 0)

	fmt.Fprint(w, "Key\tVersion\tLastModified\tUser")
	if withValues {
		fmt.Fprint(w, "\tValue")
	}
	fmt.Fprintln(w, "")

	for _, secret := range secrets {
		fmt.Fprintf(w, "%s\t%d\t%s\t%s",
			secret.Id.Key,
			secret.Meta.Version,
			secret.Meta.Created.Local().Format(ShortTimeFormat),
			secret.Meta.CreatedBy)
		if withValues {
			fmt.Fprintf(w, "\t%s", *secret.Value)
		}
		fmt.Fprintln(w, "")
	}

	w.Flush()
	return nil
}
