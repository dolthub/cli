package pr

import (
	listcmd "github.com/dolthub/cli/pkg/cmd/pr/list"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmdPR(f *cmdutil.Factory) *cobra.Command {
	c := &cobra.Command{Use: "pr", Short: "Work with pull requests"}
	c.AddCommand(listcmd.NewCmdList(f, nil))
	return c
}
