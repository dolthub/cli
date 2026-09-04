package pr

import (
	commentcmd "github.com/dolthub/cli/pkg/cmd/pr/comment"
	createcmd "github.com/dolthub/cli/pkg/cmd/pr/create"
	listcmd "github.com/dolthub/cli/pkg/cmd/pr/list"
	viewcmd "github.com/dolthub/cli/pkg/cmd/pr/view"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmdPR(f *cmdutil.Factory) *cobra.Command {
	c := &cobra.Command{Use: "pr", Short: "Work with pull requests"}
	c.AddCommand(commentcmd.NewCmdComment(f, nil))
	c.AddCommand(createcmd.NewCmdCreate(f, nil))
	c.AddCommand(listcmd.NewCmdList(f, nil))
	c.AddCommand(viewcmd.NewCmdView(f, nil))
	return c
}
