package cmd

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/pkg/errors"
	"github.com/segmentio/chamber/v2/utils"
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
	secretStore, err := getSecretStore()
	if err != nil {
		return errors.Wrap(err, "Failed to get secret store")
	}
	secrets, err := secretStore.ListServices(service, includeSecretName)
	if err != nil {
		return errors.Wrap(err, "Failed to list store contents")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, '\t', 0)
	fmt.Fprint(w, "Service")
	fmt.Fprintln(w, "")

	sort.Strings(secrets)

	for _, secret := range secrets {
		fmt.Fprintf(w, "%s",
			secret)
		fmt.Fprintln(w, "")
	}
	w.Flush()
	return nil
}
