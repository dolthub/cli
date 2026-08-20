package credentials

import (
	"errors"
	"fmt"
	"sync"
)

// FallbackStore prefers a keyring and falls back to a user-only file.
type FallbackStore struct {
	keyring Store
	file    *FileStore
	mu      sync.Mutex
}

func NewFallbackStore(keyring Store, file *FileStore) *FallbackStore {
	return &FallbackStore{keyring: keyring, file: file}
}

func (s *FallbackStore) FilePath() string { return s.file.Path() }

func (s *FallbackStore) Get(host, user string) (string, error) {
	stored, err := s.GetStored(host, user)
	return stored.Secret, err
}

func (s *FallbackStore) GetStored(host, user string) (Stored, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getStored(host, user)
}

func (s *FallbackStore) getStored(host, user string) (Stored, error) {
	secret, fileErr := s.file.Get(host, user)
	if fileErr == nil {
		return Stored{Secret: secret, Source: SourceFile}, nil
	}
	if !errors.Is(fileErr, ErrNotFound) {
		return Stored{}, fileErr
	}
	secret, keyringErr := s.keyring.Get(host, user)
	if keyringErr != nil {
		return Stored{}, keyringErr
	}
	return Stored{Secret: secret, Source: SourceKeyring}, nil
}

func (s *FallbackStore) Set(host, user, secret string) error {
	_, err := s.SetPreferred(host, user, secret)
	return err
}

func (s *FallbackStore) SetPreferred(host, user, secret string) (Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, fileErr := s.file.Get(host, user)
	hadFile := fileErr == nil
	if fileErr != nil && !errors.Is(fileErr, ErrNotFound) {
		keyringErr := s.keyring.Set(host, user, secret)
		if keyringErr == nil {
			return "", fmt.Errorf("store credential securely but file fallback is unreadable: %w", fileErr)
		}
		return "", fmt.Errorf("store credential: keyring unavailable; file fallback failed: %w", fileErr)
	}
	if hadFile {
		if err := s.file.Set(host, user, secret); err != nil {
			return "", err
		}
	}
	keyringErr := s.keyring.Set(host, user, secret)
	if keyringErr == nil {
		if !hadFile {
			return SourceKeyring, nil
		}
		if err := s.file.Delete(host, user); err == nil || errors.Is(err, ErrNotFound) {
			return SourceKeyring, nil
		}
		return SourceFile, nil
	}
	if hadFile {
		return SourceFile, nil
	}
	if fileErr := s.file.Set(host, user, secret); fileErr != nil {
		return "", fmt.Errorf("store credential: keyring unavailable; file fallback failed: %w", fileErr)
	}
	return SourceFile, nil
}

func (s *FallbackStore) SetAt(source Source, host, user, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch source {
	case SourceFile:
		return s.file.Set(host, user, secret)
	case SourceKeyring:
		return s.keyring.Set(host, user, secret)
	default:
		return errors.New("credential source is invalid")
	}
}

func (s *FallbackStore) Delete(host, user string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	keyringErr := s.keyring.Delete(host, user)
	fileErr := s.file.Delete(host, user)
	if errors.Is(keyringErr, ErrNotFound) {
		keyringErr = nil
	}
	if errors.Is(fileErr, ErrNotFound) {
		fileErr = nil
	}
	return errors.Join(keyringErr, fileErr)
}
