package cmd

import (
	"fmt"
	"os"

	"github.com/live-labs/lockenv/internal/core"
)

// Track adds files to be tracked by lockenv
func Track(patterns []string) {
	if len(patterns) == 0 {
		fmt.Fprintf(os.Stderr, "Error: track requires at least one file argument\n")
		fmt.Fprintf(os.Stderr, "Usage: lockenv track <file> [file...]\n")
		os.Exit(1)
	}

	lockenv := core.New(".")

	// Track files
	if err := lockenv.Track(patterns); err != nil {
		HandlePasswordError(err, lockenv, func(password []byte) error {
			return lockenv.TrackWithPassword(patterns, password)
		})
	}
}