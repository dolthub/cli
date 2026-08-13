package set

import (
	"strings"

	internalconfig "github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

type Options struct {
	Config     func() (internalconfig.Config, error)
	Key, Value string
}

func NewCmdSet(f *cmdutil.Factory, runF func(*Options) error) *cobra.Command {
	opts := &Options{Config: f.Config}
	if runF == nil {
		runF = setRun
	}
	return &cobra.Command{Use: "set KEY VALUE", Short: "Set a configuration value", Args: cmdutil.ExactArgs(2), RunE: func(_ *cobra.Command, args []string) error {
		opts.Key = args[0]
		opts.Value = args[1]
		return runF(opts)
	}}
}

func setRun(opts *Options) error {
	cfg, err := opts.Config()
	if err != nil {
		return err
	}
	switch opts.Key {
	case "host":
		if strings.TrimSpace(opts.Value) == "" {
			return cmdutil.FlagErrorf("host cannot be empty")
		}
		cfg.SetHost(opts.Value)
	case "repo":
		r, err := repository.Parse(opts.Value, cfg.Host())
		if err != nil {
			return &cmdutil.FlagError{Err: err}
		}
		cfg.SetDefaultRepository(r)
	default:
		return cmdutil.FlagErrorf("unsupported configuration key %q", opts.Key)
	}
	return cfg.Write()
}
