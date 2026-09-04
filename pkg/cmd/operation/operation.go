package operation

import (
	listcmd "github.com/dolthub/cli/pkg/cmd/operation/list"
	viewcmd "github.com/dolthub/cli/pkg/cmd/operation/view"
	watchcmd "github.com/dolthub/cli/pkg/cmd/operation/watch"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

// NewCmdOperation constructs the asynchronous operation command group.
func NewCmdOperation(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "operation", Short: "Work with asynchronous operations"}
	cmd.AddCommand(listcmd.NewCmdList(f, nil), viewcmd.NewCmdView(f, nil), watchcmd.NewCmdWatch(f, nil))
	return cmd
}
