package view

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/dolthub/cli/internal/browser"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/internal/tableprinter"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

var jsonFields = []string{"description", "fork_network_count", "last_write_at", "name", "network_root", "owner", "parent", "size_bytes", "star_count", "visibility"}

type apiClient interface {
	GetDatabase(context.Context, string, string) (dolthub.Database, error)
	ListForks(context.Context, string, string) ([]dolthub.DatabaseRef, error)
}

type Options struct {
	IO                *iostreams.IOStreams
	ResolveRepository func(context.Context, string) (repository.Repository, error)
	APIClientForHost  func(string) (*dolthub.Client, error)
	Browser           browser.Browser
	Repository        string
	Forks             bool
	Web               bool
	Exporter          cmdutil.Exporter
	client            apiClient
}

func NewCmdView(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	opts := &Options{IO: f.IO, ResolveRepository: f.ResolveRepository, APIClientForHost: f.APIClientForHost, Browser: f.Browser}
	if runF == nil {
		runF = viewRun
	}
	cmd := &cobra.Command{
		Use:   "view [DATABASE]",
		Short: "View a database repository",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return cmdutil.FlagErrorf("%s accepts at most one argument", cmd.CommandPath())
			}
			if len(args) == 1 {
				if opts.Repository != "" {
					return cmdutil.FlagErrorf("cannot use a database argument with --db")
				}
				opts.Repository = args[0]
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error { return runF(cmd.Context(), opts) },
	}
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		structured := cmd.Flags().Changed("json") || cmd.Flags().Changed("jq") || cmd.Flags().Changed("template")
		if opts.Web && (opts.Forks || structured) {
			return cmdutil.FlagErrorf("--web cannot be used with --forks or structured output")
		}
		return nil
	}
	cmdutil.AddDatabaseFlag(cmd, &opts.Repository)
	cmd.Flags().BoolVar(&opts.Forks, "forks", false, "Include immediate forks")
	cmd.Flags().BoolVarP(&opts.Web, "web", "w", false, "Open the database in a browser")
	cmdutil.AddJSONFlags(cmd, &opts.Exporter, append(jsonFields, "forks"))
	return cmd
}

func viewRun(ctx context.Context, opts *Options) error {
	repo, err := opts.ResolveRepository(ctx, opts.Repository)
	if err != nil {
		return err
	}
	if opts.Web {
		return opts.Browser.Browse(databaseURL(repo))
	}
	client := opts.client
	if client == nil {
		client, err = opts.APIClientForHost(repo.Host)
		if err != nil {
			return err
		}
	}
	database, err := client.GetDatabase(ctx, repo.Owner, repo.Name)
	if err != nil {
		return err
	}
	var forks []dolthub.DatabaseRef
	if opts.Forks {
		forks, err = client.ListForks(ctx, repo.Owner, repo.Name)
		if err != nil {
			return err
		}
	}
	if opts.Exporter != nil {
		value := databaseOutput{Database: database}
		if opts.Forks {
			value.Forks = forks
		}
		return opts.Exporter.Write(opts.IO, value)
	}
	return renderHuman(opts.IO, database, forks, opts.Forks)
}

type databaseOutput struct {
	dolthub.Database
	Forks []dolthub.DatabaseRef `json:"forks,omitempty"`
}

func renderHuman(streams *iostreams.IOStreams, database dolthub.Database, forks []dolthub.DatabaseRef, includeForks bool) error {
	table := tableprinter.New(streams, "FIELD", "VALUE")
	rows := [][2]string{
		{"Name", database.Owner + "/" + database.Name},
		{"Visibility", string(database.Visibility)},
		{"Description", fallback(database.Description)},
		{"Size", strconv.FormatInt(database.SizeBytes, 10)},
		{"Stars", strconv.Itoa(database.StarCount)},
		{"Last write", "-"},
		{"Parent", ref(database.Parent)},
		{"Network root", ref(database.NetworkRoot)},
		{"Fork network", strconv.Itoa(database.ForkNetworkCount)},
	}
	if database.LastWriteAt != nil {
		rows[5][1] = database.LastWriteAt.Format("2006-01-02T15:04:05Z07:00")
	}
	for _, row := range rows {
		_ = table.AddRow(row[0], row[1])
	}
	if includeForks {
		encoded, _ := json.Marshal(forks)
		_ = table.AddRow("Forks", string(encoded))
	}
	return table.Render()
}

func databaseURL(repo repository.Repository) string {
	u := url.URL{Scheme: "https", Host: repo.Host, Path: "/repositories/" + repo.Owner + "/" + repo.Name}
	u.RawPath = "/repositories/" + url.PathEscape(repo.Owner) + "/" + url.PathEscape(repo.Name)
	return u.String()
}

func fallback(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func ref(value *dolthub.DatabaseRef) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%s/%s", value.Owner, value.Name)
}
