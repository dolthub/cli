package cmdutil

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dolthub/cli/pkg/iostreams"
)

// PromptLine reads one line of interactive input after writing a prompt.
func PromptLine(streams *iostreams.IOStreams, label string) (string, error) {
	if _, err := fmt.Fprint(streams.ErrOut, label+": "); err != nil {
		return "", err
	}
	value, err := bufio.NewReader(streams.In).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

// ReadTextSource resolves mutually exclusive inline and file text inputs.
func ReadTextSource(inline, filename string) (string, error) {
	if inline != "" && filename != "" {
		return "", FlagErrorf("text and file inputs are mutually exclusive")
	}
	if filename == "" {
		return inline, nil
	}
	payload, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", filename, err)
	}
	return string(payload), nil
}
