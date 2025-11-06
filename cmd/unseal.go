package cmd

import (
	"github.com/live-labs/lockenv/internal/core"
	"github.com/live-labs/lockenv/internal/crypto"
)

// Unseal extracts files from .lockenv
func Unseal() {
	lockenv := core.New(".")

	// Get password
	password := GetPasswordOrExit("Enter password: ")
	defer crypto.ClearBytes(password)

	// Unseal files
	if err := lockenv.Unseal(password); err != nil {
		HandleError(err)
	}
}