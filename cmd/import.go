package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
)

var (
	importCmd = &cobra.Command{
		Use:   "import <service> <file|->",
		Short: "import secrets from json",
		Args:  cobra.ExactArgs(2),
		RunE:  importRun,
	}
)

func init() {
	RootCmd.AddCommand(importCmd)
}

func importRun(cmd *cobra.Command, args []string) error {
	service := strings.ToLower(args[0])
	if err := validateService(service); err != nil {
		return errors.Wrap(err, "Failed to validate service")
	}

	var in io.Reader
	var err error

	file := args[1]
	if file == "-" {
		in = os.Stdin
	} else {
		in, err = os.Open(file)
		if err != nil {
			return errors.Wrap(err, "Failed to open file")
		}
	}

	var toBeImported map[string]string

	decoder := json.NewDecoder(in)
	if err := decoder.Decode(&toBeImported); err != nil {
		return errors.Wrap(err, "Failed to decode input as json")
	}

	secretStore := store.NewSSMStore(numRetries)

	for key, value := range toBeImported {
		secretId := store.SecretId{
			Service: service,
			Key:     key,
		}
		if err := secretStore.Write(secretId, value); err != nil {
			return errors.Wrap(err, "Failed to write secret")
		}
	}

	fmt.Fprintf(os.Stdout, "Successfully imported %d secrets\n", len(toBeImported))
	return nil
}
