package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

const fileSchemaVersion = 1

type fileData struct {
	Version     int                          `json:"version"`
	Credentials map[string]map[string]string `json:"credentials,omitempty"`
}

// FileStore stores versioned credential bundles in a user-only JSON file.
type FileStore struct {
	path string
	err  error
	mu   sync.Mutex
}

func NewFileStore(path string) *FileStore          { return &FileStore{path: path} }
func NewUnavailableFileStore(err error) *FileStore { return &FileStore{err: err} }
func (s *FileStore) Path() string                  { return s.path }

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("determine credential directory: %w", err)
	}
	return filepath.Join(dir, "dh", "credentials.json"), nil
}

func (s *FileStore) Get(host, user string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return "", err
	}
	users, ok := data.Credentials[host]
	if !ok {
		return "", ErrNotFound
	}
	secret, ok := users[user]
	if !ok {
		return "", ErrNotFound
	}
	return secret, nil
}

func (s *FileStore) Set(host, user, secret string) error {
	if secret == "" {
		return errors.New("refusing to store an empty credential")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if data.Credentials == nil {
		data = fileData{Version: fileSchemaVersion, Credentials: map[string]map[string]string{}}
	}
	if data.Credentials[host] == nil {
		data.Credentials[host] = map[string]string{}
	}
	data.Credentials[host][user] = secret
	return s.write(data)
}

func (s *FileStore) Delete(host, user string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return err
	}
	users, ok := data.Credentials[host]
	if !ok {
		return ErrNotFound
	}
	if _, ok := users[user]; !ok {
		return ErrNotFound
	}
	delete(users, user)
	if len(users) == 0 {
		delete(data.Credentials, host)
	}
	return s.write(data)
}

func (s *FileStore) read() (fileData, error) {
	if s.err != nil {
		return fileData{}, s.err
	}
	info, err := os.Stat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return fileData{}, ErrNotFound
	}
	if err != nil {
		return fileData{}, fmt.Errorf("inspect credentials file %q: %w", s.path, err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return fileData{}, fmt.Errorf("credentials file %q has unsafe permissions", s.path)
	}
	b, err := os.ReadFile(s.path)
	if err != nil {
		return fileData{}, fmt.Errorf("read credentials file %q: %w", s.path, err)
	}
	var data fileData
	if err := json.Unmarshal(b, &data); err != nil {
		return fileData{}, fmt.Errorf("malformed credentials file %q", s.path)
	}
	if data.Version != fileSchemaVersion {
		return fileData{}, fmt.Errorf("unsupported credentials file version %d in %q", data.Version, s.path)
	}
	if data.Credentials == nil {
		data.Credentials = map[string]map[string]string{}
	}
	return data, nil
}

func (s *FileStore) write(data fileData) error {
	if s.err != nil {
		return s.err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create credential directory: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("secure credential directory: %w", err)
		}
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return errors.New("encode credentials file")
	}
	tmp, err := os.CreateTemp(dir, ".credentials-*")
	if err != nil {
		return fmt.Errorf("create temporary credentials file: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("secure temporary credentials file: %w", err)
	}
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return errors.New("write credentials file")
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return errors.New("sync credentials file")
	}
	if err := tmp.Close(); err != nil {
		return errors.New("close credentials file")
	}
	if err := os.Rename(name, s.path); err != nil {
		return fmt.Errorf("replace credentials file: %w", err)
	}
	return nil
}
