package browse

import (
	"context"
	"errors"
	"testing"

	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/pkg/cmdutil"
)

type fakeBrowser struct {
	url string
	err error
}

func (f *fakeBrowser) Browse(value string) error { f.url = value; return f.err }

func TestBrowseDestinations(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{nil, "https://example.test/repositories/acme%2Fwest/widgets%20plus"},
		{[]string{"17"}, "https://example.test/repositories/acme%2Fwest/widgets%20plus/pulls/17"},
		{[]string{"--pull", "18"}, "https://example.test/repositories/acme%2Fwest/widgets%20plus/pulls/18"},
		{[]string{"--branch", "feature/one"}, "https://example.test/repositories/acme%2Fwest/widgets%20plus/data/feature%2Fone"},
	}
	for _, tt := range tests {
		browser := &fakeBrowser{}
		cmd := NewCmdBrowse(&cmdutil.Factory{}, func(_ context.Context, opts *Options) error {
			opts.ResolveRepository = func(_ context.Context, explicit string) (repository.Repository, error) {
				if explicit != "chosen/repo" {
					t.Fatalf("explicit repository = %q", explicit)
				}
				return repository.Repository{Host: "example.test", Owner: "acme/west", Name: "widgets plus"}, nil
			}
			opts.Browser = browser
			return browseRun(context.Background(), opts)
		})
		cmd.SetArgs(append(tt.args, "--repo", "chosen/repo"))
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if browser.url != tt.want {
			t.Fatalf("args %v: URL = %q, want %q", tt.args, browser.url, tt.want)
		}
	}
}

func TestBrowseValidation(t *testing.T) {
	for _, args := range [][]string{{"0"}, {"nope"}, {"1", "2"}, {"1", "--pull", "2"}, {"--pull", "0"}, {"--pull", "2", "--branch", "main"}, {"--branch", ""}} {
		cmd := NewCmdBrowse(&cmdutil.Factory{}, func(context.Context, *Options) error { return nil })
		cmd.SilenceErrors, cmd.SilenceUsage = true, true
		cmd.SetArgs(args)
		err := cmd.Execute()
		var flagErr *cmdutil.FlagError
		if !errors.As(err, &flagErr) {
			t.Fatalf("args %v: error = %T %v", args, err, err)
		}
	}
}

func TestBrowsePropagatesBrowserFailure(t *testing.T) {
	want := errors.New("no browser")
	opts := &Options{
		ResolveRepository: func(context.Context, string) (repository.Repository, error) {
			return repository.Repository{Host: "example.test", Owner: "o", Name: "r"}, nil
		},
		Browser: &fakeBrowser{err: want},
	}
	if err := browseRun(context.Background(), opts); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}
