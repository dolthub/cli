package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dolthub/cli/internal/repository"
)

const schemaVersion = 1

type fileData struct {
	Version           int                    `json:"version"`
	DefaultHost       string                 `json:"default_host,omitempty"`
	ActiveUsers       map[string]string      `json:"active_users,omitempty"`
	DefaultRepository *repository.Repository `json:"default_repository,omitempty"`
}

// File is a JSON-backed configuration.
type File struct {
	path string
	data fileData
}

// Path returns the platform-specific default config path.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("determine config directory: %w", err)
	}
	return filepath.Join(dir, "dh", "config.json"), nil
}

// Load reads a config path. A missing file produces an empty config.
func Load(path string) (*File, error) {
	c := &File{path: path, data: fileData{Version: schemaVersion, ActiveUsers: map[string]string{}}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	if err := json.Unmarshal(b, &c.data); err != nil {
		return nil, fmt.Errorf("malformed config %q: %w", path, err)
	}
	if c.data.Version != schemaVersion {
		return nil, fmt.Errorf("unsupported config version %d in %q", c.data.Version, path)
	}
	if c.data.ActiveUsers == nil {
		c.data.ActiveUsers = map[string]string{}
	}
	return c, nil
}

func (c *File) Host() string {
	if c.data.DefaultHost != "" {
		return c.data.DefaultHost
	}
	return DefaultHost
}
func (c *File) SetHost(host string) { c.data.DefaultHost = host }
func (c *File) ActiveUser(host string) (string, bool) {
	v, ok := c.data.ActiveUsers[host]
	return v, ok
}
func (c *File) SetActiveUser(host, user string) { c.data.ActiveUsers[host] = user }
func (c *File) UnsetActiveUser(host string)     { delete(c.data.ActiveUsers, host) }
func (c *File) DefaultRepository() (repository.Repository, bool) {
	if c.data.DefaultRepository == nil {
		return repository.Repository{}, false
	}
	return *c.data.DefaultRepository, true
}
func (c *File) SetDefaultRepository(r repository.Repository) { c.data.DefaultRepository = &r }

// Write atomically replaces the config file.
func (c *File) Write() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	b, err := json.MarshalIndent(c.data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(c.path), ".config-*")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, c.path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
