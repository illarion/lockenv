package cmd

import (
	"fmt"
	"os"

	"github.com/illarion/lockenv/internal/core"
	"github.com/illarion/lockenv/internal/crypto"
	"github.com/illarion/lockenv/internal/keyring"
)

// KeyringSave saves the password to the OS keyring
func KeyringSave() {
	lockenv, err := core.New(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	defer lockenv.Close()

	// Prompt for password
	password, err := core.ReadPassword("Enter password: ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	defer crypto.ClearBytes(password)

	// Verify password is correct
	if err := lockenv.VerifyPassword(password); err != nil {
		HandleError(err)
	}

	// Get vault ID (create if not exists)
	vaultID, err := lockenv.GetOrCreateVaultID()
	if err != nil {
		HandleError(err)
	}

	// Save to keyring
	if err := keyring.SavePassword(vaultID, string(password)); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to save to keyring: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("Password saved to keyring")
}

// KeyringDelete removes the password from the OS keyring
func KeyringDelete() {
	lockenv, err := core.New(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	defer lockenv.Close()

	// Get vault ID
	vaultID, err := lockenv.GetVaultID()
	if err != nil {
		fmt.Println("No password stored in keyring")
		return
	}

	// Delete from keyring
	if err := keyring.DeletePassword(vaultID); err != nil {
		fmt.Println("No password stored in keyring")
		return
	}

	fmt.Println("Password removed from keyring")
}

// KeyringStatus checks if a password is stored in the keyring
func KeyringStatus() {
	lockenv, err := core.New(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	defer lockenv.Close()

	// Get vault ID
	vaultID, err := lockenv.GetVaultID()
	if err != nil {
		fmt.Println("Password: not stored")
		return
	}

	if keyring.HasPassword(vaultID) {
		fmt.Println("Password: stored in keyring")
	} else {
		fmt.Println("Password: not stored")
	}
}
