package app

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/dolthub/cli/pkg/cmd/factory"
	"github.com/dolthub/cli/pkg/cmd/root"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
	exitAuth    = 4
)

// Main runs dh with explicitly supplied process inputs and returns its exit code.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer, version string) int {
	return Run(args, iostreams.New(stdin, stdout, stderr), version)
}

// Run executes dh using the supplied I/O boundary.
func Run(args []string, streams *iostreams.IOStreams, version string) int {
	f := factory.New(version, streams)
	cmd := root.NewCmdRoot(f)
	cmd.SetArgs(args)
	return execute(cmd, streams)
}

func execute(cmd *cobra.Command, streams *iostreams.IOStreams) int {
	cmd.SetIn(streams.In)
	cmd.SetOut(streams.Out)
	cmd.SetErr(streams.ErrOut)

	if err := cmd.Execute(); err != nil {
		// Cobra does not expose a distinct error type for command lookup failures.
		// They are nevertheless command-line usage errors.
		if strings.HasPrefix(err.Error(), "unknown command ") {
			err = &cmdutil.FlagError{Err: err}
		}
		return renderError(cmd, streams, err)
	}

	return exitSuccess
}

func renderError(cmd *cobra.Command, streams *iostreams.IOStreams, err error) int {
	var noResultsErr *cmdutil.NoResultsError
	if errors.As(err, &noResultsErr) {
		return exitSuccess
	}

	var silentErr *cmdutil.SilentError
	if errors.As(err, &silentErr) {
		return exitFailure
	}

	var flagErr *cmdutil.FlagError
	if errors.As(err, &flagErr) {
		fmt.Fprintf(streams.ErrOut, "error: %v\n\n", err)
		fmt.Fprint(streams.ErrOut, cmd.UsageString())
		return exitUsage
	}

	var cancelErr *cmdutil.CancelError
	if errors.As(err, &cancelErr) {
		fmt.Fprintf(streams.ErrOut, "error: %v\n", err)
		return exitUsage
	}

	var authErr *cmdutil.AuthError
	if errors.As(err, &authErr) {
		fmt.Fprintf(streams.ErrOut, "error: %v\n", err)
		fmt.Fprintln(streams.ErrOut, "Run 'dh auth login' to authenticate.")
		return exitAuth
	}

	var externalErr *cmdutil.ExternalCommandError
	if errors.As(err, &externalErr) {
		fmt.Fprintf(streams.ErrOut, "error: %v\n", err)
		if externalErr.ExitCode > 0 && externalErr.ExitCode <= 255 {
			return externalErr.ExitCode
		}
		return exitFailure
	}

	fmt.Fprintf(streams.ErrOut, "error: %v\n", err)
	return exitFailure
}
