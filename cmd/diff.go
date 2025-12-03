package cmd

import (
	"context"

	"github.com/illarion/lockenv/internal/core"
	"github.com/illarion/lockenv/internal/crypto"
)

// Diff compares .lockenv contents with local files
func Diff(ctx context.Context) {
	lockenv, err := core.New(".")
	if err != nil {
		HandleError(err)
	}
	defer lockenv.Close()

	// Get password
	password := GetPasswordOrExit("Enter password: ")
	defer crypto.ClearBytes(password)

	// Show diff
	if err := lockenv.Diff(ctx, password); err != nil {
		HandleError(err)
	}
}
