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

var jsonFields = []string{"head_commit_sha", "last_updated_at", "name"}

type client interface {
	CreateBranch(context.Context, string, string, dolthub.CreateBranchRequest) (dolthub.Branch, error)
}
type Options struct {
	IO                                       *iostreams.IOStreams
	ResolveRepository                        func(context.Context, string) (repository.Repository, error)
	APIClientForHost                         func(string) (*dolthub.Client, error)
	Repository, Name, FromBranch, FromCommit string
	Exporter                                 cmdutil.Exporter
	client                                   client
}

func NewCmdCreate(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, ResolveRepository: f.ResolveRepository, APIClientForHost: f.APIClientForHost}
	if runF == nil {
		runF = createRun
	}
	c := &cobra.Command{Use: "create NAME", Short: "Create a database branch", Args: func(c *cobra.Command, args []string) error {
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
	cmdutil.AddJSONFlags(c, &o.Exporter, jsonFields)
	return cmdutil.WithDocs(c, "dh branch create feature/people --db OWNER/people --from-branch main", cmdutil.DocMetadata{
		Arguments:   []cmdutil.DocArgument{{Name: "NAME", Description: "New branch name.", Optional: false}},
		Constraints: []string{"Exactly one of --from-branch or --from-commit is required."},
		Output:      "Prints the created branch and commit, or selected JSON fields.",
	})
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
	b, err := c.CreateBranch(ctx, r.Owner, r.Name, dolthub.CreateBranchRequest{Name: o.Name, From: source})
	if err != nil {
		return err
	}
	if o.Exporter != nil {
		return o.Exporter.Write(o.IO, b)
	}
	t := tableprinter.New(o.IO, "NAME", "COMMIT", "UPDATED")
	updated := "-"
	if b.LastUpdatedAt != nil {
		updated = b.LastUpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	_ = t.AddRow(b.Name, b.HeadCommitSHA, updated)
	return t.Render()
}
