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
				Set("command", "read").
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

	secret, err := secretStore.Read(cmd.Context(), secretId, version)
	if err != nil {
		return fmt.Errorf("Failed to read: %w", err)
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
