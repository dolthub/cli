package root

import (
	"github.com/dolthub/cli/pkg/cmd/api"
	"github.com/dolthub/cli/pkg/cmd/auth"
	"github.com/dolthub/cli/pkg/cmd/branch"
	"github.com/dolthub/cli/pkg/cmd/browse"
	"github.com/dolthub/cli/pkg/cmd/completion"
	configcmd "github.com/dolthub/cli/pkg/cmd/config"
	"github.com/dolthub/cli/pkg/cmd/db"
	"github.com/dolthub/cli/pkg/cmd/operation"
	"github.com/dolthub/cli/pkg/cmd/pr"
	"github.com/dolthub/cli/pkg/cmd/release"
	"github.com/dolthub/cli/pkg/cmd/sql"
	"github.com/dolthub/cli/pkg/cmd/table"
	"github.com/dolthub/cli/pkg/cmd/tag"
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

	cmd.AddCommand(api.NewCmdAPI(f, nil), auth.NewCmdAuth(f), branch.NewCmdBranch(f), browse.NewCmdBrowse(f, nil), completion.NewCmdCompletion(), configcmd.NewCmdConfig(f), db.NewCmdDB(f), operation.NewCmdOperation(f), pr.NewCmdPR(f), release.NewCmdRelease(f), sql.NewCmdSQL(f, nil), tag.NewCmdTag(f), table.NewCmdTable(f), version.NewCmdVersion(f, nil))
	return cmd
}
