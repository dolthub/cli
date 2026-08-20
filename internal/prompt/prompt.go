package prompt

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/dolthub/cli/pkg/iostreams"
)

// Prompter supplies interactive confirmation.
type Prompter interface {
	Confirm(label string, defaultValue bool) (bool, error)
}

type System struct{ IO *iostreams.IOStreams }

func (p System) Confirm(label string, defaultValue bool) (bool, error) {
	suffix := "[y/N]"
	if defaultValue {
		suffix = "[Y/n]"
	}
	if _, err := fmt.Fprintf(p.IO.ErrOut, "%s %s ", label, suffix); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(p.IO.In).ReadString('\n')
	if err != nil {
		return false, err
	}
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return defaultValue, nil
	}
	return line == "y" || line == "yes", nil
}
