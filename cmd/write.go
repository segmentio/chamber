package cmd

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	analytics "github.com/segmentio/analytics-go/v3"
	"github.com/segmentio/chamber/v2/store"
	"github.com/segmentio/chamber/v2/utils"
	"github.com/spf13/cobra"
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

	secretStore, err := getSecretStore(cmd.Context())
	if err != nil {
		return fmt.Errorf("Failed to get secret store: %w", err)
	}

	secretId := store.SecretId{
		Service: service,
		Key:     key,
	}

	if skipUnchanged {
		currentSecret, err := secretStore.Read(cmd.Context(), secretId, -1)
		if err == nil && value == *currentSecret.Value {
			return nil
		}
	}

	return secretStore.Write(cmd.Context(), secretId, value)
}
