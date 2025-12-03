package core

import (
	"fmt"
	"os"
	"syscall"

	"github.com/illarion/lockenv/internal/crypto"
	"golang.org/x/term"
)

// ReadPassword reads a password from the terminal without echoing
func ReadPassword(prompt string) ([]byte, error) {
	fmt.Print(prompt)
	
	// Read password without echo
	password, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // New line after password
	
	if err != nil {
		return nil, fmt.Errorf("failed to read password: %w", err)
	}

	return password, nil
}

// ReadPasswordConfirm reads a password twice and ensures they match
func ReadPasswordConfirm() ([]byte, error) {
	password1, err := ReadPassword("Enter password: ")
	if err != nil {
		return nil, err
	}
	defer crypto.ClearBytes(password1)

	password2, err := ReadPassword("Confirm password: ")
	if err != nil {
		return nil, err
	}
	defer crypto.ClearBytes(password2)

	if !crypto.ConstantTimeCompare(password1, password2) {
		return nil, fmt.Errorf("passwords do not match")
	}

	// Return a copy of the password
	result := make([]byte, len(password1))
	copy(result, password1)
	return result, nil
}

// GetPasswordFromEnv reads password from LOCKENV_PASSWORD environment variable
func GetPasswordFromEnv() []byte {
	password := os.Getenv("LOCKENV_PASSWORD")
	if password == "" {
		return nil
	}
	// Return a copy to avoid issues when clearing the bytes
	result := make([]byte, len(password))
	copy(result, []byte(password))
	return result
}