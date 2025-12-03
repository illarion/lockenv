package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestUnlockSmart_NoConflicts(t *testing.T) {
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

	// Track and seal files (with removal)
	if err := lockenv.LockFiles(context.Background(), []string{testFile1, testFile2}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}
	if err := lockenv.FinalizeLock(context.Background(), password, true); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	// Verify files were removed
	if _, err := os.Stat(testFile1); !os.IsNotExist(err) {
		t.Error("Test file 1 should have been removed")
	}

	// Unlock with StrategyUseVault
	result, err := lockenv.Unlock(context.Background(), password, StrategyUseVault, nil)
	if err != nil {
		t.Fatalf("UnlockSmart failed: %v", err)
	}

	// Verify results
	if len(result.Extracted) != 2 {
		t.Errorf("Expected 2 extracted files, got %d", len(result.Extracted))
	}
	if len(result.Skipped) != 0 {
		t.Errorf("Expected 0 skipped files, got %d", len(result.Skipped))
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d: %v", len(result.Errors), result.Errors)
	}

	// Verify file contents
	content1, err := os.ReadFile(testFile1)
	if err != nil {
		t.Fatalf("Failed to read file 1: %v", err)
	}
	if string(content1) != "content1" {
		t.Errorf("File 1 content mismatch: got %s, want content1", content1)
	}
}

func TestUnlockSmart_FilesIdentical(t *testing.T) {
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

	// Track and seal files (keep originals)
	if err := lockenv.LockFiles(context.Background(), []string{testFile1, testFile2}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}
	if err := lockenv.FinalizeLock(context.Background(), password, false); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	// Unlock - files are identical to vault versions
	result, err := lockenv.Unlock(context.Background(), password, StrategyUseVault, nil)
	if err != nil {
		t.Fatalf("UnlockSmart failed: %v", err)
	}

	// Verify results - both files should be skipped as unchanged
	if len(result.Extracted) != 0 {
		t.Errorf("Expected 0 extracted files, got %d", len(result.Extracted))
	}
	if len(result.Skipped) != 2 {
		t.Errorf("Expected 2 skipped files, got %d", len(result.Skipped))
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d: %v", len(result.Errors), result.Errors)
	}
}

func TestUnlockSmart_StrategyKeepLocal(t *testing.T) {
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

	// Create and seal file
	testFile := filepath.Join(dir, "test.txt")
	originalContent := []byte("original content")

	if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := lockenv.LockFiles(context.Background(), []string{testFile}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}
	if err := lockenv.FinalizeLock(context.Background(), password, false); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	// Modify local file
	modifiedContent := []byte("modified content")
	if err := os.WriteFile(testFile, modifiedContent, 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Unlock with StrategyKeepLocal
	result, err := lockenv.Unlock(context.Background(), password, StrategyKeepLocal, nil)
	if err != nil {
		t.Fatalf("UnlockSmart failed: %v", err)
	}

	// Verify local file was kept (not overwritten)
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	if string(content) != string(modifiedContent) {
		t.Errorf("File should have kept local content, got %s, want %s", content, modifiedContent)
	}

	// Verify results
	if len(result.Extracted) != 0 {
		t.Errorf("Expected 0 extracted files, got %d", len(result.Extracted))
	}
	if len(result.Skipped) != 1 {
		t.Errorf("Expected 1 skipped file, got %d", len(result.Skipped))
	}
}

func TestUnlockSmart_StrategyUseSealed(t *testing.T) {
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

	// Create and seal file
	testFile := filepath.Join(dir, "test.txt")
	sealedContent := []byte("sealed content")

	if err := os.WriteFile(testFile, sealedContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := lockenv.LockFiles(context.Background(), []string{testFile}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}
	if err := lockenv.FinalizeLock(context.Background(), password, false); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	// Modify local file
	localContent := []byte("local content")
	if err := os.WriteFile(testFile, localContent, 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Unlock with StrategyUseVault
	result, err := lockenv.Unlock(context.Background(), password, StrategyUseVault, nil)
	if err != nil {
		t.Fatalf("UnlockSmart failed: %v", err)
	}

	// Verify local file was overwritten with sealed content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	if string(content) != string(sealedContent) {
		t.Errorf("File should have sealed content, got %s, want %s", content, sealedContent)
	}

	// Verify results
	if len(result.Extracted) != 1 {
		t.Errorf("Expected 1 extracted file, got %d", len(result.Extracted))
	}
	if len(result.Skipped) != 0 {
		t.Errorf("Expected 0 skipped files, got %d", len(result.Skipped))
	}
}

func TestUnlockSmart_StrategyAbort(t *testing.T) {
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

	// Create and seal two files
	testFile1 := filepath.Join(dir, "test1.txt")
	testFile2 := filepath.Join(dir, "test2.txt")

	if err := os.WriteFile(testFile1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create test file 1: %v", err)
	}
	if err := os.WriteFile(testFile2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create test file 2: %v", err)
	}

	if err := lockenv.LockFiles(context.Background(), []string{testFile1, testFile2}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}
	if err := lockenv.FinalizeLock(context.Background(), password, false); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	// Modify first file to create conflict
	if err := os.WriteFile(testFile1, []byte("modified"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Unlock with StrategyAbort
	result, err := lockenv.Unlock(context.Background(), password, StrategyAbort, nil)

	// Should complete but skip conflicted file
	if err != nil {
		t.Fatalf("UnlockSmart failed: %v", err)
	}

	// Verify the conflicted file was skipped or errored
	if len(result.Errors) == 0 && len(result.Skipped) == 0 {
		t.Error("Expected conflict to be reported in Errors or Skipped")
	}
}

func TestUnlockSmart_MixedResults(t *testing.T) {
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

	// Create three files
	testFile1 := filepath.Join(dir, "file1.txt") // Will be removed (should extract)
	testFile2 := filepath.Join(dir, "file2.txt") // Keep identical (should skip)
	testFile3 := filepath.Join(dir, "file3.txt") // Modify locally (should overwrite with sealed)

	if err := os.WriteFile(testFile1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create test file 1: %v", err)
	}
	if err := os.WriteFile(testFile2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create test file 2: %v", err)
	}
	if err := os.WriteFile(testFile3, []byte("content3"), 0644); err != nil {
		t.Fatalf("Failed to create test file 3: %v", err)
	}

	// Track and seal all files
	if err := lockenv.LockFiles(context.Background(), []string{testFile1, testFile2, testFile3}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}
	if err := lockenv.FinalizeLock(context.Background(), password, false); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	// Prepare different scenarios:
	// File 1: Remove it
	if err := os.Remove(testFile1); err != nil {
		t.Fatalf("Failed to remove test file 1: %v", err)
	}

	// File 2: Keep identical (do nothing)

	// File 3: Modify locally
	if err := os.WriteFile(testFile3, []byte("modified3"), 0644); err != nil {
		t.Fatalf("Failed to modify test file 3: %v", err)
	}

	// Unlock with StrategyUseVault
	result, err := lockenv.Unlock(context.Background(), password, StrategyUseVault, nil)
	if err != nil {
		t.Fatalf("UnlockSmart failed: %v", err)
	}

	// Verify results:
	// File 1: Extracted (was missing)
	// File 2: Skipped (identical)
	// File 3: Extracted (overwritten due to StrategyUseSealed)

	if len(result.Skipped) != 1 {
		t.Errorf("Expected 1 skipped file (file2), got %d: %v", len(result.Skipped), result.Skipped)
	}
	if len(result.Extracted) != 2 {
		t.Errorf("Expected 2 extracted files (file1, file3), got %d: %v", len(result.Extracted), result.Extracted)
	}

	// Verify file3 was overwritten with sealed content
	content3, err := os.ReadFile(testFile3)
	if err != nil {
		t.Fatalf("Failed to read test file 3: %v", err)
	}
	if string(content3) != "content3" {
		t.Errorf("File 3 should have sealed content, got %s, want content3", content3)
	}
}

func TestUnlockSmart_DirectoryCreation(t *testing.T) {
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

	// Create file in nested directory
	nestedDir := filepath.Join(dir, "dir1", "dir2")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested directory: %v", err)
	}

	testFile := filepath.Join(nestedDir, "file.txt")
	if err := os.WriteFile(testFile, []byte("nested content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Track and seal
	if err := lockenv.LockFiles(context.Background(), []string{testFile}, password); err != nil {
		t.Fatalf("Track failed: %v", err)
	}
	if err := lockenv.FinalizeLock(context.Background(), password, false); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	// Remove file and directories
	if err := os.RemoveAll(filepath.Join(dir, "dir1")); err != nil {
		t.Fatalf("Failed to remove directories: %v", err)
	}

	// Unlock - should recreate directories
	result, err := lockenv.Unlock(context.Background(), password, StrategyUseVault, nil)
	if err != nil {
		t.Fatalf("UnlockSmart failed: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
		t.Error("Nested directory should have been created")
	}

	// Verify file was extracted
	if len(result.Extracted) != 1 {
		t.Errorf("Expected 1 extracted file, got %d", len(result.Extracted))
	}

	// Verify content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	if string(content) != "nested content" {
		t.Errorf("File content mismatch: got %s, want 'nested content'", content)
	}
}

func TestUnlockSmart_HashMismatch(t *testing.T) {
	// This test would require corrupting the database or sealed data
	// which is complex to do in a unit test without exposing internal details.
	// Skipping for now as it requires more invasive testing techniques.
	// In a real scenario, you would:
	// 1. Lock a file
	// 2. Manually open the database and corrupt the encrypted file data
	// 3. Attempt unlock
	// 4. Verify integrity check failure is reported in result.Errors
	t.Skip("Hash mismatch test requires database manipulation - implement if needed")
}
