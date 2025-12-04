package core

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/term"
)

const (
	BinarySampleSize   = 8192 // Bytes to sample for text/binary detection
	BinaryThresholdPct = 10   // Max % non-printable chars for text files
)

// MergeStrategy defines how to handle file conflicts during unlock
type MergeStrategy int

const (
	StrategyAsk MergeStrategy = iota // Ask user for each conflict
	StrategyKeepLocal                // Always keep local version
	StrategyUseVault                 // Always use vault version
	StrategyKeepBoth                 // Always keep both versions (save vault as .from-vault)
	StrategyAbort                    // Abort on any conflict
)

// ConflictResolution defines the user's choice for a specific conflict
type ConflictResolution int

const (
	ResolutionKeepLocal ConflictResolution = iota
	ResolutionUseVault
	ResolutionEditMerged
	ResolutionKeepBoth
	ResolutionSkip
)

// ConflictResult contains the resolution and optionally merged data
type ConflictResult struct {
	Resolution ConflictResolution
	MergedData []byte // Populated when Resolution == ResolutionEditMerged
}

// UnlockResult contains the results of an unlock operation
type UnlockResult struct {
	Extracted []string // Successfully extracted files
	Skipped   []string // Files skipped due to conflicts or user choice
	Errors    []string // Files with errors
}

// DetectFileType determines if a file is likely text or binary.
// Returns true if the file appears to be text.
//
// Detection heuristic (in order):
//  1. Null bytes present → binary (executables, images, etc.)
//  2. Invalid UTF-8 → binary
//  3. >10% non-printable control chars → binary
func DetectFileType(data []byte) bool {
	if len(data) == 0 {
		return true
	}

	// Check for null bytes (strong indicator of binary)
	if bytes.IndexByte(data, 0) != -1 {
		return false
	}

	// Sample first portion for analysis
	sampleSize := BinarySampleSize
	if len(data) < sampleSize {
		sampleSize = len(data)
	}
	sample := data[:sampleSize]

	// Check if valid UTF-8
	if !utf8.Valid(sample) {
		return false
	}

	// Count non-printable characters
	nonPrintable := 0
	for _, b := range sample {
		// Allow common whitespace: space, tab, newline, carriage return
		if b < 32 && b != 9 && b != 10 && b != 13 {
			nonPrintable++
		}
		if b == 127 { // DEL character
			nonPrintable++
		}
	}

	// If more than threshold % non-printable, likely binary
	threshold := len(sample) * BinaryThresholdPct / 100
	return nonPrintable <= threshold
}

// CompareFiles checks if two file contents are identical
// Returns true if files are identical (based on SHA-256 hash)
func CompareFiles(local, vaultData []byte) bool {
	localHash := sha256.Sum256(local)
	vaultHash := sha256.Sum256(vaultData)
	return bytes.Equal(localHash[:], vaultHash[:])
}

// HandleConflict manages interactive conflict resolution for a file
func HandleConflict(path string, localData, vaultData []byte, strategy MergeStrategy) (*ConflictResult, error) {
	switch strategy {
	case StrategyKeepLocal:
		return &ConflictResult{Resolution: ResolutionKeepLocal}, nil
	case StrategyUseVault:
		return &ConflictResult{Resolution: ResolutionUseVault}, nil
	case StrategyKeepBoth:
		return &ConflictResult{Resolution: ResolutionKeepBoth}, nil
	case StrategyAbort:
		return &ConflictResult{Resolution: ResolutionSkip}, fmt.Errorf("conflict detected for %s (aborting)", path)
	}

	// Strategy is StrategyAsk - prompt user
	isText := DetectFileType(localData) && DetectFileType(vaultData)

	fmt.Printf("\nwarning: conflict detected: %s\n", path)
	fmt.Printf("   Local file exists and differs from vault version\n")

	fileType := "binary"
	if isText {
		fileType = "text"
	}
	fmt.Printf("   File type: %s\n", fileType)
	fmt.Printf("\nOptions:\n")
	fmt.Printf("  [l] Keep local version\n")
	fmt.Printf("  [v] Use vault version (overwrite local)\n")
	if isText {
		fmt.Printf("  [e] Edit merged (opens in $EDITOR)\n")
	}
	fmt.Printf("  [b] Keep both (save vault as .from-vault)\n")
	fmt.Printf("  [x] Skip this file\n")

	for {
		fmt.Printf("\nYour choice: ")
		choice, err := readChoice()
		if err != nil {
			return &ConflictResult{Resolution: ResolutionSkip}, err
		}

		switch choice {
		case "l":
			return &ConflictResult{Resolution: ResolutionKeepLocal}, nil
		case "v":
			return &ConflictResult{Resolution: ResolutionUseVault}, nil
		case "e":
			if !isText {
				fmt.Printf("Cannot edit merge for binary files\n")
				continue
			}
			mergedData, err := handleEditMerge(path, localData, vaultData)
			if err != nil {
				fmt.Printf("Error during merge: %v\n", err)
				continue
			}
			return &ConflictResult{Resolution: ResolutionEditMerged, MergedData: mergedData}, nil
		case "b":
			return &ConflictResult{Resolution: ResolutionKeepBoth}, nil
		case "x":
			return &ConflictResult{Resolution: ResolutionSkip}, nil
		default:
			validOptions := "l, v, b, x"
			if isText {
				validOptions = "l, v, e, b, x"
			}
			fmt.Printf("Invalid choice. Please enter %s\n", validOptions)
		}
	}
}

// readChoice reads a single character choice from the terminal
func readChoice() (string, error) {
	// Try to use raw mode for single-key input
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		// Fallback to regular input
		var input string
		_, err := fmt.Scanln(&input)
		if err != nil {
			return "", err
		}
		return strings.ToLower(strings.TrimSpace(input)), nil
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()

	buf := make([]byte, 1)
	_, err = os.Stdin.Read(buf)
	if err != nil {
		return "", err
	}

	choice := strings.ToLower(string(buf[0]))
	fmt.Printf("%s\n", choice) // Echo the choice
	return choice, nil
}

// getEditor returns the editor to use, checking environment variables with fallback
func getEditor() string {
	// Check VISUAL first (modern best practice)
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}
	// Fall back to EDITOR
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	// Platform-specific defaults
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	return "vi"
}

// createLineDiff creates a line-level diff with conflict markers only around differences.
// This produces git-style output where common lines appear once, and only differing
// sections are wrapped in conflict markers.
func createLineDiff(localData, vaultData []byte) []byte {
	dmp := diffmatchpatch.New()

	localStr := string(localData)
	vaultStr := string(vaultData)

	// Line-mode diff (more efficient for text files)
	a, b, lineArray := dmp.DiffLinesToChars(localStr, vaultStr)
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	return buildConflictFromDiffs(diffs)
}

// buildConflictFromDiffs converts diff output to conflict-marked content.
// Equal sections pass through unchanged, while delete/insert pairs become conflict hunks.
func buildConflictFromDiffs(diffs []diffmatchpatch.Diff) []byte {
	var buf bytes.Buffer

	i := 0
	for i < len(diffs) {
		d := diffs[i]

		switch d.Type {
		case diffmatchpatch.DiffEqual:
			buf.WriteString(d.Text)
			i++

		case diffmatchpatch.DiffDelete, diffmatchpatch.DiffInsert:
			// Collect consecutive delete/insert as a conflict hunk
			buf.WriteString("<<<<<<< local\n")

			// Write local (delete) lines
			for i < len(diffs) && diffs[i].Type == diffmatchpatch.DiffDelete {
				text := diffs[i].Text
				buf.WriteString(text)
				// Ensure newline after local section
				if len(text) > 0 && text[len(text)-1] != '\n' {
					buf.WriteByte('\n')
				}
				i++
			}

			buf.WriteString("=======\n")

			// Write vault (insert) lines
			for i < len(diffs) && diffs[i].Type == diffmatchpatch.DiffInsert {
				text := diffs[i].Text
				buf.WriteString(text)
				// Ensure newline after vault section
				if len(text) > 0 && text[len(text)-1] != '\n' {
					buf.WriteByte('\n')
				}
				i++
			}

			buf.WriteString(">>>>>>> vault\n")
		}
	}

	return buf.Bytes()
}

// createConflictFile creates a temporary file with git-style conflict markers.
// For text files, uses line-level diff to show only differences.
// For binary files, falls back to whole-file markers.
func createConflictFile(path string, localData, vaultData []byte) (*os.File, error) {
	// Preserve original file extension for syntax highlighting
	ext := filepath.Ext(path)
	pattern := "lockenv-merge-*" + ext

	tmpFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	if err := os.Chmod(tmpFile.Name(), 0600); err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to set temp file permissions: %w", err)
	}

	// Use line-level diff to show only differences
	content := createLineDiff(localData, vaultData)

	if _, err := tmpFile.Write(content); err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to write conflict content: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	return tmpFile, nil
}

// invokeEditor opens the specified editor and waits for user to finish
func invokeEditor(filename string) error {
	editor := getEditor()

	// Check if editor is available
	if _, err := exec.LookPath(editor); err != nil {
		return fmt.Errorf("editor '%s' not found: %w\nPlease set VISUAL or EDITOR environment variable", editor, err)
	}

	cmd := exec.Command(editor, filename)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err == nil {
		return nil
	}
	exitErr, ok := err.(*exec.ExitError)
	if ok {
		return fmt.Errorf("editor exited with code %d", exitErr.ExitCode())
	}
	return err
}

// handleEditMerge orchestrates the editor-based merge workflow
func handleEditMerge(path string, localData, vaultData []byte) ([]byte, error) {
	// Create temp file with conflict markers
	tmpFile, err := createConflictFile(path, localData, vaultData)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())

	fmt.Printf("\nopening editor for merge...\n")

	// Invoke editor
	if err := invokeEditor(tmpFile.Name()); err != nil {
		return nil, err
	}

	// Read the edited result
	mergedData, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read edited file: %w", err)
	}

	// Warn if file is empty
	if len(mergedData) == 0 {
		fmt.Printf("\nwarning: edited file is empty\n")
		fmt.Printf("Use this empty content? [y/N]: ")
		choice, err := readChoice()
		if err != nil {
			return nil, err
		}
		if choice != "y" {
			return nil, fmt.Errorf("merge aborted by user")
		}
	}

	// Check if conflict markers are still present
	if hasConflictMarkers(mergedData) {
		fmt.Printf("\nwarning: conflict markers still present in file\n")
		fmt.Printf("Continue anyway? [y/N]: ")
		choice, err := readChoice()
		if err != nil {
			return nil, err
		}
		if choice != "y" {
			return nil, fmt.Errorf("merge aborted by user")
		}
	}

	return mergedData, nil
}

// hasConflictMarkers checks if content still contains unresolved conflict markers
func hasConflictMarkers(data []byte) bool {
	return bytes.Contains(data, []byte("<<<<<<<")) ||
		bytes.Contains(data, []byte("=======")) ||
		bytes.Contains(data, []byte(">>>>>>>"))
}

// GenerateUnifiedDiff generates a unified diff using go-diff library
// Returns the diff output, or empty string if files are identical
func GenerateUnifiedDiff(path string, vaultData, localData []byte) (string, error) {
	if CompareFiles(vaultData, localData) {
		return "", nil
	}

	// Check if binary
	if !DetectFileType(vaultData) || !DetectFileType(localData) {
		return fmt.Sprintf("Binary file %s has changed\n", path), nil
	}

	dmp := diffmatchpatch.New()

	// Line-mode diff for better output
	vaultStr, localStr := string(vaultData), string(localData)
	a, b, lineArray := dmp.DiffLinesToChars(vaultStr, localStr)
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	// Create patches and format
	patches := dmp.PatchMake(vaultStr, diffs)
	if len(patches) == 0 {
		return "", nil
	}

	// Add file headers and format output
	var result strings.Builder
	result.WriteString(fmt.Sprintf("--- a/%s\n", path))
	result.WriteString(fmt.Sprintf("+++ b/%s\n", path))
	result.WriteString(dmp.PatchToText(patches))

	return result.String(), nil
}
