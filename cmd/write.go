package cmd

import (
	"errors"
	"strings"

	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
)

var (
	ErrTooManyArguments = errors.New("too many arguments")
	ErrTooFewArguments  = errors.New("too few arguments")
)

// writeCmd represents the write command
var writeCmd = &cobra.Command{
	Use:   "write <service> <key> <value>",
	Short: "write a secret",
	RunE:  write,
}

func init() {
	RootCmd.AddCommand(writeCmd)
}

func write(cmd *cobra.Command, args []string) error {
	if len(args) < 3 {
		return ErrTooFewArguments
	}
	if len(args) > 3 {
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

	value := args[2]

	secretStore := store.NewSSMStore()
	secretId := store.SecretId{
		Service: service,
		Key:     key,
	}

	return secretStore.Write(secretId, value)
}
