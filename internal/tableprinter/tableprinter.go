package tableprinter

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/dolthub/cli/pkg/iostreams"
)

// Table renders headers and aligned columns to terminals, and stable
// tab-separated rows to non-terminal output.
type Table struct {
	streams *iostreams.IOStreams
	headers []string
	rows    [][]string
}

func New(streams *iostreams.IOStreams, headers ...string) *Table {
	return &Table{streams: streams, headers: append([]string(nil), headers...)}
}

func (t *Table) AddRow(fields ...string) error {
	if len(fields) != len(t.headers) {
		return fmt.Errorf("table row has %d fields, want %d", len(fields), len(t.headers))
	}
	t.rows = append(t.rows, append([]string(nil), fields...))
	return nil
}

func (t *Table) Render() error {
	if !t.streams.IsStdoutTTY() {
		for _, row := range t.rows {
			if _, err := fmt.Fprintln(t.streams.Out, strings.Join(row, "\t")); err != nil {
				return err
			}
		}
		return nil
	}
	writer := tabwriter.NewWriter(t.streams.Out, 0, 4, 2, ' ', 0)
	headers := make([]string, len(t.headers))
	for i, header := range t.headers {
		headers[i] = strings.ToUpper(header)
	}
	if _, err := fmt.Fprintln(writer, strings.Join(headers, "\t")); err != nil {
		return err
	}
	for _, row := range t.rows {
		if _, err := fmt.Fprintln(writer, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return writer.Flush()
}
