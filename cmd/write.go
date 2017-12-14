package cmd

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
)

// writeCmd represents the write command
var writeCmd = &cobra.Command{
	Use:   "write <service> <key> <value|->",
	Short: "write a secret",
	Args:  cobra.ExactArgs(3),
	RunE:  write,
}

func init() {
	RootCmd.AddCommand(writeCmd)
}

func write(cmd *cobra.Command, args []string) error {
	service := strings.ToLower(args[0])
	if err := validateService(service); err != nil {
		return errors.Wrap(err, "Failed to validate service")
	}

	key := strings.ToLower(args[1])
	if err := validateKey(key); err != nil {
		return errors.Wrap(err, "Failed to validate key")
	}

	value := args[2]
	if value == "-" {
		// Read value from standard input
		v, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		value = string(v)
	}

	secretStore := store.NewSSMStore(numRetries)
	secretId := store.SecretId{
		Service: service,
		Key:     key,
	}

	return secretStore.Write(secretId, value)
}
