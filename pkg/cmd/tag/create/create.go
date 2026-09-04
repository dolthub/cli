package create

import (
	"context"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/internal/tableprinter"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

var jsonFields = []string{"commit_sha", "message", "name", "tagged_at"}

type client interface {
	CreateTag(context.Context, string, string, dolthub.CreateTagRequest) (dolthub.Tag, error)
}
type Options struct {
	IO                                                *iostreams.IOStreams
	ResolveRepository                                 func(context.Context, string) (repository.Repository, error)
	APIClientForHost                                  func(string) (*dolthub.Client, error)
	Repository, Name, FromBranch, FromCommit, Message string
	Exporter                                          cmdutil.Exporter
	client                                            client
}

func NewCmdCreate(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, ResolveRepository: f.ResolveRepository, APIClientForHost: f.APIClientForHost}
	if runF == nil {
		runF = createRun
	}
	c := &cobra.Command{Use: "create NAME", Short: "Create a database tag", Args: func(c *cobra.Command, args []string) error {
		if err := cmdutil.ExactArgs(1)(c, args); err != nil {
			return err
		}
		o.Name = args[0]
		return nil
	}, RunE: func(c *cobra.Command, _ []string) error { return runF(c.Context(), o) }}
	c.PreRunE = func(*cobra.Command, []string) error {
		_, err := cmdutil.RevisionSource(o.FromBranch, o.FromCommit)
		return err
	}
	cmdutil.AddDatabaseFlag(c, &o.Repository)
	c.Flags().StringVar(&o.FromBranch, "from-branch", "", "Source branch")
	c.Flags().StringVar(&o.FromCommit, "from-commit", "", "Source commit SHA")
	c.Flags().StringVarP(&o.Message, "message", "m", "", "Tag annotation message")
	cmdutil.AddJSONFlags(c, &o.Exporter, jsonFields)
	return c
}
func createRun(ctx context.Context, o *Options) error {
	source, err := cmdutil.RevisionSource(o.FromBranch, o.FromCommit)
	if err != nil {
		return err
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
	var message *string
	if o.Message != "" {
		message = &o.Message
	}
	tag, err := c.CreateTag(ctx, r.Owner, r.Name, dolthub.CreateTagRequest{Name: o.Name, From: source, Message: message})
	if err != nil {
		return err
	}
	if o.Exporter != nil {
		return o.Exporter.Write(o.IO, tag)
	}
	t := tableprinter.New(o.IO, "NAME", "COMMIT", "TAGGED")
	tagged := "-"
	if tag.TaggedAt != nil {
		tagged = tag.TaggedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	_ = t.AddRow(tag.Name, tag.CommitSHA, tagged)
	return t.Render()
}
