package db

import (
	forkscmd "github.com/dolthub/cli/pkg/cmd/db/forks"
	viewcmd "github.com/dolthub/cli/pkg/cmd/db/view"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

// NewCmdDB constructs the database command group.
func NewCmdDB(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "db", Short: "Work with DoltHub database repositories"}
	cmd.AddCommand(forkscmd.NewCmdForks(f, nil), viewcmd.NewCmdView(f, nil))
	return cmd
}
