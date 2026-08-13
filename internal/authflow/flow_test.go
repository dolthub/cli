package authflow

import (
	"context"
	"strings"
	"testing"
)

func TestUnavailableIsActionable(t *testing.T) {
	_, err := (Unavailable{}).Login(context.Background(), "www.dolthub.com")
	if err == nil || !strings.Contains(err.Error(), "OAuth client registration") {
		t.Fatalf("error = %v", err)
	}
}
