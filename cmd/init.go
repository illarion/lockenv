package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/live-labs/lockenv/internal/core"
	"github.com/live-labs/lockenv/internal/crypto"
)

// Init creates a new .lockenv file
func Init() {
	lockenv := core.New(".")
	
	// Read password (env var or prompt with confirmation)
	password, err := GetPasswordForInit()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	defer crypto.ClearBytes(password)

	// Initialize lockenv
	if err := lockenv.Init(password); err != nil {
		if errors.Is(err, core.ErrAlreadyExists) {
			fmt.Fprintf(os.Stderr, "Error: .lockenv already exists in this directory\n")
			fmt.Fprintf(os.Stderr, "Use 'lockenv status' to see current state\n")
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
		os.Exit(1)
	}

	fmt.Println("✓ Initialized .lockenv")
}