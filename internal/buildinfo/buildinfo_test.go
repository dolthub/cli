package buildinfo

import (
	"strings"
	"testing"
)

func TestProvenance(t *testing.T) {
	sha := strings.Repeat("a", 40)
	for _, tt := range []struct {
		name, version, commit, dirty string
		release, ready               bool
	}{
		{"release", "1.2.3", sha, "false", true, true},
		{"ordinary", "1.2.3", sha, "false", false, false},
		{"dirty", "1.2.3", sha, "true", true, false},
		{"unknown", "dev", "", "", false, false},
		{"bad revision", "1.2.3", "short", "false", true, false},
		{"dev version", "dev", sha, "false", true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.version, tt.commit, tt.dirty, tt.release)
			if got.PublicationReady != tt.ready {
				t.Fatalf("provenance: %+v", got)
			}
			if tt.commit == "" && got.Commit != nil {
				t.Fatal("invented revision")
			}
			if tt.dirty == "" && got.Dirty != nil {
				t.Fatal("invented clean state")
			}
		})
	}
}
