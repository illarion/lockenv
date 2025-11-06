package cmd

import (
	"github.com/live-labs/lockenv/internal/core"
	"github.com/live-labs/lockenv/internal/crypto"
)

// Diff compares .lockenv contents with local files
func Diff() {
	lockenv := core.New(".")

	// Get password
	password := GetPasswordOrExit("Enter password: ")
	defer crypto.ClearBytes(password)

	// Show diff
	if err := lockenv.Diff(password); err != nil {
		HandleError(err)
	}
}