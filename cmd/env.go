package cmd

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/alessio/shellescape"
	analytics "github.com/segmentio/analytics-go/v3"
	"github.com/segmentio/chamber/v2/utils"

	"github.com/spf13/cobra"
)

// originally ported from github.com/joho/godotenv
const doubleQuoteSpecialChars = "\\\n\r\"!$`"

var (
	// envCmd represents the env command
	envCmd = &cobra.Command{
		Use:   "env <service>",
		Short: "Print the secrets from the parameter store in a format to export as environment variables",
		Args:  cobra.ExactArgs(1),
		RunE:  env,
	}
	preserveCase   bool
	escapeSpecials bool
)

func init() {
	envCmd.Flags().SortFlags = false
	envCmd.Flags().BoolVarP(&preserveCase, "preserve-case", "p", false, "preserve variable name case")
	envCmd.Flags().BoolVarP(&escapeSpecials, "escape-strings", "e", false, "escape special characters in values")
	RootCmd.AddCommand(envCmd)
}

// Print all secrets to standard out as valid shell key-value
// pairs or return an error if secrets cannot be safely
// represented as shell words.
func env(cmd *cobra.Command, args []string) error {
	envVars, err := exportEnv(cmd, args)
	if err != nil {
		return err
	}

	for i := range envVars {
		fmt.Println(envVars[i])
	}

	return nil
}

// Handle the actual work of retrieving and validating secrets.
// Returns a []string, with each string being a `key=value` pair,
// and returns any errors encountered along the way.
// Keys will be converted into valid shell variable names,
// and converted to uppercase unless --preserve is passed.
// Key ordering is non-deterministic and unstable, as returned
// value from a given secret store is non-deterministic and unstable.
func exportEnv(cmd *cobra.Command, args []string) ([]string, error) {
	service := utils.NormalizeService(args[0])
	if err := validateService(service); err != nil {
		return nil, fmt.Errorf("Failed to validate service: %w", err)
	}

	secretStore, err := getSecretStore(cmd.Context())
	if err != nil {
		return nil, fmt.Errorf("Failed to get secret store: %w", err)
	}

	rawSecrets, err := secretStore.ListRaw(cmd.Context(), service)
	if err != nil {
		return nil, fmt.Errorf("Failed to list store contents: %w", err)
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

	params := make(map[string]string)
	for _, rawSecret := range rawSecrets {
		params[key(rawSecret.Key)] = rawSecret.Value
	}

	out, err := buildEnvOutput(params)
	if err != nil {
		return nil, err
	}

	// ensure output prints variable declarations as exported
	for i := range out {
		// Sprintf because each declaration already ends in a newline
		out[i] = fmt.Sprintf("export %s", out[i])
	}

	return out, nil
}

// output will be returned lexically sorted by key name
func buildEnvOutput(params map[string]string) ([]string, error) {
	out := []string{}
	for _, key := range sortedKeys(params) {
		name := sanitizeKey(key)
		if !preserveCase {
			name = strings.ToUpper(name)
		}

		if err := validateShellName(name); err != nil {
			return nil, err
		}

		// the default format prints all escape sequences as
		// string literals, and wraps values in single quotes
		// if they're unsafe or multi-line strings.
		s := fmt.Sprintf(`%s=%s`, name, shellescape.Quote(params[key]))
		if escapeSpecials {
			// this format collapses special characters like newlines
			// or carriage returns. requires escape sequences to be interpolated
			// by whatever parses our key="value" pairs.
			s = fmt.Sprintf(`%s="%s"`, name, doubleQuoteEscape(params[key]))
		}

		// don't rely on printf to handle properly quoting or
		// escaping shell output -- just white-knuckle it ourselves.

		out = append(out, s)
	}

	return out, nil
}

// The name of a variable can contain only letters (a-z, case insensitive),
// numbers (0-9) or the underscore character (_). It may only begin with
// a letter or an underscore.
func validateShellName(s string) error {
	shellChars := regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString
	validShellName := regexp.MustCompile(`^[A-Za-z_]{1}`).MatchString

	if !shellChars(s) {
		return fmt.Errorf("cmd: %q contains invalid characters for a shell variable name", s)
	}

	if !validShellName(s) {
		return fmt.Errorf("cmd: shell variable name %q must start with a letter or underscore", s)
	}

	return nil
}

// note that all character width will be preserved; a single space
// (or period, tab, or newline) will be replaced with a single underscore.
// no squeezing/collapsing of replaced characters is performed at all.
func sanitizeKey(s string) string {
	// I promise, we don't actually care about allocations here.
	// allocate *away*.
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	// whitespace gets a visit from The Big Hammer that is regex.
	s = regexp.MustCompile(`[[:space:]]`).ReplaceAllString(s, "_")

	return s
}

// originally ported from github.com/joho/godotenv
func doubleQuoteEscape(line string) string {
	for _, c := range doubleQuoteSpecialChars {
		toReplace := "\\" + string(c)
		if c == '\n' {
			toReplace = `\n`
		}
		if c == '\r' {
			toReplace = `\r`
		}
		line = strings.Replace(line, string(c), toReplace, -1)
	}
	return line
}

// return the keys from params, sorted by keyname.
// note that sort.Strings() is not case insensitive.
// e.g. []string{"A", "b", "cat", "Dog", "dog"} will sort as:
// []string{"A", "Dog", "b", "cat", "dog"}. That doesn't
// really matter here but it may lead to surprises.
func sortedKeys(params map[string]string) []string {
	keys := []string{}

	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
