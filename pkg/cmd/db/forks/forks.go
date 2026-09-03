package forks

import (
	"context"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/internal/tableprinter"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

var jsonFields = []string{"name", "owner"}

type client interface {
	ListForks(context.Context, string, string) ([]dolthub.DatabaseRef, error)
}
type Options struct {
	IO                *iostreams.IOStreams
	ResolveRepository func(context.Context, string) (repository.Repository, error)
	APIClientForHost  func(string) (*dolthub.Client, error)
	Repository        string
	Exporter          cmdutil.Exporter
	client            client
}

func NewCmdForks(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, ResolveRepository: f.ResolveRepository, APIClientForHost: f.APIClientForHost}
	if runF == nil {
		runF = forksRun
	}
	c := &cobra.Command{Use: "forks [DATABASE]", Short: "List immediate database forks", Args: func(c *cobra.Command, a []string) error {
		if len(a) > 1 {
			return cmdutil.FlagErrorf("%s accepts at most one argument", c.CommandPath())
		}
		if len(a) == 1 {
			if o.Repository != "" {
				return cmdutil.FlagErrorf("cannot use a database argument with --repo")
			}
			o.Repository = a[0]
		}
		return nil
	}, RunE: func(c *cobra.Command, _ []string) error { return runF(c.Context(), o) }}
	c.Flags().StringVarP(&o.Repository, "repo", "R", "", "Select a database repository using [HOST/]OWNER/REPO")
	cmdutil.AddJSONFlags(c, &o.Exporter, jsonFields)
	return c
}
func forksRun(ctx context.Context, o *Options) error {
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
	items, e := c.ListForks(ctx, r.Owner, r.Name)
	if e != nil {
		return e
	}
	if o.Exporter != nil {
		return o.Exporter.Write(o.IO, items)
	}
	t := tableprinter.New(o.IO, "OWNER", "NAME")
	for _, v := range items {
		_ = t.AddRow(v.Owner, v.Name)
	}
	return t.Render()
}
