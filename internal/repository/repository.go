package repository

import (
	"fmt"
	"net/url"
	"strings"
)

// Repository identifies a DoltHub repository.
type Repository struct {
	Host  string `json:"host"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

// FullName returns OWNER/REPOSITORY.
func (r Repository) FullName() string { return r.Owner + "/" + r.Name }

// Parse parses OWNER/REPOSITORY, HOST/OWNER/REPOSITORY, or a DoltHub URL.
func Parse(value, defaultHost string) (Repository, error) {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "://") {
		u, err := url.Parse(value)
		if err != nil || u.Scheme != "https" || u.RawQuery != "" || u.Fragment != "" {
			return Repository{}, fmt.Errorf("invalid repository %q", value)
		}
		parts := splitPath(u.Path)
		if len(parts) >= 3 && parts[0] == "repositories" {
			parts = parts[1:3]
		}
		if len(parts) != 2 {
			return Repository{}, fmt.Errorf("invalid repository URL %q", value)
		}
		return validate(Repository{Host: u.Hostname(), Owner: parts[0], Name: parts[1]})
	}
	parts := splitPath(value)
	switch len(parts) {
	case 2:
		return validate(Repository{Host: defaultHost, Owner: parts[0], Name: parts[1]})
	case 3:
		return validate(Repository{Host: parts[0], Owner: parts[1], Name: parts[2]})
	default:
		return Repository{}, fmt.Errorf("repository must be OWNER/REPO or HOST/OWNER/REPO")
	}
}

func splitPath(value string) []string {
	trimmed := strings.Trim(value, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func validate(r Repository) (Repository, error) {
	if r.Host == "" || r.Owner == "" || r.Name == "" || strings.ContainsAny(r.Host+r.Owner+r.Name, " \\?#") {
		return Repository{}, fmt.Errorf("invalid repository")
	}
	return r, nil
}
