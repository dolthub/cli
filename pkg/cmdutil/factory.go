package cmdutil

import (
	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/credentials"
	"github.com/dolthub/cli/pkg/iostreams"
)

// Factory supplies shared capabilities to commands.
//
// Add capabilities only when an implemented command needs them. Commands should
// copy the narrow dependencies they use into their own Options structures.
type Factory struct {
	AppVersion  string
	IO          *iostreams.IOStreams
	Config      func() (config.Config, error)
	Credentials credentials.Store
}
