package credentials

import (
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("credential not found")

type Source string

const (
	SourceKeyring Source = "keyring"
	SourceFile    Source = "file"
)

type Stored struct {
	Secret string
	Source Source
}

// Store persists authentication tokens outside ordinary configuration.
type Store interface {
	Get(host, user string) (string, error)
	Set(host, user, token string) error
	Delete(host, user string) error
}

// SourceStore preserves credential provenance for refresh and rollback.
type SourceStore interface {
	Store
	GetStored(host, user string) (Stored, error)
	SetPreferred(host, user, secret string) (Source, error)
	SetAt(source Source, host, user, secret string) error
}

func GetStored(store Store, host, user string) (Stored, error) {
	if sourced, ok := store.(SourceStore); ok {
		return sourced.GetStored(host, user)
	}
	secret, err := store.Get(host, user)
	return Stored{Secret: secret, Source: SourceKeyring}, err
}

func SetPreferred(store Store, host, user, secret string) (Source, error) {
	if sourced, ok := store.(SourceStore); ok {
		return sourced.SetPreferred(host, user, secret)
	}
	return SourceKeyring, store.Set(host, user, secret)
}

func SetAt(store Store, source Source, host, user, secret string) error {
	if sourced, ok := store.(SourceStore); ok {
		return sourced.SetAt(source, host, user, secret)
	}
	return store.Set(host, user, secret)
}

func FallbackFilePath(store Store) (string, bool) {
	type filePather interface{ FilePath() string }
	pather, ok := store.(filePather)
	if !ok {
		return "", false
	}
	path := pather.FilePath()
	return path, path != ""
}

// Backend is the narrow secure-storage capability used by KeyringStore.
type Backend interface {
	Get(service, account string) (string, error)
	Set(service, account, secret string) error
	Delete(service, account string) error
}

// KeyringStore stores tokens in an operating-system credential backend.
type KeyringStore struct{ backend Backend }

func NewKeyringStore(backend Backend) *KeyringStore { return &KeyringStore{backend: backend} }
func account(host, user string) string              { return host + ":" + user }
func (s *KeyringStore) Get(host, user string) (string, error) {
	return s.backend.Get("dh", account(host, user))
}
func (s *KeyringStore) Set(host, user, token string) error {
	if token == "" {
		return errors.New("refusing to store an empty token")
	}
	if err := s.backend.Set("dh", account(host, user), token); err != nil {
		return fmt.Errorf("store credential securely: %w", err)
	}
	return nil
}
func (s *KeyringStore) Delete(host, user string) error {
	return s.backend.Delete("dh", account(host, user))
}

// EnvironmentStore gives DH_TOKEN precedence without persisting or deleting it.
type EnvironmentStore struct {
	Store
	LookupEnv func(string) (string, bool)
}

func (s EnvironmentStore) Get(host, user string) (string, error) {
	if token, ok := s.LookupEnv("DH_TOKEN"); ok && token != "" {
		return token, nil
	}
	return s.Store.Get(host, user)
}

// MemoryStore is an in-memory credential store for tests.
type MemoryStore struct {
	Values map[string]string
	Err    error
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{Values: map[string]string{}} }
func (s *MemoryStore) Get(host, user string) (string, error) {
	if s.Err != nil {
		return "", s.Err
	}
	v, ok := s.Values[account(host, user)]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}
func (s *MemoryStore) Set(host, user, token string) error {
	if s.Err != nil {
		return s.Err
	}
	s.Values[account(host, user)] = token
	return nil
}
func (s *MemoryStore) Delete(host, user string) error {
	if s.Err != nil {
		return s.Err
	}
	delete(s.Values, account(host, user))
	return nil
}
