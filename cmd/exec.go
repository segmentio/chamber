package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
)

var (
	ErrCommandMissing = errors.New("must specify command to run")
)

// execCmd represents the exec command
var execCmd = &cobra.Command{
	Use:   "exec <service...> -- <command>",
	Short: "Executes a command with secrets loaded into the environment",
	RunE:  execRun,
}

func init() {
	RootCmd.AddCommand(execCmd)
}

func execRun(cmd *cobra.Command, args []string) error {
	dashIx := cmd.ArgsLenAtDash()
	if dashIx == -1 {
		return ErrCommandMissing
	}

	args, commandPart := args[:dashIx], args[dashIx:]
	if len(args) < 1 {
		return ErrTooFewArguments
	}

	if len(commandPart) == 0 {
		return errors.New("must specify command to run")
	}

	command := commandPart[0]

	var commandArgs []string
	if len(commandPart) > 1 {
		commandArgs = commandPart[1:]
	}

	env := environ(os.Environ())
	secretStore := store.NewSSMStore(numRetries)
	for _, service := range args {
		if err := validateService(service); err != nil {
			return errors.Wrap(err, "Failed to validate service")
		}

		secrets, err := secretStore.List(strings.ToLower(service), true)
		if err != nil {
			return errors.Wrap(err, "Failed to list store contents")
		}
		for _, secret := range secrets {
			envVarKey := strings.ToUpper(key(secret.Meta.Key))
			envVarKey = strings.Replace(envVarKey, "-", "_", -1)

			if env.IsSet(envVarKey) {
				fmt.Fprintf(os.Stderr, "warning: overwriting environment variable %s\n", envVarKey)
			}
			env.Set(envVarKey, *secret.Value)
		}
	}

	return exec(command, commandArgs, env)
}

// environ is a slice of strings representing the environment, in the form "key=value".
type environ []string

// Unset an environment variable by key
func (e *environ) Unset(key string) {
	for i := range *e {
		if strings.HasPrefix((*e)[i], key+"=") {
			(*e)[i] = (*e)[len(*e)-1]
			*e = (*e)[:len(*e)-1]
			break
		}
	}
}

// IsSet returns whether or not a key is currently set in the environ
func (e *environ) IsSet(key string) bool {
	for i := range *e {
		if strings.HasPrefix((*e)[i], key+"=") {
			return true
		}
	}
	return false
}

// Set adds an environment variable, replacing any existing ones of the same key
func (e *environ) Set(key, val string) {
	e.Unset(key)
	*e = append(*e, key+"="+val)
}
