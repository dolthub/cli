package pr

import (
	closecmd "github.com/dolthub/cli/pkg/cmd/pr/close"
	commentcmd "github.com/dolthub/cli/pkg/cmd/pr/comment"
	createcmd "github.com/dolthub/cli/pkg/cmd/pr/create"
	editcmd "github.com/dolthub/cli/pkg/cmd/pr/edit"
	listcmd "github.com/dolthub/cli/pkg/cmd/pr/list"
	mergecmd "github.com/dolthub/cli/pkg/cmd/pr/merge"
	reopencmd "github.com/dolthub/cli/pkg/cmd/pr/reopen"
	viewcmd "github.com/dolthub/cli/pkg/cmd/pr/view"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmdPR(f *cmdutil.Factory) *cobra.Command {
	c := &cobra.Command{Use: "pr", Short: "Work with pull requests"}
	c.AddCommand(closecmd.NewCmdClose(f, nil))
	c.AddCommand(commentcmd.NewCmdComment(f, nil))
	c.AddCommand(createcmd.NewCmdCreate(f, nil))
	c.AddCommand(editcmd.NewCmdEdit(f, nil))
	c.AddCommand(listcmd.NewCmdList(f, nil))
	c.AddCommand(mergecmd.NewCmdMerge(f, nil))
	c.AddCommand(reopencmd.NewCmdReopen(f, nil))
	c.AddCommand(viewcmd.NewCmdView(f, nil))
	return c
}
