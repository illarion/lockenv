package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/illarion/lockenv/internal/core"
	"github.com/illarion/lockenv/internal/crypto"
)

// Unlock extracts files from .lockenv with smart conflict resolution
func Unlock(ctx context.Context, patterns []string, force bool, keepLocal bool, keepBoth bool) {
	// Validate mutually exclusive flags
	flagCount := boolToInt(force) + boolToInt(keepLocal) + boolToInt(keepBoth)
	if flagCount > 1 {
		fmt.Fprintf(os.Stderr, "error: --force, --keep-local, and --keep-both are mutually exclusive\n")
		os.Exit(1)
	}

	lockenv, err := core.New(".")
	if err != nil {
		HandleError(err)
	}
	defer lockenv.Close()

	// Get vault ID for keyring lookup
	vaultID, _ := lockenv.GetVaultID()

	// Get password with retry on stale keyring
	password, source, err := GetPasswordWithRetry("Enter password: ", vaultID, lockenv.VerifyPassword)
	if err != nil {
		HandleError(err)
	}
	defer crypto.ClearBytes(password)

	// Determine merge strategy
	var strategy core.MergeStrategy
	switch {
	case force:
		strategy = core.StrategyUseVault
	case keepLocal:
		strategy = core.StrategyKeepLocal
	case keepBoth:
		strategy = core.StrategyKeepBoth
	default:
		strategy = core.StrategyAsk
	}

	// Unlock files with smart merge
	result, err := lockenv.Unlock(ctx, password, strategy, patterns)
	if err != nil {
		HandleError(err)
	}

	// Print summary
	fmt.Printf("\n")
	if len(result.Extracted) > 0 {
		fmt.Printf("unlocked: %d files\n", len(result.Extracted))
	}
	if len(result.Skipped) > 0 {
		fmt.Printf("skipped: %d files\n", len(result.Skipped))
	}
	if len(result.Errors) > 0 {
		fmt.Printf("error: %d errors occurred\n", len(result.Errors))
	}

	// Offer to save password if it was entered manually
	if source == SourcePrompt {
		vaultID, err := lockenv.GetOrCreateVaultID()
		if err != nil {
			return
		}
		OfferToSavePassword(vaultID, password)
	}
}
