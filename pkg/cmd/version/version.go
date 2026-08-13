package version

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewCmdVersion constructs the version command.
func NewCmdVersion(appVersion string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show dh version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "dh version %s\n", appVersion)
			return err
		},
	}
}
