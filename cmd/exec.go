package cmd

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
)

const (
	NoopEnv = "CHAMBER_NOOP"
)

var (
	ErrCommandMissing = errors.New("must specify command to run")
)

// execCmd represents the exec command
var execCmd = &cobra.Command{
	Use:   "exec <service> -- <command>",
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

	service := strings.ToLower(args[0])
	if err := validateService(service); err != nil {
		return err
	}

	command := commandPart[0]

	var commandArgs []string
	if len(commandPart) > 1 {
		commandArgs = commandPart[1:]
	}

	// If NoopEnv is set then we do not want to actually fetch and list our
	// secrets from AWS. This is so development services have an easy escape
	// hatch to turn off chamber from fetching secrets.
	var secrets []store.Secret
	if _, ok := os.LookupEnv(NoopEnv); !ok {
		secretStore := store.NewSSMStore()
		secretList, err := secretStore.List(service, true)
		if err != nil {
			return err
		}
		secrets = secretList
	}

	env := environ(os.Environ())
	for _, secret := range secrets {
		env.Set(strings.ToUpper(key(secret.Meta.Key)), *secret.Value)
	}

	ecmd := exec.Command(command, commandArgs...)
	ecmd.Stdin = os.Stdin
	ecmd.Stdout = os.Stdout
	ecmd.Stderr = os.Stderr
	ecmd.Env = env

	// Forward SIGINT, SIGTERM, SIGKILL to the child command
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, os.Interrupt, os.Kill)

	go func() {
		sig := <-sigChan
		if ecmd.Process != nil {
			ecmd.Process.Signal(sig)
		}
	}()

	var waitStatus syscall.WaitStatus
	if err := ecmd.Run(); err != nil {
		if err != nil {
			return err
		}
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus = exitError.Sys().(syscall.WaitStatus)
			os.Exit(waitStatus.ExitStatus())
		}
	}
	return nil
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

// Set adds an environment variable, replacing any existing ones of the same key
func (e *environ) Set(key, val string) {
	e.Unset(key)
	*e = append(*e, key+"="+val)
}
