package app

import (
	"fmt"
	"io"

	"github.com/dolthub/cli/pkg/cmd/root"
)

// Main runs dh with explicitly supplied process inputs and returns its exit code.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer, version string) int {
	cmd := root.NewCmdRoot(version)
	cmd.SetArgs(args)
	cmd.SetIn(stdin)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(stderr, "error: %v\n\n", err)
		fmt.Fprint(stderr, cmd.UsageString())
		return 1
	}

	return 0
}
