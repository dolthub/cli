// Package factory constructs production command dependencies.
package factory

import (
	"os"
	"sync"

	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/credentials"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

// New constructs the production command factory from process-level inputs.
func New(appVersion string, io *iostreams.IOStreams) *cmdutil.Factory {
	var once sync.Once
	var cfg config.Config
	var cfgErr error
	f := &cmdutil.Factory{
		AppVersion:  appVersion,
		IO:          io,
		Credentials: credentials.EnvironmentStore{Store: credentials.NewSystemStore(), LookupEnv: os.LookupEnv},
	}
	f.Config = func() (config.Config, error) {
		once.Do(func() {
			path, err := config.Path()
			if err != nil {
				cfgErr = err
				return
			}
			var file *config.File
			file, cfgErr = config.Load(path)
			if cfgErr == nil {
				cfg = config.Environment{Config: file, LookupEnv: os.LookupEnv}
			}
		})
		return cfg, cfgErr
	}
	return f
}
