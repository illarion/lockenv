package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/illarion/lockenv/internal/core"
	"github.com/illarion/lockenv/internal/git"
)

// formatSize formats bytes into human-readable format
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

// getStatusIcon returns an icon for each status
func getStatusIcon(status string) string {
	switch status {
	case "vault only":
		return "*"
	case "modified":
		return "*"
	case "unchanged":
		return "."
	case "error":
		return "!"
	default:
		return " "
	}
}

// Status shows the current state of lockenv
func Status(ctx context.Context) {
	lockenv, err := core.New(".")
	if err != nil {
		HandleError(err)
	}
	defer lockenv.Close()

	// Check if .lockenv exists
	if _, err := os.Stat(core.LockEnvFile); os.IsNotExist(err) {
		fmt.Println("No .lockenv file found in current directory")
		fmt.Println("Run 'lockenv init' to create one")
		return
	}
	if _, err := os.Stat(core.LockEnvFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	// Get status (no password required)
	status, err := lockenv.Status(ctx)
	if err != nil {
		HandleError(err)
	}

	// Show header
	fmt.Printf("\nVault Status\n")
	fmt.Printf("===========================================\n\n")

	// Show statistics
	fmt.Printf("Statistics:\n")
	fmt.Printf("   Files in vault: %d\n", status.TrackedCount)
	fmt.Printf("   Total size:     %s\n", formatSize(status.TotalSize))
	if !status.LastSealed.IsZero() {
		fmt.Printf("   Last locked:    %s\n", status.LastSealed.Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("   Encryption:     %s (PBKDF2 iterations: %d)\n", status.Algorithm, status.KDFIterations)
	fmt.Printf("   Version:        %d\n\n", status.Version)

	// Show file state summary
	if status.TrackedCount > 0 {
		fmt.Printf("Summary:\n")
		if status.UnchangedCount > 0 {
			fmt.Printf("   .  %d unchanged\n", status.UnchangedCount)
		}
		if status.ModifiedCount > 0 {
			fmt.Printf("   *  %d modified\n", status.ModifiedCount)
		}
		if status.SealedCount > 0 {
			fmt.Printf("   *  %d vault only\n", status.SealedCount)
		}
		fmt.Println()
	}

	// Show files in vault
	fmt.Printf("Files:\n")
	if len(status.Files) == 0 {
		fmt.Println("   (no files in vault)")
	} else {
		for _, file := range status.Files {
			icon := getStatusIcon(file.Status)
			fmt.Printf("   %s %s (%s)\n", icon, file.Path, file.Status)
		}
	}

	// Show git integration status
	if status.GitStatus != nil {
		fmt.Print(git.FormatGitStatus(status.GitStatus))
	}

	fmt.Printf("\n===========================================\n")
}
