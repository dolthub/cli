package config

import "github.com/dolthub/cli/internal/repository"

// Memory is an in-memory configuration for tests.
type Memory struct {
	DefaultHost string
	Users       map[string]string
	Repo        *repository.Repository
	WriteErr    error
	Writes      int
}

func NewMemory() *Memory { return &Memory{Users: map[string]string{}} }
func (c *Memory) Host() string {
	if c.DefaultHost != "" {
		return c.DefaultHost
	}
	return DefaultHost
}
func (c *Memory) SetHost(v string)                   { c.DefaultHost = v }
func (c *Memory) ActiveUser(h string) (string, bool) { v, ok := c.Users[h]; return v, ok }
func (c *Memory) SetActiveUser(h, u string)          { c.Users[h] = u }
func (c *Memory) UnsetActiveUser(h string)           { delete(c.Users, h) }
func (c *Memory) DefaultRepository() (repository.Repository, bool) {
	if c.Repo == nil {
		return repository.Repository{}, false
	}
	return *c.Repo, true
}
func (c *Memory) SetDefaultRepository(r repository.Repository) { c.Repo = &r }
func (c *Memory) Write() error                                 { c.Writes++; return c.WriteErr }
