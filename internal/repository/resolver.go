package repository

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"sort"
	"strings"
)

type Remote struct {
	Name string
	URL  string
}

type RemoteReader func(context.Context) ([]Remote, error)
type Selector func(string, []string) (int, error)

// Resolver applies the command-wide repository precedence rules lazily.
type Resolver struct {
	Host       func() string
	LookupEnv  func(string) (string, bool)
	Configured func() (Repository, bool)
	Remotes    RemoteReader
	CanPrompt  func() bool
	Select     Selector
}

func (r Resolver) Resolve(ctx context.Context, explicit string) (Repository, error) {
	host := r.Host()
	if strings.TrimSpace(explicit) != "" {
		return Parse(explicit, host)
	}
	if value, _, ok := LookupDatabaseEnv(r.LookupEnv); ok {
		return Parse(value, host)
	}
	if r.Configured != nil {
		if configured, ok := r.Configured(); ok {
			return configured, nil
		}
	}

	var candidates []Repository
	if r.Remotes != nil {
		remotes, err := r.Remotes(ctx)
		if err != nil {
			return Repository{}, fmt.Errorf("read Dolt remotes: %w", err)
		}
		candidates = repositoriesFromRemotes(remotes, host)
	}
	switch len(candidates) {
	case 1:
		return candidates[0], nil
	case 0:
		return Repository{}, errors.New("could not determine a DoltHub database; use --db, DH_DB, or 'dh config set db OWNER/DB'")
	}

	if r.CanPrompt == nil || !r.CanPrompt() || r.Select == nil {
		return Repository{}, errors.New("multiple DoltHub remotes found; use --db to select a database")
	}
	labels := make([]string, len(candidates))
	for i, candidate := range candidates {
		labels[i] = candidate.Host + "/" + candidate.FullName()
	}
	selected, err := r.Select("Select a DoltHub database", labels)
	if err != nil {
		return Repository{}, err
	}
	if selected < 0 || selected >= len(candidates) {
		return Repository{}, errors.New("repository selection is out of range")
	}
	return candidates[selected], nil
}

func repositoriesFromRemotes(remotes []Remote, configuredHost string) []Repository {
	seen := map[Repository]bool{}
	var repositories []Repository
	for _, remote := range remotes {
		repository, err := parseRemoteURL(remote.URL)
		if err != nil || !strings.EqualFold(repository.Host, configuredHost) || seen[repository] {
			continue
		}
		seen[repository] = true
		repositories = append(repositories, repository)
	}
	sort.SliceStable(repositories, func(i, j int) bool {
		return repositories[i].FullName() < repositories[j].FullName()
	})
	return repositories
}

func parseRemoteURL(raw string) (Repository, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return Repository{}, errors.New("not a recognized DoltHub remote")
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "doltremoteapi.dolthub.com":
		host = "www.dolthub.com"
	case "www.dolthub.com":
	default:
		return Repository{}, errors.New("not a recognized DoltHub remote")
	}
	parts := splitPath(u.Path)
	if len(parts) == 3 && parts[0] == "repositories" {
		parts = parts[1:]
	}
	if len(parts) != 2 {
		return Repository{}, errors.New("not a recognized DoltHub remote")
	}
	return validate(Repository{Host: host, Owner: parts[0], Name: strings.TrimSuffix(parts[1], ".git")})
}

// ReadDoltRemotes returns remotes configured in the current Dolt repository.
// A non-Dolt working directory has no remotes and is not itself an error.
func ReadDoltRemotes(ctx context.Context) ([]Remote, error) {
	output, err := exec.CommandContext(ctx, "dolt", "remote", "-v").Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) || errors.Is(err, exec.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return parseRemoteOutput(string(output)), nil
}

func parseRemoteOutput(output string) []Remote {
	var remotes []Remote
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			remotes = append(remotes, Remote{Name: fields[0], URL: fields[1]})
		}
	}
	return remotes
}
