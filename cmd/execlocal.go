package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
)

// execLocalCmd represents the execlocal command
var execLocalCmd = &cobra.Command{
	Use:   "execlocal <file...> -- <command> [<arg...>]",
	Short: "Executes a command with secrets loaded into the environment",
	Args: func(cmd *cobra.Command, args []string) error {
		dashIx := cmd.ArgsLenAtDash()
		if dashIx == -1 {
			return errors.New("please separate files and command with '--'. See usage")
		}
		if err := cobra.MinimumNArgs(1)(cmd, args[:dashIx]); err != nil {
			return errors.Wrap(err, "at least one file must be specified")
		}
		if err := cobra.MinimumNArgs(1)(cmd, args[dashIx:]); err != nil {
			return errors.Wrap(err, "must specify command to run. See usage")
		}
		return nil
	},
	RunE: execLocalRun,
}

func init() {
	RootCmd.AddCommand(execLocalCmd)
}

func execLocalRun(cmd *cobra.Command, args []string) error {
	dashIx := cmd.ArgsLenAtDash()
	files, command, commandArgs := args[:dashIx], args[dashIx], args[dashIx+1:]

	env := environ(os.Environ())
	for _, file := range files {
		if err := validateFile(file); err != nil {
			return errors.Wrap(err, "Failed to validate file")
		}

		rawSecrets, err := listRaw(file)
		if err != nil {
			return errors.Wrap(err, "Failed to list file contents")
		}
		for _, rawSecret := range rawSecrets {
			if env.IsSet(rawSecret.Key) {
				fmt.Fprintf(os.Stderr, "warning: overwriting environment variable %s\n", rawSecret.Key)
			}
			env.Set(rawSecret.Key, rawSecret.Value)
		}
	}

	return exec(command, commandArgs, env)
}

func listRaw(file string) ([]store.RawSecret, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var secrets []store.RawSecret

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := scanner.Text()

		if len(text) == 0 || text[0] == '#' {
			continue
		}

		idx := strings.Index(text, "=")
		if idx <= 0 {
			fmt.Fprintf(os.Stderr, "warning: invalid line in file \"%s\"\n", text)
			continue
		}

		secrets = append(secrets, store.RawSecret{
			Key:   text[0:idx],
			Value: text[idx+1:],
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return secrets, nil
}

func validateFile(file string) error {
	_, err := os.Stat(file)
	return err
}
