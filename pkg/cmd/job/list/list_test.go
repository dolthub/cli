package list

import (
	"context"
	"strings"
	"testing"

	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

type fakeClient struct{ items []dolthub.Operation }

func (c *fakeClient) ListOperations(context.Context, string, string, string) ([]dolthub.Operation, string, error) {
	return c.items, "", nil
}

func TestListDisplaysUUID(t *testing.T) {
	const uuid = "716a6b3f-4bd4-432e-b7ae-87bead012a3f"
	const full = "repositoryOwners/dolthub/repositories/people/jobs/" + uuid
	for _, mode := range []string{"human", "json", "jq", "template"} {
		t.Run(mode, func(t *testing.T) {
			streams, _, out, _ := iostreams.NewTest()
			client := &fakeClient{items: []dolthub.Operation{{ID: full, Status: dolthub.OperationSucceeded}}}
			f := &cmdutil.Factory{IO: streams, ResolveRepository: func(context.Context, string) (repository.Repository, error) {
				return repository.Repository{Host: "h", Owner: "dolthub", Name: "people"}, nil
			}}
			cmd := NewCmdList(f, func(ctx context.Context, o *Options) error { o.client = client; return listRun(ctx, o) })
			args := []string{}
			if mode != "human" {
				args = append(args, "--json", "id,status")
			}
			if mode == "jq" {
				args = append(args, "--jq", ".[0].id")
			}
			if mode == "template" {
				args = append(args, "--template", "{{range .}}{{.id}}{{end}}")
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), uuid) || strings.Contains(out.String(), "repositoryOwners") || client.items[0].ID != full {
				t.Fatalf("output=%q original=%q", out.String(), client.items[0].ID)
			}
		})
	}
}
