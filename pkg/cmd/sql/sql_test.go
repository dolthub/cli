package sql

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/operationwaiter"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

type fakeClient struct {
	readRequest  dolthub.SQLReadRequest
	writeRequest dolthub.SQLWriteRequest
	readResult   dolthub.QueryResult
	ref          dolthub.OperationRef
	operation    dolthub.Operation
	pollErr      error
	reads        int
	writes       int
}

func (f *fakeClient) RunSQLRead(_ context.Context, _, _ string, request dolthub.SQLReadRequest) (dolthub.QueryResult, error) {
	f.reads++
	f.readRequest = request
	return f.readResult, nil
}
func (f *fakeClient) RunSQLWrite(_ context.Context, _, _ string, request dolthub.SQLWriteRequest) (dolthub.OperationRef, error) {
	f.writes++
	f.writeRequest = request
	return f.ref, nil
}
func (f *fakeClient) GetOperation(context.Context, string) (dolthub.Operation, error) {
	return f.operation, nil
}
func (f *fakeClient) GetOperationURL(context.Context, string) (dolthub.Operation, error) {
	return f.operation, f.pollErr
}

func resolve(context.Context, string) (repository.Repository, error) {
	return repository.Repository{Host: "example.test", Owner: "acme", Name: "widgets"}, nil
}

func strptr(value string) *string { return &value }

func TestReadRendersDynamicRowsAndWarnings(t *testing.T) {
	io, _, out, errOut := iostreams.NewTest()
	c := &fakeClient{readResult: dolthub.QueryResult{
		Columns:  []dolthub.QueryColumn{{Name: "id"}, {Name: "note"}},
		Rows:     [][]*string{{strptr("1"), nil}, {strptr("2"), strptr("a\tb\nc\\d")}},
		Status:   dolthub.QuerySuccess,
		Warnings: []string{"result warning"},
	}}
	o := &Options{IO: io, ResolveRepository: resolve, Query: "select * from t", Ref: "main", client: c}
	if err := sqlRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "1\tNULL\n2\ta\\tb\\nc\\\\d\n" {
		t.Fatalf("stdout = %q", got)
	}
	if got := errOut.String(); got != "warning: result warning\n" {
		t.Fatalf("stderr = %q", got)
	}
	if c.readRequest.Ref != "main" || c.readRequest.Query != "select * from t" {
		t.Fatalf("request = %#v", c.readRequest)
	}
}

func TestReadStatusAndMalformedRowsFail(t *testing.T) {
	for _, test := range []struct {
		name   string
		result dolthub.QueryResult
		match  string
	}{
		{name: "query status", result: dolthub.QueryResult{Status: dolthub.QueryTimeout, Message: "too slow"}, match: "SQL query timeout"},
		{name: "unknown status", result: dolthub.QueryResult{Status: "future"}, match: "SQL query future"},
		{name: "row width", result: dolthub.QueryResult{Columns: []dolthub.QueryColumn{{Name: "a"}}, Rows: [][]*string{{}}, Status: dolthub.QuerySuccess}, match: "row 1 has 0 cells"},
	} {
		t.Run(test.name, func(t *testing.T) {
			io, _, _, errOut := iostreams.NewTest()
			o := &Options{IO: io, ResolveRepository: resolve, Query: "select 1", Ref: "main", client: &fakeClient{readResult: test.result}}
			err := sqlRun(context.Background(), o)
			if err == nil || !strings.Contains(err.Error(), test.match) {
				t.Fatalf("error = %v", err)
			}
			if test.result.Message != "" && errOut.String() != test.result.Message+"\n" {
				t.Fatalf("stderr = %q", errOut.String())
			}
		})
	}
}

func TestWriteDefaultsFromBranchAndNoWait(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{ref: dolthub.OperationRef{ID: "job/1", Href: "https://example.test/op/1"}}
	o := &Options{IO: io, ResolveRepository: resolve, Query: "insert into t values (1)", Write: true, Branch: "main", NoWait: true, client: c}
	if err := sqlRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if c.writeRequest.FromBranch != "main" || c.writeRequest.ToBranch != "main" || !strings.Contains(out.String(), "job/1") {
		t.Fatalf("request = %#v, stdout = %q", c.writeRequest, out.String())
	}
}

func TestWriteWaitFailureIsRendered(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	failed := dolthub.Operation{ID: "job/1", Type: dolthub.OperationSQLWrite, Status: dolthub.OperationFailed}
	c := &fakeClient{ref: dolthub.OperationRef{ID: "job/1", Href: "https://example.test/op/1"}}
	o := &Options{IO: io, ResolveRepository: resolve, Query: "delete from t", Write: true, Branch: "feature", FromBranch: "main", client: c, wait: func(context.Context, dolthub.OperationRef) (dolthub.Operation, error) {
		return failed, &operationwaiter.FailedError{Operation: failed}
	}}
	err := sqlRun(context.Background(), o)
	var failedErr *operationwaiter.FailedError
	if !errors.As(err, &failedErr) || !strings.Contains(out.String(), "failed") || c.writeRequest.FromBranch != "main" {
		t.Fatalf("error = %v, request = %#v, stdout = %q", err, c.writeRequest, out.String())
	}
}

func TestWriteReportsStatusInTTY(t *testing.T) {
	io, _, _, errOut := iostreams.NewTest()
	io.SetStderrTTY(true)
	c := &fakeClient{
		ref:       dolthub.OperationRef{ID: "job/1", Href: "https://example.test/op/1"},
		operation: dolthub.Operation{ID: "job/1", Type: dolthub.OperationSQLWrite, Status: dolthub.OperationSucceeded},
	}
	o := &Options{IO: io, ResolveRepository: resolve, Query: "update t set n=1", Write: true, Branch: "main", client: c}
	if err := sqlRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if got := errOut.String(); !strings.Contains(got, "Waiting for job job/1: succeeded") {
		t.Fatalf("stderr = %q", got)
	}
}

func TestWriteFinishesProgressBeforeRendering(t *testing.T) {
	for _, status := range []dolthub.OperationStatus{dolthub.OperationSucceeded, dolthub.OperationFailed} {
		t.Run(string(status), func(t *testing.T) {
			streams, _, output, _ := iostreams.NewTest()
			streams.ErrOut = output
			streams.SetStderrTTY(true)
			c := &fakeClient{
				ref:       dolthub.OperationRef{ID: "job/1", Href: "https://example.test/op/1"},
				operation: dolthub.Operation{ID: "job/1", Type: dolthub.OperationSQLWrite, Status: status},
			}
			o := &Options{IO: streams, ResolveRepository: resolve, Query: "update t set n=1", Write: true, Branch: "main", client: c}
			err := sqlRun(context.Background(), o)
			if (err != nil) != (status == dolthub.OperationFailed) {
				t.Fatalf("error = %v", err)
			}
			if !strings.Contains(output.String(), "Waiting for job job/1: "+string(status)+"\nID\tjob/1\n") {
				t.Fatalf("combined output = %q", output.String())
			}
		})
	}
}

func TestWritePollingErrorDoesNotRenderEmptyJob(t *testing.T) {
	streams, _, output, errOutput := iostreams.NewTest()
	streams.SetStderrTTY(true)
	pollErr := &dolthub.APIError{Status: 404, Method: "GET", Path: "/api/v2/operations/job/1", Detail: "no such repository"}
	c := &fakeClient{
		ref:     dolthub.OperationRef{ID: "job/1", Href: "https://example.test/op/1"},
		pollErr: pollErr,
	}
	o := &Options{IO: streams, ResolveRepository: resolve, Query: "update t set n=1", Write: true, Branch: "main", client: c}
	if err := sqlRun(context.Background(), o); !errors.Is(err, pollErr) {
		t.Fatalf("error = %v, want %v", err, pollErr)
	}
	if output.Len() != 0 {
		t.Fatalf("unexpected job output = %q", output.String())
	}
	if got := errOutput.String(); got != "Waiting for job job/1...\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestCommandReadsFileAndConvertsOptions(t *testing.T) {
	io, _, _, _ := iostreams.NewTest()
	c := &fakeClient{readResult: dolthub.QueryResult{Status: dolthub.QuerySuccess}}
	f := &cmdutil.Factory{IO: io, ResolveRepository: resolve}
	cmd := NewCmdSQL(f, func(ctx context.Context, o *Options) error {
		o.client = c
		o.ReadFile = func(name string) ([]byte, error) {
			if name != "query.sql" {
				t.Fatalf("file = %q", name)
			}
			return []byte("select\n  1;"), nil
		}
		return sqlRun(ctx, o)
	})
	cmd.SetArgs([]string{"--file", "query.sql", "--ref", "main", "--limit", "7", "--timeout", "2500ms"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if c.readRequest.Query != "select\n  1;" || c.readRequest.Limit != 7 || c.readRequest.TimeoutMS != 2500 {
		t.Fatalf("request = %#v", c.readRequest)
	}
}

func TestCommandStructuredRead(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{readResult: dolthub.QueryResult{Columns: []dolthub.QueryColumn{{Name: "a"}, {Name: "b"}}, Rows: [][]*string{{strptr("1"), nil}}, Status: dolthub.QuerySuccess}}
	f := &cmdutil.Factory{IO: io, ResolveRepository: resolve}
	cmd := NewCmdSQL(f, func(ctx context.Context, o *Options) error { o.client = c; return sqlRun(ctx, o) })
	cmd.SetArgs([]string{"select 1", "--ref", "main", "--json", "rows,status"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil || got["status"] != "success" {
		t.Fatalf("output = %q, decoded = %#v, error = %v", out.String(), got, err)
	}
}

func TestCommandValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing query", args: []string{"--ref", "main"}},
		{name: "missing ref", args: []string{"select 1"}},
		{name: "write missing branch", args: []string{"--write", "insert"}},
		{name: "read write flag", args: []string{"select 1", "--ref", "main", "--branch", "main"}},
		{name: "write read flag", args: []string{"--write", "insert", "--branch", "main", "--limit", "1"}},
		{name: "bad limit", args: []string{"select 1", "--ref", "main", "--limit", "0"}},
		{name: "sub millisecond", args: []string{"select 1", "--ref", "main", "--timeout", "1500us"}},
		{name: "long timeout", args: []string{"select 1", "--ref", "main", "--timeout", "61s"}},
		{name: "conflicting input", args: []string{"select 1", "--file", "q.sql", "--ref", "main"}},
		{name: "read field in write", args: []string{"--write", "insert", "--branch", "main", "--json", "rows"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			io, _, _, _ := iostreams.NewTest()
			io.SetStdinTTY(true)
			cmd := NewCmdSQL(&cmdutil.Factory{IO: io}, sqlRun)
			cmd.SetArgs(test.args)
			if err := cmd.Execute(); err == nil {
				t.Fatal("command succeeded")
			}
		})
	}
}

func TestReadQuerySources(t *testing.T) {
	io, in, _, _ := iostreams.NewTest()
	in.WriteString("select from pipe")
	query, err := readQuery(&Options{IO: io})
	if err != nil || query != "select from pipe" {
		t.Fatalf("query = %q, error = %v", query, err)
	}

	io, in, _, _ = iostreams.NewTest()
	in.WriteString("select from dash")
	query, err = readQuery(&Options{IO: io, File: "-"})
	if err != nil || query != "select from dash" {
		t.Fatalf("query = %q, error = %v", query, err)
	}
}

func TestTimeoutBoundary(t *testing.T) {
	for _, timeout := range []time.Duration{time.Millisecond, 60 * time.Second} {
		if err := validateMode(&Options{Ref: "main", Timeout: timeout, TimeoutSet: true}); err != nil {
			t.Fatalf("timeout %s: %v", timeout, err)
		}
	}
}
