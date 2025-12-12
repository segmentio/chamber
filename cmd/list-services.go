package cmd

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/segmentio/chamber/v3/utils"
	"github.com/spf13/cobra"
)

// listServicesCmd represents the list command
var listServicesCmd = &cobra.Command{
	Use:   "list-services <service>",
	Short: "List services",
	RunE:  listServices,
}

var (
	includeSecretName bool
)

func init() {
	listServicesCmd.Flags().BoolVarP(&includeSecretName, "secrets", "s", false, "Include secret names in the list")
	RootCmd.AddCommand(listServicesCmd)
}

func listServices(cmd *cobra.Command, args []string) error {
	var service string
	if len(args) == 0 {
		service = ""
	} else {
		service = utils.NormalizeService(args[0])

	}
	secretStore, err := getSecretStore(cmd.Context())
	if err != nil {
		return fmt.Errorf("Failed to get secret store: %w", err) //nolint:staticcheck // ST1005 pre-existing
	}
	secrets, err := secretStore.ListServices(cmd.Context(), service, includeSecretName)
	if err != nil {
		return fmt.Errorf("Failed to list store contents: %w", err) //nolint:staticcheck // ST1005 pre-existing
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, '\t', 0)
	fmt.Fprint(w, "Service")   //nolint:errcheck // pre-existing
	fmt.Fprintln(w, "") //nolint:errcheck // pre-existing

	sort.Strings(secrets)

	for _, secret := range secrets {
		fmt.Fprintf(w, "%s", //nolint:errcheck // pre-existing
			secret)
		fmt.Fprintln(w, "") //nolint:errcheck // pre-existing
	}
	w.Flush() //nolint:errcheck // pre-existing
	return nil
}
