package dolthub

import (
	"context"
	"net/http"
	"testing"

	"github.com/dolthub/cli/test/httpmock"
)

func TestDatabaseEndpointsEscapePathSegments(t *testing.T) {
	reg := httpmock.New(t)
	reg.Register(http.MethodGet, "/api/v2/databases/acme%2Fwest/widgets%20plus", 200, `{"data":{"owner":"acme/west","name":"widgets plus","visibility":"private","fork_network_count":1,"star_count":2,"size_bytes":3}}`)
	reg.Register(http.MethodGet, "/api/v2/databases/acme%2Fwest/widgets%20plus/forks", 200, `{"data":[{"owner":"forker","name":"widgets"}]}`)
	client := newTestClient(t, reg)
	database, err := client.GetDatabase(context.Background(), "acme/west", "widgets plus")
	if err != nil || database.Owner != "acme/west" {
		t.Fatalf("database = %#v, error = %v", database, err)
	}
	forks, err := client.ListForks(context.Background(), "acme/west", "widgets plus")
	if err != nil || len(forks) != 1 || forks[0].Owner != "forker" {
		t.Fatalf("forks = %#v, error = %v", forks, err)
	}
}
