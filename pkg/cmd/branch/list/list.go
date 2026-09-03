package list

import (
	"context"
	"fmt"

	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/pagination"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/internal/tableprinter"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

var jsonFields = []string{"head_commit_sha", "last_updated_at", "name"}

type client interface {
	ListBranches(context.Context, string, string, string) ([]dolthub.Branch, string, error)
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
	cmd := &cobra.Command{Use: "list", Short: "List database branches", Args: cmdutil.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return runF(cmd.Context(), o) }}
	cmd.PreRunE = func(*cobra.Command, []string) error {
		if o.Limit < 1 {
			return cmdutil.FlagErrorf("--limit must be greater than zero")
		}
		return nil
	}
	cmd.Flags().StringVarP(&o.Repository, "repo", "R", "", "Select a database repository using [HOST/]OWNER/REPO")
	cmd.Flags().IntVar(&o.Limit, "limit", 30, "Maximum number of branches")
	cmdutil.AddJSONFlags(cmd, &o.Exporter, jsonFields)
	return cmd
}

func listRun(ctx context.Context, o *Options) error {
	r, err := o.ResolveRepository(ctx, o.Repository)
	if err != nil {
		return err
	}
	c := o.client
	if c == nil {
		c, err = o.APIClientForHost(r.Host)
		if err != nil {
			return err
		}
	}
	items, err := pagination.Collect(ctx, o.Limit, func(ctx context.Context, token string) (pagination.Page[dolthub.Branch], error) {
		items, next, err := c.ListBranches(ctx, r.Owner, r.Name, token)
		return pagination.Page[dolthub.Branch]{Items: items, NextPageToken: next}, err
	})
	if err != nil {
		return err
	}
	if o.Exporter != nil {
		return o.Exporter.Write(o.IO, items)
	}
	t := tableprinter.New(o.IO, "NAME", "HEAD", "UPDATED")
	for _, b := range items {
		updated := "-"
		if b.LastUpdatedAt != nil {
			updated = b.LastUpdatedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		if err := t.AddRow(b.Name, b.HeadCommitSHA, updated); err != nil {
			return err
		}
	}
	if err := t.Render(); err != nil {
		return fmt.Errorf("render branches: %w", err)
	}
	return nil
}
