package auth

import (
	"github.com/dolthub/cli/pkg/cmd/auth/login"
	"github.com/dolthub/cli/pkg/cmd/auth/logout"
	statuscmd "github.com/dolthub/cli/pkg/cmd/auth/status"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmdAuth(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Authenticate dh with DoltHub"}
	cmd.AddCommand(login.NewCmdLogin(f, nil), statuscmd.NewCmdStatus(f, nil), logout.NewCmdLogout(f, nil))
	return cmd
}
