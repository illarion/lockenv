package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/illarion/lockenv/internal/core"
	"github.com/illarion/lockenv/internal/crypto"
)

// Remove removes files from the vault
func Remove(ctx context.Context, patterns []string) {
	if len(patterns) == 0 {
		fmt.Fprintf(os.Stderr, "Error: rm requires at least one file argument\n")
		fmt.Fprintf(os.Stderr, "Usage: lockenv rm <file> [file...]\n")
		os.Exit(1)
	}

	lockenv, err := core.New(".")
	if err != nil {
		HandleError(err)
	}
	defer lockenv.Close()

	// Get password once for both operations
	password := GetPasswordOrExit("Enter password: ")
	defer crypto.ClearBytes(password)

	// Remove files from vault
	if err := lockenv.RemoveFiles(ctx, patterns, password); err != nil {
		HandleError(err)
	}

	// Re-encrypt to save updated state
	if err := lockenv.FinalizeLock(ctx, password, false); err != nil {
		// If there are no files left in vault, that's okay
		if err != core.ErrNoTrackedFiles {
			HandleError(err)
		}
	}

	// Compact database to reclaim space
	if err := lockenv.Compact(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: compaction failed: %s\n", err)
	}
}
