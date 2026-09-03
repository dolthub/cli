package view

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/credentials"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/tableprinter"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

var jsonFields = []string{"cancelable", "created_at", "error", "id", "result", "status", "type"}

type apiClient interface {
	GetOperation(context.Context, string) (dolthub.Operation, error)
}

type Options struct {
	IO               *iostreams.IOStreams
	Config           func() (config.Config, error)
	Credentials      credentials.Store
	LookupEnv        func(string) (string, bool)
	APIClientForHost func(string) (*dolthub.Client, error)
	ID               string
	Exporter         cmdutil.Exporter
	client           apiClient
}

func NewCmdView(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	opts := &Options{IO: f.IO, Config: f.Config, Credentials: f.Credentials, LookupEnv: f.LookupEnv, APIClientForHost: f.APIClientForHost}
	if runF == nil {
		runF = viewRun
	}
	cmd := &cobra.Command{
		Use:   "view ID",
		Short: "View an asynchronous operation",
		Args:  cmdutil.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if strings.TrimSpace(opts.ID) == "" {
				return cmdutil.FlagErrorf("operation ID must not be empty")
			}
			return runF(cmd.Context(), opts)
		},
	}
	cmdutil.AddJSONFlags(cmd, &opts.Exporter, jsonFields)
	return cmd
}

func viewRun(ctx context.Context, opts *Options) error {
	cfg, err := opts.Config()
	if err != nil {
		return err
	}
	host := cfg.Host()
	if err := requireAuthentication(cfg, opts.Credentials, opts.LookupEnv, host); err != nil {
		return err
	}
	client := opts.client
	if client == nil {
		client, err = opts.APIClientForHost(host)
		if err != nil {
			return err
		}
	}
	operation, err := client.GetOperation(ctx, opts.ID)
	if err != nil {
		return err
	}
	if opts.Exporter != nil {
		return opts.Exporter.Write(opts.IO, operation)
	}
	return renderHuman(opts.IO, operation)
}

func requireAuthentication(cfg config.Config, store credentials.Store, lookup func(string) (string, bool), host string) error {
	if lookup != nil {
		if token, ok := lookup("DH_TOKEN"); ok && strings.TrimSpace(token) != "" {
			return nil
		}
	}
	user, ok := cfg.ActiveUser(host)
	if !ok || store == nil {
		return &cmdutil.AuthError{Err: fmt.Errorf("log in to %s with dh auth login", host)}
	}
	_, err := credentials.GetStoredOAuthToken(store, host, user)
	if errors.Is(err, credentials.ErrNotFound) {
		return &cmdutil.AuthError{Err: fmt.Errorf("log in to %s with dh auth login", host)}
	}
	return err
}

func renderHuman(streams *iostreams.IOStreams, operation dolthub.Operation) error {
	table := tableprinter.New(streams, "FIELD", "VALUE")
	rows := [][2]string{
		{"ID", operation.ID},
		{"Type", string(operation.Type)},
		{"Status", string(operation.Status)},
		{"Created", operation.CreatedAt.Format("2006-01-02T15:04:05Z07:00")},
		{"Cancelable", fmt.Sprint(operation.Cancelable)},
		{"Error", "-"},
		{"Result", rawJSON(operation.Result)},
	}
	if operation.Error != nil {
		encoded, _ := json.Marshal(operation.Error)
		rows[5][1] = string(encoded)
	}
	for _, row := range rows {
		_ = table.AddRow(row[0], row[1])
	}
	return table.Render()
}

func rawJSON(value json.RawMessage) string {
	if len(value) == 0 || string(value) == "null" {
		return "-"
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, value); err != nil {
		return string(value)
	}
	return compact.String()
}
