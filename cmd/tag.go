package cmd

import (
	"github.com/spf13/cobra"
)

var (
	// tagCmd represents the tag command
	tagCmd = &cobra.Command{
		Use:   "tag <subcommand> ...",
		Short: "work with tags on secrets",
	}
)

func init() {
	RootCmd.AddCommand(tagCmd)
}
