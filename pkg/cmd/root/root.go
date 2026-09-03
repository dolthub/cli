package root

import (
	"github.com/dolthub/cli/pkg/cmd/auth"
	"github.com/dolthub/cli/pkg/cmd/browse"
	"github.com/dolthub/cli/pkg/cmd/completion"
	configcmd "github.com/dolthub/cli/pkg/cmd/config"
	"github.com/dolthub/cli/pkg/cmd/db"
	"github.com/dolthub/cli/pkg/cmd/version"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

// NewCmdRoot constructs the root of the dh command tree.
func NewCmdRoot(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "dh",
		Short:         "DoltHub from the command line",
		Long:          "Work with DoltHub from the command line.",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &cmdutil.FlagError{Err: err}
	})

	cmd.AddCommand(auth.NewCmdAuth(f), browse.NewCmdBrowse(f, nil), completion.NewCmdCompletion(), configcmd.NewCmdConfig(f), db.NewCmdDB(f), version.NewCmdVersion(f, nil))
	return cmd
}
