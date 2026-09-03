package api

import (
	"context"
	"reflect"
	"testing"

	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

func TestRequestBodyAndMethodInputs(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	body, err := requestBody(&Options{IO: streams, RawFields: []string{"name=42"}, TypedFields: []string{"count=42", "enabled=true", "nothing=null"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(body); got != `{"count":42,"enabled":true,"name":"42","nothing":null}` {
		t.Fatalf("body = %s", got)
	}
}

func TestPaginationHelpers(t *testing.T) {
	token, err := nextToken([]byte(`{"data":[],"meta":{"next_page_token":"opaque/+=="}}`))
	if err != nil || token != "opaque/+==" {
		t.Fatalf("token=%q err=%v", token, err)
	}
	if got := setPageToken("databases/o/r/tags?x=1", token); got != "databases/o/r/tags?page_token=opaque%2F%2B%3D%3D&x=1" {
		t.Fatalf("endpoint=%q", got)
	}
}

func TestCommandFlags(t *testing.T) {
	var got *Options
	c := NewCmdAPI(nilFactory(), func(_ context.Context, o *Options) error { got = o; return nil })
	c.SetArgs([]string{"databases/o/r/branches", "--paginate", "--slurp", "-F", "n=1"})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if got.Endpoint != "databases/o/r/branches" || !got.Paginate || !got.Slurp || !reflect.DeepEqual(got.TypedFields, []string{"n=1"}) {
		t.Fatalf("options=%#v", got)
	}
}

func nilFactory() *cmdutil.Factory {
	streams, _, _, _ := iostreams.NewTest()
	return &cmdutil.Factory{IO: streams}
}
