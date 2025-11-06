package cmd

import (
	"github.com/live-labs/lockenv/internal/core"
	"github.com/live-labs/lockenv/internal/crypto"
)

// Seal encrypts tracked files into .lockenv
func Seal(remove bool) {
	lockenv := core.New(".")

	// Get password
	password := GetPasswordOrExit("Enter password: ")
	defer crypto.ClearBytes(password)

	// Seal files
	if err := lockenv.Seal(password, remove); err != nil {
		HandleError(err)
	}
}