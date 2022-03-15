package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	analytics "gopkg.in/segmentio/analytics-go.v3"
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
	service := strings.ToLower(args[0])
	if err := validateService(service); err != nil {
		return errors.Wrap(err, "Failed to validate service")
	}

	secretStore, err := getSecretStore()
	if err != nil {
		return errors.Wrap(err, "Failed to get secret store")
	}
	rawSecrets, err := secretStore.ListRaw(service)
	if err != nil {
		return errors.Wrap(err, "Failed to list store contents")
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
