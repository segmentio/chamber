package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/segmentio/chamber/environ"
	"github.com/spf13/cobra"
	analytics "gopkg.in/segmentio/analytics-go.v3"
)

// When true, only use variables retrieved from the backend, do not inherit existing environment variables
var pristine bool

// execCmd represents the exec command
var execCmd = &cobra.Command{
	Use:   "exec <service...> -- <command> [<arg...>]",
	Short: "Executes a command with secrets loaded into the environment",
	Args: func(cmd *cobra.Command, args []string) error {
		dashIx := cmd.ArgsLenAtDash()
		if dashIx == -1 {
			return errors.New("please separate services and command with '--'. See usage")
		}
		if err := cobra.MinimumNArgs(1)(cmd, args[:dashIx]); err != nil {
			return errors.Wrap(err, "at least one service must be specified")
		}
		if err := cobra.MinimumNArgs(1)(cmd, args[dashIx:]); err != nil {
			return errors.Wrap(err, "must specify command to run. See usage")
		}
		return nil
	},
	RunE: execRun,
}

func init() {
	execCmd.Flags().BoolVar(&pristine, "pristine", false, "only use variables retrieved from the backend, do not inherit existing environment variables")
	RootCmd.AddCommand(execCmd)
}

func execRun(cmd *cobra.Command, args []string) error {
	dashIx := cmd.ArgsLenAtDash()
	services, command, commandArgs := args[:dashIx], args[dashIx], args[dashIx+1:]

	if analyticsEnabled && analyticsClient != nil {
		analyticsClient.Enqueue(analytics.Track{
			UserId: username,
			Event:  "Ran Command",
			Properties: analytics.NewProperties().
				Set("command", "exec").
				Set("chamber-version", chamberVersion).
				Set("services", services).
				Set("backend", backend),
		})
	}

	env := environ.Environ{}
	if !pristine {
		env = environ.Environ(os.Environ())
	}

	secretStore, err := getSecretStore()
	if err != nil {
		return errors.Wrap(err, "Failed to get secret store")
	}
	for _, service := range services {
		if err := validateService(service); err != nil {
			return errors.Wrap(err, "Failed to validate service")
		}

		collisions := make([]string, 0)
		var err error
		if _, noPaths := os.LookupEnv("CHAMBER_NO_PATHS"); noPaths {
			err = env.LoadNoPaths(secretStore, service, &collisions)
		} else {
			err = env.Load(secretStore, service, &collisions)
		}
		if err != nil {
			return errors.Wrap(err, "Failed to list store contents")
		}

		for _, c := range collisions {
			fmt.Fprintf(os.Stderr, "warning: service %s overwriting environment variable %s\n", service, c)
		}
	}

	if verbose {
		fmt.Fprintf(os.Stdout, "info: With environment %s\n", strings.Join(env, ","))
	}

	return exec(command, commandArgs, env)
}
