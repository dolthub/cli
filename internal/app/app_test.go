package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

func TestMain(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
		wantErr  []string
	}{
		{
			name:     "help",
			args:     []string{"--help"},
			wantCode: exitSuccess,
			wantOut:  "DoltHub from the command line",
		},
		{
			name:     "version",
			args:     []string{"version"},
			wantCode: exitSuccess,
			wantOut:  "dh version test\n",
		},
		{
			name:     "unknown command",
			args:     []string{"unknown"},
			wantCode: exitUsage,
			wantErr:  []string{`unknown command "unknown"`, "Usage:"},
		},
		{
			name:     "unknown flag",
			args:     []string{"--unknown"},
			wantCode: exitUsage,
			wantErr:  []string{"unknown flag", "Usage:"},
		},
		{
			name:     "unexpected argument",
			args:     []string{"version", "extra"},
			wantCode: exitUsage,
			wantErr:  []string{"accepts no arguments", "Usage:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streams, _, stdout, stderr := iostreams.NewTest()
			code := Run(tt.args, streams, "test")

			if code != tt.wantCode {
				t.Fatalf("Run() code = %d, want %d", code, tt.wantCode)
			}
			if tt.wantOut != "" && !strings.Contains(stdout.String(), tt.wantOut) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantOut)
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr = %q, want it to contain %q", stderr.String(), want)
				}
			}
		})
	}
}

func TestSemanticErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		writeFirst  bool
		wantCode    int
		wantErr     string
		wantUsage   bool
		wantSilence bool
	}{
		{name: "general failure", err: errors.New("boom"), wantCode: exitFailure, wantErr: "error: boom\n"},
		{name: "flag error", err: cmdutil.FlagErrorf("bad flags"), wantCode: exitUsage, wantErr: "error: bad flags", wantUsage: true},
		{name: "cancellation", err: &cmdutil.CancelError{}, wantCode: exitUsage, wantErr: "error: operation canceled\n"},
		{name: "authentication", err: &cmdutil.AuthError{}, wantCode: exitAuth, wantErr: "Run 'dh auth login' to authenticate."},
		{name: "silent failure", err: &cmdutil.SilentError{}, writeFirst: true, wantCode: exitFailure, wantErr: "already shown\n"},
		{name: "no results", err: &cmdutil.NoResultsError{}, wantCode: exitSuccess, wantSilence: true},
		{name: "external command", err: &cmdutil.ExternalCommandError{Err: errors.New("dolt failed"), ExitCode: 23}, wantCode: 23, wantErr: "error: dolt failed\n"},
		{name: "invalid external code", err: &cmdutil.ExternalCommandError{ExitCode: 999}, wantCode: exitFailure, wantErr: "error: external command failed\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streams, _, stdout, stderr := iostreams.NewTest()
			cmd := &cobra.Command{
				Use:           "test",
				SilenceErrors: true,
				SilenceUsage:  true,
				RunE: func(_ *cobra.Command, _ []string) error {
					if tt.writeFirst {
						fmt.Fprintln(streams.ErrOut, "already shown")
					}
					return tt.err
				},
			}

			code := execute(cmd, streams)
			if code != tt.wantCode {
				t.Fatalf("execute() code = %d, want %d", code, tt.wantCode)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			if tt.wantSilence && stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
			if tt.wantErr != "" && !strings.Contains(stderr.String(), tt.wantErr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantErr)
			}
			if got := strings.Contains(stderr.String(), "Usage:"); got != tt.wantUsage {
				t.Errorf("usage present = %v, want %v; stderr = %q", got, tt.wantUsage, stderr.String())
			}
		})
	}
}
