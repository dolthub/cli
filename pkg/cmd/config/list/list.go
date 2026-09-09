package list

import (
	"fmt"
	"strings"

	internalconfig "github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/internal/tableprinter"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

type Options struct {
	IO        *iostreams.IOStreams
	Config    func() (internalconfig.Config, error)
	LookupEnv func(string) (string, bool)
}

func NewCmdList(f *cmdutil.Factory, runF func(*Options) error) *cobra.Command {
	opts := &Options{IO: f.IO, Config: f.Config, LookupEnv: f.LookupEnv}
	if runF == nil {
		runF = listRun
	}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configuration values",
		Args:  cmdutil.NoArgs,
		RunE:  func(_ *cobra.Command, _ []string) error { return runF(opts) },
	}
	return cmdutil.WithDocs(cmd, "dh config list", cmdutil.DocMetadata{
		Output: "Prints KEY, VALUE, and SOURCE for host and db, showing DH_HOST/DH_DB, saved configuration, or default/unset values. It does not discover local remotes.",
	})
}

func listRun(opts *Options) error {
	cfg, err := opts.Config()
	if err != nil {
		return err
	}
	host, hostSource := hostValue(cfg, opts.LookupEnv)
	repo, repoSource, err := repoValue(cfg, opts.LookupEnv, host)
	if err != nil {
		return err
	}
	table := tableprinter.New(opts.IO, "KEY", "VALUE", "SOURCE")
	_ = table.AddRow("host", host, hostSource)
	_ = table.AddRow("db", repo, repoSource)
	return table.Render()
}

func hostValue(cfg internalconfig.Config, lookup func(string) (string, bool)) (string, string) {
	if value, ok := env(lookup, "DH_HOST"); ok {
		return value, "environment"
	}
	if value, ok := cfg.ConfiguredHost(); ok {
		return value, "config"
	}
	return internalconfig.DefaultHost, "default"
}

func repoValue(cfg internalconfig.Config, lookup func(string) (string, bool), host string) (string, string, error) {
	if value, name, ok := repository.LookupDatabaseEnv(lookup); ok {
		repo, err := repository.Parse(value, host)
		if err != nil {
			return "", "", fmt.Errorf("invalid %s: %w", name, err)
		}
		return displayRepository(repo, host), "environment", nil
	}
	if repo, ok := cfg.ConfiguredRepository(); ok {
		return displayRepository(repo, host), "config", nil
	}
	return "", "unset", nil
}

func env(lookup func(string) (string, bool), key string) (string, bool) {
	if lookup == nil {
		return "", false
	}
	value, ok := lookup(key)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}

func displayRepository(repo repository.Repository, effectiveHost string) string {
	if strings.EqualFold(repo.Host, effectiveHost) {
		return repo.FullName()
	}
	return repo.Host + "/" + repo.FullName()
}
