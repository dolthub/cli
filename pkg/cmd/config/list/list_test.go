package list

import (
	"strings"
	"testing"

	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/pkg/iostreams"
)

func TestListSourcesAndNonTTYOutput(t *testing.T) {
	cfg := config.NewMemory()
	cfg.DefaultHost = "stored.example"
	cfg.SetDefaultRepository(repository.Repository{Host: "stored.example", Owner: "old", Name: "repo"})
	streams, _, out, _ := iostreams.NewTest()
	opts := &Options{
		IO:     streams,
		Config: func() (config.Config, error) { return cfg, nil },
		LookupEnv: func(key string) (string, bool) {
			values := map[string]string{"DH_HOST": "env.example", "DH_REPO": "new/repo"}
			value, ok := values[key]
			return value, ok
		},
	}
	if err := listRun(opts); err != nil {
		t.Fatal(err)
	}
	want := "host\tenv.example\tenvironment\nrepo\tnew/repo\tenvironment\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}

func TestListDefaultAndUnset(t *testing.T) {
	streams, _, out, _ := iostreams.NewTest()
	opts := &Options{IO: streams, Config: func() (config.Config, error) { return config.NewMemory(), nil }}
	if err := listRun(opts); err != nil {
		t.Fatal(err)
	}
	want := "host\twww.dolthub.com\tdefault\nrepo\t\tunset\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}

func TestListConfiguredAndTTYOutput(t *testing.T) {
	cfg := config.NewMemory()
	cfg.DefaultHost = config.DefaultHost
	cfg.SetDefaultRepository(repository.Repository{Host: config.DefaultHost, Owner: "o", Name: "r"})
	streams, _, out, _ := iostreams.NewTest()
	streams.SetStdoutTTY(true)
	if err := listRun(&Options{IO: streams, Config: func() (config.Config, error) { return cfg, nil }}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"KEY", "VALUE", "SOURCE", "host", "config", "repo", "o/r"} {
		if !strings.Contains(out.String(), value) {
			t.Fatalf("output %q does not contain %q", out.String(), value)
		}
	}
}

func TestListRejectsInvalidEnvironmentRepository(t *testing.T) {
	cfg := config.NewMemory()
	cfg.SetDefaultRepository(repository.Repository{Host: config.DefaultHost, Owner: "fallback", Name: "repo"})
	opts := &Options{
		IO:     func() *iostreams.IOStreams { streams, _, _, _ := iostreams.NewTest(); return streams }(),
		Config: func() (config.Config, error) { return cfg, nil },
		LookupEnv: func(key string) (string, bool) {
			return "not-a-repository", key == "DH_REPO"
		},
	}
	if err := listRun(opts); err == nil || !strings.Contains(err.Error(), "invalid DH_REPO") {
		t.Fatalf("error = %v", err)
	}
}
