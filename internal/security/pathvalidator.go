package security

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrPathEscapes  = errors.New("path escapes repository")
	ErrAbsolutePath = errors.New("absolute paths are not allowed")
	ErrEmptyPath    = errors.New("empty path not allowed")
)

// PathValidator provides secure path validation and file operations
// that are confined to the repository root using Go 1.24's os.Root API.
type PathValidator struct {
	repoRoot *os.Root
	repoPath string
}

// New creates a new PathValidator for the repository at the given path.
// The validator uses os.Root to ensure all file operations stay within
// the repository, preventing path traversal attacks.
func New(repoPath string) (*PathValidator, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	root, err := os.OpenRoot(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository root: %w", err)
	}

	return &PathValidator{
		repoRoot: root,
		repoPath: absPath,
	}, nil
}

// Close releases resources held by the PathValidator.
// It should be called when the validator is no longer needed.
func (pv *PathValidator) Close() error {
	if pv.repoRoot != nil {
		return pv.repoRoot.Close()
	}
	return nil
}

// ValidateAndNormalize validates a user-provided path and returns a normalized
// relative path suitable for storage. It rejects:
// - Empty paths
// - Absolute paths
// - Paths that escape the repository (using ..)
// - Windows reserved names (CON, NUL, etc.)
// - Paths that are not local (using filepath.IsLocal)
func (pv *PathValidator) ValidateAndNormalize(userPath string) (string, error) {
	if userPath == "" {
		return "", ErrEmptyPath
	}

	// Use filepath.IsLocal for initial validation (Go 1.20+)
	// This rejects absolute paths, escaping paths, reserved names, etc.
	if !filepath.IsLocal(userPath) {
		if filepath.IsAbs(userPath) {
			return "", fmt.Errorf("%w: %s", ErrAbsolutePath, userPath)
		}
		return "", fmt.Errorf("%w: %s", ErrPathEscapes, userPath)
	}

	// Clean the path (lexical normalization)
	cleanPath := filepath.Clean(userPath)

	// Verify clean path is still local
	if !filepath.IsLocal(cleanPath) {
		return "", fmt.Errorf("%w: %s", ErrPathEscapes, cleanPath)
	}

	// Convert to absolute path within repo for double-checking
	absPath := filepath.Join(pv.repoPath, cleanPath)

	// Verify containment using filepath.Rel
	relPath, err := filepath.Rel(pv.repoPath, absPath)
	if err != nil {
		return "", fmt.Errorf("failed to compute relative path: %w", err)
	}

	// Final check: ensure path doesn't escape (shouldn't happen after IsLocal, but defense in depth)
	if strings.HasPrefix(relPath, "..") || filepath.IsAbs(relPath) {
		return "", fmt.Errorf("%w: %s", ErrPathEscapes, userPath)
	}

	// Use forward slashes for storage (platform-independent)
	normalizedPath := filepath.ToSlash(relPath)

	return normalizedPath, nil
}

// ValidateExistingPath validates a path that was previously stored in the vault.
// This is used during unlock operations to ensure the vault hasn't been tampered with.
// It applies the same validation rules as ValidateAndNormalize.
func (pv *PathValidator) ValidateExistingPath(storedPath string) (string, error) {
	// Convert from storage format (forward slashes) to platform format
	platformPath := filepath.FromSlash(storedPath)

	// Validate using the same rules
	return pv.ValidateAndNormalize(platformPath)
}

// WriteFileInRoot safely writes a file within the repository using os.Root.
// The path must be relative and will be validated. This prevents writing
// files outside the repository even if the path contains .. or is absolute.
func (pv *PathValidator) WriteFileInRoot(path string, data []byte, perm os.FileMode) error {
	// Convert from storage format if needed
	platformPath := filepath.FromSlash(path)

	// Validate first
	if _, err := pv.ValidateAndNormalize(platformPath); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Use os.Root for secure write
	return pv.repoRoot.WriteFile(platformPath, data, perm)
}

// MkdirAllInRoot safely creates directories within the repository using os.Root.
// The path must be relative and will be validated.
func (pv *PathValidator) MkdirAllInRoot(path string, perm os.FileMode) error {
	// Convert from storage format if needed
	platformPath := filepath.FromSlash(path)

	// Validate first
	if _, err := pv.ValidateAndNormalize(platformPath); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Use os.Root for secure mkdir
	return pv.repoRoot.MkdirAll(platformPath, perm)
}

// ReadFileInRoot safely reads a file within the repository using os.Root.
// The path must be relative and will be validated.
func (pv *PathValidator) ReadFileInRoot(path string) ([]byte, error) {
	// Convert from storage format if needed
	platformPath := filepath.FromSlash(path)

	// Validate first
	if _, err := pv.ValidateAndNormalize(platformPath); err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Use os.Root for secure read
	return pv.repoRoot.ReadFile(platformPath)
}

// StatInRoot safely stats a file within the repository using os.Root.
// The path must be relative and will be validated.
func (pv *PathValidator) StatInRoot(path string) (os.FileInfo, error) {
	// Convert from storage format if needed
	platformPath := filepath.FromSlash(path)

	// Validate first
	if _, err := pv.ValidateAndNormalize(platformPath); err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Use os.Root for secure stat
	return pv.repoRoot.Stat(platformPath)
}
