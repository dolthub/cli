package watch

import (
	"context"
	"time"

	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/credentials"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/operationwaiter"
	"github.com/dolthub/cli/pkg/cmd/job/progress"
	viewcmd "github.com/dolthub/cli/pkg/cmd/job/view"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

type apiClient interface {
	GetOperation(context.Context, string) (dolthub.Operation, error)
	GetOperationURL(context.Context, string) (dolthub.Operation, error)
}
type Options struct {
	IO               *iostreams.IOStreams
	Config           func() (config.Config, error)
	Credentials      credentials.Store
	LookupEnv        func(string) (string, bool)
	APIClientForHost func(string) (*dolthub.Client, error)
	ID               string
	Interval         time.Duration
	Exporter         cmdutil.Exporter
	client           apiClient
	wait             func(context.Context, string) (dolthub.Operation, error)
}

func NewCmdWatch(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, Config: f.Config, Credentials: f.Credentials, LookupEnv: f.LookupEnv, APIClientForHost: f.APIClientForHost, Interval: operationwaiter.DefaultInterval}
	if runF == nil {
		runF = watchRun
	}
	c := &cobra.Command{Use: "watch ID", Short: "Watch an asynchronous job", Args: cmdutil.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		o.ID = args[0]
		if o.ID == "" {
			return cmdutil.FlagErrorf("job ID must not be empty")
		}
		return runF(c.Context(), o)
	}}
	c.PreRunE = func(*cobra.Command, []string) error {
		if o.Interval <= 0 {
			return cmdutil.FlagErrorf("--interval must be greater than zero")
		}
		return nil
	}
	c.Flags().DurationVar(&o.Interval, "interval", operationwaiter.DefaultInterval, "Initial polling interval")
	cmdutil.AddJSONFlags(c, &o.Exporter, viewcmd.JSONFields)
	return cmdutil.WithDocs(c, "dh job watch JOB_ID --json id,status,result", cmdutil.DocMetadata{
		Arguments:   []cmdutil.DocArgument{{Name: "ID", Description: "Job ID returned by an asynchronous command; resolved on the configured host.", Optional: false}},
		Constraints: []string{"Authentication is required. --interval must be positive; it sets the initial polling interval. Uses the configured host."},
		Output:      "Polls until the job finishes and prints its details or selected JSON fields. Progress goes to stderr; failed or canceled jobs return a nonzero exit status. Stopping the local wait does not cancel remote work.",
	})
}
func watchRun(ctx context.Context, o *Options) error {
	cfg, err := o.Config()
	if err != nil {
		return err
	}
	host := cfg.Host()
	if err := viewcmd.RequireAuthentication(cfg, o.Credentials, o.LookupEnv, host); err != nil {
		return err
	}
	wait := o.wait
	var reporter *progress.Reporter
	if wait == nil {
		c := o.client
		if c == nil {
			c, err = o.APIClientForHost(host)
			if err != nil {
				return err
			}
		}
		reporter = progress.New(o.IO, o.ID)
		reporter.Start()
		w := operationwaiter.Waiter{Client: c, Interval: o.Interval, Observe: reporter.Observe}
		wait = w.WaitID
	}
	operation, waitErr := wait(ctx, o.ID)
	reporter.Done()
	if waitErr != nil && operation.ID == "" {
		return waitErr
	}
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
