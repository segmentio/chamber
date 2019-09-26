package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/pkg/errors"
	"github.com/segmentio/chamber/v2/store"
	"github.com/spf13/cobra"
	analytics "gopkg.in/segmentio/analytics-go.v3"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list <service>",
	Short: "List the secrets set for a service",
	Args:  cobra.ExactArgs(1),
	RunE:  list,
}

var (
	withValues    bool
	sortByTime    bool
	sortByUser    bool
	sortByVersion bool
)

func init() {
	listCmd.Flags().BoolVarP(&withValues, "expand", "e", false, "Expand parameter list with values")
	listCmd.Flags().BoolVarP(&sortByTime, "time", "t", false, "Sort by modified time")
	listCmd.Flags().BoolVarP(&sortByUser, "user", "u", false, "Sort by user")
	listCmd.Flags().BoolVarP(&sortByVersion, "version", "v", false, "Sort by version")
	RootCmd.AddCommand(listCmd)
}

func list(cmd *cobra.Command, args []string) error {
	service := strings.ToLower(args[0])
	if err := validateService(service); err != nil {
		return errors.Wrap(err, "Failed to validate service")
	}

	if analyticsEnabled && analyticsClient != nil {
		analyticsClient.Enqueue(analytics.Track{
			UserId: username,
			Event:  "Ran Command",
			Properties: analytics.NewProperties().
				Set("command", "list").
				Set("chamber-version", chamberVersion).
				Set("service", service).
				Set("backend", backend),
		})
	}

	secretStore, err := getSecretStore()
	if err != nil {
		return errors.Wrap(err, "Failed to get secret store")
	}
	secrets, err := secretStore.List(service, withValues)
	if err != nil {
		return errors.Wrap(err, "Failed to list store contents")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, '\t', 0)

	fmt.Fprint(w, "Key\tVersion\tLastModified\tUser")
	if withValues {
		fmt.Fprint(w, "\tValue")
	}
	fmt.Fprintln(w, "")

	sort.Sort(ByName(secrets))
	if sortByTime {
		sort.Sort(ByTime(secrets))
	}
	if sortByUser {
		sort.Sort(ByUser(secrets))
	}
	if sortByVersion {
		sort.Sort(ByVersion(secrets))
	}

	for _, secret := range secrets {
		fmt.Fprintf(w, "%s\t%d\t%s\t%s",
			key(secret.Meta.Key),
			secret.Meta.Version,
			secret.Meta.Created.Local().Format(ShortTimeFormat),
			secret.Meta.CreatedBy)
		if withValues {
			fmt.Fprintf(w, "\t%s", *secret.Value)
		}
		fmt.Fprintln(w, "")
	}

	w.Flush()
	return nil
}

func key(s string) string {
	_, noPaths := os.LookupEnv("CHAMBER_NO_PATHS")
	sep := "/"
	if noPaths {
		sep = "."
	}

	tokens := strings.Split(s, sep)
	secretKey := tokens[len(tokens)-1]
	return secretKey
}

type ByName []store.Secret

func (a ByName) Len() int           { return len(a) }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return a[i].Meta.Key < a[j].Meta.Key }

type ByTime []store.Secret

func (a ByTime) Len() int           { return len(a) }
func (a ByTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByTime) Less(i, j int) bool { return a[i].Meta.Created.Before(a[j].Meta.Created) }

type ByUser []store.Secret

func (a ByUser) Len() int           { return len(a) }
func (a ByUser) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByUser) Less(i, j int) bool { return a[i].Meta.CreatedBy < a[j].Meta.CreatedBy }

type ByVersion []store.Secret

func (a ByVersion) Len() int           { return len(a) }
func (a ByVersion) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByVersion) Less(i, j int) bool { return a[i].Meta.Version < a[j].Meta.Version }
