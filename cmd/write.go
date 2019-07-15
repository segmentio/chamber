package cmd

import (
	"bufio"
	"io/ioutil"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
	analytics "gopkg.in/segmentio/analytics-go.v3"
)

var (
	singleline    bool
	skipUnchanged bool

	// writeCmd represents the write command
	writeCmd = &cobra.Command{
		Use:   "write <service> <key> [--] <value|->",
		Short: "write a secret",
		Args:  cobra.ExactArgs(3),
		RunE:  write,
	}
)

func init() {
	writeCmd.Flags().BoolVarP(&singleline, "singleline", "s", false, "Insert single line parameter (end with \\n)")
	writeCmd.Flags().BoolVarP(&skipUnchanged, "skip-unchanged", "", false, "Skip writing secret if value is unchanged")
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

	if analyticsEnabled && analyticsClient != nil {
		analyticsClient.Enqueue(analytics.Track{
			UserId: username,
			Event:  "Ran Command",
			Properties: analytics.NewProperties().
				Set("command", "write").
				Set("chamber-version", chamberVersion).
				Set("service", service).
				Set("backend", backend).
				Set("key", key),
		})
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

	secretStore, err := getSecretStore()
	if err != nil {
		return errors.Wrap(err, "Failed to get secret store")
	}

	secretId := store.SecretId{
		Service: service,
		Key:     key,
	}

	if skipUnchanged {
		currentSecret, err := secretStore.Read(secretId, -1)
		if err == nil && value == *currentSecret.Value {
			return nil
		}
	}

	return secretStore.Write(secretId, value)
}
