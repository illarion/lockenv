package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/illarion/lockenv/internal/crypto"
	"github.com/illarion/lockenv/internal/git"
	"github.com/illarion/lockenv/internal/security"
	"github.com/illarion/lockenv/internal/storage"
)

const (
	LockEnvFile         = ".lockenv"
	DirPermSecure       = 0700 // Directory: owner rwx only
	FilePermSecure      = 0600 // File: owner rw only
	MaxVaultCopies      = 100  // Max numbered .from-vault.N backups
	passwordCheckString = "lockenv-password-check"
)

var (
	ErrNotInitialized   = errors.New("lockenv not initialized")
	ErrAlreadyExists    = errors.New("lockenv already exists")
	ErrWrongPassword    = errors.New("wrong password")
	ErrPasswordRequired = errors.New("password required")
	ErrNoTrackedFiles   = errors.New("no files in vault")
)

// LockEnv manages encrypted file storage
type LockEnv struct {
	path      string
	db        *storage.Storage
	validator *security.PathValidator
}

// New creates a new LockEnv instance
func New(path string) (*LockEnv, error) {
	validator, err := security.New(path)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize path validator: %w", err)
	}

	return &LockEnv{
		path:      filepath.Join(path, LockEnvFile),
		validator: validator,
	}, nil
}

// Close releases resources held by the LockEnv instance
func (l *LockEnv) Close() error {
	if l.validator != nil {
		return l.validator.Close()
	}
	return nil
}

// updateManifestEntry updates a manifest entry
func (l *LockEnv) updateManifestEntry(db *storage.Storage, path string, size int64, modTime time.Time, hash string) error {
	return db.UpdateManifest(path, size, modTime, hash)
}

// normalizeToRelative converts an absolute path to relative from repo root.
// Returns the path unchanged if already relative, or error if outside repo.
func (l *LockEnv) normalizeToRelative(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return path, nil
	}
	repoRoot := filepath.Dir(l.path)
	relPath, err := filepath.Rel(repoRoot, path)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", fmt.Errorf("path %s is outside repository", path)
	}
	return relPath, nil
}

// secureFileMode masks a file mode to preserve execute for owner only, removes group/other.
// Returns FilePermSecure (0600) if the result would be zero.
func secureFileMode(mode uint32) os.FileMode {
	secure := os.FileMode(mode) & 0700
	if secure == 0 {
		return FilePermSecure
	}
	return secure
}

// lockSingleFile validates and adds one file to the vault.
// Prints warnings for skipped files, returns error only for fatal failures.
func (l *LockEnv) lockSingleFile(db *storage.Storage, file string, metadata *storage.Metadata) error {
	// Convert absolute paths to relative
	inputPath, err := l.normalizeToRelative(file)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return nil
	}

	// Validate path to ensure it's within repository
	validPath, err := l.validator.ValidateAndNormalize(inputPath)
	if err != nil {
		fmt.Printf("error: invalid path %s: %v\n", file, err)
		return nil
	}

	// Check if file exists using validated path
	repoRoot := filepath.Dir(l.path)
	platformPath := filepath.Join(repoRoot, filepath.FromSlash(validPath))
	info, err := os.Stat(platformPath)
	if err != nil {
		fmt.Printf("warning: cannot access %s: %v\n", validPath, err)
		return nil
	}

	if info.IsDir() {
		fmt.Printf("warning: skipping directory %s\n", validPath)
		return nil
	}

	// Read and hash file content for accurate change detection
	content, err := os.ReadFile(platformPath)
	if err != nil {
		fmt.Printf("warning: cannot read %s: %v\n", validPath, err)
		return nil
	}
	hashBytes := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hashBytes[:])
	crypto.ClearBytes(content)

	// Add to metadata
	metadata.AddFile(storage.FileEntry{
		Path:    validPath,
		Size:    info.Size(),
		Mode:    uint32(info.Mode()),
		ModTime: info.ModTime(),
		Hash:    hashStr,
	})

	// Update manifest with hash
	if err := l.updateManifestEntry(db, validPath, info.Size(), info.ModTime(), hashStr); err != nil {
		return fmt.Errorf("failed to update manifest: %w", err)
	}

	fmt.Printf("locking: %s\n", validPath)
	return nil
}

// getManifestEntries retrieves manifest entries
func (l *LockEnv) getManifestEntries(db *storage.Storage) ([]storage.ManifestEntry, error) {
	return db.GetManifest()
}

// Init initializes a new .lockenv file
func (l *LockEnv) Init(password []byte) error {
	// Check if already exists
	if _, err := os.Stat(l.path); err == nil {
		return ErrAlreadyExists
	}

	// Open database
	db, err := storage.Open(l.path)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	defer db.Close()

	// Initialize bucket structure
	if err := db.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Create KDF
	kdf, err := crypto.NewKDF()
	if err != nil {
		return fmt.Errorf("failed to create KDF: %w", err)
	}

	// Store salt and iterations
	if err := db.SetSalt(kdf.Salt); err != nil {
		return fmt.Errorf("failed to store salt: %w", err)
	}
	if err := db.SetIterations(uint32(kdf.Iterations)); err != nil {
		return fmt.Errorf("failed to store iterations: %w", err)
	}

	// Derive key
	key := kdf.DeriveKey(password)
	defer crypto.ClearBytes(key)

	// Create encryptor
	enc := crypto.NewEncryptor(key)
	defer enc.Destroy()

	// Create and store password verification checksum
	checksum := sha256.Sum256([]byte(passwordCheckString))
	checksumData, err := enc.Encrypt([]byte(hex.EncodeToString(checksum[:])))
	if err != nil {
		return fmt.Errorf("failed to encrypt checksum: %w", err)
	}

	if err := db.StoreMetadataBytes("checksum", checksumData); err != nil {
		return fmt.Errorf("failed to store checksum: %w", err)
	}

	// Create empty metadata
	metadata := storage.NewMetadata()
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	encryptedMetadata, err := enc.Encrypt(metadataJSON)
	if err != nil {
		return fmt.Errorf("failed to encrypt metadata: %w", err)
	}

	if err := db.StoreMetadataBytes("files", encryptedMetadata); err != nil {
		return fmt.Errorf("failed to store metadata: %w", err)
	}

	return nil
}

// LockFiles adds files to the tracking list using the CLI "lock" terminology.
func (l *LockEnv) LockFiles(ctx context.Context, patterns []string, password []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Open database
	db, err := storage.Open(l.path)
	if err != nil {
		return ErrNotInitialized
	}
	defer db.Close()
	l.db = db

	// Read current metadata
	metadata, enc, err := l.readMetadata(password)
	if err != nil {
		return err
	}
	defer enc.Destroy()

	repoRoot := filepath.Dir(l.path)

	// Track new files
	for _, pattern := range patterns {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Glob relative to repo root, not CWD
		absPattern := pattern
		if !filepath.IsAbs(pattern) {
			absPattern = filepath.Join(repoRoot, pattern)
		}

		matches, err := filepath.Glob(absPattern)
		if err != nil {
			return fmt.Errorf("invalid pattern %s: %w", pattern, err)
		}

		if len(matches) == 0 {
			// Direct file path - use absolute path relative to repo root
			if !filepath.IsAbs(pattern) {
				matches = []string{filepath.Join(repoRoot, pattern)}
			} else {
				matches = []string{pattern}
			}
		}

		for _, file := range matches {
			if err := l.lockSingleFile(db, file, metadata); err != nil {
				return err
			}
		}
	}

	// Save updated metadata
	return l.saveMetadata(metadata, enc)
}

func (l *LockEnv) FinalizeLock(ctx context.Context, password []byte, remove bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Open database
	db, err := storage.Open(l.path)
	if err != nil {
		return ErrNotInitialized
	}
	defer db.Close()
	l.db = db

	// Read metadata with password
	metadata, enc, err := l.readMetadata(password)
	if err != nil {
		return err
	}
	defer enc.Destroy()

	if len(metadata.Files) == 0 {
		return ErrNoTrackedFiles
	}

	// Two-phase approach for atomicity:
	// Phase 1: Read and encrypt all files, collect results
	// Phase 2: Store to DB and update metadata only if Phase 1 succeeds

	type pendingFile struct {
		index     int
		path      string
		absPath   string
		encrypted []byte
		hash      string
		size      int64
		mode      uint32
		modTime   time.Time
	}

	repoRoot := filepath.Dir(l.path)
	var pending []pendingFile

	// Phase 1: Read and encrypt all files
	for i := range metadata.Files {
		if err := ctx.Err(); err != nil {
			// Clear any pending encrypted data
			for _, p := range pending {
				crypto.ClearBytes(p.encrypted)
			}
			return err
		}

		file := &metadata.Files[i]
		absPath := filepath.Join(repoRoot, filepath.FromSlash(file.Path))

		// Read file
		data, err := os.ReadFile(absPath)
		if err != nil {
			fmt.Printf("warning: cannot read %s: %v\n", file.Path, err)
			continue
		}

		// Get file info
		info, err := os.Stat(absPath)
		if err != nil {
			crypto.ClearBytes(data)
			fmt.Printf("warning: cannot stat %s: %v\n", file.Path, err)
			continue
		}

		// Calculate hash
		hashBytes := sha256.Sum256(data)
		hashStr := hex.EncodeToString(hashBytes[:])

		// Encrypt
		encryptedData, err := enc.Encrypt(data)
		crypto.ClearBytes(data)
		if err != nil {
			// Clear any pending encrypted data on failure
			for _, p := range pending {
				crypto.ClearBytes(p.encrypted)
			}
			return fmt.Errorf("failed to encrypt %s: %w", file.Path, err)
		}

		pending = append(pending, pendingFile{
			index:     i,
			path:      file.Path,
			absPath:   absPath,
			encrypted: encryptedData,
			hash:      hashStr,
			size:      info.Size(),
			mode:      uint32(info.Mode()),
			modTime:   info.ModTime(),
		})
	}

	if len(pending) == 0 {
		return fmt.Errorf("no files could be processed")
	}

	// Phase 2: Store to DB and update metadata (only after all encryptions succeed)
	var processedFiles []string
	for _, p := range pending {
		if err := ctx.Err(); err != nil {
			// Clear remaining encrypted data
			for j := range pending {
				crypto.ClearBytes(pending[j].encrypted)
			}
			return err
		}

		// Store encrypted data
		if err := db.StoreFileData(p.path, p.encrypted); err != nil {
			crypto.ClearBytes(p.encrypted)
			return fmt.Errorf("failed to store %s: %w", p.path, err)
		}

		// Update manifest (fail fast instead of warning)
		if err := l.updateManifestEntry(db, p.path, p.size, p.modTime, p.hash); err != nil {
			return fmt.Errorf("failed to update manifest for %s: %w", p.path, err)
		}

		// Now update metadata
		file := &metadata.Files[p.index]
		file.Hash = p.hash
		file.Size = p.size
		file.Mode = p.mode
		file.ModTime = p.modTime

		crypto.ClearBytes(p.encrypted)
		processedFiles = append(processedFiles, p.absPath)
		fmt.Printf("encrypted: %s\n", p.path)
	}

	// Save updated metadata
	metadata.Modified = time.Now()
	if err := l.saveMetadata(metadata, enc); err != nil {
		return err
	}

	// Update modification time
	if err := db.UpdateModified(); err != nil {
		fmt.Printf("warning: failed to update modification time: %v\n", err)
	}

	// Remove original files if requested
	if remove {
		for _, file := range processedFiles {
			if err := os.Remove(file); err != nil {
				fmt.Printf("warning: cannot remove %s: %v\n", file, err)
			} else {
				fmt.Printf("removed: %s\n", file)
			}
		}
	}

	fmt.Printf("locked: %d files into %s\n", len(processedFiles), LockEnvFile)
	return nil
}

// Unlock extracts files with smart conflict resolution (implements `lockenv unlock`).
// If patterns is non-empty, only files matching the patterns are unlocked.
func (l *LockEnv) Unlock(ctx context.Context, password []byte, strategy MergeStrategy, patterns []string) (*UnlockResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Open database
	db, err := storage.Open(l.path)
	if err != nil {
		return nil, ErrNotInitialized
	}
	defer db.Close()
	l.db = db

	// Read metadata with password
	metadata, enc, err := l.readMetadata(password)
	if err != nil {
		return nil, err
	}
	defer enc.Destroy()

	result := &UnlockResult{
		Extracted: []string{},
		Skipped:   []string{},
		Errors:    []string{},
	}

	// Get repository root for path operations
	repoRoot := filepath.Dir(l.path)

	// Filter files if patterns provided
	filesToUnlock := metadata.Files
	if len(patterns) > 0 {
		filesToUnlock = filterFilesByPatterns(metadata.Files, patterns)
		if len(filesToUnlock) == 0 {
			return nil, fmt.Errorf("no files match the specified patterns")
		}
	}

	// Extract each file
	for _, file := range filesToUnlock {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// Read encrypted file data
		encryptedData, err := db.GetFileData(file.Path)
		if err != nil {
			msg := fmt.Sprintf("%s: cannot read from storage: %v", file.Path, err)
			result.Errors = append(result.Errors, msg)
			fmt.Printf("error: %s\n", msg)
			continue
		}

		// Decrypt file
		sealedData, err := enc.Decrypt(encryptedData)
		if err != nil {
			msg := fmt.Sprintf("%s: cannot decrypt: %v", file.Path, err)
			result.Errors = append(result.Errors, msg)
			fmt.Printf("error: %s\n", msg)
			continue
		}

		// Verify hash
		hash := sha256.Sum256(sealedData)
		if hex.EncodeToString(hash[:]) != file.Hash {
			crypto.ClearBytes(sealedData)
			msg := fmt.Sprintf("%s: failed integrity check", file.Path)
			result.Errors = append(result.Errors, msg)
			fmt.Printf("error: %s\n", msg)
			continue
		}

		// Validate path from vault to prevent path traversal attacks
		validPath, err := l.validator.ValidateExistingPath(file.Path)
		if err != nil {
			crypto.ClearBytes(sealedData)
			msg := fmt.Sprintf("%s: invalid path from vault: %v", file.Path, err)
			result.Errors = append(result.Errors, msg)
			fmt.Printf("error: %s\n", msg)
			continue
		}

		// Check if local file exists (use absolute path with repoRoot)
		platformPath := filepath.Join(repoRoot, filepath.FromSlash(validPath))
		localData, err := os.ReadFile(platformPath)
		fileExists := err == nil

		if fileExists {
			// Compare files
			if CompareFiles(localData, sealedData) {
				// Files are identical, skip
				crypto.ClearBytes(sealedData)
				crypto.ClearBytes(localData)
				result.Skipped = append(result.Skipped, validPath)
				fmt.Printf("skipped: %s (unchanged)\n", validPath)
				continue
			}

			// Files differ - handle conflict
			conflictResult, err := HandleConflict(validPath, localData, sealedData, strategy)
			if err != nil {
				crypto.ClearBytes(sealedData)
				crypto.ClearBytes(localData)
				result.Errors = append(result.Errors, err.Error())
				fmt.Printf("error: %s\n", err.Error())
				continue
			}

			switch conflictResult.Resolution {
			case ResolutionKeepLocal:
				crypto.ClearBytes(sealedData)
				crypto.ClearBytes(localData)
				result.Skipped = append(result.Skipped, validPath)
				fmt.Printf("skipped: %s (kept local version)\n", validPath)
				continue
			case ResolutionSkip:
				crypto.ClearBytes(sealedData)
				crypto.ClearBytes(localData)
				result.Skipped = append(result.Skipped, validPath)
				fmt.Printf("skipped: %s\n", validPath)
				continue
			case ResolutionEditMerged:
				// Clear original decrypted data before using merged data
				crypto.ClearBytes(sealedData)
				crypto.ClearBytes(localData)
				// Use the merged data from editor
				sealedData = conflictResult.MergedData
			case ResolutionKeepBoth:
				// Save vault version with .from-vault suffix
				vaultPath := validPath + ".from-vault"

				// Check if .from-vault file already exists
				vaultPlatformPath := filepath.Join(repoRoot, filepath.FromSlash(vaultPath))
				foundSlot := true
				if _, err := os.Stat(vaultPlatformPath); err == nil {
					// File exists, find available numbered suffix
					foundSlot = false
					for i := 1; i < MaxVaultCopies; i++ {
						vaultPath = fmt.Sprintf("%s.from-vault.%d", validPath, i)
						vaultPlatformPath = filepath.Join(repoRoot, filepath.FromSlash(vaultPath))
						if _, err := os.Stat(vaultPlatformPath); os.IsNotExist(err) {
							foundSlot = true
							break
						}
					}
				}

				// Error if all backup slots exhausted
				if !foundSlot {
					crypto.ClearBytes(sealedData)
					crypto.ClearBytes(localData)
					msg := fmt.Sprintf("%s: too many backup copies (max %d)", validPath, MaxVaultCopies)
					result.Errors = append(result.Errors, msg)
					fmt.Printf("error: %s\n", msg)
					continue
				}

				// Validate the vault copy path
				if _, err := l.validator.ValidateAndNormalize(vaultPath); err != nil {
					crypto.ClearBytes(sealedData)
					crypto.ClearBytes(localData)
					msg := fmt.Sprintf("%s: invalid vault copy path: %v", vaultPath, err)
					result.Errors = append(result.Errors, msg)
					fmt.Printf("error: %s\n", msg)
					continue
				}

				// Create directory for vault copy if needed using secure operation
				vaultDir := filepath.Dir(vaultPath)
				if vaultDir != "." && vaultDir != "/" {
					if err := l.validator.MkdirAllInRoot(vaultDir, DirPermSecure); err != nil {
						crypto.ClearBytes(sealedData)
						crypto.ClearBytes(localData)
						msg := fmt.Sprintf("%s: cannot create directory for vault copy: %v", vaultPath, err)
						result.Errors = append(result.Errors, msg)
						fmt.Printf("error: %s\n", msg)
						continue
					}
				}

				// Write vault version to alternate path using secure operation
				if err := l.validator.WriteFileInRoot(vaultPath, sealedData, secureFileMode(file.Mode)); err != nil {
					msg := fmt.Sprintf("%s: cannot write vault copy: %v", vaultPath, err)
					result.Errors = append(result.Errors, msg)
					fmt.Printf("error: %s\n", msg)
				} else {
					result.Extracted = append(result.Extracted, vaultPath)
					fmt.Printf("saved: %s (vault version)\n", vaultPath)
				}

				// Clear sensitive data from memory
				crypto.ClearBytes(sealedData)
				crypto.ClearBytes(localData)

				// Keep local file unchanged
				result.Skipped = append(result.Skipped, validPath)
				fmt.Printf("skipped: %s (kept local version)\n", validPath)
				continue
			case ResolutionUseVault:
				// Continue to write vault version
			}
		}

		// Create directory if needed using secure operation
		dir := filepath.Dir(validPath)
		if dir != "." && dir != "/" {
			if err := l.validator.MkdirAllInRoot(dir, DirPermSecure); err != nil {
				crypto.ClearBytes(sealedData)
				if fileExists {
					crypto.ClearBytes(localData)
				}
				msg := fmt.Sprintf("%s: cannot create directory: %v", validPath, err)
				result.Errors = append(result.Errors, msg)
				fmt.Printf("error: %s\n", msg)
				continue
			}
		}

		// Write file using secure operation
		if err := l.validator.WriteFileInRoot(validPath, sealedData, secureFileMode(file.Mode)); err != nil {
			crypto.ClearBytes(sealedData)
			if fileExists {
				crypto.ClearBytes(localData)
			}
			msg := fmt.Sprintf("%s: cannot write file: %v", validPath, err)
			result.Errors = append(result.Errors, msg)
			fmt.Printf("error: %s\n", msg)
			continue
		}

		// Set modification time
		if err := os.Chtimes(platformPath, time.Now(), file.ModTime); err != nil {
			// Non-critical error
		}

		// Clear sensitive data from memory
		crypto.ClearBytes(sealedData)
		if fileExists {
			crypto.ClearBytes(localData)
		}

		result.Extracted = append(result.Extracted, validPath)
		fmt.Printf("unlocked: %s\n", validPath)
	}

	return result, nil
}

// filterFilesByPatterns filters files by patterns (exact match or glob)
func filterFilesByPatterns(files []storage.FileEntry, patterns []string) []storage.FileEntry {
	var result []storage.FileEntry
	for _, file := range files {
		for _, pattern := range patterns {
			normalizedPattern := filepath.ToSlash(pattern)
			// Direct match
			if file.Path == normalizedPattern {
				result = append(result, file)
				break
			}
			// Glob match
			if matched, _ := filepath.Match(normalizedPattern, file.Path); matched {
				result = append(result, file)
				break
			}
		}
	}
	return result
}

// RemoveFiles removes files from tracking with a password (implements `lockenv rm`).
func (l *LockEnv) RemoveFiles(ctx context.Context, patterns []string, password []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Open database
	db, err := storage.Open(l.path)
	if err != nil {
		return ErrNotInitialized
	}
	defer db.Close()
	l.db = db

	// Read current metadata
	metadata, enc, err := l.readMetadata(password)
	if err != nil {
		return err
	}
	defer enc.Destroy()

	repoRoot := filepath.Dir(l.path)

	// Remove files
	removed := 0
	for _, pattern := range patterns {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Glob relative to repo root, not CWD
		absPattern := pattern
		if !filepath.IsAbs(pattern) {
			absPattern = filepath.Join(repoRoot, pattern)
		}

		matches, err := filepath.Glob(absPattern)
		if err != nil {
			return fmt.Errorf("invalid pattern %s: %w", pattern, err)
		}

		if len(matches) == 0 {
			// Direct file path - use absolute path relative to repo root
			if !filepath.IsAbs(pattern) {
				matches = []string{filepath.Join(repoRoot, pattern)}
			} else {
				matches = []string{pattern}
			}
		}

		for _, file := range matches {
			// Convert absolute paths to relative
			inputPath, err := l.normalizeToRelative(file)
			if err != nil {
				fmt.Printf("warning: %v\n", err)
				continue
			}

			// Validate and normalize path to match stored format
			storedPath, err := l.validator.ValidateAndNormalize(inputPath)
			if err != nil {
				fmt.Printf("warning: invalid path %s: %v\n", file, err)
				continue
			}

			if metadata.RemoveFile(storedPath) {
				// Remove from manifest
				if err := db.RemoveFromManifest(storedPath); err != nil {
					fmt.Printf("warning: failed to remove %s from manifest: %v\n", storedPath, err)
				}
				// Remove encrypted file data
				if err := db.RemoveFile(storedPath); err != nil {
					// Ignore error - file might not be sealed yet
				}
				removed++
				fmt.Printf("removed: %s from vault\n", storedPath)
			}
		}
	}

	if removed == 0 {
		fmt.Println("No matching files found in vault")
		return nil
	}

	// Save updated metadata
	return l.saveMetadata(metadata, enc)
}

// List returns tracked files from the manifest (no password required)
func (l *LockEnv) List(ctx context.Context) ([]storage.FileEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Check if exists
	if _, err := os.Stat(l.path); err != nil {
		return nil, ErrNotInitialized
	}

	// Open database
	db, err := storage.Open(l.path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Get manifest entries
	entries, err := l.getManifestEntries(db)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	// Convert to FileEntry format, validating paths
	files := make([]storage.FileEntry, 0, len(entries))
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// Validate path to filter out tampered entries
		validPath, err := l.validator.ValidateExistingPath(e.Path)
		if err != nil {
			// Skip invalid entries
			continue
		}

		files = append(files, storage.FileEntry{
			Path: validPath,
			Size: e.Size,
		})
	}

	return files, nil
}

// ChangePassword changes the password for the .lockenv file
func (l *LockEnv) ChangePassword(currentPassword, newPassword []byte) error {
	// Open database
	db, err := storage.Open(l.path)
	if err != nil {
		return ErrNotInitialized
	}
	defer db.Close()
	l.db = db

	// Read metadata with current password
	metadata, currentEnc, err := l.readMetadata(currentPassword)
	if err != nil {
		return err
	}
	defer currentEnc.Destroy()

	// Read all file data with current password
	type fileData struct {
		path string
		data []byte
	}
	var files []fileData
	// Ensure all decrypted file data is cleared from memory on all exit paths
	defer func() {
		for i := range files {
			crypto.ClearBytes(files[i].data)
		}
	}()

	for _, entry := range metadata.Files {
		encData, err := db.GetFileData(entry.Path)
		if err != nil {
			return fmt.Errorf("file %s not sealed in vault: %w", entry.Path, err)
		}

		data, err := currentEnc.Decrypt(encData)
		if err != nil {
			return fmt.Errorf("failed to decrypt file %s: %w", entry.Path, err)
		}
		files = append(files, fileData{path: entry.Path, data: data})
	}

	// Create new KDF with new password
	newKDF, err := crypto.NewKDF()
	if err != nil {
		return fmt.Errorf("failed to create new KDF: %w", err)
	}

	newKey := newKDF.DeriveKey(newPassword)
	defer crypto.ClearBytes(newKey)

	newEnc := crypto.NewEncryptor(newKey)
	defer newEnc.Destroy()

	// Update salt and iterations
	if err := db.SetSalt(newKDF.Salt); err != nil {
		return fmt.Errorf("failed to update salt: %w", err)
	}
	if err := db.SetIterations(uint32(newKDF.Iterations)); err != nil {
		return fmt.Errorf("failed to update iterations: %w", err)
	}

	// Re-encrypt all files with new key
	for _, file := range files {
		encData, err := newEnc.Encrypt(file.data)
		if err != nil {
			return fmt.Errorf("failed to re-encrypt file %s: %w", file.path, err)
		}
		if err := db.StoreFileData(file.path, encData); err != nil {
			return fmt.Errorf("failed to store re-encrypted file %s: %w", file.path, err)
		}
		// Clear file data from memory
		crypto.ClearBytes(file.data)
	}

	// Re-encrypt checksum
	checksum := sha256.Sum256([]byte(passwordCheckString))
	checksumData, err := newEnc.Encrypt([]byte(hex.EncodeToString(checksum[:])))
	if err != nil {
		return fmt.Errorf("failed to encrypt checksum: %w", err)
	}
	if err := db.StoreMetadataBytes("checksum", checksumData); err != nil {
		return fmt.Errorf("failed to store checksum: %w", err)
	}

	// Re-encrypt metadata
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	encryptedMetadata, err := newEnc.Encrypt(metadataJSON)
	if err != nil {
		return fmt.Errorf("failed to encrypt metadata: %w", err)
	}
	if err := db.StoreMetadataBytes("files", encryptedMetadata); err != nil {
		return fmt.Errorf("failed to store metadata: %w", err)
	}

	return nil
}

// Diff compares .lockenv contents with local files showing actual content differences
func (l *LockEnv) Diff(ctx context.Context, password []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Open database
	db, err := storage.Open(l.path)
	if err != nil {
		return ErrNotInitialized
	}
	defer db.Close()
	l.db = db

	// Read metadata
	metadata, enc, err := l.readMetadata(password)
	if err != nil {
		return err
	}
	defer enc.Destroy()

	hasChanges := false
	repoRoot := filepath.Dir(l.path)

	// Compare each file
	for _, file := range metadata.Files {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Validate path to prevent path traversal
		validPath, err := l.validator.ValidateExistingPath(file.Path)
		if err != nil {
			// Skip invalid entries
			continue
		}
		platformPath := filepath.Join(repoRoot, filepath.FromSlash(validPath))

		// Check if file exists locally
		localData, err := os.ReadFile(platformPath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("File not in working directory: %s\n", validPath)
			} else {
				fmt.Printf("error: cannot read %s: %v\n", validPath, err)
			}
			continue
		}

		// Read encrypted file data from vault (use original path for db key)
		encryptedData, err := db.GetFileData(file.Path)
		if err != nil {
			crypto.ClearBytes(localData)
			fmt.Printf("error: cannot read %s from vault: %v\n", validPath, err)
			continue
		}

		// Decrypt file
		vaultData, err := enc.Decrypt(encryptedData)
		if err != nil {
			crypto.ClearBytes(localData)
			fmt.Printf("error: cannot decrypt %s: %v\n", validPath, err)
			continue
		}

		// Generate diff
		diff, err := GenerateUnifiedDiff(validPath, vaultData, localData)
		if err != nil {
			crypto.ClearBytes(vaultData)
			crypto.ClearBytes(localData)
			fmt.Printf("error: cannot generate diff for %s: %v\n", validPath, err)
			continue
		}

		if diff != "" {
			fmt.Print(diff)
			hasChanges = true
		}

		// Clear sensitive data from memory
		crypto.ClearBytes(vaultData)
		crypto.ClearBytes(localData)
	}

	if !hasChanges {
		fmt.Println("No changes detected")
	}

	return nil
}

// FileStatus represents the status of a tracked file
type FileStatus struct {
	Path   string
	Status string
}

// StatusInfo contains status information
type StatusInfo struct {
	Files          []FileStatus
	LastSealed     time.Time
	TrackedCount   int
	SealedCount    int
	ModifiedCount  int
	UnchangedCount int
	TotalSize      int64
	Algorithm      string
	KDFIterations  uint32
	Version        int
	GitStatus      *git.GitStatus
}

// Status returns the current status (no password required)
func (l *LockEnv) Status(ctx context.Context) (*StatusInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Check if exists
	if _, err := os.Stat(l.path); err != nil {
		return nil, ErrNotInitialized
	}

	// Open database
	db, err := storage.Open(l.path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Get last modified time
	lastModified, err := db.GetModified()
	if err != nil {
		// Not critical
		lastModified = time.Time{}
	}

	// Get KDF iterations
	iterations, err := db.GetIterations()
	if err != nil {
		iterations = 0
	}

	status := &StatusInfo{
		LastSealed:     lastModified,
		Files:          make([]FileStatus, 0),
		Algorithm:      "AES-256-GCM",
		KDFIterations:  iterations,
		Version:        1,
		TotalSize:      0,
		TrackedCount:   0,
		SealedCount:    0,
		ModifiedCount:  0,
		UnchangedCount: 0,
	}

	// Get manifest entries
	entries, err := l.getManifestEntries(db)
	if err != nil {
		return status, nil // Return empty status
	}

	repoRoot := filepath.Dir(l.path)

	// Check each file
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// Validate path to prevent path traversal from tampered manifest
		validPath, err := l.validator.ValidateExistingPath(entry.Path)
		if err != nil {
			// Skip invalid entries (possible tampered manifest)
			continue
		}

		fs := FileStatus{Path: validPath}
		status.TrackedCount++
		status.TotalSize += entry.Size

		// Check if file exists locally using validated path
		platformPath := filepath.Join(repoRoot, filepath.FromSlash(validPath))
		_, err = os.Stat(platformPath)

		if os.IsNotExist(err) {
			fs.Status = "vault only"
			status.SealedCount++
			status.Files = append(status.Files, fs)
			continue
		}
		if err != nil {
			fs.Status = "error"
			status.Files = append(status.Files, fs)
			continue
		}

		// Hash-based comparison
		content, err := os.ReadFile(platformPath)
		if err != nil {
			fs.Status = "modified"
			status.ModifiedCount++
			status.Files = append(status.Files, fs)
			continue
		}

		localHash := sha256.Sum256(content)
		localHashStr := hex.EncodeToString(localHash[:])
		crypto.ClearBytes(content)

		if localHashStr != entry.Hash {
			fs.Status = "modified"
			status.ModifiedCount++
			status.Files = append(status.Files, fs)
			continue
		}

		fs.Status = "unchanged"
		status.UnchangedCount++
		status.Files = append(status.Files, fs)
	}

	// Check git integration (use validated paths only)
	trackedPaths := make([]string, 0, len(status.Files))
	for _, fs := range status.Files {
		trackedPaths = append(trackedPaths, fs.Path)
	}

	workDir := filepath.Dir(l.path)
	gitStatus, err := git.CheckGitIntegration(workDir, trackedPaths)
	if err == nil && gitStatus.IsRepo {
		status.GitStatus = gitStatus
	}

	return status, nil
}

// ChangedFilesResult contains the result of analyzing tracked files
type ChangedFilesResult struct {
	Changed   []string // Files that have been modified
	Unchanged []string // Files that are the same
	Missing   []string // Files that don't exist locally (vault only)
}

// GetChangedFiles analyzes all tracked files and determines which have changed
// Uses content hash comparison for accurate detection
func (l *LockEnv) GetChangedFiles(ctx context.Context, password []byte) (*ChangedFilesResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Check if exists
	if _, err := os.Stat(l.path); err != nil {
		return nil, ErrNotInitialized
	}

	// Open database temporarily
	db, err := storage.Open(l.path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Set l.db temporarily for readMetadata
	l.db = db
	defer func() { l.db = nil }()

	// Read metadata (requires password)
	metadata, enc, err := l.readMetadata(password)
	if err != nil {
		return nil, err
	}
	defer enc.Destroy() // Clear derived key from memory

	result := &ChangedFilesResult{
		Changed:   make([]string, 0),
		Unchanged: make([]string, 0),
		Missing:   make([]string, 0),
	}

	repoRoot := filepath.Dir(l.path)

	// Check each tracked file
	for _, file := range metadata.Files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// Validate path to prevent path traversal
		validPath, err := l.validator.ValidateExistingPath(file.Path)
		if err != nil {
			// Skip invalid entries
			continue
		}
		platformPath := filepath.Join(repoRoot, filepath.FromSlash(validPath))

		// Check if file exists
		if _, err := os.Stat(platformPath); err != nil {
			if os.IsNotExist(err) {
				result.Missing = append(result.Missing, validPath)
				continue
			}
			// Other error (permission, etc.) - treat as missing
			result.Missing = append(result.Missing, validPath)
			continue
		}

		// Read file content
		content, err := os.ReadFile(platformPath)
		if err != nil {
			// Can't read - treat as missing
			result.Missing = append(result.Missing, validPath)
			continue
		}

		// Calculate hash
		hash := sha256.Sum256(content)
		currentHash := hex.EncodeToString(hash[:])

		// Compare with stored hash
		if currentHash != file.Hash {
			result.Changed = append(result.Changed, validPath)
		} else {
			result.Unchanged = append(result.Unchanged, validPath)
		}
	}

	return result, nil
}

// readMetadata reads and decrypts metadata
func (l *LockEnv) readMetadata(password []byte) (*storage.Metadata, *crypto.Encryptor, error) {
	if l.db == nil {
		return nil, nil, fmt.Errorf("database not open")
	}

	// Get salt and iterations
	salt, err := l.db.GetSalt()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get salt: %w", err)
	}

	iterations, err := l.db.GetIterations()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get iterations: %w", err)
	}

	// Create KDF
	kdf := &crypto.KDF{
		Salt:       salt,
		Iterations: int(iterations),
	}

	// If no password provided, prompt for it
	if password == nil {
		// This will be handled by the command layer
		return nil, nil, ErrPasswordRequired
	}

	// Derive key
	key := kdf.DeriveKey(password)
	// Don't clear the key here - it's still needed by the encryptor

	// Create encryptor
	enc := crypto.NewEncryptor(key)

	// Verify password with checksum
	encChecksum, err := l.db.GetMetadataBytes("checksum")
	if err != nil {
		enc.Destroy()
		return nil, nil, ErrWrongPassword
	}

	checksumData, err := enc.Decrypt(encChecksum)
	if err != nil {
		enc.Destroy()
		return nil, nil, ErrWrongPassword
	}

	checksum := sha256.Sum256([]byte(passwordCheckString))
	if string(checksumData) != hex.EncodeToString(checksum[:]) {
		enc.Destroy()
		return nil, nil, ErrWrongPassword
	}

	// Read encrypted metadata
	encMetadata, err := l.db.GetMetadataBytes("files")
	if err != nil {
		enc.Destroy()
		return nil, nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	metadataData, err := enc.Decrypt(encMetadata)
	if err != nil {
		enc.Destroy()
		return nil, nil, fmt.Errorf("failed to decrypt metadata: %w", err)
	}

	var metadata storage.Metadata
	if err := json.Unmarshal(metadataData, &metadata); err != nil {
		enc.Destroy()
		return nil, nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &metadata, enc, nil
}

// saveMetadata saves updated metadata
func (l *LockEnv) saveMetadata(metadata *storage.Metadata, enc *crypto.Encryptor) error {
	if l.db == nil {
		return fmt.Errorf("database not open")
	}

	metadata.Modified = time.Now()

	// Marshal metadata
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Encrypt metadata
	encryptedMetadata, err := enc.Encrypt(metadataJSON)
	if err != nil {
		return fmt.Errorf("failed to encrypt metadata: %w", err)
	}

	// Store metadata
	if err := l.db.StoreMetadataBytes("files", encryptedMetadata); err != nil {
		return fmt.Errorf("failed to store metadata: %w", err)
	}

	// Update modification time
	return l.db.UpdateModified()
}

// Compact compacts the database to reclaim unused space.
// This is useful after removing files from the vault.
func (l *LockEnv) Compact() error {
	// Open database if not already open
	if l.db == nil {
		db, err := storage.Open(l.path)
		if err != nil {
			return ErrNotInitialized
		}
		defer db.Close()
		return db.Compact()
	}
	return l.db.Compact()
}

// GetVaultID retrieves the vault ID from storage
func (l *LockEnv) GetVaultID() (string, error) {
	if _, err := os.Stat(l.path); err != nil {
		return "", ErrNotInitialized
	}

	db, err := storage.Open(l.path)
	if err != nil {
		return "", ErrNotInitialized
	}
	defer db.Close()

	return db.GetVaultID()
}

// GetOrCreateVaultID retrieves existing vault ID or generates a new one
func (l *LockEnv) GetOrCreateVaultID() (string, error) {
	if _, err := os.Stat(l.path); err != nil {
		return "", ErrNotInitialized
	}

	db, err := storage.Open(l.path)
	if err != nil {
		return "", ErrNotInitialized
	}
	defer db.Close()

	return db.GetOrCreateVaultID()
}

// VerifyPassword checks if the password is correct for this vault
func (l *LockEnv) VerifyPassword(password []byte) error {
	if _, err := os.Stat(l.path); err != nil {
		return ErrNotInitialized
	}

	db, err := storage.Open(l.path)
	if err != nil {
		return ErrNotInitialized
	}
	defer db.Close()
	l.db = db

	_, enc, err := l.readMetadata(password)
	if err != nil {
		return err
	}
	enc.Destroy()

	return nil
}
