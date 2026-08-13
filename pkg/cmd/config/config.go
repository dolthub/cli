package configcmd

import (
	getcmd "github.com/dolthub/cli/pkg/cmd/config/get"
	setcmd "github.com/dolthub/cli/pkg/cmd/config/set"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

// NewCmdConfig constructs the config command group.
func NewCmdConfig(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Manage dh configuration"}
	cmd.AddCommand(getcmd.NewCmdGet(f, nil), setcmd.NewCmdSet(f, nil))
	return cmd
}
