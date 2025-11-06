package cmd

import (
	"fmt"
	"os"

	"github.com/live-labs/lockenv/internal/core"
)

// Forget removes files from tracking
func Forget(patterns []string) {
	if len(patterns) == 0 {
		fmt.Fprintf(os.Stderr, "Error: forget requires at least one file argument\n")
		fmt.Fprintf(os.Stderr, "Usage: lockenv forget <file> [file...]\n")
		os.Exit(1)
	}

	lockenv := core.New(".")

	// Forget files
	if err := lockenv.Forget(patterns); err != nil {
		HandlePasswordError(err, lockenv, func(password []byte) error {
			return lockenv.ForgetWithPassword(patterns, password)
		})
	}
}