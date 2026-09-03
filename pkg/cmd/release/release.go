package release

import (
	listcmd "github.com/dolthub/cli/pkg/cmd/release/list"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmdRelease(f *cmdutil.Factory) *cobra.Command {
	c := &cobra.Command{Use: "release", Short: "Work with database releases"}
	c.AddCommand(listcmd.NewCmdList(f, nil))
	return c
}
