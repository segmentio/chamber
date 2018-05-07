package cmd

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete <service> <key>",
	Short: "Delete a secret, including all versions",
	Args:  cobra.ExactArgs(2),
	RunE:  delete,
}

func init() {
	RootCmd.AddCommand(deleteCmd)
}

func delete(cmd *cobra.Command, args []string) error {
	service := strings.ToLower(args[0])

	key := strings.ToLower(args[1])
	if err := validateKey(key); err != nil {
		return errors.Wrap(err, "Failed to validate key")
	}

	secretStore := store.NewSSMStore(numRetries)
	secretId := store.SecretId{
		Service: service,
		Key:     key,
	}

	return secretStore.Delete(secretId)
}
