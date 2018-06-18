package cmd

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/chanzuckerberg/chamber/store"
	"github.com/spf13/cobra"
)

var (
	force bool

	// deleteCmd represents the delete command
	deleteCmd = &cobra.Command{
		Use:   "delete <service> <key>",
		Short: "Tag a secreted as deleted (it will not be Delete a secret, including all versions",
		Args:  cobra.ExactArgs(2),
		RunE:  delete,
	}
)

func init() {
	deleteCmd.Flags().BoolVarP(&force, "force", "f", false, "Actually delete a secret, including all versions (non-reversible).")
	RootCmd.AddCommand(deleteCmd)
}

func delete(cmd *cobra.Command, args []string) error {
	service := strings.ToLower(args[0])
	if err := validateService(service); err != nil {
		return errors.Wrap(err, "Failed to validate service")
	}

	key := strings.ToLower(args[1])
	if err := validateKey(key); err != nil {
		return errors.Wrap(err, "Failed to validate key")
	}

	secretStore := store.NewSSMStore(numRetries)
	secretId := store.SecretId{
		Service: service,
		Key:     key,
	}

	if force {
		return secretStore.Delete(secretId)
	}
	return secretStore.TagDeleted(secretId)
}
