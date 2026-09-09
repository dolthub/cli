package cmdutil

import "github.com/spf13/cobra"

// AddDatabaseFlag adds the canonical database selector and a hidden --repo
// compatibility alias. Both flags write to the same destination.
func AddDatabaseFlag(cmd *cobra.Command, target *string) {
	if cmd.Long == "" {
		cmd.Long = cmd.Short
	}
	cmd.Long += "\n\nSelect a database with --db, then DH_DB, saved database configuration, or local Dolt remotes (in that order). Use dh config set db OWNER/DB to save a default. Multiple remotes prompt interactively; scripts must select a database explicitly when discovery is ambiguous."
	cmd.Flags().StringVarP(target, "db", "R", "", "Select a database using [HOST/]OWNER/DB")
	cmd.Flags().StringVar(target, "repo", "", "Select a database using [HOST/]OWNER/DB")
	_ = cmd.Flags().MarkHidden("repo")
}
