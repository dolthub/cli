package release

import (
	createcmd "github.com/dolthub/cli/pkg/cmd/release/create"
	listcmd "github.com/dolthub/cli/pkg/cmd/release/list"
	viewcmd "github.com/dolthub/cli/pkg/cmd/release/view"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmdRelease(f *cmdutil.Factory) *cobra.Command {
	c := &cobra.Command{Use: "release", Short: "Work with database releases"}
	c.AddCommand(createcmd.NewCmdCreate(f, nil))
	c.AddCommand(listcmd.NewCmdList(f, nil))
	c.AddCommand(viewcmd.NewCmdView(f, nil))
	return c
}
