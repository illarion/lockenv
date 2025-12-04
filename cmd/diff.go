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

	// Get vault ID for keyring lookup
	vaultID, _ := lockenv.GetVaultID()

	// Get password with retry on stale keyring
	password, _, err := GetPasswordWithRetry("Enter password: ", vaultID, lockenv.VerifyPassword)
	if err != nil {
		HandleError(err)
	}
	defer crypto.ClearBytes(password)

	// Show diff
	if err := lockenv.Diff(ctx, password); err != nil {
		HandleError(err)
	}
}
