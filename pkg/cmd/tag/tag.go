package tag

import (
	createcmd "github.com/dolthub/cli/pkg/cmd/tag/create"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmdTag(f *cmdutil.Factory) *cobra.Command {
	c := &cobra.Command{Use: "tag", Short: "Work with database tags"}
	c.AddCommand(createcmd.NewCmdCreate(f, nil))
	return c
}
