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
	cmd := &cobra.Command{Use: "set KEY VALUE", Short: "Set a configuration value", Args: cmdutil.ExactArgs(2), RunE: func(_ *cobra.Command, args []string) error {
		opts.Key = args[0]
		opts.Value = args[1]
		return runF(opts)
	}}
	return cmdutil.WithDocs(cmd, "dh config set db OWNER/DATABASE\ndh config set host www.dolthub.com", cmdutil.DocMetadata{
		Arguments:   []cmdutil.DocArgument{{Name: "KEY", Description: "Supported configuration key: host or db; repo remains accepted as an alias for db.", Optional: false}, {Name: "VALUE", Description: "Hostname for host, or [HOST/]OWNER/DB for db.", Optional: false}},
		Constraints: []string{"Values must not be empty. Config is saved under the platform user config directory in dh/config.json."},
		Output:      "Saves the configuration without printing a success message. DH_HOST and DH_DB overrides still take precedence.",
	})
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
	case "db", "repo":
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
