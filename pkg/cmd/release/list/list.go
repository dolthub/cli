package list

import (
	"context"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/pagination"
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
	Repository        string
	Limit             int
	Exporter          cmdutil.Exporter
	client            client
}

func NewCmdList(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, ResolveRepository: f.ResolveRepository, APIClientForHost: f.APIClientForHost, Limit: 30}
	if runF == nil {
		runF = listRun
	}
	c := &cobra.Command{Use: "list", Short: "List database releases", Args: cmdutil.NoArgs, RunE: func(c *cobra.Command, _ []string) error { return runF(c.Context(), o) }}
	c.PreRunE = func(*cobra.Command, []string) error {
		if o.Limit < 1 {
			return cmdutil.FlagErrorf("--limit must be greater than zero")
		}
		return nil
	}
	cmdutil.AddDatabaseFlag(c, &o.Repository)
	c.Flags().IntVar(&o.Limit, "limit", 30, "Maximum number of releases")
	cmdutil.AddJSONFlags(c, &o.Exporter, jsonFields)
	return c
}
func listRun(ctx context.Context, o *Options) error {
	r, e := o.ResolveRepository(ctx, o.Repository)
	if e != nil {
		return e
	}
	c := o.client
	if c == nil {
		c, e = o.APIClientForHost(r.Host)
		if e != nil {
			return e
		}
	}
	items, e := pagination.Collect(ctx, o.Limit, func(ctx context.Context, t string) (pagination.Page[dolthub.Release], error) {
		v, n, e := c.ListReleases(ctx, r.Owner, r.Name, t)
		return pagination.Page[dolthub.Release]{Items: v, NextPageToken: n}, e
	})
	if e != nil {
		return e
	}
	if o.Exporter != nil {
		return o.Exporter.Write(o.IO, items)
	}
	table := tableprinter.New(o.IO, "TAG", "TITLE", "COMMIT", "CREATED", "UPDATED")
	for _, v := range items {
		_ = table.AddRow(v.Tag, v.Title, v.CommitSHA, v.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), v.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
	}
	return table.Render()
}
