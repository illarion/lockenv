package cmd

import (
	"fmt"
	"os"

	"github.com/live-labs/lockenv/internal/core"
	"github.com/live-labs/lockenv/internal/crypto"
)

// Chpasswd changes the password for .lockenv
func Chpasswd() {
	lockenv := core.New(".")

	// Get current password
	currentPassword := GetPasswordOrExit("Enter current password: ")
	defer crypto.ClearBytes(currentPassword)

	// Get new password
	newPassword, err := core.ReadPasswordConfirm()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	defer crypto.ClearBytes(newPassword)

	// Change password
	if err := lockenv.ChangePassword(currentPassword, newPassword); err != nil {
		HandleError(err)
	}

	fmt.Println("✓ Password changed successfully")
}