package edit

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
	IO                             *iostreams.IOStreams
	ResolveRepository              func(context.Context, string) (repository.Repository, error)
	APIClientForHost               func(string) (*dolthub.Client, error)
	Repository                     string
	Number                         int64
	Title, Body, BodyFile          string
	TitleSet, BodySet, BodyFileSet bool
	Exporter                       cmdutil.Exporter
	client                         client
}

func NewCmdEdit(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, ResolveRepository: f.ResolveRepository, APIClientForHost: f.APIClientForHost}
	if runF == nil {
		runF = editRun
	}
	c := &cobra.Command{Use: "edit NUMBER", Short: "Edit a pull request", Args: func(c *cobra.Command, args []string) error {
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
	c.PreRunE = func(c *cobra.Command, _ []string) error {
		o.TitleSet = c.Flags().Changed("title")
		o.BodySet = c.Flags().Changed("body")
		o.BodyFileSet = c.Flags().Changed("body-file")
		if o.BodySet && o.BodyFileSet {
			return cmdutil.FlagErrorf("--body and --body-file are mutually exclusive")
		}
		if !o.TitleSet && !o.BodySet && !o.BodyFileSet {
			return cmdutil.FlagErrorf("at least one of --title, --body, or --body-file is required")
		}
		return nil
	}
	cmdutil.AddDatabaseFlag(c, &o.Repository)
	c.Flags().StringVarP(&o.Title, "title", "t", "", "Set the pull request title")
	c.Flags().StringVarP(&o.Body, "body", "b", "", "Set the pull request body")
	c.Flags().StringVarP(&o.BodyFile, "body-file", "F", "", "Read the pull request body from file")
	cmdutil.AddJSONFlags(c, &o.Exporter, shared.JSONFields)
	return c
}
func editRun(ctx context.Context, o *Options) error {
	request := dolthub.UpdatePullRequest{}
	if o.TitleSet {
		request.Title = &o.Title
	}
	if o.BodySet || o.BodyFileSet {
		body, err := cmdutil.ReadTextSource(o.Body, o.BodyFile)
		if err != nil {
			return err
		}
		request.Description = &body
	}
	if request.Title == nil && request.Description == nil {
		return cmdutil.FlagErrorf("at least one field is required")
	}
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
	pull, err := c.UpdatePull(ctx, r.Owner, r.Name, o.Number, request)
	if err != nil {
		return err
	}
	if o.Exporter != nil {
		return o.Exporter.Write(o.IO, pull)
	}
	return shared.RenderPull(o.IO, pull)
}
