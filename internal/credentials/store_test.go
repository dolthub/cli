package credentials

import (
	"errors"
	"strings"
	"testing"
)

func TestMemoryStoreLifecycle(t *testing.T) {
	s := NewMemoryStore()
	if _, err := s.Get("h", "u"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := s.Set("h", "u", "fake-token"); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.Get("h", "u"); v != "fake-token" {
		t.Fatal("wrong token")
	}
	if err := s.Delete("h", "u"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("h", "u"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}
func TestFailuresDoNotExposeToken(t *testing.T) {
	secret := "fake-super-secret"
	b := &fakeBackend{err: errors.New("keyring unavailable")}
	s := NewKeyringStore(b)
	err := s.Set("h", "u", secret)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("token leaked")
	}
}

func TestFallbackFilePathForStoreWithoutFallback(t *testing.T) {
	if path, ok := FallbackFilePath(NewMemoryStore()); ok || path != "" {
		t.Fatalf("path = %q, ok = %v", path, ok)
	}
}

type fakeBackend struct{ err error }

func (f *fakeBackend) Get(string, string) (string, error) { return "", f.err }
func (f *fakeBackend) Set(string, string, string) error   { return f.err }
func (f *fakeBackend) Delete(string, string) error        { return f.err }
