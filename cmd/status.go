package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/live-labs/lockenv/internal/core"
)

// Status shows the current state of lockenv
func Status() {
	lockenv := core.New(".")

	// Check if .lockenv exists
	if _, err := os.Stat(core.LockEnvFile); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No .lockenv file found in current directory")
			fmt.Println("Run 'lockenv init' to create one")
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
		return
	}

	// Get status (no password required)
	status, err := lockenv.Status()
	if err != nil {
		HandleError(err)
	}

	// Show tracked files
	fmt.Println("Tracked files:")
	if len(status.Files) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, file := range status.Files {
			fmt.Printf("  %s (%s)\n", file.Path, file.Status)
		}
	}

	fmt.Printf("\n.lockenv: present (last sealed: %s)\n", status.LastSealed.Format(time.RFC3339))
}