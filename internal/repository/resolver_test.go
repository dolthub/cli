package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestResolverPrecedence(t *testing.T) {
	configured := Repository{Host: "www.dolthub.com", Owner: "config", Name: "db"}
	resolver := Resolver{
		Host: func() string { return "www.dolthub.com" },
		LookupEnv: func(name string) (string, bool) {
			values := map[string]string{DatabaseEnv: "env/db", RepositoryEnvAlias: "legacy/db"}
			value, ok := values[name]
			return value, ok
		},
		Configured: func() (Repository, bool) { return configured, true },
		Remotes: func(context.Context) ([]Remote, error) {
			t.Fatal("remotes should not be read")
			return nil, nil
		},
	}
	got, err := resolver.Resolve(context.Background(), "flag/db")
	if err != nil || got.Owner != "flag" {
		t.Fatalf("explicit result = %#v, %v", got, err)
	}
	got, err = resolver.Resolve(context.Background(), "")
	if err != nil || got.Owner != "env" {
		t.Fatalf("environment result = %#v, %v", got, err)
	}
	resolver.LookupEnv = nil
	got, err = resolver.Resolve(context.Background(), "")
	if err != nil || got != configured {
		t.Fatalf("configured result = %#v, %v", got, err)
	}
}

func TestResolverAcceptsLegacyRepositoryEnvironmentAlias(t *testing.T) {
	resolver := Resolver{
		Host:      func() string { return "www.dolthub.com" },
		LookupEnv: func(name string) (string, bool) { return "legacy/db", name == RepositoryEnvAlias },
	}
	got, err := resolver.Resolve(context.Background(), "")
	if err != nil || got.FullName() != "legacy/db" {
		t.Fatalf("result = %#v, %v", got, err)
	}
}

func TestResolverUsesDoltHubRemote(t *testing.T) {
	resolver := Resolver{
		Host: func() string { return "www.dolthub.com" },
		Remotes: func(context.Context) ([]Remote, error) {
			return []Remote{
				{Name: "origin", URL: "https://doltremoteapi.dolthub.com/acme/widgets"},
				{Name: "origin", URL: "https://doltremoteapi.dolthub.com/acme/widgets"},
				{Name: "other", URL: "https://example.test/acme/other-db"},
			}, nil
		},
	}
	got, err := resolver.Resolve(context.Background(), "")
	if err != nil || got != (Repository{Host: "www.dolthub.com", Owner: "acme", Name: "widgets"}) {
		t.Fatalf("result = %#v, %v", got, err)
	}
}

func TestResolverAmbiguousRemoteBehavior(t *testing.T) {
	resolver := Resolver{
		Host: func() string { return "www.dolthub.com" },
		Remotes: func(context.Context) ([]Remote, error) {
			return []Remote{
				{Name: "origin", URL: "https://doltremoteapi.dolthub.com/zeta/db"},
				{Name: "upstream", URL: "https://doltremoteapi.dolthub.com/acme/db"},
			}, nil
		},
		CanPrompt: func() bool { return false },
	}
	if _, err := resolver.Resolve(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "multiple") {
		t.Fatalf("non-interactive error = %v", err)
	}
	resolver.CanPrompt = func() bool { return true }
	resolver.Select = func(_ string, options []string) (int, error) {
		if options[0] != "www.dolthub.com/acme/db" {
			t.Fatalf("options = %v", options)
		}
		return 1, nil
	}
	got, err := resolver.Resolve(context.Background(), "")
	if err != nil || got.Owner != "zeta" {
		t.Fatalf("selected result = %#v, %v", got, err)
	}
}

func TestResolverErrors(t *testing.T) {
	resolver := Resolver{Host: func() string { return "www.dolthub.com" }, Remotes: func(context.Context) ([]Remote, error) { return nil, errors.New("boom") }}
	if _, err := resolver.Resolve(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("remote error = %v", err)
	}
	resolver.Remotes = nil
	if _, err := resolver.Resolve(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "config set db") || strings.Contains(err.Error(), "config set repo") {
		t.Fatalf("missing error = %v", err)
	}
}

func TestParseRemoteOutput(t *testing.T) {
	got := parseRemoteOutput("origin https://doltremoteapi.dolthub.com/a/b (fetch)\norigin\thttps://doltremoteapi.dolthub.com/a/b (push)\n")
	if len(got) != 2 || got[0].Name != "origin" {
		t.Fatalf("remotes = %#v", got)
	}
}
