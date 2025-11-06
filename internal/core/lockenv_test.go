package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCommand(t *testing.T) {
	dir := t.TempDir()
	lockenv := New(dir)
	
	// Test init
	password := []byte("test123")
	if err := lockenv.Init(password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	
	// Test init again (should fail)
	if err := lockenv.Init(password); err != ErrAlreadyExists {
		t.Errorf("Expected ErrAlreadyExists, got %v", err)
	}
	
	// Check file exists
	lockenvPath := filepath.Join(dir, LockEnvFile)
	if _, err := os.Stat(lockenvPath); err != nil {
		t.Errorf("Lockenv file should exist: %v", err)
	}
}

func TestTrackSealUnseal(t *testing.T) {
	dir := t.TempDir()
	lockenv := New(dir)
	password := []byte("test123")
	
	// Init
	if err := lockenv.Init(password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	
	// Create test files
	testFile1 := filepath.Join(dir, "test1.txt")
	testFile2 := filepath.Join(dir, "test2.txt")
	
	if err := os.WriteFile(testFile1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create test file 1: %v", err)
	}
	if err := os.WriteFile(testFile2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create test file 2: %v", err)
	}
	
	// Track files
	if err := lockenv.TrackWithPassword([]string{testFile1, testFile2}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}
	
	// List tracked files
	files, err := lockenv.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("Expected 2 tracked files, got %d", len(files))
	}
	
	// Seal files
	if err := lockenv.Seal(password, true); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}
	
	// Check files were removed
	if _, err := os.Stat(testFile1); !os.IsNotExist(err) {
		t.Error("Test file 1 should have been removed")
	}
	if _, err := os.Stat(testFile2); !os.IsNotExist(err) {
		t.Error("Test file 2 should have been removed")
	}
	
	// Unseal files
	if err := lockenv.Unseal(password); err != nil {
		t.Fatalf("Unseal failed: %v", err)
	}
	
	// Check files were restored
	content1, err := os.ReadFile(testFile1)
	if err != nil {
		t.Fatalf("Failed to read unsealed file 1: %v", err)
	}
	if string(content1) != "content1" {
		t.Errorf("File 1 content mismatch: got %s, want content1", content1)
	}
	
	content2, err := os.ReadFile(testFile2)
	if err != nil {
		t.Fatalf("Failed to read unsealed file 2: %v", err)
	}
	if string(content2) != "content2" {
		t.Errorf("File 2 content mismatch: got %s, want content2", content2)
	}
}

func TestForget(t *testing.T) {
	dir := t.TempDir()
	lockenv := New(dir)
	password := []byte("test123")
	
	// Init
	if err := lockenv.Init(password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	
	// Create and track test file
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	if err := lockenv.TrackWithPassword([]string{testFile}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}
	
	// Verify tracked
	files, err := lockenv.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("Expected 1 tracked file, got %d", len(files))
	}
	
	// Forget file
	if err := lockenv.ForgetWithPassword([]string{testFile}, password); err != nil {
		t.Fatalf("Forget failed: %v", err)
	}
	
	// Verify forgotten
	files, err = lockenv.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("Expected 0 tracked files, got %d", len(files))
	}
}

func TestWrongPassword(t *testing.T) {
	dir := t.TempDir()
	lockenv := New(dir)
	password := []byte("test123")
	wrongPassword := []byte("wrong")
	
	// Init
	if err := lockenv.Init(password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	
	// Create and track test file
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	if err := lockenv.TrackWithPassword([]string{testFile}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}
	
	// Try to seal with wrong password
	if err := lockenv.Seal(wrongPassword, false); err != ErrWrongPassword {
		t.Errorf("Expected ErrWrongPassword, got %v", err)
	}
	
	// Seal with correct password
	if err := lockenv.Seal(password, false); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}
	
	// Try to unseal with wrong password
	if err := lockenv.Unseal(wrongPassword); err != ErrWrongPassword {
		t.Errorf("Expected ErrWrongPassword, got %v", err)
	}
}

func TestChangePassword(t *testing.T) {
	dir := t.TempDir()
	lockenv := New(dir)
	oldPassword := []byte("oldpass")
	newPassword := []byte("newpass")
	
	// Init
	if err := lockenv.Init(oldPassword); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	
	// Create and track test file
	testFile := filepath.Join(dir, "test.txt")
	testContent := []byte("secret content")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	if err := lockenv.TrackWithPassword([]string{testFile}, oldPassword); err != nil {
		t.Fatalf("Track failed: %v", err)
	}
	
	// Seal with old password
	if err := lockenv.Seal(oldPassword, true); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}
	
	// Change password
	if err := lockenv.ChangePassword(oldPassword, newPassword); err != nil {
		t.Fatalf("Change password failed: %v", err)
	}
	
	// Try to unseal with old password (should fail)
	if err := lockenv.Unseal(oldPassword); err != ErrWrongPassword {
		t.Errorf("Expected ErrWrongPassword with old password, got %v", err)
	}
	
	// Unseal with new password
	if err := lockenv.Unseal(newPassword); err != nil {
		t.Fatalf("Unseal with new password failed: %v", err)
	}
	
	// Verify content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read unsealed file: %v", err)
	}
	if string(content) != string(testContent) {
		t.Errorf("Content mismatch: got %s, want %s", content, testContent)
	}
}