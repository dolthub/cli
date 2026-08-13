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
func TestEnvironmentPrecedence(t *testing.T) {
	s := NewMemoryStore()
	_ = s.Set("h", "u", "stored")
	e := EnvironmentStore{Store: s, LookupEnv: func(string) (string, bool) { return "environment", true }}
	if got, _ := e.Get("h", "u"); got != "environment" {
		t.Fatal(got)
	}
	_ = e.Set("h", "u", "new")
	if got, _ := s.Get("h", "u"); got != "new" {
		t.Fatal("set did not reach secure store")
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

type fakeBackend struct{ err error }

func (f *fakeBackend) Get(string, string) (string, error) { return "", f.err }
func (f *fakeBackend) Set(string, string, string) error   { return f.err }
func (f *fakeBackend) Delete(string, string) error        { return f.err }
