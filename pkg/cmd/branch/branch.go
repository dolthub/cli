package branch

import (
	listcmd "github.com/dolthub/cli/pkg/cmd/branch/list"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmdBranch(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "branch", Short: "Work with database branches"}
	cmd.AddCommand(listcmd.NewCmdList(f, nil))
	return cmd
}
