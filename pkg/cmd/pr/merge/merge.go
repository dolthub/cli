package merge

import (
	"context"

	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/operationwaiter"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/internal/tableprinter"
	"github.com/dolthub/cli/pkg/cmd/operation/progress"
	viewcmd "github.com/dolthub/cli/pkg/cmd/operation/view"
	"github.com/dolthub/cli/pkg/cmd/pr/shared"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

var jsonFields = []string{"cancelable", "created_at", "error", "href", "id", "result", "status", "type"}

type apiClient interface {
	MergePull(context.Context, string, string, int64) (dolthub.OperationRef, error)
	GetOperation(context.Context, string) (dolthub.Operation, error)
	GetOperationURL(context.Context, string) (dolthub.Operation, error)
}
type Options struct {
	IO                *iostreams.IOStreams
	ResolveRepository func(context.Context, string) (repository.Repository, error)
	APIClientForHost  func(string) (*dolthub.Client, error)
	Repository        string
	Number            int64
	NoWait            bool
	Exporter          cmdutil.Exporter
	client            apiClient
	wait              func(context.Context, dolthub.OperationRef) (dolthub.Operation, error)
}

func NewCmdMerge(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, ResolveRepository: f.ResolveRepository, APIClientForHost: f.APIClientForHost}
	if runF == nil {
		runF = mergeRun
	}
	c := &cobra.Command{Use: "merge NUMBER", Short: "Merge a pull request", Args: func(c *cobra.Command, args []string) error {
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
	c.Flags().BoolVar(&o.NoWait, "no-wait", false, "Return after the merge is accepted")
	cmdutil.AddJSONFlags(c, &o.Exporter, jsonFields)
	return cmdutil.WithDocs(c, "dh pr merge 1 --db OWNER/people", cmdutil.DocMetadata{
		Arguments: []cmdutil.DocArgument{{Name: "NUMBER", Description: "Positive pull request number in the selected database.", Optional: false}},
		Output:    "Waits for completion and prints operation details by default. Progress goes to stderr. --json selects operation fields. With --no-wait, prints the accepted ID and HREF instead; use --json id,href for structured acceptance. Acceptance is not completion. A failed operation returns a nonzero exit status. Interrupting the local wait does not cancel the remote operation.",
	})
}
func mergeRun(ctx context.Context, o *Options) error {
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
	ref, err := c.MergePull(ctx, r.Owner, r.Name, o.Number)
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
