package cmdutil

import "github.com/spf13/cobra"

// AddDatabaseFlag adds the canonical database selector and a hidden --repo
// compatibility alias. Both flags write to the same destination.
func AddDatabaseFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVarP(target, "db", "R", "", "Select a database using [HOST/]OWNER/DB")
	cmd.Flags().StringVar(target, "repo", "", "Select a database using [HOST/]OWNER/DB")
	_ = cmd.Flags().MarkHidden("repo")
}
