package importcmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

func TestImportFlow(t *testing.T) {
	for _, mode := range []string{"wait", "no-wait", "failed", "upload-failed"} {
		t.Run(mode, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "people.csv")
			if err := os.WriteFile(file, []byte("id,name\n1,Alice\n"), 0600); err != nil {
				t.Fatal(err)
			}
			calls := []string{}
			var server *httptest.Server
			server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.URL.Path)
				switch r.URL.Path {
				case "/api/v2/databases/owner/db/imports/uploads":
					var body dolthub.CreateImportUploadRequest
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body.ContentLength != 16 || body.NumParts != 1 || body.FileType != dolthub.ImportCSV {
						t.Errorf("upload request %#v", body)
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": dolthub.ImportUpload{Token: "token", ContentsKey: "key", HTTPMethod: "PUT", Parts: []dolthub.ImportUploadPart{{PartNumber: 1, URL: server.URL + "/storage"}}}})
				case "/storage":
					if r.Header.Get("Authorization") != "" {
						t.Error("credentials sent to storage")
					}
					data, _ := io.ReadAll(r.Body)
					if string(data) != "id,name\n1,Alice\n" {
						t.Error("wrong file contents")
					}
					if mode == "upload-failed" {
						w.WriteHeader(403)
						return
					}
					w.Header().Set("ETag", `"etag"`)
				case "/api/v2/databases/owner/db/imports":
					var body dolthub.CreateImportRequest
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body.Token != "token" || body.ContentsKey != "key" || body.BranchName != "main" || body.TableName != "people" || body.FileName != "people.csv" || body.PrimaryKeys == nil || len(body.PrimaryKeys) != 1 || body.PrimaryKeys[0] != "id" || body.ImportOperation != dolthub.ImportCreate || body.CommitMessage == nil || *body.CommitMessage != "Import people" || body.FilePartsMD5 == "" || len(body.CompletedParts) != 1 || body.CompletedParts[0].ETag != `"etag"` {
						t.Errorf("import request %#v", body)
					}
					w.WriteHeader(202)
					_ = json.NewEncoder(w).Encode(map[string]any{"data": dolthub.OperationRef{ID: "repositoryOwners/dolthub/repositories/people/jobs/716a6b3f-4bd4-432e-b7ae-87bead012a3f", Href: server.URL + "/api/v2/operations/job"}})
				case "/api/v2/operations/job":
					status := dolthub.OperationSucceeded
					if mode == "failed" {
						status = dolthub.OperationFailed
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": dolthub.Operation{ID: "repositoryOwners/dolthub/repositories/people/jobs/716a6b3f-4bd4-432e-b7ae-87bead012a3f", Type: dolthub.OperationImport, Status: status}})
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			old := http.DefaultTransport
			http.DefaultTransport = server.Client().Transport
			defer func() { http.DefaultTransport = old }()
			base, _ := url.Parse(server.URL + "/api/v2/")
			api, err := dolthub.NewClient(server.Client(), base)
			if err != nil {
				t.Fatal(err)
			}
			streams, _, out, _ := iostreams.NewTest()
			f := &cmdutil.Factory{IO: streams, ResolveRepository: func(context.Context, string) (repository.Repository, error) {
				return repository.Repository{Host: "host", Owner: "owner", Name: "db"}, nil
			}, APIClientForHost: func(string) (*dolthub.Client, error) { return api, nil }}
			cmd := NewCmdImport(f, nil)
			args := []string{"people", file, "--branch", "main", "--primary-key", "id", "--message", "Import people", "--json", "id"}
			if mode == "no-wait" {
				args = append(args, "--no-wait")
			}
			cmd.SetArgs(args)
			err = cmd.Execute()
			if mode == "failed" || mode == "upload-failed" {
				if err == nil {
					t.Fatal("expected error")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			expected := 4
			if mode == "no-wait" {
				expected = 3
			}
			if mode == "upload-failed" {
				expected = 2
			}
			if len(calls) != expected {
				t.Fatalf("calls %v", calls)
			}
			if mode != "upload-failed" && !strings.Contains(out.String(), `"id":"716a6b3f-4bd4-432e-b7ae-87bead012a3f"`) {
				t.Errorf("output %s", out.String())
			}
		})
	}
}

func TestValidationBeforeNetwork(t *testing.T) {
	for _, args := range [][]string{
		{"people", "data.csv"},
		{"people", "data.csv", "--branch", "main", "--update", "--replace"},
		{"people", "data.json", "--branch", "main"},
		{"people", "data.sql", "--branch", "main"},
		{"people", "data.csv", "--branch", "main", "--primary-key", ","},
		{"people"},
	} {
		streams, _, _, _ := iostreams.NewTest()
		c := NewCmdImport(&cmdutil.Factory{IO: streams}, nil)
		c.SetArgs(args)
		if err := c.Execute(); err == nil {
			t.Fatalf("expected error for %v", args)
		}
	}
	for _, kind := range []string{"empty", "directory"} {
		path := t.TempDir()
		if kind == "empty" {
			path = filepath.Join(path, "empty.csv")
			if err := os.WriteFile(path, nil, 0600); err != nil {
				t.Fatal(err)
			}
		}
		streams, _, _, _ := iostreams.NewTest()
		c := NewCmdImport(&cmdutil.Factory{IO: streams}, nil)
		c.SetArgs([]string{"people", path, "--branch", "main", "--file-type", "csv"})
		if err := c.Execute(); err == nil {
			t.Fatalf("expected error for %s", kind)
		}
	}
}
