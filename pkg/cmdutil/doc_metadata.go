package cmdutil

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

const docAnnotation = "docs:metadata"

// DocArgument describes positional input, including conditional optionality.
type DocArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Optional    bool   `json:"optional,omitempty"`
	Repeated    bool   `json:"repeated,omitempty"`
}

// DocMode supplements flags whose meaning or output differs by mode.
type DocMode struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	JSONFields  []string `json:"json_fields,omitempty"`
}

// DocMetadata contains only facts that Cobra cannot infer from command flags.
// Keep it beside the owning command and share runtime constants where possible.
type DocMetadata struct {
	Arguments   []DocArgument `json:"arguments,omitempty"`
	Constraints []string      `json:"constraints,omitempty"`
	Output      string        `json:"output"`
	Modes       []DocMode     `json:"modes,omitempty"`
}

// WithDocs attaches documentation and returns cmd for constructors that return
// a command literal. JSON encoding cannot fail for this string-only structure.
func WithDocs(cmd *cobra.Command, example string, metadata DocMetadata) *cobra.Command {
	cmd.Example = example
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	data, _ := json.Marshal(metadata)
	cmd.Annotations[docAnnotation] = string(data)
	return cmd
}

func CommandDocs(cmd *cobra.Command) (DocMetadata, error) {
	var metadata DocMetadata
	value, ok := cmd.Annotations[docAnnotation]
	if !ok {
		return metadata, fmt.Errorf("%s: missing documentation metadata", cmd.CommandPath())
	}
	if err := json.Unmarshal([]byte(value), &metadata); err != nil {
		return metadata, fmt.Errorf("%s: invalid documentation metadata: %w", cmd.CommandPath(), err)
	}
	return metadata, nil
}
