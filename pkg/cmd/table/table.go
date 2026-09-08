package table

import (
	importcmd "github.com/dolthub/cli/pkg/cmd/table/import"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmdTable(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "table", Short: "Work with DoltHub tables"}
	cmd.AddCommand(importcmd.NewCmdImport(f, nil))
	return cmd
}
