package dolthub

import (
	"context"
	"github.com/dolthub/cli/test/httpmock"
	"net/http"
	"testing"
)

func TestImportEndpoints(t *testing.T) {
	reg := httpmock.New(t)
	reg.Register(http.MethodPost, "/api/v2/databases/acme%2Fwest/data%20set/imports/uploads", 200, `{"data":{"token":"token","contents_key":"key","parts":[{"part_number":1,"url":"https://storage.example"}],"http_method":"PUT","headers":{}}}`)
	reg.Register(http.MethodPost, "/api/v2/databases/acme%2Fwest/data%20set/imports", 202, `{"data":{"id":"job","href":"https://example.com/api/v2/operations/job"}}`)
	c := newTestClient(t, reg)
	s, err := c.CreateImportUpload(context.Background(), "acme/west", "data set", CreateImportUploadRequest{ContentLength: 1, NumParts: 1, FileType: ImportCSV})
	if err != nil || s.Token != "token" || len(s.Parts) != 1 {
		t.Fatalf("session %#v: %v", s, err)
	}
	ref, err := c.CreateImport(context.Background(), "acme/west", "data set", CreateImportRequest{Token: s.Token})
	if err != nil || ref.ID != "job" {
		t.Fatalf("ref %#v: %v", ref, err)
	}
}
