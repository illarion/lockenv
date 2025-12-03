package cmd

import (
	"fmt"
	"os"

	"github.com/illarion/lockenv/internal/core"
	"github.com/illarion/lockenv/internal/crypto"
)

// Init creates a new .lockenv file
func Init() {
	lockenv, err := core.New(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	defer lockenv.Close()

	// Read password (env var or prompt with confirmation)
	password, err := GetPasswordForInit()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	defer crypto.ClearBytes(password)

	// Initialize lockenv
	if err := lockenv.Init(password); err != nil {
		HandleError(err)
	}

	fmt.Println("initialized: .lockenv")
}