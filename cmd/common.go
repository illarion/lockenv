package cmd

import (
	"fmt"
	"os"

	"github.com/live-labs/lockenv/internal/core"
	"github.com/live-labs/lockenv/internal/crypto"
)

// GetPassword retrieves password from environment or prompts user
// The caller is responsible for calling crypto.ClearBytes on the returned password
func GetPassword(prompt string) ([]byte, error) {
	// Try environment variable first
	password := core.GetPasswordFromEnv()
	if password != nil {
		return password, nil
	}

	// Prompt user
	password, err := core.ReadPassword(prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to read password: %w", err)
	}

	return password, nil
}

// GetPasswordOrExit is like GetPassword but exits on error
func GetPasswordOrExit(prompt string) []byte {
	password, err := GetPassword(prompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	return password
}

// HandlePasswordError handles password-related errors consistently
func HandlePasswordError(err error, lockenv *core.LockEnv, operation func([]byte) error) {
	if err.Error() == "password required" {
		password := GetPasswordOrExit("Enter password: ")
		defer crypto.ClearBytes(password)
		
		if err := operation(password); err != nil {
			HandleError(err)
		}
	} else {
		HandleError(err)
	}
}

// GetPasswordForInit retrieves password for init command
// Checks environment variable first, then prompts with confirmation
func GetPasswordForInit() ([]byte, error) {
	// Try environment variable first
	password := core.GetPasswordFromEnv()
	if password != nil {
		return password, nil
	}
	
	// Fall back to confirmation prompt
	return core.ReadPasswordConfirm()
}

// HandleError handles common errors consistently
func HandleError(err error) {
	switch err {
	case core.ErrNotInitialized:
		fmt.Fprintf(os.Stderr, "Error: lockenv not initialized\n")
		fmt.Fprintf(os.Stderr, "Run 'lockenv init' first\n")
	case core.ErrAlreadyExists:
		fmt.Fprintf(os.Stderr, "Error: .lockenv already exists in this directory\n")
		fmt.Fprintf(os.Stderr, "Use 'lockenv status' to see current state\n")
	case core.ErrWrongPassword:
		fmt.Fprintf(os.Stderr, "Error: wrong password\n")
	case core.ErrNoTrackedFiles:
		fmt.Fprintf(os.Stderr, "Error: no tracked files\n")
		fmt.Fprintf(os.Stderr, "Use 'lockenv track' to add files\n")
	default:
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	}
	os.Exit(1)
}