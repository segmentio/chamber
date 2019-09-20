package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/segmentio/chamber/environ"
	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
	analytics "gopkg.in/segmentio/analytics-go.v3"
)

// When true, only use variables retrieved from the backend, do not inherit existing environment variables
var pristine bool

// When true,
var strict bool

// The value to expect in strict mode
var strictValue string

const strictValueDefault = "chamberme"

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
	Example: `
Given a store like:

{"username": "root", "password": "hunter22"}

$ HOME=/tmp USERNAME=chamberme PASSWORD=chamberme chamber exec --strict service exec -- env
HOME=/tmp
USERNAME=root
PASSWORD=hunter22

$ HOME=/tmp USERNAME=chamberme PASSWORD=chamberme EXTRA=chamberme chamber exec --strict service exec -- env
chamber: extra unfilled env var EXTRA
exit 1

$ HOME=/tmp USERNAME=chamberme EXTRA=chamberme chamber exec --strict service exec -- env
chamber: missing filled env var PASSWORD
exit 1

--pristine takes effect after checking for --strict values
$ HOME=/tmp USERNAME=chamberme PASSWORD=chamberme chamber exec --strict --pristine service exec -- env
USERNAME=root
PASSWORD=hunter22
`,
}

func init() {
	execCmd.Flags().BoolVar(&pristine, "pristine", false, "only use variables retrieved from the backend; do not inherit existing environment variables")
	execCmd.Flags().BoolVar(&strict, "strict", false, "fail unless for every secret in chamber there is a corresponding env var KEY=<strict-value>, and there are no extra KEY=<strict-value> env vars")
	execCmd.Flags().StringVar(&strictValue, "strict-value", strictValueDefault, "value to expect in --strict mode")
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

	for _, service := range services {
		if err := validateService(service); err != nil {
			return errors.Wrap(err, "Failed to validate service")
		}
	}

	secretStore, err := getSecretStore()
	if err != nil {
		return errors.Wrap(err, "Failed to get secret store")
	}
	_, noPaths := os.LookupEnv("CHAMBER_NO_PATHS")

	var env []string
	// TODO: combine these into a single LoadAll or something
	if strict {
		loader := &environ.EnvironStrict{
			Parent:        environ.Environ(os.Environ()),
			ValueExpected: strictValue,
			Pristine:      pristine,
		}
		rawSecrets := []store.RawSecret{}
		for _, service := range services {
			rawSecretsNext, err := secretStore.ListRaw(strings.ToLower(service))
			if err != nil {
				return errors.Wrap(err, "Failed to list store contents")
			}
			rawSecrets = append(rawSecrets, rawSecretsNext...)
		}
		err = loader.LoadFromSecrets(rawSecrets, noPaths)
		if err != nil {
			return err
		}
		env = []string(loader.Environ)
	} else {
		loader := environ.Environ{}
		if !pristine {
			loader = environ.Environ(os.Environ())
		}
		for _, service := range services {
			if err := validateService(service); err != nil {
				return errors.Wrap(err, "Failed to validate service")
			}

			collisions := make([]string, 0)
			var err error
			if noPaths {
				err = loader.LoadNoPaths(secretStore, service, &collisions)
			} else {
				err = loader.Load(secretStore, service, &collisions)
			}
			if err != nil {
				return errors.Wrap(err, "Failed to list store contents")
			}

			for _, c := range collisions {
				fmt.Fprintf(os.Stderr, "warning: service %s overwriting environment variable %s\n", service, c)
			}
		}
		env = []string(loader)
	}

	if verbose {
		fmt.Fprintf(os.Stdout, "info: With environment %s\n", strings.Join(env, ","))
	}

	return exec(command, commandArgs, env)
}
