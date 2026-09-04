package branch

import (
	createcmd "github.com/dolthub/cli/pkg/cmd/branch/create"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmdBranch(f *cmdutil.Factory) *cobra.Command {
	c := &cobra.Command{Use: "branch", Short: "Work with database branches"}
	c.AddCommand(createcmd.NewCmdCreate(f, nil))
	return c
}
