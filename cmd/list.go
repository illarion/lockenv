package cmd

import (
	"fmt"

	"github.com/live-labs/lockenv/internal/core"
)

// List shows files stored in .lockenv
func List() {
	lockenv := core.New(".")

	// List files (no password required)
	files, err := lockenv.List()
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

// formatSize formats a file size in human-readable form
func formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.1f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.1f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.1f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d bytes", size)
	}
}