// +build !linux,!darwin

package cmd

import (
	"os"
	osexec "os/exec"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"
)

// exec executes the given command, passing it args and setting its environment
// to env.
// The exec function is allowed to never return and cause the program to exit.
func exec(command string, args []string, env []string) error {
	ecmd := osexec.Command(command, args...)
	ecmd.Stdin = os.Stdin
	ecmd.Stdout = os.Stdout
	ecmd.Stderr = os.Stderr
	ecmd.Env = env

	signals := make([]os.Signal, 31)
	for i := range signals {
		signals[i] = syscall.Signal(i + 1)
	}

	// Forward SIGINT, SIGTERM, SIGKILL to the child command
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, signals...)

	go func() {
		sig := <-sigChan
		if ecmd.Process != nil {
			ecmd.Process.Signal(sig)
		}
	}()

	var waitStatus syscall.WaitStatus
	if err := ecmd.Run(); err != nil {
		if err != nil {
			return errors.Wrap(err, "Failed to run command")
		}
		if exitError, ok := err.(*osexec.ExitError); ok {
			waitStatus = exitError.Sys().(syscall.WaitStatus)
			os.Exit(waitStatus.ExitStatus())
		}
	}

	return nil
}
