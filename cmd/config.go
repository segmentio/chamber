package cmd

import (
	"fmt"

	"github.com/segmentio/chamber/store"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show/save configuration",
}

// configShowCmd represents the config show command
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Dumps effective config to standard output",
	RunE:  runConfigShow,
}

// configSaveCmd represents the config save command
var configSaveCmd = &cobra.Command{
	Use:               "save",
	Short:             "Saves current config to current base path",
	RunE:              runConfigSave,
	PersistentPreRunE: optionalBaseConfigPreRunE,
}

// configClear represents the config clear command
var configClearCmd = &cobra.Command{
	Use:               "clear",
	Short:             "Clear config at current base path",
	RunE:              runConfigClear,
	PersistentPreRunE: optionalBaseConfigPreRunE,
}

func init() {
	RootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSaveCmd)
	configCmd.AddCommand(configClearCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	s, err := config.Marshal()
	if err != nil {
		return err
	}
	fmt.Printf(s)
	return nil
}

func runConfigSave(cmd *cobra.Command, args []string) error {
	secretStore, err := store.NewSSMStore(config)
	if err != nil {
		return err
	}
	return secretStore.SaveCurrentConfigToSSM()
}

func runConfigClear(cmd *cobra.Command, args []string) error {
	secretStore, err := store.NewSSMStore(config)
	if err != nil {
		return err
	}
	return secretStore.ClearCurrentConfig()
}
