package credentials

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestFallbackStorePrefersKeyring(t *testing.T) {
	keyring := NewMemoryStore()
	file := NewFileStore(filepath.Join(t.TempDir(), "credentials.json"))
	store := NewFallbackStore(keyring, file)
	source, err := store.SetPreferred("example.com", "alice", "secret")
	if err != nil || source != SourceKeyring {
		t.Fatalf("source = %q, error = %v", source, err)
	}
	stored, err := store.GetStored("example.com", "alice")
	if err != nil || stored.Source != SourceKeyring || stored.Secret != "secret" {
		t.Fatalf("stored = %#v, error = %v", stored, err)
	}
}

func TestFallbackStoreUsesFileWhenKeyringFails(t *testing.T) {
	keyring := NewMemoryStore()
	keyring.Err = errors.New("keyring unavailable")
	file := NewFileStore(filepath.Join(t.TempDir(), "credentials.json"))
	store := NewFallbackStore(keyring, file)
	source, err := store.SetPreferred("example.com", "alice", "secret")
	if err != nil || source != SourceFile {
		t.Fatalf("source = %q, error = %v", source, err)
	}
	stored, err := store.GetStored("example.com", "alice")
	if err != nil || stored.Source != SourceFile || stored.Secret != "secret" {
		t.Fatalf("stored = %#v, error = %v", stored, err)
	}
}

func TestFallbackStoreExplainsUnavailableKeyringWhenFileCredentialIsMissing(t *testing.T) {
	keyringErr := errors.New("The name is not activatable")
	keyring := NewMemoryStore()
	keyring.Err = keyringErr
	file := NewFileStore(filepath.Join(t.TempDir(), "credentials.json"))
	store := NewFallbackStore(keyring, file)

	_, err := store.GetStored("dev.example", "alice")
	if !errors.Is(err, keyringErr) {
		t.Fatalf("error = %v, want wrapped keyring error", err)
	}
	for _, text := range []string{"no file credential exists", "system credential store is unavailable", "dh auth login --hostname dev.example", "DH_TOKEN"} {
		if !strings.Contains(err.Error(), text) {
			t.Fatalf("error %q does not contain %q", err, text)
		}
	}
}

func TestFallbackStoreMissingCredentialRemainsNotFound(t *testing.T) {
	store := NewFallbackStore(NewMemoryStore(), NewFileStore(filepath.Join(t.TempDir(), "credentials.json")))
	if _, err := store.GetStored("example.com", "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestFallbackStoreFailsSafelyWhenBothStoresFail(t *testing.T) {
	secret := "credential-secret-must-not-leak"
	keyring := NewMemoryStore()
	keyring.Err = errors.New("keyring unavailable")
	store := NewFallbackStore(keyring, NewUnavailableFileStore(errors.New("disk unavailable")))
	_, err := store.SetPreferred("example.com", "alice", secret)
	if err == nil || !strings.Contains(err.Error(), "file fallback failed") || strings.Contains(err.Error(), secret) {
		t.Fatalf("error = %v", err)
	}
}

func TestFallbackStoreMigratesFileCredentialToRecoveredKeyring(t *testing.T) {
	keyring := NewMemoryStore()
	keyring.Err = errors.New("keyring unavailable")
	file := NewFileStore(filepath.Join(t.TempDir(), "credentials.json"))
	store := NewFallbackStore(keyring, file)
	if _, err := store.SetPreferred("example.com", "alice", "old"); err != nil {
		t.Fatal(err)
	}
	keyring.Err = nil
	source, err := store.SetPreferred("example.com", "alice", "new")
	if err != nil || source != SourceKeyring {
		t.Fatalf("source = %q, error = %v", source, err)
	}
	if _, err := file.Get("example.com", "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("fallback entry error = %v", err)
	}
	if got, _ := keyring.Get("example.com", "alice"); got != "new" {
		t.Fatalf("keyring credential = %q", got)
	}
}

func TestFallbackStoreSetAtDoesNotChangeBackend(t *testing.T) {
	keyring := NewMemoryStore()
	file := NewFileStore(filepath.Join(t.TempDir(), "credentials.json"))
	store := NewFallbackStore(keyring, file)
	if err := store.SetAt(SourceFile, "example.com", "alice", "rotated"); err != nil {
		t.Fatal(err)
	}
	if _, err := keyring.Get("example.com", "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("keyring error = %v", err)
	}
	stored, err := store.GetStored("example.com", "alice")
	if err != nil || stored.Source != SourceFile || stored.Secret != "rotated" {
		t.Fatalf("stored = %#v, error = %v", stored, err)
	}
}

func TestFallbackStoreDeleteRemovesBothBackends(t *testing.T) {
	keyring := NewMemoryStore()
	file := NewFileStore(filepath.Join(t.TempDir(), "credentials.json"))
	store := NewFallbackStore(keyring, file)
	if err := keyring.Set("example.com", "alice", "keyring-copy"); err != nil {
		t.Fatal(err)
	}
	if err := file.Set("example.com", "alice", "file-copy"); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("example.com", "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := keyring.Get("example.com", "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("keyring error = %v", err)
	}
	if _, err := file.Get("example.com", "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("file error = %v", err)
	}
}

func TestFallbackStoreDeleteAtFileIgnoresUnavailableKeyring(t *testing.T) {
	keyring := NewMemoryStore()
	keyring.Err = errors.New("keyring unavailable")
	file := NewFileStore(filepath.Join(t.TempDir(), "credentials.json"))
	store := NewFallbackStore(keyring, file)
	if err := file.Set("example.com", "alice", "file-copy"); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteAt(SourceFile, "example.com", "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Get("example.com", "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("file error = %v", err)
	}
}

func TestFallbackStoreDeleteAtReportsAuthoritativeFailure(t *testing.T) {
	keyring := NewMemoryStore()
	fileErr := errors.New("disk unavailable")
	store := NewFallbackStore(keyring, NewUnavailableFileStore(fileErr))
	if err := store.DeleteAt(SourceFile, "example.com", "alice"); !errors.Is(err, fileErr) {
		t.Fatalf("error = %v", err)
	}
}
