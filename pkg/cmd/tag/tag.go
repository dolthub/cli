package tag

import (
	listcmd "github.com/dolthub/cli/pkg/cmd/tag/list"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmdTag(f *cmdutil.Factory) *cobra.Command {
	c := &cobra.Command{Use: "tag", Short: "Work with database tags"}
	c.AddCommand(listcmd.NewCmdList(f, nil))
	return c
}
