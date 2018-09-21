package cmd

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/magiconair/properties"
	"github.com/pkg/errors"
	analytics "github.com/segmentio/analytics-go"
	"github.com/spf13/cobra"
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
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "json", "Output format (json, java-properties, csv, tsv, dotenv)")
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

	secretStore, err := getSecretStore()
	if err != nil {
		return err
	}
	params := make(map[string]string)
	for _, service := range args {
		if err := validateService(service); err != nil {
			return errors.Wrapf(err, "Failed to validate service %s", service)
		}

		rawSecrets, err := secretStore.ListRaw(strings.ToLower(service))
		if err != nil {
			return errors.Wrapf(err, "Failed to list store contents for service %s", service)
		}
		for _, rawSecret := range rawSecrets {
			k := key(rawSecret.Key)
			if _, ok := params[k]; ok {
				fmt.Fprintf(os.Stderr, "warning: parameter %s specified more than once (overriden by service %s)\n", k, service)
			}
			params[k] = rawSecret.Value
		}
	}

	file := os.Stdout
	if exportOutput != "" {
		if file, err = os.OpenFile(exportOutput, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644); err != nil {
			return errors.Wrap(err, "Failed to open output file for writing")
		}
		defer file.Close()
		defer file.Sync()
	}
	w := bufio.NewWriter(file)
	defer w.Flush()

	switch strings.ToLower(exportFormat) {
	case "json":
		err = exportAsJson(params, w)
	case "java-properties", "properties":
		err = exportAsJavaProperties(params, w)
	case "csv":
		err = exportAsCsv(params, w)
	case "tsv":
		err = exportAsTsv(params, w)
	case "dotenv":
		err = exportAsEnvFile(params, w)
	default:
		err = errors.Errorf("Unsupported export format: %s", exportFormat)
	}

	if err != nil {
		return errors.Wrap(err, "Unable to export parameters")
	}

	return nil
}

func exportAsEnvFile(params map[string]string, w io.Writer) error {
	// Env File like:
	// KEY=VAL
	// OTHER=OTHERVAL
	for _, k := range sortedKeys(params) {
		key := strings.ToUpper(k)
		key = strings.Replace(key, "-", "_", -1)
		w.Write([]byte(fmt.Sprintf("%s=%s\n", key, params[k])))
	}
	return nil
}

func exportAsJson(params map[string]string, w io.Writer) error {
	// JSON like:
	// {"param1":"value1","param2":"value2"}
	// NOTE: json encoder does sorting by key
	return json.NewEncoder(w).Encode(params)
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
			return errors.Wrapf(err, "Failed to write param %s to CSV file", k)
		}
	}
	return nil
}

func exportAsTsv(params map[string]string, w io.Writer) error {
	// TSV (Tab Separated Values) like:
	for _, k := range sortedKeys(params) {
		if _, err := fmt.Fprintf(w, "%s\t%s\n", k, params[k]); err != nil {
			return errors.Wrapf(err, "Failed to write param %s to TSV file", k)
		}
	}
	return nil
}

func sortedKeys(params map[string]string) []string {
	keys := make([]string, len(params))
	i := 0
	for k := range params {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}
