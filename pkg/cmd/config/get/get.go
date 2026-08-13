package get

import (
	"fmt"

	internalconfig "github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

type Options struct {
	IO     *iostreams.IOStreams
	Config func() (internalconfig.Config, error)
	Key    string
}

func NewCmdGet(f *cmdutil.Factory, runF func(*Options) error) *cobra.Command {
	opts := &Options{IO: f.IO, Config: f.Config}
	if runF == nil {
		runF = getRun
	}
	return &cobra.Command{Use: "get KEY", Short: "Get a configuration value", Args: cmdutil.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error { opts.Key = args[0]; return runF(opts) }}
}

func getRun(opts *Options) error {
	cfg, err := opts.Config()
	if err != nil {
		return err
	}
	var value string
	switch opts.Key {
	case "host":
		value = cfg.Host()
	case "repo":
		r, ok := cfg.DefaultRepository()
		if !ok {
			return &cmdutil.NoResultsError{}
		}
		value = r.FullName()
	default:
		return cmdutil.FlagErrorf("unsupported configuration key %q", opts.Key)
	}
	_, err = fmt.Fprintln(opts.IO.Out, value)
	return err
}
