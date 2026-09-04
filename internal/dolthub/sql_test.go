package dolthub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestSQLEndpoints(t *testing.T) {
	transport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			return nil, fmt.Errorf("method = %s", r.Method)
		}
		switch r.URL.EscapedPath() {
		case "/api/v2/databases/acme%2Fwest/widgets%20plus/sql":
			var request SQLReadRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request.Ref != "main" || request.Query != "select 1" || request.Limit != 10 || request.TimeoutMS != 2500 {
				t.Fatalf("request = %#v", request)
			}
			return response(r, http.StatusOK, `{"data":{"columns":[{"name":"n","type":"BIGINT"}],"rows":[["1"]],"status":"success"}}`), nil
		case "/api/v2/databases/acme%2Fwest/widgets%20plus/sql-writes":
			var request SQLWriteRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request.FromBranch != "main" || request.ToBranch != "feature/x" || request.Query != "insert into t values (1)" {
				t.Fatalf("request = %#v", request)
			}
			return response(r, http.StatusAccepted, `{"data":{"id":"job/1","href":"https://example.test/api/v2/operations/job%2F1"}}`), nil
		default:
			return nil, fmt.Errorf("path = %s", r.URL.EscapedPath())
		}
	})

	client := newTestClient(t, transport)
	read, err := client.RunSQLRead(context.Background(), "acme/west", "widgets plus", SQLReadRequest{Ref: "main", Query: "select 1", Limit: 10, TimeoutMS: 2500})
	if err != nil || read.Status != QuerySuccess || len(read.Rows) != 1 {
		t.Fatalf("read = %#v, error = %v", read, err)
	}
	write, err := client.RunSQLWrite(context.Background(), "acme/west", "widgets plus", SQLWriteRequest{FromBranch: "main", ToBranch: "feature/x", Query: "insert into t values (1)"})
	if err != nil || write.ID != "job/1" {
		t.Fatalf("write = %#v, error = %v", write, err)
	}
}
