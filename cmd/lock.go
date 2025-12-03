package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/illarion/lockenv/internal/core"
	"github.com/illarion/lockenv/internal/crypto"
)

// Lock encrypts and stores files in the vault
func Lock(ctx context.Context, patterns []string, remove bool) {
	lockenv, err := core.New(".")
	if err != nil {
		HandleError(err)
	}
	defer lockenv.Close()

	// Get password once for both operations
	password := GetPasswordOrExit("Enter password: ")
	defer crypto.ClearBytes(password)

	// Add files to vault
	if err := lockenv.LockFiles(ctx, patterns, password); err != nil {
		HandleError(err)
	}

	// Encrypt files
	if err := lockenv.FinalizeLock(ctx, password, remove); err != nil {
		HandleError(err)
	}
}

// LockAll locks all tracked files that have been modified
func LockAll(ctx context.Context, remove bool, force bool) {
	lockenv, err := core.New(".")
	if err != nil {
		HandleError(err)
	}
	defer lockenv.Close()

	// Get password first (needed for hash comparison)
	password := GetPasswordOrExit("Enter password: ")
	defer crypto.ClearBytes(password)

	// Analyze all tracked files
	result, err := lockenv.GetChangedFiles(ctx, password)
	if err != nil {
		HandleError(err)
	}

	totalFiles := len(result.Changed) + len(result.Unchanged) + len(result.Missing)

	// Check if vault is empty
	if totalFiles == 0 {
		fmt.Println("No tracked files in vault")
		fmt.Println("Run 'lockenv lock <file>' to add files")
		return
	}

	// Show summary
	fmt.Printf("Vault status:\n")
	fmt.Printf("  %d files total\n", totalFiles)

	if len(result.Changed) > 0 {
		fmt.Printf("  %d modified:\n", len(result.Changed))
		for _, path := range result.Changed {
			fmt.Printf("    - %s\n", path)
		}
	}

	if len(result.Unchanged) > 0 {
		fmt.Printf("  %d unchanged\n", len(result.Unchanged))
	}

	if len(result.Missing) > 0 {
		fmt.Printf("  %d missing (vault only):\n", len(result.Missing))
		for _, path := range result.Missing {
			fmt.Printf("    - %s\n", path)
		}
	}

	// Check if there are any changes to lock
	if len(result.Changed) == 0 {
		fmt.Println("\nNo changes to lock")
		return
	}

	// Ask for confirmation unless --force
	if !force {
		if remove {
			fmt.Printf("\nLock %d modified file(s) and remove originals? [Y/n]: ", len(result.Changed))
		} else {
			fmt.Printf("\nLock %d modified file(s)? [Y/n]: ", len(result.Changed))
		}

		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))

		// Default is Yes, so only cancel on explicit 'n' or 'no'
		if response == "n" || response == "no" {
			fmt.Println("Cancelled")
			return
		}
	}

	// Lock the changed files
	if err := lockenv.LockFiles(ctx, result.Changed, password); err != nil {
		HandleError(err)
	}

	if err := lockenv.FinalizeLock(ctx, password, remove); err != nil {
		HandleError(err)
	}
}
