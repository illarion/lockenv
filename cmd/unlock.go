package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/illarion/lockenv/internal/core"
	"github.com/illarion/lockenv/internal/crypto"
)

// Unlock extracts files from .lockenv with smart conflict resolution
func Unlock(ctx context.Context, force bool, keepLocal bool, keepBoth bool) {
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

	// Get password
	password := GetPasswordOrExit("Enter password: ")
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
	result, err := lockenv.Unlock(ctx, password, strategy)
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
}
