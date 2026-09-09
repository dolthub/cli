package generatedocs

import (
	"fmt"
	"strings"

	"github.com/dolthub/cli/internal/buildinfo"
	"github.com/dolthub/cli/internal/docgen"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

// NewCmdGenerateDocs stores newRoot without invoking it during construction.
func NewCmdGenerateDocs(f *cmdutil.Factory, newRoot func() *cobra.Command) *cobra.Command {
	var output string
	var check bool
	cmd := &cobra.Command{
		Use: "generate-docs --output DIRECTORY", Hidden: true,
		Short: "Export the command reference for documentation maintainers",
		Long:  "Export a grouped Markdown reference and manifest without authentication or network access.\nUse an empty directory or an existing generated bundle. --check compares without writing.",
		Args:  cmdutil.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(output) == "" {
				return cmdutil.FlagErrorf("--output is required and must not be empty")
			}
			bundle, err := docgen.Generate(newRoot(), buildinfo.Current(f.AppVersion))
			if err != nil {
				return err
			}
			if err := docgen.Write(output, bundle, check); err != nil {
				return err
			}
			action := "Generated"
			if check {
				action = "Verified"
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s command documentation in %s\n", action, output)
			return err
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "Bundle output directory (required)")
	cmd.Flags().BoolVar(&check, "check", false, "Compare the existing bundle without writing")
	return cmd
}
