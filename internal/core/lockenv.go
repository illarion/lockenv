package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/live-labs/lockenv/internal/crypto"
	"github.com/live-labs/lockenv/internal/storage"
)

const (
	LockEnvFile = ".lockenv"
)

var (
	ErrNotInitialized = errors.New("lockenv not initialized")
	ErrAlreadyExists  = errors.New("lockenv already exists")
	ErrWrongPassword  = errors.New("wrong password")
	ErrNoTrackedFiles = errors.New("no tracked files")
)

// LockEnv manages encrypted file storage
type LockEnv struct {
	path string
	db   *storage.Storage
}

// New creates a new LockEnv instance
func New(path string) *LockEnv {
	return &LockEnv{
		path: filepath.Join(path, LockEnvFile),
	}
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
	checksum := sha256.Sum256([]byte("lockenv-password-check"))
	checksumData, err := enc.Encrypt([]byte(hex.EncodeToString(checksum[:])))
	if err != nil {
		return fmt.Errorf("failed to encrypt checksum: %w", err)
	}

	if err := db.StoreMetadata("checksum", checksumData); err != nil {
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

	if err := db.StoreMetadata("files", encryptedMetadata); err != nil {
		return fmt.Errorf("failed to store metadata: %w", err)
	}

	return nil
}

// Track adds files to the tracking list
func (l *LockEnv) Track(patterns []string) error {
	return l.TrackWithPassword(patterns, nil)
}

// TrackWithPassword adds files to the tracking list with a password
func (l *LockEnv) TrackWithPassword(patterns []string, password []byte) error {
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

	// Track new files
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("invalid pattern %s: %w", pattern, err)
		}

		if len(matches) == 0 {
			// Direct file path
			matches = []string{pattern}
		}

		for _, file := range matches {
			// Check if file exists
			info, err := os.Stat(file)
			if err != nil {
				fmt.Printf("Warning: cannot access %s: %v\n", file, err)
				continue
			}

			if info.IsDir() {
				fmt.Printf("Warning: skipping directory %s\n", file)
				continue
			}

			// Add to metadata
			metadata.AddFile(storage.FileEntry{
				Path:    file,
				Size:    info.Size(),
				Mode:    uint32(info.Mode()),
				ModTime: info.ModTime(),
			})

			// Update manifest (unencrypted)
			if err := db.UpdateManifest(file, info.Size(), info.ModTime()); err != nil {
				return fmt.Errorf("failed to update manifest: %w", err)
			}

			fmt.Printf("✓ Tracking %s\n", file)
		}
	}

	// Save updated metadata
	return l.saveMetadata(metadata, enc)
}

// Seal encrypts and stores tracked files
func (l *LockEnv) Seal(password []byte, remove bool) error {
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

	// Process each file
	var processedFiles []string
	for i := range metadata.Files {
		file := &metadata.Files[i]
		
		// Read file
		data, err := os.ReadFile(file.Path)
		if err != nil {
			fmt.Printf("Warning: cannot read %s: %v\n", file.Path, err)
			continue
		}

		// Calculate hash
		hash := sha256.Sum256(data)
		file.Hash = hex.EncodeToString(hash[:])

		// Encrypt and store file
		encryptedData, err := enc.Encrypt(data)
		if err != nil {
			return fmt.Errorf("failed to encrypt %s: %w", file.Path, err)
		}

		if err := db.StoreFile(file.Path, encryptedData); err != nil {
			return fmt.Errorf("failed to store %s: %w", file.Path, err)
		}

		// Update file info in manifest
		info, err := os.Stat(file.Path)
		if err == nil {
			file.Size = info.Size()
			file.ModTime = info.ModTime()
			if err := db.UpdateManifest(file.Path, info.Size(), info.ModTime()); err != nil {
				fmt.Printf("Warning: failed to update manifest for %s: %v\n", file.Path, err)
			}
		}

		processedFiles = append(processedFiles, file.Path)
		fmt.Printf("✓ Sealed %s\n", file.Path)
	}

	// Save updated metadata
	metadata.Modified = time.Now()
	if err := l.saveMetadata(metadata, enc); err != nil {
		return err
	}

	// Update modification time
	if err := db.UpdateModified(); err != nil {
		fmt.Printf("Warning: failed to update modification time: %v\n", err)
	}

	// Remove original files if requested
	if remove {
		for _, file := range processedFiles {
			if err := os.Remove(file); err != nil {
				fmt.Printf("Warning: cannot remove %s: %v\n", file, err)
			} else {
				fmt.Printf("✓ Removed %s\n", file)
			}
		}
	}

	fmt.Printf("✓ Sealed %d files into %s\n", len(processedFiles), LockEnvFile)
	return nil
}

// Unseal extracts files from the lockenv
func (l *LockEnv) Unseal(password []byte) error {
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

	// Extract each file
	extracted := 0
	for _, file := range metadata.Files {
		// Read encrypted file
		encryptedData, err := db.GetFile(file.Path)
		if err != nil {
			fmt.Printf("Warning: cannot read %s from storage: %v\n", file.Path, err)
			continue
		}

		// Decrypt file
		data, err := enc.Decrypt(encryptedData)
		if err != nil {
			fmt.Printf("Warning: cannot decrypt %s: %v\n", file.Path, err)
			continue
		}

		// Verify hash
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != file.Hash {
			fmt.Printf("Warning: %s failed integrity check\n", file.Path)
			continue
		}

		// Create directory if needed
		dir := filepath.Dir(file.Path)
		if dir != "." && dir != "/" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Printf("Warning: cannot create directory %s: %v\n", dir, err)
				continue
			}
		}

		// Write file
		if err := os.WriteFile(file.Path, data, os.FileMode(file.Mode)); err != nil {
			fmt.Printf("Warning: cannot write %s: %v\n", file.Path, err)
			continue
		}

		// Set modification time
		if err := os.Chtimes(file.Path, time.Now(), file.ModTime); err != nil {
			// Non-critical error
		}

		extracted++
		fmt.Printf("✓ Extracted %s\n", file.Path)
	}

	fmt.Printf("✓ Extracted %d files from %s\n", extracted, LockEnvFile)
	return nil
}

// Forget removes files from tracking
func (l *LockEnv) Forget(patterns []string) error {
	return l.ForgetWithPassword(patterns, nil)
}

// ForgetWithPassword removes files from tracking with a password
func (l *LockEnv) ForgetWithPassword(patterns []string, password []byte) error {
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

	// Remove files
	removed := 0
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("invalid pattern %s: %w", pattern, err)
		}

		if len(matches) == 0 {
			// Direct file path
			matches = []string{pattern}
		}

		for _, file := range matches {
			if metadata.RemoveFile(file) {
				// Remove from manifest
				if err := db.RemoveFromManifest(file); err != nil {
					fmt.Printf("Warning: failed to remove %s from manifest: %v\n", file, err)
				}
				// Remove encrypted file data
				if err := db.RemoveFile(file); err != nil {
					// Ignore error - file might not be sealed yet
				}
				removed++
				fmt.Printf("✓ No longer tracking %s\n", file)
			}
		}
	}

	if removed == 0 {
		fmt.Println("No matching tracked files found")
		return nil
	}

	// Save updated metadata
	return l.saveMetadata(metadata, enc)
}

// List returns tracked files from the manifest (no password required)
func (l *LockEnv) List() ([]storage.FileEntry, error) {
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
	entries, err := db.GetManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	// Convert to FileEntry format
	files := make([]storage.FileEntry, len(entries))
	for i, e := range entries {
		files[i] = storage.FileEntry{
			Path: e.Path,
			Size: e.Size,
		}
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

	for _, entry := range metadata.Files {
		encData, err := db.GetFile(entry.Path)
		if err != nil {
			// File might not be sealed yet
			continue
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
		if err := db.StoreFile(file.path, encData); err != nil {
			return fmt.Errorf("failed to store re-encrypted file %s: %w", file.path, err)
		}
		// Clear file data from memory
		crypto.ClearBytes(file.data)
	}

	// Re-encrypt checksum
	checksum := sha256.Sum256([]byte("lockenv-password-check"))
	checksumData, err := newEnc.Encrypt([]byte(hex.EncodeToString(checksum[:])))
	if err != nil {
		return fmt.Errorf("failed to encrypt checksum: %w", err)
	}
	if err := db.StoreMetadata("checksum", checksumData); err != nil {
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
	if err := db.StoreMetadata("files", encryptedMetadata); err != nil {
		return fmt.Errorf("failed to store metadata: %w", err)
	}

	return nil
}

// Diff compares .lockenv contents with local files
func (l *LockEnv) Diff(password []byte) error {
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

	// Compare each file
	for _, entry := range metadata.Files {
		// Check if file exists locally
		info, err := os.Stat(entry.Path)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("%s: not in working directory\n", entry.Path)
			} else {
				fmt.Printf("%s: error accessing file: %v\n", entry.Path, err)
			}
			continue
		}

		// Compare modification time and size
		if info.Size() != entry.Size || info.ModTime().After(entry.ModTime) {
			fmt.Printf("%s: modified\n", entry.Path)
		} else {
			fmt.Printf("%s: unchanged\n", entry.Path)
		}
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
	Files      []FileStatus
	LastSealed time.Time
}

// Status returns the current status (uses manifest, no password required)
func (l *LockEnv) Status() (*StatusInfo, error) {
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

	status := &StatusInfo{
		LastSealed: lastModified,
		Files:      make([]FileStatus, 0),
	}

	// Get manifest entries
	entries, err := db.GetManifest()
	if err != nil {
		return status, nil // Return empty status
	}

	// Check each file
	for _, entry := range entries {
		fs := FileStatus{Path: entry.Path}

		// Check if file exists locally
		info, err := os.Stat(entry.Path)
		if err != nil {
			if os.IsNotExist(err) {
				fs.Status = "sealed only"
			} else {
				fs.Status = "error"
			}
		} else {
			// Compare with manifest
			if info.Size() != entry.Size {
				fs.Status = "modified"
			} else {
				fs.Status = "unchanged"
			}
		}

		status.Files = append(status.Files, fs)
	}

	return status, nil
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
		return nil, nil, fmt.Errorf("password required")
	}

	// Derive key
	key := kdf.DeriveKey(password)
	// Don't clear the key here - it's still needed by the encryptor

	// Create encryptor
	enc := crypto.NewEncryptor(key)

	// Verify password with checksum
	encChecksum, err := l.db.GetMetadata("checksum")
	if err != nil {
		enc.Destroy()
		return nil, nil, ErrWrongPassword
	}

	checksumData, err := enc.Decrypt(encChecksum)
	if err != nil {
		enc.Destroy()
		return nil, nil, ErrWrongPassword
	}

	checksum := sha256.Sum256([]byte("lockenv-password-check"))
	if string(checksumData) != hex.EncodeToString(checksum[:]) {
		enc.Destroy()
		return nil, nil, ErrWrongPassword
	}

	// Read encrypted metadata
	encMetadata, err := l.db.GetMetadata("files")
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
	if err := l.db.StoreMetadata("files", encryptedMetadata); err != nil {
		return fmt.Errorf("failed to store metadata: %w", err)
	}

	// Update modification time
	return l.db.UpdateModified()
}