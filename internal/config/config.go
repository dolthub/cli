package config

import "github.com/dolthub/cli/internal/repository"

const DefaultHost = "www.dolthub.com"

// Config stores non-secret CLI configuration.
type Config interface {
	Host() string
	ConfiguredHost() (string, bool)
	SetHost(string)
	ActiveUser(host string) (string, bool)
	SetActiveUser(host, user string)
	UnsetActiveUser(host string)
	DefaultRepository() (repository.Repository, bool)
	ConfiguredRepository() (repository.Repository, bool)
	SetDefaultRepository(repository.Repository)
	Write() error
}

// Environment overlays environment values on persisted configuration.
type Environment struct {
	Config
	LookupEnv func(string) (string, bool)
}

func (c Environment) Host() string {
	if value, ok := c.LookupEnv("DH_HOST"); ok && value != "" {
		return value
	}
	return c.Config.Host()
}

func (c Environment) ConfiguredHost() (string, bool) { return c.Config.ConfiguredHost() }

func (c Environment) DefaultRepository() (repository.Repository, bool) {
	if value, ok := c.LookupEnv("DH_REPO"); ok && value != "" {
		r, err := repository.Parse(value, c.Host())
		return r, err == nil
	}
	return c.Config.DefaultRepository()
}

func (c Environment) ConfiguredRepository() (repository.Repository, bool) {
	return c.Config.ConfiguredRepository()
}
