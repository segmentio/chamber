package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print version",
	RunE:  versionRun,
}

func init() {
	RootCmd.AddCommand(versionCmd)
}

func versionRun(cmd *cobra.Command, args []string) error {
	fmt.Fprintf(os.Stdout, "chamber %s\n", chamberVersion)
	return nil
}
