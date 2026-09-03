package completion

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func TestCompletionGeneratesEverySupportedShell(t *testing.T) {
	for _, shell := range []string{"bash", "fish", "powershell", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			var out bytes.Buffer
			root := &cobra.Command{Use: "dh"}
			root.SetOut(&out)
			root.AddCommand(NewCmdCompletion())
			root.SetArgs([]string{"completion", shell})
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(out.String()) == "" {
				t.Fatal("completion output is empty")
			}
		})
	}
}

func TestCompletionValidatesArguments(t *testing.T) {
	for _, args := range [][]string{{"completion"}, {"completion", "tcsh"}, {"completion", "bash", "extra"}} {
		root := &cobra.Command{Use: "dh", SilenceUsage: true, SilenceErrors: true}
		root.AddCommand(NewCmdCompletion())
		root.SetArgs(args)
		err := root.Execute()
		var flagErr *cmdutil.FlagError
		if !errors.As(err, &flagErr) {
			t.Fatalf("Execute(%v) error = %T %v, want FlagError", args, err, err)
		}
	}
}
