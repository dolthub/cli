package fork

import (
	"context"

	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/operationwaiter"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/internal/tableprinter"
	"github.com/dolthub/cli/pkg/cmd/operation/progress"
	viewcmd "github.com/dolthub/cli/pkg/cmd/operation/view"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

var jsonFields = []string{"cancelable", "created_at", "error", "href", "id", "result", "status", "type"}

type apiClient interface {
	CurrentUser(context.Context) (dolthub.User, error)
	CreateFork(context.Context, string, string, dolthub.CreateForkRequest) (dolthub.OperationRef, error)
	GetOperation(context.Context, string) (dolthub.Operation, error)
	GetOperationURL(context.Context, string) (dolthub.Operation, error)
}
type Options struct {
	IO                       *iostreams.IOStreams
	ResolveRepository        func(context.Context, string) (repository.Repository, error)
	APIClientForHost         func(string) (*dolthub.Client, error)
	Repository, Organization string
	NoWait                   bool
	Exporter                 cmdutil.Exporter
	client                   apiClient
	wait                     func(context.Context, dolthub.OperationRef) (dolthub.Operation, error)
}

func NewCmdFork(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, ResolveRepository: f.ResolveRepository, APIClientForHost: f.APIClientForHost}
	if runF == nil {
		runF = forkRun
	}
	c := &cobra.Command{Use: "fork [DATABASE]", Short: "Fork a database repository", Args: func(c *cobra.Command, args []string) error {
		if len(args) > 1 {
			return cmdutil.FlagErrorf("%s accepts at most one argument", c.CommandPath())
		}
		if len(args) == 1 {
			if o.Repository != "" {
				return cmdutil.FlagErrorf("cannot use a database argument with --db")
			}
			o.Repository = args[0]
		}
		return nil
	}, RunE: func(c *cobra.Command, _ []string) error { return runF(c.Context(), o) }}
	cmdutil.AddDatabaseFlag(c, &o.Repository)
	c.Flags().StringVar(&o.Organization, "org", "", "Organization or user to own the fork")
	c.Flags().BoolVar(&o.NoWait, "no-wait", false, "Return after the fork is accepted")
	cmdutil.AddJSONFlags(c, &o.Exporter, jsonFields)
	return cmdutil.WithDocs(c, "dh db fork OWNER/people --org MY_USER", cmdutil.DocMetadata{
		Arguments:   []cmdutil.DocArgument{{Name: "DATABASE", Description: "Database in [HOST/]OWNER/DB form; when omitted, use --db, DH_DB, a saved database, or local Dolt remotes.", Optional: true}},
		Constraints: []string{"A positional DATABASE and --db cannot be combined. --org selects the owner of the new fork; when omitted, the authenticated user owns it."},
		Output:      "Waits for completion and prints operation details by default. Progress goes to stderr. --json selects operation fields. With --no-wait, prints the accepted ID and HREF instead; use --json id,href for structured acceptance. Acceptance is not completion. A failed operation returns a nonzero exit status. Interrupting the local wait does not cancel the remote operation.",
	})
}
func forkRun(ctx context.Context, o *Options) error {
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
	owner := o.Organization
	if owner == "" {
		user, e := c.CurrentUser(ctx)
		if e != nil {
			return e
		}
		owner = user.Username
	}
	ref, err := c.CreateFork(ctx, r.Owner, r.Name, dolthub.CreateForkRequest{Owner: owner})
	if err != nil {
		return err
	}
	if o.NoWait {
		if o.Exporter != nil {
			return o.Exporter.Write(o.IO, ref)
		}
		t := tableprinter.New(o.IO, "ID", "HREF")
		_ = t.AddRow(ref.ID, ref.Href)
		return t.Render()
	}
	wait := o.wait
	if wait == nil {
		reporter := progress.New(o.IO, ref.ID)
		reporter.Start()
		defer reporter.Done()
		wait = operationwaiter.Waiter{Client: c, Observe: reporter.Observe}.Wait
	}
	operation, waitErr := wait(ctx, ref)
	var renderErr error
	if o.Exporter != nil {
		renderErr = o.Exporter.Write(o.IO, operation)
	} else {
		renderErr = viewcmd.RenderHuman(o.IO, operation)
	}
	if renderErr != nil {
		return renderErr
	}
	return waitErr
}
