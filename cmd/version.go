package cmd

import (
	"fmt"
	"os"

	analytics "github.com/segmentio/analytics-go/v3"
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
	if analyticsEnabled && analyticsClient != nil {
		_ = analyticsClient.Enqueue(analytics.Track{
			UserId: username,
			Event:  "Ran Command",
			Properties: analytics.NewProperties().
				Set("command", "version").
				Set("chamber-version", chamberVersion).
				Set("backend", backend),
		})
	}
	return nil
}
