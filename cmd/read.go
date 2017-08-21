package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
)

var (
	version int
	quiet   bool
)

// readCmd represents the read command
var readCmd = &cobra.Command{
	Use:   "read <service> <key>",
	Short: "Read a specific secret from the parameter store",
	RunE:  read,
}

func init() {
	readCmd.Flags().IntVarP(&version, "version", "v", -1, "The version number of the secret. Defaults to latest.")
	readCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Only print the secret")
	RootCmd.AddCommand(readCmd)
}

func read(cmd *cobra.Command, args []string) error {
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

	secret, err := secretStore.Read(secretId, version)
	if err != nil {
		return err
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
