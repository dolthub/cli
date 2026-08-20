package credentials

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFileStoreLifecycleAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dh", "credentials.json")
	store := NewFileStore(path)
	secret := "dh.oauth.v1:{fake-secret}"
	if err := store.Set("example.com", "alice", secret); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("example.com", "alice")
	if err != nil || got != secret {
		t.Fatalf("credential = %q, error = %v", got, err)
	}
	if runtime.GOOS != "windows" {
		dirInfo, err := os.Stat(filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		fileInfo, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := dirInfo.Mode().Perm(); got != 0o700 {
			t.Fatalf("directory permissions = %o", got)
		}
		if got := fileInfo.Mode().Perm(); got != 0o600 {
			t.Fatalf("file permissions = %o", got)
		}
	}
	if err := store.Delete("example.com", "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("example.com", "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestFileStoreRejectsMalformedFileWithoutOverwriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	original := []byte(`{"version":1,"credentials":`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewFileStore(path)
	if err := store.Set("example.com", "alice", "secret"); err == nil || !strings.Contains(err.Error(), "malformed credentials file") {
		t.Fatalf("error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(original) {
		t.Fatalf("file was overwritten: %q, error = %v", got, err)
	}
}

func TestFileStoreRejectsUnsafePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions do not apply")
	}
	path := filepath.Join(t.TempDir(), "credentials.json")
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := NewFileStore(path).Get("example.com", "alice")
	if err == nil || !strings.Contains(err.Error(), "unsafe permissions") {
		t.Fatalf("error = %v", err)
	}
}

func TestFileStoreErrorsDoNotExposeCredential(t *testing.T) {
	secret := "credential-secret-must-not-leak"
	store := NewUnavailableFileStore(errors.New("disk unavailable"))
	err := store.Set("example.com", "alice", secret)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("error = %v", err)
	}
}
