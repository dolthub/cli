package operation

import (
	viewcmd "github.com/dolthub/cli/pkg/cmd/operation/view"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

// NewCmdOperation constructs the asynchronous operation command group.
func NewCmdOperation(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "operation", Short: "Work with asynchronous operations"}
	cmd.AddCommand(viewcmd.NewCmdView(f, nil))
	return cmd
}
