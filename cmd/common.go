package cmd

import (
	"fmt"
	"os"

	"github.com/illarion/lockenv/internal/core"
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

// boolToInt returns 1 if b is true, 0 otherwise
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
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
	case core.ErrPasswordRequired:
		fmt.Fprintf(os.Stderr, "Error: password is required\n")
		fmt.Fprintf(os.Stderr, "Set LOCKENV_PASSWORD environment variable or run without it to be prompted\n")
	case core.ErrWrongPassword:
		fmt.Fprintf(os.Stderr, "Error: wrong password\n")
	case core.ErrNoTrackedFiles:
		fmt.Fprintf(os.Stderr, "Error: no files in vault\n")
		fmt.Fprintf(os.Stderr, "Use 'lockenv lock' to add files\n")
	default:
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	}
	os.Exit(1)
}
