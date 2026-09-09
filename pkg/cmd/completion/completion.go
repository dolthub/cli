package completion

import (
	"io"

	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

// NewCmdCompletion constructs the shell completion command.
func NewCmdCompletion() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion {bash|fish|powershell|zsh}",
		Short: "Generate shell completion scripts",
		Args:  cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generate(cmd.Root(), cmd.OutOrStdout(), args[0])
		},
	}
	return cmdutil.WithDocs(cmd, "dh completion bash > dh.bash\ndh completion zsh > _dh", cmdutil.DocMetadata{
		Arguments: []cmdutil.DocArgument{{Name: "SHELL", Description: "One of bash, fish, powershell, or zsh.", Optional: false}},
		Output:    "Writes a shell completion script to stdout. Save or source it using your shell's completion setup; this command does not install the script.",
	})
}

func generate(root *cobra.Command, out io.Writer, shell string) error {
	switch shell {
	case "bash":
		return root.GenBashCompletionV2(out, true)
	case "fish":
		return root.GenFishCompletion(out, true)
	case "powershell":
		return root.GenPowerShellCompletionWithDesc(out)
	case "zsh":
		return root.GenZshCompletion(out)
	default:
		return cmdutil.FlagErrorf("unsupported shell %q; expected bash, fish, powershell, or zsh", shell)
	}
}
