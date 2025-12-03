package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/illarion/lockenv/internal/core"
)

// Compact compacts the .lockenv database to reclaim unused space
func Compact(ctx context.Context) {
	lockenv, err := core.New(".")
	if err != nil {
		HandleError(err)
	}
	defer lockenv.Close()

	// Get file size before
	info, err := os.Stat(core.LockEnvFile)
	if err != nil {
		HandleError(err)
	}
	sizeBefore := info.Size()

	if err := lockenv.Compact(); err != nil {
		HandleError(err)
	}

	// Get file size after
	info, err = os.Stat(core.LockEnvFile)
	if err != nil {
		HandleError(err)
	}
	sizeAfter := info.Size()

	fmt.Printf("Compacted: %s -> %s\n", formatSize(sizeBefore), formatSize(sizeAfter))
}
