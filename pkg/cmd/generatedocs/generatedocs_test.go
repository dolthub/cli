package generatedocs

import (
	"errors"
	"testing"

	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func TestUsageDoesNotConstructTree(t *testing.T) {
	for _, args := range [][]string{nil, {"--output", ""}, {"--output", " "}, {"extra"}, {"--output", "dir", "extra"}} {
		cmd := NewCmdGenerateDocs(&cmdutil.Factory{}, func() *cobra.Command { t.Fatal("constructed tree for invalid invocation"); return nil })
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		cmd.SetArgs(args)
		var usage *cmdutil.FlagError
		if err := cmd.Execute(); !errors.As(err, &usage) {
			t.Errorf("%v: want usage error, got %v", args, err)
		}
	}
}
