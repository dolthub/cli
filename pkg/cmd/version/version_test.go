package version

import (
	"testing"

	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

func TestNewCmdVersionParsesOptions(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	f := &cmdutil.Factory{
		AppVersion: "1.2.3",
		IO:         streams,
	}

	var gotOpts *Options
	cmd := NewCmdVersion(f, func(opts *Options) error {
		gotOpts = opts
		return nil
	})
	cmd.SetArgs([]string{"--short"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotOpts == nil {
		t.Fatal("injected run function was not called")
	}
	if gotOpts.IO != streams {
		t.Fatal("Options.IO does not contain the injected streams")
	}
	if got, want := gotOpts.AppVersion, "1.2.3"; got != want {
		t.Fatalf("Options.AppVersion = %q, want %q", got, want)
	}
	if !gotOpts.Short {
		t.Fatal("Options.Short = false, want true")
	}
}

func TestVersionRun(t *testing.T) {
	tests := []struct {
		name  string
		short bool
		want  string
	}{
		{name: "full", want: "dh version 1.2.3\n"},
		{name: "short", short: true, want: "1.2.3\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streams, _, stdout, _ := iostreams.NewTest()
			opts := &Options{
				IO:         streams,
				AppVersion: "1.2.3",
				Short:      tt.short,
			}

			if err := versionRun(opts); err != nil {
				t.Fatal(err)
			}
			if got := stdout.String(); got != tt.want {
				t.Fatalf("output = %q, want %q", got, tt.want)
			}
		})
	}
}
