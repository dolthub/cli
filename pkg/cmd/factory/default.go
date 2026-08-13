// Package factory constructs production command dependencies.
package factory

import (
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

// New constructs the production command factory from process-level inputs.
func New(appVersion string, io *iostreams.IOStreams) *cmdutil.Factory {
	return &cmdutil.Factory{
		AppVersion: appVersion,
		IO:         io,
	}
}
