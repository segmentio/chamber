package cmd

import (
	"bufio"
	"io/ioutil"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
)

var (
	singleline bool

	// writeCmd represents the write command
	writeCmd = &cobra.Command{
		Use:   "write <service> <key> <value|->",
		Short: "write a secret",
		Args:  cobra.ExactArgs(3),
		RunE:  write,
	}
)

func init() {
	writeCmd.Flags().BoolVarP(&singleline, "singleline", "s", false, "Insert single line parameter (end with \\n)")
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
		if singleline {
			buf := bufio.NewReader(os.Stdin)
			v, err := buf.ReadString('\n')
			if err != nil {
				return err
			}
			value = strings.TrimSuffix(v, "\n")
		} else {
			v, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				return err
			}
			value = string(v)
		}
	}

	secretStore := store.NewSSMStore(numRetries)
	secretId := store.SecretId{
		Service: service,
		Key:     key,
	}

	return secretStore.Write(secretId, value)
}
