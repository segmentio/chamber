package cmd

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/segmentio/chamber/v2/store"
	"github.com/spf13/cobra"
)

var (
	includeCmd = &cobra.Command{
		Use:   "include <service> <another-service>",
		Short: "create a run-time include. see documentation",
		Args:  cobra.ExactArgs(2),
		RunE:  include,
	}
)

func init() {
	RootCmd.AddCommand(includeCmd)
}

func include(cmd *cobra.Command, args []string) error {
	service := strings.ToLower(args[0])
	if err := validateService(service); err != nil {
		return errors.Wrap(err, "Failed to validate service")
	}

	includeService := strings.ToLower(args[1])
	if err := validateService(includeService); err != nil {
		return errors.Wrap(err, "Failed to validate service to import")
	}

	secretStore, err := getSecretStore()
	if err != nil {
		return errors.Wrap(err, "Failed to get secret store")
	}

	secretId := store.SecretId{
		Service: service,
		Key:     fmt.Sprintf("chamber-include-%s", includeService),
	}

	return secretStore.WriteInclude(secretId, includeService)
}
