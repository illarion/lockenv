package cmd

import (
	"fmt"
	"os"

	"github.com/illarion/lockenv/internal/core"
	"github.com/illarion/lockenv/internal/crypto"
	"github.com/illarion/lockenv/internal/keyring"
)

// Passwd changes the password for .lockenv
func Passwd() {
	lockenv, err := core.New(".")
	if err != nil {
		HandleError(err)
	}
	defer lockenv.Close()

	// Get vault ID for keyring lookup
	vaultID, _ := lockenv.GetVaultID()

	// Get current password with retry on stale keyring
	currentPassword, _, err := GetPasswordWithRetry("Enter current password: ", vaultID, lockenv.VerifyPassword)
	if err != nil {
		HandleError(err)
	}
	defer crypto.ClearBytes(currentPassword)

	// Get new password
	newPassword, err := core.ReadPasswordConfirm()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	defer crypto.ClearBytes(newPassword)

	// Change password
	if err := lockenv.ChangePassword(currentPassword, newPassword); err != nil {
		HandleError(err)
	}

	// Always try to update keyring if vault ID exists
	// This handles both updating existing entry and cases where keyring was unavailable before
	if vaultID != "" {
		if err := keyring.SavePassword(vaultID, string(newPassword)); err == nil {
			fmt.Println("Keyring updated with new password")
		}
	}

	// Compact database after rewriting all data
	if err := lockenv.Compact(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: compaction failed: %s\n", err)
	}

	fmt.Println("password changed successfully")
}
