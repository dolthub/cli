package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dolthub/cli/internal/repository"
)

func TestFileRoundTripAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Host() != DefaultHost {
		t.Fatal("missing config did not use default host")
	}
	c.SetHost("example.com")
	c.SetActiveUser("example.com", "alice")
	c.SetDefaultRepository(repository.Repository{Host: "example.com", Owner: "o", Name: "r"})
	if err := c.Write(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Host() != "example.com" {
		t.Fatal("host not preserved")
	}
	if u, _ := got.ActiveUser("example.com"); u != "alice" {
		t.Fatal("user not preserved")
	}
}

func TestMalformedConfigFailsWithoutOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	const bad = "{bad"
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("err=%v", err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != bad {
		t.Fatal("malformed config changed")
	}
}

func TestEnvironmentPrecedence(t *testing.T) {
	m := NewMemory()
	m.DefaultHost = "stored"
	m.SetDefaultRepository(repository.Repository{Host: "stored", Owner: "old", Name: "repo"})
	values := map[string]string{"DH_HOST": "env.example", "DH_REPO": "new/repo"}
	c := Environment{Config: m, LookupEnv: func(k string) (string, bool) { v, ok := values[k]; return v, ok }}
	if c.Host() != "env.example" {
		t.Fatal(c.Host())
	}
	r, ok := c.DefaultRepository()
	if !ok || r.Host != "env.example" || r.FullName() != "new/repo" {
		t.Fatalf("repo=%#v", r)
	}
}
