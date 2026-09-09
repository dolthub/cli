package config

import (
	"os"
	"path/filepath"
	"runtime"
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
	// Windows enforces access through ACLs and does not expose Unix permission
	// bits through os.FileMode. Chmod and Mode().Perm() are therefore not a
	// meaningful assertion on that platform.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
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
	values := map[string]string{"DH_HOST": "env.example", "DH_DB": "new/db", "DH_REPO": "old/repo"}
	c := Environment{Config: m, LookupEnv: func(k string) (string, bool) { v, ok := values[k]; return v, ok }}
	if c.Host() != "env.example" {
		t.Fatal(c.Host())
	}
	r, ok := c.DefaultRepository()
	if !ok || r.Host != "env.example" || r.FullName() != "new/db" {
		t.Fatalf("repo=%#v", r)
	}
}

func TestEnvironmentLegacyRepositoryAlias(t *testing.T) {
	m := NewMemory()
	c := Environment{Config: m, LookupEnv: func(k string) (string, bool) {
		return "legacy/repo", k == repository.RepositoryEnvAlias
	}}
	r, ok := c.DefaultRepository()
	if !ok || r.FullName() != "legacy/repo" {
		t.Fatalf("repo=%#v", r)
	}
}
