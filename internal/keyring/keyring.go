package keyring

import (
	"github.com/zalando/go-keyring"
)

const serviceName = "lockenv"

// SavePassword stores a password in the OS keyring
func SavePassword(vaultID string, password string) error {
	return keyring.Set(serviceName, vaultID, password)
}

// GetPassword retrieves a password from the OS keyring
func GetPassword(vaultID string) (string, error) {
	return keyring.Get(serviceName, vaultID)
}

// DeletePassword removes a password from the OS keyring
func DeletePassword(vaultID string) error {
	return keyring.Delete(serviceName, vaultID)
}

// HasPassword checks if a password is stored in the keyring
func HasPassword(vaultID string) bool {
	_, err := keyring.Get(serviceName, vaultID)
	return err == nil
}
