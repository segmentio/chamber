// +build linux darwin

package cmd

import (
	osexec "os/exec"

	"golang.org/x/sys/unix"
)

func exec(command string, args []string, env []string) error {
	argv0, err := osexec.LookPath(command)
	if err != nil {
		return err
	}

	argv := make([]string, 0, 1+len(args))
	argv = append(argv, command)
	argv = append(argv, args...)

	// Only returns if the execution fails.
	return unix.Exec(argv0, argv, env)
}
