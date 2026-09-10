package job

import (
	listcmd "github.com/dolthub/cli/pkg/cmd/job/list"
	viewcmd "github.com/dolthub/cli/pkg/cmd/job/view"
	watchcmd "github.com/dolthub/cli/pkg/cmd/job/watch"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

// NewCmdJob constructs the asynchronous job command group.
func NewCmdJob(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "job", Short: "Work with asynchronous jobs"}
	cmd.AddCommand(listcmd.NewCmdList(f, nil), viewcmd.NewCmdView(f, nil), watchcmd.NewCmdWatch(f, nil))
	return cmd
}
