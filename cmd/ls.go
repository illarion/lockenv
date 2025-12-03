package cmd

import (
	"context"
	"fmt"

	"github.com/illarion/lockenv/internal/core"
)

// Ls shows files stored in .lockenv
func Ls(ctx context.Context) {
	lockenv, err := core.New(".")
	if err != nil {
		HandleError(err)
	}
	defer lockenv.Close()

	// List files (no password required)
	files, err := lockenv.List(ctx)
	if err != nil {
		HandleError(err)
	}

	if len(files) == 0 {
		fmt.Println("No files in .lockenv")
		return
	}

	fmt.Println("Files in .lockenv:")
	for _, file := range files {
		fmt.Printf("  %s (%s)\n", file.Path, formatSize(file.Size))
	}
}
