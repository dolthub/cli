package root

import (
	"github.com/dolthub/cli/pkg/cmd/version"
	"github.com/spf13/cobra"
)

// NewCmdRoot constructs the root of the dh command tree.
func NewCmdRoot(appVersion string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "dh",
		Short:         "DoltHub from the command line",
		Long:          "Work with DoltHub from the command line.",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.AddCommand(version.NewCmdVersion(appVersion))
	return cmd
}
