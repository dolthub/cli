package cmdutil

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestAddDatabaseFlag(t *testing.T) {
	for _, args := range [][]string{{"--db", "owner/database"}, {"-R", "owner/database"}, {"--repo", "owner/database"}} {
		var database string
		cmd := &cobra.Command{Use: "test", Run: func(*cobra.Command, []string) {}}
		AddDatabaseFlag(cmd, &database)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("args %v: %v", args, err)
		}
		if database != "owner/database" {
			t.Fatalf("args %v: database = %q", args, database)
		}
	}
}

func TestAddDatabaseFlagHidesRepoAlias(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	var database string
	AddDatabaseFlag(cmd, &database)
	if help := cmd.Flags().FlagUsages(); strings.Contains(help, "--repo") || !strings.Contains(help, "--db") {
		t.Fatalf("flag help = %q", help)
	}
	if strings.Contains(cmd.Long, "config set repo") || !strings.Contains(cmd.Long, "config set db") {
		t.Fatalf("long help = %q", cmd.Long)
	}
	if strings.Contains(cmd.Long, "DH_REPO") || !strings.Contains(cmd.Long, "DH_DB") {
		t.Fatalf("long help = %q", cmd.Long)
	}
}
