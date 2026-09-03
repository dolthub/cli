package cmdutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/cli/go-gh/v2/pkg/jq"
	ghtemplate "github.com/cli/go-gh/v2/pkg/template"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type Exporter interface {
	Fields() []string
	Write(*iostreams.IOStreams, any) error
}

// AddJSONFlags adds gh-compatible structured output flags and validates the
// requested fields before command execution.
func AddJSONFlags(cmd *cobra.Command, target *Exporter, fields []string) {
	cmd.Flags().StringSlice("json", nil, "Output JSON with the specified `fields`")
	cmd.Flags().String("jq", "", "Filter JSON output using a jq `expression`")
	cmd.Flags().String("template", "", "Format JSON output using a Go template")

	allowed := append([]string(nil), fields...)
	sort.Strings(allowed)
	oldPreRun := cmd.PreRunE
	cmd.PreRunE = func(command *cobra.Command, args []string) error {
		if oldPreRun != nil {
			if err := oldPreRun(command, args); err != nil {
				return err
			}
		}
		exporter, err := exporterFromFlags(command.Flags(), allowed)
		if err != nil {
			return &FlagError{Err: err}
		}
		*target = exporter
		return nil
	}
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["help:json-fields"] = strings.Join(allowed, ",")
}

func exporterFromFlags(flags *pflag.FlagSet, allowed []string) (Exporter, error) {
	jsonFlag := flags.Lookup("json")
	jqFlag := flags.Lookup("jq")
	templateFlag := flags.Lookup("template")
	if !jsonFlag.Changed {
		if jqFlag.Changed {
			return nil, errors.New("cannot use --jq without specifying --json")
		}
		if templateFlag.Changed {
			return nil, errors.New("cannot use --template without specifying --json")
		}
		return nil, nil
	}
	fields := jsonFlag.Value.(pflag.SliceValue).GetSlice()
	allowedSet := make(map[string]bool, len(allowed))
	for _, field := range allowed {
		allowedSet[field] = true
	}
	for _, field := range fields {
		if !allowedSet[field] {
			return nil, fmt.Errorf("unknown JSON field %q; available fields: %s", field, strings.Join(allowed, ", "))
		}
	}
	return &jsonExporter{fields: fields, filter: jqFlag.Value.String(), template: templateFlag.Value.String()}, nil
}

type jsonExporter struct {
	fields   []string
	filter   string
	template string
}

func (e *jsonExporter) Fields() []string { return append([]string(nil), e.fields...) }

func (e *jsonExporter) Write(streams *iostreams.IOStreams, data any) error {
	selected, err := selectFields(data, e.fields)
	if err != nil {
		return err
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(selected); err != nil {
		return err
	}
	if e.filter != "" {
		indent := ""
		if streams.IsStdoutTTY() {
			indent = "  "
		}
		return jq.EvaluateFormatted(&buffer, streams.Out, e.filter, indent, false)
	}
	if e.template != "" {
		template := ghtemplate.New(streams.Out, 80, false)
		if err := template.Parse(e.template); err != nil {
			return err
		}
		if err := template.Execute(&buffer); err != nil {
			return err
		}
		return template.Flush()
	}
	_, err = io.Copy(streams.Out, &buffer)
	return err
}

func selectFields(data any, fields []string) (any, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(encoded, &value); err != nil {
		return nil, err
	}
	return projectValue(reflect.ValueOf(value), fields), nil
}

func projectValue(value reflect.Value, fields []string) any {
	if !value.IsValid() {
		return nil
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return nil
		}
		return projectValue(value.Elem(), fields)
	case reflect.Slice:
		result := make([]any, value.Len())
		for i := range result {
			result[i] = projectValue(value.Index(i), fields)
		}
		return result
	case reflect.Map:
		result := make(map[string]any, len(fields))
		for _, field := range fields {
			item := value.MapIndex(reflect.ValueOf(field))
			if item.IsValid() {
				result[field] = item.Interface()
			}
		}
		return result
	default:
		return value.Interface()
	}
}
