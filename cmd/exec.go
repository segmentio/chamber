package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var format string

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
	execCmd.Flags().StringVarP(&format, "format", "f", "{{. | upper}}", "Environment variable name format")
	RootCmd.AddCommand(execCmd)
}

func render(key string, tpl *template.Template) (string, error) {
	var buffer bytes.Buffer
	if err := tpl.Execute(&buffer, key); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func execRun(cmd *cobra.Command, args []string) error {
	dashIx := cmd.ArgsLenAtDash()
	services, command, commandArgs := args[:dashIx], args[dashIx], args[dashIx+1:]

	env := environ(os.Environ())
	secretStore := getSecretStore()
	tpl, err := template.New("env").Funcs(sprig.TxtFuncMap()).Parse(format)
	if err != nil {
		return err
	}

	for _, service := range services {
		if err := validateService(service); err != nil {
			return errors.Wrap(err, "Failed to validate service")
		}

		rawSecrets, err := secretStore.ListRaw(strings.ToLower(service))
		if err != nil {
			return errors.Wrap(err, "Failed to list store contents")
		}
		for _, rawSecret := range rawSecrets {
			envVarKey, err := render(key(rawSecret.Key), tpl)
			if err != nil {
				return err
			}
			envVarKey = strings.Replace(envVarKey, "-", "_", -1)

			if env.IsSet(envVarKey) {
				fmt.Fprintf(os.Stderr, "warning: overwriting environment variable %s\n", envVarKey)
			}
			env.Set(envVarKey, rawSecret.Value)
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
