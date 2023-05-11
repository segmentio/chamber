package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/segmentio/chamber/v2/store"
	"github.com/spf13/cobra"
)

// findCmd represents the find command
var findCmd = &cobra.Command{
	Use:   "find <secret name>",
	Short: "Find the given secret across all services",
	Args:  cobra.ExactArgs(1),
	RunE:  find,
}

var (
	blankService   string
	byValue        bool
	includeSecrets bool
	matches        []store.SecretId
)

func init() {
	findCmd.Flags().BoolVarP(&byValue, "by-value", "v", false, "Find parameters by value")
	RootCmd.AddCommand(findCmd)
}

func find(cmd *cobra.Command, args []string) error {
	findSecret := args[0]

	if byValue {
		includeSecrets = false
	} else {
		includeSecrets = true
	}

	secretStore, err := getSecretStore()
	if err != nil {
		return fmt.Errorf("Failed to get secret store: %w", err)
	}
	services, err := secretStore.ListServices(blankService, includeSecrets)
	if err != nil {
		return fmt.Errorf("Failed to list store contents: %w", err)
	}

	if byValue {
		for _, service := range services {
			allSecrets, err := secretStore.List(service, true)
			if err == nil {
				matches = append(matches, findValueMatch(allSecrets, findSecret)...)
			}
		}
	} else {
		matches = append(matches, findKeyMatch(services, findSecret)...)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, '\t', 0)
	fmt.Fprint(w, "Service")
	if byValue {
		fmt.Fprint(w, "\tKey")
	}
	fmt.Fprintln(w, "")

	for _, match := range matches {
		fmt.Fprintf(w, "%s", match.Service)
		if byValue {
			fmt.Fprintf(w, "\t%s", match.Key)
		}
		fmt.Fprintln(w, "")
	}
	w.Flush()

	return nil
}

func findKeyMatch(services []string, searchTerm string) []store.SecretId {
	keyMatches := []store.SecretId{}

	for _, service := range services {
		if searchTerm == key(service) {

			keyMatches = append(keyMatches, store.SecretId{
				Service: path(service),
				Key:     key(service),
			})
		}
	}
	return keyMatches
}

func findValueMatch(secrets []store.Secret, searchTerm string) []store.SecretId {
	valueMatches := []store.SecretId{}

	for _, secret := range secrets {
		if *secret.Value == searchTerm {
			valueMatches = append(valueMatches, store.SecretId{
				Service: path(secret.Meta.Key),
				Key:     key(secret.Meta.Key),
			})
		}
	}
	return valueMatches
}

func path(s string) string {
	_, noPaths := os.LookupEnv("CHAMBER_NO_PATHS")
	sep := "/"
	if noPaths {
		sep = "."
	}

	tokens := strings.Split(s, sep)
	secretPath := strings.Join(tokens[1:len(tokens)-1], "/")
	return secretPath
}
