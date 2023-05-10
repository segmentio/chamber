package cmd

import (
	"fmt"
	"regexp"
	"strings"

	analytics "github.com/segmentio/analytics-go/v3"
	"github.com/segmentio/chamber/v2/utils"
	"github.com/spf13/cobra"
)

var (
	// envCmd represents the env command
	envCmd = &cobra.Command{
		Use:   "env <service>",
		Short: "Print the secrets from the parameter store in a format to export as environment variables",
		Args:  cobra.ExactArgs(1),
		RunE:  env,
	}
	pattern *regexp.Regexp
)

func init() {
	RootCmd.AddCommand(envCmd)
	pattern = regexp.MustCompile(`[^\w@%+=:,./-]`)
}

func env(cmd *cobra.Command, args []string) error {
	service := utils.NormalizeService(args[0])
	if err := validateService(service); err != nil {
		return fmt.Errorf("Failed to validate service: %w", err)
	}

	secretStore, err := getSecretStore()
	if err != nil {
		return fmt.Errorf("Failed to get secret store: %w", err)
	}
	rawSecrets, err := secretStore.ListRaw(service)
	if err != nil {
		return fmt.Errorf("Failed to list store contents: %w", err)
	}

	if analyticsEnabled && analyticsClient != nil {
		analyticsClient.Enqueue(analytics.Track{
			UserId: username,
			Event:  "Ran Command",
			Properties: analytics.NewProperties().
				Set("command", "env").
				Set("chamber-version", chamberVersion).
				Set("service", service).
				Set("backend", backend),
		})
	}

	for _, rawSecret := range rawSecrets {
		fmt.Printf("export %s=%s\n",
			strings.ToUpper(key(rawSecret.Key)),
			shellescape(rawSecret.Value))
	}

	return nil
}

// shellescape returns a shell-escaped version of the string s. The returned value
// is a string that can safely be used as one token in a shell command line.
func shellescape(s string) string {
	if len(s) == 0 {
		return "''"
	}
	if pattern.MatchString(s) {
		return "'" + strings.Replace(s, "'", "'\"'\"'", -1) + "'"
	}

	return s
}
