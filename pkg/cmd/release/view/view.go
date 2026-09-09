package view

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/dolthub/cli/internal/browser"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/internal/tableprinter"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

var jsonFields = []string{"commit_sha", "created_at", "description", "tag", "title", "updated_at"}

type client interface {
	ListReleases(context.Context, string, string, string) ([]dolthub.Release, string, error)
}

type Options struct {
	IO                *iostreams.IOStreams
	ResolveRepository func(context.Context, string) (repository.Repository, error)
	APIClientForHost  func(string) (*dolthub.Client, error)
	Browser           browser.Browser
	Repository        string
	Tag               string
	Web               bool
	Exporter          cmdutil.Exporter
	client            client
}

func NewCmdView(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, ResolveRepository: f.ResolveRepository, APIClientForHost: f.APIClientForHost, Browser: f.Browser}
	if runF == nil {
		runF = viewRun
	}
	c := &cobra.Command{
		Use: "view TAG", Short: "View a database release", Args: cmdutil.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error { o.Tag = args[0]; return runF(c.Context(), o) },
	}
	c.PreRunE = func(c *cobra.Command, _ []string) error {
		if o.Web && (c.Flags().Changed("json") || c.Flags().Changed("jq") || c.Flags().Changed("template")) {
			return cmdutil.FlagErrorf("--web cannot be used with structured output")
		}
		return nil
	}
	cmdutil.AddDatabaseFlag(c, &o.Repository)
	c.Flags().BoolVarP(&o.Web, "web", "w", false, "Open the release in a browser")
	cmdutil.AddJSONFlags(c, &o.Exporter, jsonFields)
	return cmdutil.WithDocs(c, "dh release view v1 --db OWNER/people", cmdutil.DocMetadata{
		Arguments:   []cmdutil.DocArgument{{Name: "TAG", Description: "Release tag name.", Optional: false}},
		Constraints: []string{"--web cannot be combined with structured output."},
		Output:      "Prints release details or selected JSON fields. --web opens the release in the browser.",
	})
}

func viewRun(ctx context.Context, o *Options) error {
	r, err := o.ResolveRepository(ctx, o.Repository)
	if err != nil {
		return err
	}
	if o.Web {
		return o.Browser.Browse(releaseURL(r))
	}
	c := o.client
	if c == nil {
		c, err = o.APIClientForHost(r.Host)
		if err != nil {
			return err
		}
	}
	token, seen := "", map[string]bool{}
	for {
		items, next, err := c.ListReleases(ctx, r.Owner, r.Name, token)
		if err != nil {
			return err
		}
		for _, release := range items {
			if release.Tag == o.Tag {
				return render(o, release)
			}
		}
		if next == "" {
			return fmt.Errorf("release %q not found", o.Tag)
		}
		if next == token || seen[next] {
			return fmt.Errorf("pagination token %q was repeated", next)
		}
		seen[next], token = true, next
	}
}

func render(o *Options, release dolthub.Release) error {
	if o.Exporter != nil {
		return o.Exporter.Write(o.IO, release)
	}
	t := tableprinter.New(o.IO, "FIELD", "VALUE")
	rows := [][2]string{{"Title", release.Title}, {"Tag", release.Tag}, {"Commit", release.CommitSHA}, {"Created", release.CreatedAt.Format("2006-01-02T15:04:05Z07:00")}, {"Updated", release.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")}, {"Description", fallback(release.Description)}}
	for _, row := range rows {
		_ = t.AddRow(row[0], row[1])
	}
	return t.Render()
}

func releaseURL(r repository.Repository) string {
	path := "/repositories/" + r.Owner + "/" + r.Name + "/releases"
	raw := "/repositories/" + url.PathEscape(r.Owner) + "/" + url.PathEscape(r.Name) + "/releases"
	return (&url.URL{Scheme: "https", Host: r.Host, Path: path, RawPath: raw}).String()
}

func fallback(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
