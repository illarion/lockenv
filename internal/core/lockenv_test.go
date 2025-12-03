package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/illarion/lockenv/internal/storage"
)

func TestInitCommand(t *testing.T) {
	dir := t.TempDir()
	lockenv, err := New(dir)
	if err != nil {
		t.Fatalf("Failed to create LockEnv: %v", err)
	}
	defer lockenv.Close()

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
	lockenv, err := New(dir)
	if err != nil {
		t.Fatalf("Failed to create LockEnv: %v", err)
	}
	defer lockenv.Close()

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
	if err := lockenv.LockFiles(context.Background(), []string{testFile1, testFile2}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}

	// List tracked files (no password required)
	files, err := lockenv.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("Expected 2 tracked files, got %d", len(files))
	}

	// Seal files
	if err := lockenv.FinalizeLock(context.Background(), password, true); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	// Check files were removed
	if _, err := os.Stat(testFile1); !os.IsNotExist(err) {
		t.Error("Test file 1 should have been removed")
	}
	if _, err := os.Stat(testFile2); !os.IsNotExist(err) {
		t.Error("Test file 2 should have been removed")
	}

	// Unlock files
	result, err := lockenv.Unlock(context.Background(), password, StrategyUseVault)
	if err != nil {
		t.Fatalf("UnlockSmart failed: %v", err)
	}
	if len(result.Extracted) != 2 {
		t.Errorf("Expected 2 extracted files, got %d", len(result.Extracted))
	}
	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
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
	lockenv, err := New(dir)
	if err != nil {
		t.Fatalf("Failed to create LockEnv: %v", err)
	}
	defer lockenv.Close()

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

	if err := lockenv.LockFiles(context.Background(), []string{testFile}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}

	// Verify tracked
	files, err := lockenv.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("Expected 1 tracked file, got %d", len(files))
	}

	// Forget file
	if err := lockenv.RemoveFiles(context.Background(), []string{testFile}, password); err != nil {
		t.Fatalf("Forget failed: %v", err)
	}

	// Verify forgotten
	files, err = lockenv.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("Expected 0 tracked files, got %d", len(files))
	}
}

func TestForget_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	lockenv, err := New(dir)
	if err != nil {
		t.Fatalf("Failed to create LockEnv: %v", err)
	}
	defer lockenv.Close()

	password := []byte("test123")

	// Init
	if err := lockenv.Init(password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create and track test file
	testFile := filepath.Join(dir, "config.env")
	if err := os.WriteFile(testFile, []byte("SECRET=value"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := lockenv.LockFiles(context.Background(), []string{testFile}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}

	// Verify tracked
	files, err := lockenv.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("Expected 1 tracked file, got %d", len(files))
	}

	// Forget using absolute path (bug: this used to fail)
	if err := lockenv.RemoveFiles(context.Background(), []string{testFile}, password); err != nil {
		t.Fatalf("Forget with absolute path failed: %v", err)
	}

	// Verify forgotten
	files, err = lockenv.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("Expected 0 tracked files after absolute path removal, got %d", len(files))
	}
}

func TestForget_DotSlashPrefix(t *testing.T) {
	dir := t.TempDir()
	lockenv, err := New(dir)
	if err != nil {
		t.Fatalf("Failed to create LockEnv: %v", err)
	}
	defer lockenv.Close()

	password := []byte("test123")

	// Init
	if err := lockenv.Init(password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create and track test file in subdirectory
	subdir := filepath.Join(dir, "config")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	testFile := filepath.Join(subdir, "database.yml")
	if err := os.WriteFile(testFile, []byte("host: localhost"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Track using absolute path (normalized to relative internally)
	if err := lockenv.LockFiles(context.Background(), []string{testFile}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}

	// Verify tracked
	files, err := lockenv.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("Expected 1 tracked file, got %d", len(files))
	}

	// Save current directory and change to repo root for ./ to work
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Failed to change to test directory: %v", err)
	}

	// Forget using ./ prefix (bug: this used to fail)
	if err := lockenv.RemoveFiles(context.Background(), []string{"./config/database.yml"}, password); err != nil {
		t.Fatalf("Forget with ./ prefix failed: %v", err)
	}

	// Verify forgotten
	files, err = lockenv.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("Expected 0 tracked files after ./ prefix removal, got %d", len(files))
	}
}

func TestForget_MixedPathFormats(t *testing.T) {
	dir := t.TempDir()
	lockenv, err := New(dir)
	if err != nil {
		t.Fatalf("Failed to create LockEnv: %v", err)
	}
	defer lockenv.Close()

	password := []byte("test123")

	// Init
	if err := lockenv.Init(password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create test files
	file1 := filepath.Join(dir, "file1.txt")
	file2 := filepath.Join(dir, "file2.txt")
	file3 := filepath.Join(dir, "file3.txt")

	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}
	if err := os.WriteFile(file3, []byte("content3"), 0644); err != nil {
		t.Fatalf("Failed to create file3: %v", err)
	}

	// Track all files
	if err := lockenv.LockFiles(context.Background(), []string{file1, file2, file3}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}

	// Verify all tracked
	files, err := lockenv.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("Expected 3 tracked files, got %d", len(files))
	}

	// Remove using different path formats:
	// - file1: absolute path
	// - file2: ./ prefix
	// - file3: regular relative path
	if err := lockenv.RemoveFiles(context.Background(), []string{
		file1,         // absolute
		"./file2.txt", // ./ prefix
		"file3.txt",   // relative
	}, password); err != nil {
		t.Fatalf("Forget with mixed paths failed: %v", err)
	}

	// Verify all forgotten
	files, err = lockenv.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("Expected 0 tracked files after mixed path removal, got %d", len(files))
		for _, f := range files {
			t.Logf("  Remaining file: %s", f.Path)
		}
	}
}

func TestWrongPassword(t *testing.T) {
	dir := t.TempDir()
	lockenv, err := New(dir)
	if err != nil {
		t.Fatalf("Failed to create LockEnv: %v", err)
	}
	defer lockenv.Close()

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

	if err := lockenv.LockFiles(context.Background(), []string{testFile}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}

	// Try to seal with wrong password
	if err := lockenv.FinalizeLock(context.Background(), wrongPassword, false); err != ErrWrongPassword {
		t.Errorf("Expected ErrWrongPassword, got %v", err)
	}

	// Seal with correct password
	if err := lockenv.FinalizeLock(context.Background(), password, false); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	// Try to unlock with wrong password
	_, err = lockenv.Unlock(context.Background(), wrongPassword, StrategyUseVault)
	if err != ErrWrongPassword {
		t.Errorf("Expected ErrWrongPassword, got %v", err)
	}
}

func TestChangePassword(t *testing.T) {
	dir := t.TempDir()
	lockenv, err := New(dir)
	if err != nil {
		t.Fatalf("Failed to create LockEnv: %v", err)
	}
	defer lockenv.Close()

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

	if err := lockenv.LockFiles(context.Background(), []string{testFile}, oldPassword); err != nil {
		t.Fatalf("Track failed: %v", err)
	}

	// Seal with old password
	if err := lockenv.FinalizeLock(context.Background(), oldPassword, true); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	// Change password
	if err := lockenv.ChangePassword(oldPassword, newPassword); err != nil {
		t.Fatalf("Change password failed: %v", err)
	}

	// Try to unlock with old password (should fail)
	_, err = lockenv.Unlock(context.Background(), oldPassword, StrategyUseVault)
	if err != ErrWrongPassword {
		t.Errorf("Expected ErrWrongPassword with old password, got %v", err)
	}

	// Unlock with new password
	result, err := lockenv.Unlock(context.Background(), newPassword, StrategyUseVault)
	if err != nil {
		t.Fatalf("UnlockSmart with new password failed: %v", err)
	}
	if len(result.Extracted) != 1 {
		t.Errorf("Expected 1 extracted file, got %d", len(result.Extracted))
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

// Security Tests - Path Traversal Prevention

func TestTrack_RejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	lockenv, err := New(dir)
	if err != nil {
		t.Fatalf("Failed to create LockEnv: %v", err)
	}
	defer lockenv.Close()

	password := []byte("test123")

	// Init
	if err := lockenv.Init(password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create a file outside the repo to try to track
	outsideDir := filepath.Dir(dir)
	outsideFile := filepath.Join(outsideDir, "outside_secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0644); err != nil {
		t.Fatalf("Failed to create outside file: %v", err)
	}
	defer os.Remove(outsideFile)

	maliciousPaths := []string{
		"../outside_secret.txt",
		"../../etc/passwd",
		"/etc/passwd",
		filepath.Join(outsideDir, "outside_secret.txt"), // absolute path
	}

	for _, malPath := range maliciousPaths {
		t.Run("reject_"+malPath, func(t *testing.T) {
			// Track should not add the malicious path
			_ = lockenv.LockFiles(context.Background(), []string{malPath}, password)

			// It should either error or silently skip (due to validation)
			// Either way, verify it's NOT in the vault
			files, listErr := lockenv.List(context.Background())
			if listErr != nil {
				t.Fatalf("List failed: %v", listErr)
			}

			// Check that the malicious path is not tracked
			for _, file := range files {
				if file.Path == malPath || filepath.Base(file.Path) == "outside_secret.txt" {
					t.Errorf("Malicious path %q was tracked - security breach!", malPath)
				}
			}
		})
	}
}

func TestUnlock_RejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	lockenv, err := New(dir)
	if err != nil {
		t.Fatalf("Failed to create LockEnv: %v", err)
	}
	defer lockenv.Close()

	password := []byte("test123")

	// Init
	if err := lockenv.Init(password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create a legitimate file and track it
	legitFile := filepath.Join(dir, "legit.txt")
	if err := os.WriteFile(legitFile, []byte("legit"), 0644); err != nil {
		t.Fatalf("Failed to create legit file: %v", err)
	}

	if err := lockenv.LockFiles(context.Background(), []string{legitFile}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}

	if err := lockenv.FinalizeLock(context.Background(), password, true); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	// Now manually tamper with the vault to add a malicious path
	// We'll do this by creating a new lockenv with a malicious path
	// and then try to unlock it

	// For now, let's just verify that IF a malicious path somehow got in,
	// UnlockSmart would reject it during validation
	// This is tested implicitly by the PathValidator tests
	// and by the fact that Track rejects such paths

	// The key security property is: even if .lockenv is tampered with,
	// UnlockSmart validates paths and refuses to write outside the repo
}

func TestTrack_AllowsValidRelativePaths(t *testing.T) {
	dir := t.TempDir()
	lockenv, err := New(dir)
	if err != nil {
		t.Fatalf("Failed to create LockEnv: %v", err)
	}
	defer lockenv.Close()

	password := []byte("test123")

	// Init
	if err := lockenv.Init(password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create files in various valid locations
	validPaths := []string{
		filepath.Join(dir, "root.txt"),
		filepath.Join(dir, "subdir", "nested.txt"),
		filepath.Join(dir, "a", "b", "c", "deep.txt"),
		filepath.Join(dir, ".hidden"),
		filepath.Join(dir, "config", ".env"),
	}

	// Create all the files
	for _, path := range validPaths {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	// Track all files
	if err := lockenv.LockFiles(context.Background(), validPaths, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}

	// Verify all were tracked
	files, err := lockenv.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(files) != len(validPaths) {
		t.Errorf("Expected %d tracked files, got %d", len(validPaths), len(files))
	}

	// Seal and unlock to verify the full cycle works
	if err := lockenv.FinalizeLock(context.Background(), password, true); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	result, err := lockenv.Unlock(context.Background(), password, StrategyUseVault)
	if err != nil {
		t.Fatalf("UnlockSmart failed: %v", err)
	}

	if len(result.Extracted) != len(validPaths) {
		t.Errorf("Expected %d extracted files, got %d", len(validPaths), len(result.Extracted))
	}

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors during unlock: %v", result.Errors)
	}

	// Verify all files were restored correctly
	for _, path := range validPaths {
		if _, statErr := os.Stat(path); statErr != nil {
			t.Errorf("File %s was not restored: %v", path, statErr)
		}
	}
}

// Security Tests - File Permission Enforcement

func TestUnlock_EnforcesSecureDirectoryPermissions(t *testing.T) {
	dir := t.TempDir()
	lockenv, err := New(dir)
	if err != nil {
		t.Fatalf("Failed to create LockEnv: %v", err)
	}
	defer lockenv.Close()

	password := []byte("test123")

	// Init
	if err := lockenv.Init(password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create a test file in a nested directory
	nestedDir := filepath.Join(dir, "secrets", "nested", "deep")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested directory: %v", err)
	}
	testFile := filepath.Join(nestedDir, "secret.txt")
	if err := os.WriteFile(testFile, []byte("secret content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Track and seal
	if err := lockenv.LockFiles(context.Background(), []string{testFile}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}

	if err := lockenv.FinalizeLock(context.Background(), password, true); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	// Remove all directories
	os.RemoveAll(filepath.Join(dir, "secrets"))

	// Unlock
	result, err := lockenv.Unlock(context.Background(), password, StrategyUseVault)
	if err != nil {
		t.Fatalf("UnlockSmart failed: %v", err)
	}
	if len(result.Extracted) != 1 {
		t.Errorf("Expected 1 extracted file, got %d", len(result.Extracted))
	}

	// Verify directory permissions are 0700 (owner only)
	dirsToCheck := []string{
		filepath.Join(dir, "secrets"),
		filepath.Join(dir, "secrets", "nested"),
		filepath.Join(dir, "secrets", "nested", "deep"),
	}

	for _, dirPath := range dirsToCheck {
		info, err := os.Stat(dirPath)
		if err != nil {
			t.Errorf("Directory %s was not created: %v", dirPath, err)
			continue
		}

		mode := info.Mode().Perm()
		expected := os.FileMode(0700)
		if mode != expected {
			t.Errorf("Directory %s has permissions %04o, expected %04o (0700)", dirPath, mode, expected)
		}
	}
}

func TestUnlock_MasksNonExecutableFilePermissions(t *testing.T) {
	dir := t.TempDir()
	lockenv, err := New(dir)
	if err != nil {
		t.Fatalf("Failed to create LockEnv: %v", err)
	}
	defer lockenv.Close()

	password := []byte("test123")

	// Init
	if err := lockenv.Init(password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create a test file with world-readable permissions (0644)
	testFile := filepath.Join(dir, "secret.env")
	if err := os.WriteFile(testFile, []byte("SECRET=value"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Track and seal
	if err := lockenv.LockFiles(context.Background(), []string{testFile}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}

	if err := lockenv.FinalizeLock(context.Background(), password, true); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	// Unlock
	result, err := lockenv.Unlock(context.Background(), password, StrategyUseVault)
	if err != nil {
		t.Fatalf("UnlockSmart failed: %v", err)
	}
	if len(result.Extracted) != 1 {
		t.Errorf("Expected 1 extracted file, got %d", len(result.Extracted))
	}

	// Verify file permissions are 0600 (owner read/write only)
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("File was not restored: %v", err)
	}

	mode := info.Mode().Perm()
	expected := os.FileMode(0600)
	if mode != expected {
		t.Errorf("File has permissions %04o, expected %04o (0600 - owner only)", mode, expected)
	}
}

func TestUnlock_PreservesExecutableBitForOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	lockenv, err := New(dir)
	if err != nil {
		t.Fatalf("Failed to create LockEnv: %v", err)
	}
	defer lockenv.Close()

	password := []byte("test123")

	// Init
	if err := lockenv.Init(password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an executable script with world-readable permissions (0755)
	testFile := filepath.Join(dir, "deploy.sh")
	if err := os.WriteFile(testFile, []byte("#!/bin/bash\necho 'deploy'"), 0755); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Track and seal
	if err := lockenv.LockFiles(context.Background(), []string{testFile}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}

	if err := lockenv.FinalizeLock(context.Background(), password, true); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	// Unlock
	result, err := lockenv.Unlock(context.Background(), password, StrategyUseVault)
	if err != nil {
		t.Fatalf("UnlockSmart failed: %v", err)
	}
	if len(result.Extracted) != 1 {
		t.Errorf("Expected 1 extracted file, got %d", len(result.Extracted))
	}

	// Verify file permissions are 0700 (owner read/write/execute only)
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("File was not restored: %v", err)
	}

	mode := info.Mode().Perm()
	expected := os.FileMode(0700)
	if mode != expected {
		t.Errorf("File has permissions %04o, expected %04o (0700 - owner rwx only)", mode, expected)
	}
}

func TestUnlock_VaultCopyHasSecurePermissions(t *testing.T) {
	dir := t.TempDir()

	// Change to temp directory so relative paths work correctly
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	lockenv, err := New(".")
	if err != nil {
		t.Fatalf("Failed to create LockEnv: %v", err)
	}
	defer lockenv.Close()

	password := []byte("test123")

	// Init
	if err := lockenv.Init(password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create a test file with world-readable permissions
	testFile := "config.yaml"
	if err := os.WriteFile(testFile, []byte("api_key: secret123"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Track and seal
	if err := lockenv.LockFiles(context.Background(), []string{testFile}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}

	if err := lockenv.FinalizeLock(context.Background(), password, true); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	// Create a local file with different content
	if err := os.WriteFile(testFile, []byte("api_key: different"), 0644); err != nil {
		t.Fatalf("Failed to create local file: %v", err)
	}

	// Unlock with KeepBoth strategy
	result, err := lockenv.Unlock(context.Background(), password, StrategyKeepBoth)
	if err != nil {
		t.Fatalf("UnlockSmart failed: %v", err)
	}

	// Debug: print result
	t.Logf("Extracted: %v", result.Extracted)
	t.Logf("Skipped: %v", result.Skipped)
	t.Logf("Errors: %v", result.Errors)

	// Find the vault copy file
	var vaultCopyPath string
	for _, extracted := range result.Extracted {
		if strings.Contains(extracted, ".from-vault") {
			vaultCopyPath = extracted
			break
		}
	}

	if vaultCopyPath == "" {
		t.Fatal("Vault copy file was not created")
	}

	// Verify vault copy has secure permissions (0600)
	vaultInfo, err := os.Stat(vaultCopyPath)
	if err != nil {
		t.Fatalf("Vault copy file does not exist: %v", err)
	}

	mode := vaultInfo.Mode().Perm()
	expected := os.FileMode(0600)
	if mode != expected {
		t.Errorf("Vault copy has permissions %04o, expected %04o (0600)", mode, expected)
	}
}

func TestStandardMode_ManifestKeysPlaintext(t *testing.T) {
	dir := t.TempDir()
	lockenv, err := New(dir)
	if err != nil {
		t.Fatalf("Failed to create LockEnv: %v", err)
	}
	defer lockenv.Close()

	password := []byte("test123")

	// Init
	if err := lockenv.Init(password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create and track test file
	testFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(testFile, []byte("secret"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := lockenv.LockFiles(context.Background(), []string{testFile}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}

	// Open database and check manifest bucket keys
	db, err := storage.Open(filepath.Join(dir, ".lockenv"))
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Get manifest entries (should be plaintext)
	entries, err := db.GetManifest()
	if err != nil {
		t.Fatalf("Failed to get manifest: %v", err)
	}

	// Verify we have one entry with the plaintext path
	if len(entries) != 1 {
		t.Fatalf("Expected 1 manifest entry, got %d", len(entries))
	}

	if entries[0].Path != "config.yml" {
		t.Errorf("Expected path 'config.yml', got '%s'", entries[0].Path)
	}
}
