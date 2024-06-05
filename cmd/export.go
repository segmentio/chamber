package cmd

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/magiconair/properties"

	analytics "github.com/segmentio/analytics-go/v3"
	"github.com/segmentio/chamber/v2/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// exportCmd represents the export command
var (
	exportFormat string
	exportOutput string

	exportCmd = &cobra.Command{
		Use:   "export <service...>",
		Short: "Exports parameters in the specified format",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runExport,
	}
)

func init() {
	exportCmd.Flags().SortFlags = false
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "json", "Output format (json, yaml, java-properties, csv, tsv, dotenv, tfvars)")
	exportCmd.Flags().StringVarP(&exportOutput, "output-file", "o", "", "Output file (default is standard output)")

	RootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	var err error

	if analyticsEnabled && analyticsClient != nil {
		analyticsClient.Enqueue(analytics.Track{
			UserId: username,
			Event:  "Ran Command",
			Properties: analytics.NewProperties().
				Set("command", "export").
				Set("chamber-version", chamberVersion).
				Set("services", args).
				Set("backend", backend),
		})
	}

	secretStore, err := getSecretStore(cmd.Context())
	if err != nil {
		return err
	}
	params := make(map[string]string)
	for _, service := range args {
		service = utils.NormalizeService(service)
		if err := validateService(service); err != nil {
			return fmt.Errorf("Failed to validate service %s: %w", service, err)
		}

		rawSecrets, err := secretStore.ListRaw(cmd.Context(), service)
		if err != nil {
			return fmt.Errorf("Failed to list store contents for service %s: %w", service, err)
		}
		for _, rawSecret := range rawSecrets {
			k := key(rawSecret.Key)
			if _, ok := params[k]; ok {
				fmt.Fprintf(os.Stderr, "warning: parameter %s specified more than once (overridden by service %s)\n", k, service)
			}
			params[k] = rawSecret.Value
		}
	}

	file := os.Stdout
	if exportOutput != "" {
		if file, err = os.OpenFile(exportOutput, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644); err != nil {
			return fmt.Errorf("Failed to open output file for writing: %w", err)
		}
		defer file.Close()
		defer file.Sync()
	}
	w := bufio.NewWriter(file)
	defer w.Flush()

	switch strings.ToLower(exportFormat) {
	case "json":
		err = exportAsJson(params, w)
	case "yaml":
		err = exportAsYaml(params, w)
	case "java-properties", "properties":
		err = exportAsJavaProperties(params, w)
	case "csv":
		err = exportAsCsv(params, w)
	case "tsv":
		err = exportAsTsv(params, w)
	case "dotenv":
		err = exportAsEnvFile(params, w)
	case "tfvars":
		err = exportAsTFvars(params, w)
	default:
		err = fmt.Errorf("Unsupported export format: %s", exportFormat)
	}

	if err != nil {
		return fmt.Errorf("Unable to export parameters: %w", err)
	}

	return nil
}

// this is fundamentally broken, in that there is no actual .env file
// spec. some parsers support values spanned over multiple lines
// as long as they're quoted, others only support character literals
// inside of quotes. we should probably offer the option to control
// which spec we adhere to, or use a marshaler that provides a
// spec instead of hoping for the best.
func exportAsEnvFile(params map[string]string, w io.Writer) error {
	// use top-level escapeSpecials variable to ensure that
	// the dotenv format prints escaped values every time
	escapeSpecials = true
	out, err := buildEnvOutput(params)
	if err != nil {
		return err
	}

	for i := range out {
		_, err := w.Write([]byte(fmt.Sprintln(out[i])))
		if err != nil {
			return err
		}
	}

	return nil
}

func exportAsTFvars(params map[string]string, w io.Writer) error {
	// Terraform Variables is like dotenv, but removes the TF_VAR and keeps lowercase
	for _, k := range sortedKeys(params) {
		key := sanitizeKey(strings.TrimPrefix(k, "tf_var_"))

		w.Write([]byte(fmt.Sprintf(`%s = "%s"`+"\n", key, doubleQuoteEscape(params[k]))))
	}
	return nil
}

func exportAsJson(params map[string]string, w io.Writer) error {
	// JSON like:
	// {"param1":"value1","param2":"value2"}
	// NOTE: json encoder does sorting by key
	return json.NewEncoder(w).Encode(params)
}

func exportAsYaml(params map[string]string, w io.Writer) error {
	return yaml.NewEncoder(w).Encode(params)
}

func exportAsJavaProperties(params map[string]string, w io.Writer) error {
	// Java Properties like:
	// param1 = value1
	// param2 = value2
	// ...

	// Load params
	p := properties.NewProperties()
	p.DisableExpansion = true
	for _, k := range sortedKeys(params) {
		p.Set(k, params[k])
	}

	// Java expects properties in ISO-8859-1 by default
	_, err := p.Write(w, properties.ISO_8859_1)
	return err
}

func exportAsCsv(params map[string]string, w io.Writer) error {
	// CSV (Comma Separated Values) like:
	// param1,value1
	// param2,value2
	csvWriter := csv.NewWriter(w)
	defer csvWriter.Flush()
	for _, k := range sortedKeys(params) {
		if err := csvWriter.Write([]string{k, params[k]}); err != nil {
			return fmt.Errorf("Failed to write param %q to CSV file: %w", k, err)
		}
	}
	return nil
}

func exportAsTsv(params map[string]string, w io.Writer) error {
	// TSV (Tab Separated Values) like:
	tsvWriter := csv.NewWriter(w)
	tsvWriter.Comma = '\t'
	defer tsvWriter.Flush()
	for _, k := range sortedKeys(params) {
		if err := tsvWriter.Write([]string{k, params[k]}); err != nil {
			return fmt.Errorf("Failed to write param %q to TSV file: %w", k, err)
		}
	}
	return nil
}
