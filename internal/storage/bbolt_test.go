package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func TestOpenAndInitialize(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.lockenv")

	// Open database
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Initialize
	if err := db.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Check if initialized
	initialized, err := db.IsInitialized()
	if err != nil {
		t.Fatalf("Failed to check initialization: %v", err)
	}
	if !initialized {
		t.Error("Database should be initialized")
	}
}

func TestSaltAndIterations(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.lockenv")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Set salt
	salt := []byte("test-salt-32-bytes-long-exactly!")
	if err := db.SetSalt(salt); err != nil {
		t.Fatalf("Failed to set salt: %v", err)
	}

	// Get salt
	retrievedSalt, err := db.GetSalt()
	if err != nil {
		t.Fatalf("Failed to get salt: %v", err)
	}

	if string(retrievedSalt) != string(salt) {
		t.Errorf("Salt mismatch: got %v, want %v", retrievedSalt, salt)
	}

	// Set iterations
	iterations := uint32(100000)
	if err := db.SetIterations(iterations); err != nil {
		t.Fatalf("Failed to set iterations: %v", err)
	}

	// Get iterations
	retrievedIters, err := db.GetIterations()
	if err != nil {
		t.Fatalf("Failed to get iterations: %v", err)
	}

	if retrievedIters != iterations {
		t.Errorf("Iterations mismatch: got %d, want %d", retrievedIters, iterations)
	}
}

func TestManifestOperations(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.lockenv")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Add entry
	modTime := time.Now()
	if err := db.UpdateManifest("test.txt", 1234, modTime, "abc123hash"); err != nil {
		t.Fatalf("Failed to update manifest: %v", err)
	}

	// Get all entries
	entries, err := db.GetManifest()
	if err != nil {
		t.Fatalf("Failed to get manifest: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	if entries[0].Path != "test.txt" {
		t.Errorf("Path mismatch: got %s, want test.txt", entries[0].Path)
	}
	if entries[0].Size != 1234 {
		t.Errorf("Size mismatch: got %d, want 1234", entries[0].Size)
	}

	// Get single entry
	entry, err := db.GetManifestEntry("test.txt")
	if err != nil {
		t.Fatalf("Failed to get manifest entry: %v", err)
	}

	if entry == nil {
		t.Fatal("Entry should not be nil")
	}

	// Remove entry
	if err := db.RemoveFromManifest("test.txt"); err != nil {
		t.Fatalf("Failed to remove from manifest: %v", err)
	}

	// Check removed
	entry, err = db.GetManifestEntry("test.txt")
	if err != nil {
		t.Fatalf("Failed to get manifest entry: %v", err)
	}
	if entry != nil {
		t.Error("Entry should be nil after removal")
	}
}

func TestFileStorage(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.lockenv")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Store file data
	data := []byte("encrypted file content")
	if err := db.StoreFileData("secret.txt", data); err != nil {
		t.Fatalf("Failed to store file data: %v", err)
	}

	// Get file data
	retrieved, err := db.GetFileData("secret.txt")
	if err != nil {
		t.Fatalf("Failed to get file: %v", err)
	}

	if string(retrieved) != string(data) {
		t.Errorf("Data mismatch: got %v, want %v", retrieved, data)
	}

	// Remove file
	if err := db.RemoveFile("secret.txt"); err != nil {
		t.Fatalf("Failed to remove file: %v", err)
	}

	// Check removed
	_, err = db.GetFileData("secret.txt")
	if err == nil {
		t.Error("Expected error for removed file")
	}
}

func TestMetadataStorage(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.lockenv")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Store metadata bytes
	data := []byte("encrypted metadata")
	if err := db.StoreMetadataBytes("checksum", data); err != nil {
		t.Fatalf("Failed to store metadata bytes: %v", err)
	}

	// Get metadata bytes
	retrieved, err := db.GetMetadataBytes("checksum")
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}

	if string(retrieved) != string(data) {
		t.Errorf("Data mismatch: got %v, want %v", retrieved, data)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.lockenv")

	// Create and populate database
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	if err := db.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Add some data
	salt := []byte("test-salt-32-bytes-long-exactly!")
	if err := db.SetSalt(salt); err != nil {
		t.Fatalf("Failed to set salt: %v", err)
	}

	if err := db.StoreFileData("test.txt", []byte("data")); err != nil {
		t.Fatalf("Failed to store file data: %v", err)
	}

	db.Close()

	// Reopen and verify
	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to reopen database: %v", err)
	}
	defer db2.Close()

	// Check data persisted
	_, err = db2.GetSalt()
	if err != nil {
		t.Fatalf("Failed to get salt: %v", err)
	}

	// Check data persisted
	data, err := db2.GetFileData("test.txt")
	if err != nil {
		t.Fatalf("Failed to get file: %v", err)
	}

	if string(data) != "data" {
		t.Error("File data not persisted correctly")
	}
}
