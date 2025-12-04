package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/illarion/lockenv/internal/core"
	"github.com/illarion/lockenv/internal/crypto"
	"github.com/illarion/lockenv/internal/keyring"
	"golang.org/x/term"
)

// PasswordSource indicates where the password was retrieved from
type PasswordSource int

const (
	SourcePrompt PasswordSource = iota
	SourceEnv
	SourceKeyring
)

// GetPasswordWithSource retrieves password and indicates where it came from
func GetPasswordWithSource(prompt string, vaultID string) ([]byte, PasswordSource, error) {
	// Try environment variable first
	password := core.GetPasswordFromEnv()
	if password != nil {
		return password, SourceEnv, nil
	}

	// Try keyring if vault ID is available
	if vaultID != "" {
		if pwd, err := keyring.GetPassword(vaultID); err == nil {
			result := make([]byte, len(pwd))
			copy(result, []byte(pwd))
			return result, SourceKeyring, nil
		}
	}

	// Prompt user
	password, err := core.ReadPassword(prompt)
	if err != nil {
		return nil, SourcePrompt, fmt.Errorf("failed to read password: %w", err)
	}

	return password, SourcePrompt, nil
}

// GetPasswordWithRetry gets password and retries on keyring failure
func GetPasswordWithRetry(prompt string, vaultID string, verify func([]byte) error) ([]byte, PasswordSource, error) {
	password, source, err := GetPasswordWithSource(prompt, vaultID)
	if err != nil {
		return nil, source, err
	}

	verifyErr := verify(password)
	if verifyErr == nil {
		return password, source, nil
	}

	// If keyring password failed and we're in a terminal, retry with prompt
	if verifyErr == core.ErrWrongPassword && source == SourceKeyring && IsTerminal() {
		crypto.ClearBytes(password)
		fmt.Fprintln(os.Stderr, "Warning: keyring password is incorrect, removing stale entry")
		keyring.DeletePassword(vaultID)

		password, err = core.ReadPassword(prompt)
		if err != nil {
			return nil, SourcePrompt, err
		}
		return password, SourcePrompt, nil
	}

	crypto.ClearBytes(password)
	return nil, source, verifyErr
}

// GetPassword retrieves password from environment or prompts user
// The caller is responsible for calling crypto.ClearBytes on the returned password
func GetPassword(prompt string) ([]byte, error) {
	password, _, err := GetPasswordWithSource(prompt, "")
	return password, err
}

// GetPasswordOrExit is like GetPassword but exits on error
func GetPasswordOrExit(prompt string) []byte {
	password, err := GetPassword(prompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	return password
}

// GetPasswordForInit retrieves password for init command
// Checks environment variable first, then prompts with confirmation
func GetPasswordForInit() ([]byte, error) {
	// Try environment variable first
	password := core.GetPasswordFromEnv()
	if password != nil {
		return password, nil
	}

	// Fall back to confirmation prompt
	return core.ReadPasswordConfirm()
}

// boolToInt returns 1 if b is true, 0 otherwise
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// HandleError handles common errors consistently
func HandleError(err error) {
	switch err {
	case core.ErrNotInitialized:
		fmt.Fprintf(os.Stderr, "Error: lockenv not initialized\n")
		fmt.Fprintf(os.Stderr, "Run 'lockenv init' first\n")
	case core.ErrAlreadyExists:
		fmt.Fprintf(os.Stderr, "Error: .lockenv already exists in this directory\n")
		fmt.Fprintf(os.Stderr, "Use 'lockenv status' to see current state\n")
	case core.ErrPasswordRequired:
		fmt.Fprintf(os.Stderr, "Error: password is required\n")
		fmt.Fprintf(os.Stderr, "Set LOCKENV_PASSWORD environment variable or run without it to be prompted\n")
	case core.ErrWrongPassword:
		fmt.Fprintf(os.Stderr, "Error: wrong password\n")
	case core.ErrNoTrackedFiles:
		fmt.Fprintf(os.Stderr, "Error: no files in vault\n")
		fmt.Fprintf(os.Stderr, "Use 'lockenv lock' to add files\n")
	default:
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	}
	os.Exit(1)
}

// IsTerminal returns true if stdin is a terminal
func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// AskYesNo prompts user with a yes/no question, returns true for yes
func AskYesNo(prompt string) bool {
	if !IsTerminal() {
		return false
	}

	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

// OfferToSavePassword offers to save password to keyring if conditions are met
func OfferToSavePassword(vaultID string, password []byte) {
	if !IsTerminal() {
		return
	}

	if keyring.HasPassword(vaultID) {
		return
	}

	if !AskYesNo("Save password to keyring? [y/N] ") {
		return
	}

	if err := keyring.SavePassword(vaultID, string(password)); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save to keyring: %s\n", err)
		return
	}

	fmt.Println("Password saved to keyring")
}
