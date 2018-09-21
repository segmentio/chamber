package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/pkg/errors"
	analytics "github.com/segmentio/analytics-go"
	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
)

var (
	version int
	quiet   bool

	// readCmd represents the read command
	readCmd = &cobra.Command{
		Use:   "read <service> <key>",
		Short: "Read a specific secret from the parameter store",
		Args:  cobra.ExactArgs(2),
		RunE:  read,
	}
)

func init() {
	readCmd.Flags().IntVarP(&version, "version", "v", -1, "The version number of the secret. Defaults to latest.")
	readCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Only print the secret")
	RootCmd.AddCommand(readCmd)
}

func read(cmd *cobra.Command, args []string) error {
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
				Set("command", "read").
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

	secret, err := secretStore.Read(secretId, version)
	if err != nil {
		return errors.Wrap(err, "Failed to read")
	}

	if quiet {
		fmt.Fprintf(os.Stdout, "%s\n", *secret.Value)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, '\t', 0)
	fmt.Fprintln(w, "Key\tValue\tVersion\tLastModified\tUser")
	fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
		key,
		*secret.Value,
		secret.Meta.Version,
		secret.Meta.Created.Local().Format(ShortTimeFormat),
		secret.Meta.CreatedBy)
	w.Flush()
	return nil
}
