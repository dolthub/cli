package repository

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		value string
		want  Repository
	}{
		{"owner/repo", Repository{"www.dolthub.com", "owner", "repo"}},
		{"example.com/owner/repo", Repository{"example.com", "owner", "repo"}},
		{"https://www.dolthub.com/repositories/owner/repo", Repository{"www.dolthub.com", "owner", "repo"}},
	}
	for _, tt := range tests {
		got, err := Parse(tt.value, "www.dolthub.com")
		if err != nil || got != tt.want {
			t.Errorf("Parse(%q) = %#v, %v; want %#v", tt.value, got, err, tt.want)
		}
	}
}

func TestParseRejectsMalformed(t *testing.T) {
	for _, value := range []string{"", "owner", "a/b/c/d", "http://www.dolthub.com/repositories/a/b", "a /b"} {
		if _, err := Parse(value, "www.dolthub.com"); err == nil {
			t.Errorf("Parse(%q) unexpectedly succeeded", value)
		}
	}
}
