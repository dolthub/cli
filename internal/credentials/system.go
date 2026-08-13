package credentials

import "github.com/zalando/go-keyring"

type systemBackend struct{}

func (systemBackend) Get(service, account string) (string, error) {
	v, err := keyring.Get(service, account)
	if err == keyring.ErrNotFound {
		return "", ErrNotFound
	}
	return v, err
}
func (systemBackend) Set(service, account, secret string) error {
	return keyring.Set(service, account, secret)
}
func (systemBackend) Delete(service, account string) error { return keyring.Delete(service, account) }

// NewSystemStore returns a store backed by the operating system credential manager.
func NewSystemStore() Store { return NewKeyringStore(systemBackend{}) }
