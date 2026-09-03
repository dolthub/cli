package list

import (
	"context"
	"fmt"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/internal/tableprinter"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

var jsonFields = []string{"created_at", "creator", "description", "pull_number", "state", "title"}

type client interface {
	ListPulls(context.Context, string, string, string) ([]dolthub.PullSummary, string, error)
}
type Options struct {
	IO                *iostreams.IOStreams
	ResolveRepository func(context.Context, string) (repository.Repository, error)
	APIClientForHost  func(string) (*dolthub.Client, error)
	Repository        string
	Limit             int
	State             string
	Exporter          cmdutil.Exporter
	client            client
}

func NewCmdList(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, ResolveRepository: f.ResolveRepository, APIClientForHost: f.APIClientForHost, Limit: 30, State: "open"}
	if runF == nil {
		runF = listRun
	}
	c := &cobra.Command{Use: "list", Short: "List pull requests", Args: cmdutil.NoArgs, RunE: func(c *cobra.Command, _ []string) error { return runF(c.Context(), o) }}
	c.PreRunE = func(*cobra.Command, []string) error {
		if o.Limit < 1 {
			return cmdutil.FlagErrorf("--limit must be greater than zero")
		}
		switch o.State {
		case "open", "closed", "merged", "all":
			return nil
		}
		return cmdutil.FlagErrorf("--state must be one of open, closed, merged, or all")
	}
	c.Flags().StringVarP(&o.Repository, "repo", "R", "", "Select a database repository using [HOST/]OWNER/REPO")
	c.Flags().IntVar(&o.Limit, "limit", 30, "Maximum number of pull requests")
	c.Flags().StringVar(&o.State, "state", "open", "Filter by state: open, closed, merged, or all")
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
	items := make([]dolthub.PullSummary, 0, o.Limit)
	token := ""
	seen := map[string]bool{}
	for len(items) < o.Limit {
		page, next, e := c.ListPulls(ctx, r.Owner, r.Name, token)
		if e != nil {
			return e
		}
		for _, v := range page {
			if o.State == "all" || string(v.State) == o.State {
				items = append(items, v)
				if len(items) == o.Limit {
					break
				}
			}
		}
		if len(items) == o.Limit || next == "" {
			break
		}
		if next == token || seen[next] {
			return fmt.Errorf("pagination token %q was repeated", next)
		}
		seen[next] = true
		token = next
	}
	if o.Exporter != nil {
		return o.Exporter.Write(o.IO, items)
	}
	t := tableprinter.New(o.IO, "NUMBER", "TITLE", "STATE", "CREATOR", "CREATED")
	for _, v := range items {
		_ = t.AddRow(fmt.Sprint(v.PullNumber), v.Title, string(v.State), v.Creator, v.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	}
	return t.Render()
}
