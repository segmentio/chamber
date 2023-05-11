//go:build !linux && !darwin

package cmd

import (
	"fmt"
	"os"
	osexec "os/exec"
	"os/signal"
	"syscall"
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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan)

	if err := ecmd.Start(); err != nil {
		return fmt.Errorf("Failed to start command: %w", err)
	}

	go func() {
		for {
			sig := <-sigChan
			ecmd.Process.Signal(sig)
		}
	}()

	if err := ecmd.Wait(); err != nil {
		ecmd.Process.Signal(os.Kill)
		return fmt.Errorf("Failed to wait for command termination: %w", err)
	}

	waitStatus := ecmd.ProcessState.Sys().(syscall.WaitStatus)
	os.Exit(waitStatus.ExitStatus())
	return nil // unreachable but Go doesn't know about it
}
