package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/dolthub/cli/pkg/iostreams"
)

// Prompter supplies interactive confirmation.
type Prompter interface {
	Confirm(label string, defaultValue bool) (bool, error)
}

// Select prompts for one option and returns its zero-based index.
func (p System) Select(label string, options []string) (int, error) {
	if len(options) == 0 {
		return -1, fmt.Errorf("%s has no options", label)
	}
	if _, err := fmt.Fprintln(p.IO.ErrOut, label); err != nil {
		return -1, err
	}
	for i, option := range options {
		if _, err := fmt.Fprintf(p.IO.ErrOut, "  %d. %s\n", i+1, option); err != nil {
			return -1, err
		}
	}
	if _, err := fmt.Fprint(p.IO.ErrOut, "Choose an option: "); err != nil {
		return -1, err
	}
	line, err := bufio.NewReader(p.IO.In).ReadString('\n')
	if err != nil {
		return -1, err
	}
	choice, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || choice < 1 || choice > len(options) {
		return -1, errors.New("invalid selection")
	}
	return choice - 1, nil
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
