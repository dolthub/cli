package reopen

import (
	"context"

	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/pkg/cmd/pr/shared"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

type client interface {
	UpdatePull(context.Context, string, string, int64, dolthub.UpdatePullRequest) (dolthub.Pull, error)
}
type Options struct {
	IO                *iostreams.IOStreams
	ResolveRepository func(context.Context, string) (repository.Repository, error)
	APIClientForHost  func(string) (*dolthub.Client, error)
	Repository        string
	Number            int64
	Exporter          cmdutil.Exporter
	client            client
}

func NewCmdReopen(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, ResolveRepository: f.ResolveRepository, APIClientForHost: f.APIClientForHost}
	if runF == nil {
		runF = reopenRun
	}
	c := &cobra.Command{Use: "reopen NUMBER", Short: "Reopen a pull request", Args: func(c *cobra.Command, args []string) error {
		if err := cmdutil.ExactArgs(1)(c, args); err != nil {
			return err
		}
		n, err := shared.ParseNumber(args[0])
		if err != nil {
			return err
		}
		o.Number = n
		return nil
	}, RunE: func(c *cobra.Command, _ []string) error { return runF(c.Context(), o) }}
	cmdutil.AddDatabaseFlag(c, &o.Repository)
	cmdutil.AddJSONFlags(c, &o.Exporter, shared.JSONFields)
	return c
}
func reopenRun(ctx context.Context, o *Options) error {
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
	state := dolthub.PullStateOpen
	pull, err := c.UpdatePull(ctx, r.Owner, r.Name, o.Number, dolthub.UpdatePullRequest{State: &state})
	if err != nil {
		return err
	}
	if o.Exporter != nil {
		return o.Exporter.Write(o.IO, pull)
	}
	return shared.RenderPull(o.IO, pull)
}
