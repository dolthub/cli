package sql

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/operationwaiter"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/internal/tableprinter"
	"github.com/dolthub/cli/pkg/cmd/job/progress"
	viewcmd "github.com/dolthub/cli/pkg/cmd/job/view"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

var (
	readJSONFields  = []string{"columns", "message", "rows", "status", "warnings"}
	writeJSONFields = []string{"cancelable", "created_at", "error", "href", "id", "result", "status", "type"}
	allJSONFields   = []string{"cancelable", "columns", "created_at", "error", "href", "id", "message", "result", "rows", "status", "type", "warnings"}
)

type apiClient interface {
	RunSQLRead(context.Context, string, string, dolthub.SQLReadRequest) (dolthub.QueryResult, error)
	RunSQLWrite(context.Context, string, string, dolthub.SQLWriteRequest) (dolthub.OperationRef, error)
	GetOperation(context.Context, string) (dolthub.Operation, error)
	GetOperationURL(context.Context, string) (dolthub.Operation, error)
}

type Options struct {
	IO                *iostreams.IOStreams
	ResolveRepository func(context.Context, string) (repository.Repository, error)
	APIClientForHost  func(string) (*dolthub.Client, error)
	ReadFile          func(string) ([]byte, error)
	Repository        string
	Query             string
	File              string
	Ref               string
	Branch            string
	FromBranch        string
	Limit             int
	Timeout           time.Duration
	Write             bool
	NoWait            bool
	LimitSet          bool
	TimeoutSet        bool
	RefSet            bool
	BranchSet         bool
	FromBranchSet     bool
	NoWaitSet         bool
	QuerySet          bool
	Exporter          cmdutil.Exporter
	client            apiClient
	wait              func(context.Context, dolthub.OperationRef) (dolthub.Operation, error)
}

// QueryError reports a SQL-level failure returned in an HTTP 200 response.
type QueryError struct {
	Status  dolthub.QueryStatus
	Message string
}

func (e *QueryError) Error() string {
	return fmt.Sprintf("SQL query %s", e.Status)
}

func NewCmdSQL(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, ResolveRepository: f.ResolveRepository, APIClientForHost: f.APIClientForHost, ReadFile: os.ReadFile}
	if runF == nil {
		runF = sqlRun
	}
	c := &cobra.Command{
		Use:   "sql [QUERY]",
		Short: "Run SQL against a database",
		Args: func(c *cobra.Command, args []string) error {
			if len(args) > 1 {
				return cmdutil.FlagErrorf("%s accepts at most one query argument", c.CommandPath())
			}
			if len(args) == 1 {
				o.Query = args[0]
				o.QuerySet = true
			}
			return nil
		},
		RunE: func(c *cobra.Command, _ []string) error {
			o.LimitSet = c.Flags().Changed("limit")
			o.TimeoutSet = c.Flags().Changed("timeout")
			o.RefSet = c.Flags().Changed("ref")
			o.BranchSet = c.Flags().Changed("branch")
			o.FromBranchSet = c.Flags().Changed("from-branch")
			o.NoWaitSet = c.Flags().Changed("no-wait")
			return runF(c.Context(), o)
		},
	}
	cmdutil.AddDatabaseFlag(c, &o.Repository)
	c.Flags().StringVar(&o.File, "file", "", "Read SQL from `file` (use - for stdin)")
	c.Flags().StringVar(&o.Ref, "ref", "", "Branch, tag, or commit for a read query")
	c.Flags().IntVar(&o.Limit, "limit", 0, "Maximum rows to return")
	c.Flags().DurationVar(&o.Timeout, "timeout", 0, "Server-side query timeout")
	c.Flags().BoolVar(&o.Write, "write", false, "Run an asynchronous write query")
	c.Flags().StringVar(&o.Branch, "branch", "", "Branch on which to run a write query")
	c.Flags().StringVar(&o.FromBranch, "from-branch", "", "Base branch for a write query (defaults to --branch)")
	c.Flags().BoolVar(&o.NoWait, "no-wait", false, "Return after a write query is accepted")
	cmdutil.AddJSONFlags(c, &o.Exporter, allJSONFields)
	return cmdutil.WithDocs(c, "dh sql --db OWNER/people --ref main \"select * from people limit 10\"\ndh sql --write --db OWNER/people --branch feature/people --from-branch main --file update.sql", cmdutil.DocMetadata{
		Arguments:   []cmdutil.DocArgument{{Name: "QUERY", Description: "SQL text. If omitted, use --file or pipe SQL through stdin.", Optional: true}},
		Constraints: []string{"Provide only one SQL source: argument, --file, or stdin. --file - reads stdin. Empty queries are rejected.", "--jq and --template require --json. JSON fields must be valid for the selected read/write mode."},
		Output:      "Reads print rows to stdout and warnings to stderr, or selected JSON fields. Unsuccessful query status returns a nonzero exit code. Writes wait for a job and print its details; --no-wait prints ID/HREF after acceptance. For acceptance JSON use --json id,href.",
		Modes:       []cmdutil.DocMode{{Name: "Read queries", Description: "Requires --ref with a branch, tag, or commit. --limit must be positive when supplied; --timeout must be between 1ms and 60s in whole milliseconds. --branch, --from-branch, and --no-wait require write mode.", JSONFields: readJSONFields}, {Name: "Write queries", Description: "Requires --write and --branch. --from-branch defaults to --branch and supplies the source branch. --ref, --limit, and --timeout are read-only flags. Acceptance does not imply successful completion; watch the returned job ID.", JSONFields: writeJSONFields}},
	})
}

func sqlRun(ctx context.Context, o *Options) error {
	if err := validateMode(o); err != nil {
		return err
	}
	query, err := readQuery(o)
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
	if o.Write {
		return runWrite(ctx, o, c, r, query)
	}
	return runRead(ctx, o, c, r, query)
}

func validateMode(o *Options) error {
	if o.Write {
		if strings.TrimSpace(o.Branch) == "" {
			return cmdutil.FlagErrorf("--branch is required with --write")
		}
		if o.RefSet || o.LimitSet || o.TimeoutSet {
			return cmdutil.FlagErrorf("--ref, --limit, and --timeout are read-only flags")
		}
		return validateJSONFields(o.Exporter, writeJSONFields, "write")
	}
	if strings.TrimSpace(o.Ref) == "" {
		return cmdutil.FlagErrorf("--ref is required for read queries")
	}
	if o.BranchSet || o.FromBranchSet || o.NoWaitSet {
		return cmdutil.FlagErrorf("--branch, --from-branch, and --no-wait require --write")
	}
	if o.LimitSet && o.Limit <= 0 {
		return cmdutil.FlagErrorf("--limit must be greater than zero")
	}
	if o.TimeoutSet && (o.Timeout <= 0 || o.Timeout > 60*time.Second || o.Timeout%time.Millisecond != 0) {
		return cmdutil.FlagErrorf("--timeout must be between 1ms and 60s in whole milliseconds")
	}
	return validateJSONFields(o.Exporter, readJSONFields, "read")
}

func validateJSONFields(exporter cmdutil.Exporter, allowed []string, mode string) error {
	if exporter == nil {
		return nil
	}
	set := make(map[string]bool, len(allowed))
	for _, field := range allowed {
		set[field] = true
	}
	for _, field := range exporter.Fields() {
		if !set[field] {
			return cmdutil.FlagErrorf("JSON field %q is not available in SQL %s mode; available fields: %s", field, mode, strings.Join(allowed, ", "))
		}
	}
	return nil
}

func readQuery(o *Options) (string, error) {
	querySet := o.QuerySet || o.Query != ""
	if querySet && o.File != "" {
		return "", cmdutil.FlagErrorf("query argument and --file are mutually exclusive")
	}
	query := o.Query
	switch {
	case o.File == "-":
		payload, err := io.ReadAll(o.IO.In)
		if err != nil {
			return "", fmt.Errorf("read SQL from stdin: %w", err)
		}
		query = string(payload)
	case o.File != "":
		payload, err := o.ReadFile(o.File)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", o.File, err)
		}
		query = string(payload)
	case !querySet && !o.IO.IsStdinTTY():
		payload, err := io.ReadAll(o.IO.In)
		if err != nil {
			return "", fmt.Errorf("read SQL from stdin: %w", err)
		}
		query = string(payload)
	}
	if strings.TrimSpace(query) == "" {
		return "", cmdutil.FlagErrorf("SQL query is required as an argument, --file, or stdin")
	}
	return query, nil
}

func runRead(ctx context.Context, o *Options, c apiClient, r repository.Repository, query string) error {
	request := dolthub.SQLReadRequest{Ref: o.Ref, Query: query}
	if o.LimitSet {
		request.Limit = o.Limit
	}
	if o.TimeoutSet {
		request.TimeoutMS = int(o.Timeout / time.Millisecond)
	}
	result, err := c.RunSQLRead(ctx, r.Owner, r.Name, request)
	if err != nil {
		return err
	}
	if err := validateRows(result); err != nil {
		return err
	}
	if o.Exporter != nil {
		if err := o.Exporter.Write(o.IO, result); err != nil {
			return err
		}
	} else {
		if err := renderReadHuman(o.IO, result); err != nil {
			return err
		}
	}
	if result.Status != dolthub.QuerySuccess {
		if result.Message != "" {
			_, _ = fmt.Fprintln(o.IO.ErrOut, result.Message)
		}
		return &QueryError{Status: result.Status, Message: result.Message}
	}
	return nil
}

func validateRows(result dolthub.QueryResult) error {
	for i, row := range result.Rows {
		if len(row) != len(result.Columns) {
			return fmt.Errorf("malformed SQL result: row %d has %d cells, want %d", i+1, len(row), len(result.Columns))
		}
	}
	return nil
}

func renderReadHuman(streams *iostreams.IOStreams, result dolthub.QueryResult) error {
	if len(result.Columns) > 0 {
		headers := make([]string, len(result.Columns))
		for i, column := range result.Columns {
			headers[i] = escapeCell(column.Name)
		}
		table := tableprinter.New(streams, headers...)
		for _, source := range result.Rows {
			row := make([]string, len(source))
			for i, cell := range source {
				if cell == nil {
					row[i] = "NULL"
				} else {
					row[i] = escapeCell(*cell)
				}
			}
			if err := table.AddRow(row...); err != nil {
				return err
			}
		}
		if err := table.Render(); err != nil {
			return err
		}
	}
	for _, warning := range result.Warnings {
		if _, err := fmt.Fprintf(streams.ErrOut, "warning: %s\n", warning); err != nil {
			return err
		}
	}
	return nil
}

func escapeCell(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "\t", "\\t", "\r", "\\r", "\n", "\\n")
	return replacer.Replace(value)
}

func runWrite(ctx context.Context, o *Options, c apiClient, r repository.Repository, query string) error {
	from := o.FromBranch
	if strings.TrimSpace(from) == "" {
		from = o.Branch
	}
	ref, err := c.RunSQLWrite(ctx, r.Owner, r.Name, dolthub.SQLWriteRequest{FromBranch: from, ToBranch: o.Branch, Query: query})
	if err != nil {
		return err
	}
	if o.NoWait {
		if o.Exporter != nil {
			return o.Exporter.Write(o.IO, ref)
		}
		table := tableprinter.New(o.IO, "ID", "HREF")
		_ = table.AddRow(ref.ID, ref.Href)
		return table.Render()
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
	return errors.Join(renderErr, waitErr)
}
