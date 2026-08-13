package version

import (
	"fmt"

	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

// Options contains the dependencies and parsed flags for the version command.
type Options struct {
	IO         *iostreams.IOStreams
	AppVersion string
	Short      bool
}

// NewCmdVersion constructs the version command.
func NewCmdVersion(f *cmdutil.Factory, runF func(*Options) error) *cobra.Command {
	opts := &Options{
		IO:         f.IO,
		AppVersion: f.AppVersion,
	}
	if runF == nil {
		runF = versionRun
	}

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show dh version information",
		Args:  cmdutil.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runF(opts)
		},
	}
	cmd.Flags().BoolVar(&opts.Short, "short", false, "Print only the version number")
	return cmd
}

func versionRun(opts *Options) error {
	if opts.Short {
		_, err := fmt.Fprintln(opts.IO.Out, opts.AppVersion)
		return err
	}
	_, err := fmt.Fprintf(opts.IO.Out, "dh version %s\n", opts.AppVersion)
	return err
}
