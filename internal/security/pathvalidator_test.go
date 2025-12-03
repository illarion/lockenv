package security

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPathValidator_ValidateAndNormalize(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	validator, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	tests := []struct {
		name      string
		input     string
		shouldErr bool
		errType   error
	}{
		// Valid paths
		{"simple file", "test.txt", false, nil},
		{"file in subdirectory", "subdir/test.txt", false, nil},
		{"nested subdirectory", "a/b/c/test.txt", false, nil},
		{"hidden file", ".env", false, nil},
		{"hidden in subdirectory", "config/.env", false, nil},

		// Path traversal attempts
		{"parent directory", "../test.txt", true, ErrPathEscapes},
		{"parent then child", "../sibling/test.txt", true, ErrPathEscapes},
		{"nested parent", "a/../../test.txt", true, ErrPathEscapes},
		{"multiple parents", "../../etc/passwd", true, ErrPathEscapes},
		{"absolute path unix", "/etc/passwd", true, ErrAbsolutePath},

		// Empty path
		{"empty path", "", true, ErrEmptyPath},

		// Clean should normalize these
		{"dot slash", "./test.txt", false, nil},
		{"redundant slashes", "a//b///c/test.txt", false, nil},
		{"dot segments", "a/./b/./test.txt", false, nil},
	}

	// Windows-specific tests
	if runtime.GOOS == "windows" {
		tests = append(tests, []struct {
			name      string
			input     string
			shouldErr bool
			errType   error
		}{
			{"absolute path windows", "C:\\Windows\\System32\\config", true, ErrAbsolutePath},
			{"unc path", "\\\\server\\share\\file", true, ErrPathEscapes},
		}...)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.ValidateAndNormalize(tt.input)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error for input %q, got none", tt.input)
					return
				}
				if tt.errType != nil && !strings.Contains(err.Error(), tt.errType.Error()) {
					t.Errorf("Expected error type %v, got %v", tt.errType, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %q: %v", tt.input, err)
					return
				}

				// Verify result uses forward slashes
				if strings.Contains(result, "\\") {
					t.Errorf("Result should use forward slashes, got %q", result)
				}

				// Verify result doesn't start with ..
				if strings.HasPrefix(result, "..") {
					t.Errorf("Result should not start with .., got %q", result)
				}

				// Verify result is not absolute
				if filepath.IsAbs(result) {
					t.Errorf("Result should not be absolute, got %q", result)
				}
			}
		})
	}
}

func TestPathValidator_ValidateExistingPath(t *testing.T) {
	tmpDir := t.TempDir()

	validator, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	tests := []struct {
		name      string
		stored    string
		shouldErr bool
	}{
		{"normal path", "config/.env", false},
		{"forward slashes", "a/b/c/file.txt", false},
		{"path traversal", "../etc/passwd", true},
		{"absolute path", "/etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validator.ValidateExistingPath(tt.stored)

			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for stored path %q, got none", tt.stored)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Unexpected error for stored path %q: %v", tt.stored, err)
			}
		})
	}
}

func TestPathValidator_WriteFileInRoot(t *testing.T) {
	tmpDir := t.TempDir()

	validator, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	tests := []struct {
		name      string
		path      string
		data      []byte
		shouldErr bool
		needsDir  bool
	}{
		{"valid file", "test.txt", []byte("hello"), false, false},
		{"nested file", "a/b/c/test.txt", []byte("world"), false, true},
		{"path traversal attempt", "../outside.txt", []byte("bad"), true, false},
		{"absolute path", "/etc/shadow", []byte("bad"), true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create parent directory if needed
			if tt.needsDir {
				dir := filepath.Dir(tt.path)
				if err := validator.MkdirAllInRoot(dir, 0755); err != nil {
					t.Fatalf("Failed to create parent directory: %v", err)
				}
			}

			err := validator.WriteFileInRoot(tt.path, tt.data, 0644)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error when writing to %q, got none", tt.path)
					// Check that file was NOT created outside tmpDir
					absPath := filepath.Join(filepath.Dir(tmpDir), filepath.Base(tt.path))
					if _, statErr := os.Stat(absPath); statErr == nil {
						t.Errorf("File was created outside repository at %q", absPath)
						os.Remove(absPath)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error writing to %q: %v", tt.path, err)
					return
				}

				// Verify file was created inside tmpDir
				fullPath := filepath.Join(tmpDir, filepath.FromSlash(tt.path))
				content, err := os.ReadFile(fullPath)
				if err != nil {
					t.Errorf("Failed to read written file: %v", err)
					return
				}
				if string(content) != string(tt.data) {
					t.Errorf("File content mismatch: got %q, want %q", content, tt.data)
				}
			}
		})
	}
}

func TestPathValidator_MkdirAllInRoot(t *testing.T) {
	tmpDir := t.TempDir()

	validator, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	tests := []struct {
		name      string
		path      string
		shouldErr bool
	}{
		{"valid dir", "testdir", false},
		{"nested dir", "a/b/c/d", false},
		{"path traversal", "../outside", true},
		{"absolute path", "/tmp/evil", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.MkdirAllInRoot(tt.path, 0755)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error when creating dir %q, got none", tt.path)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error creating dir %q: %v", tt.path, err)
					return
				}

				// Verify directory was created inside tmpDir
				fullPath := filepath.Join(tmpDir, filepath.FromSlash(tt.path))
				if info, statErr := os.Stat(fullPath); statErr != nil || !info.IsDir() {
					t.Errorf("Directory was not created at %q", fullPath)
				}
			}
		})
	}
}

func TestPathValidator_ReadFileInRoot(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("test content")
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a subdirectory with a file
	subdir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}
	subdirFile := filepath.Join(subdir, "nested.txt")
	if err := os.WriteFile(subdirFile, []byte("nested"), 0644); err != nil {
		t.Fatalf("Failed to create nested file: %v", err)
	}

	validator, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	tests := []struct {
		name      string
		path      string
		expected  string
		shouldErr bool
	}{
		{"valid file", "test.txt", "test content", false},
		{"nested file", "subdir/nested.txt", "nested", false},
		{"nonexistent file", "missing.txt", "", true},
		{"path traversal", "../outside.txt", "", true},
		{"absolute path", "/etc/passwd", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := validator.ReadFileInRoot(tt.path)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error when reading %q, got none", tt.path)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error reading %q: %v", tt.path, err)
					return
				}
				if string(data) != tt.expected {
					t.Errorf("Content mismatch: got %q, want %q", data, tt.expected)
				}
			}
		})
	}
}

func TestPathValidator_StatInRoot(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	validator, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	tests := []struct {
		name      string
		path      string
		shouldErr bool
	}{
		{"valid file", "test.txt", false},
		{"nonexistent file", "missing.txt", true},
		{"path traversal", "../outside.txt", true},
		{"absolute path", "/etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := validator.StatInRoot(tt.path)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error when stat %q, got none", tt.path)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error stat %q: %v", tt.path, err)
					return
				}
				if info == nil {
					t.Error("Expected file info, got nil")
				}
			}
		})
	}
}

// Test that os.Root actually prevents escaping
func TestPathValidator_ActualEscapePrevention(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file OUTSIDE the repo to try to overwrite
	outsideDir := filepath.Dir(tmpDir)
	targetFile := filepath.Join(outsideDir, "should_not_be_written.txt")

	// Clean up if it exists
	defer os.Remove(targetFile)

	validator, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	// Attempt to write outside using path traversal
	maliciousPath := "../should_not_be_written.txt"
	err = validator.WriteFileInRoot(maliciousPath, []byte("pwned"), 0644)

	if err == nil {
		t.Error("Expected error when trying to write outside root, got none")
	}

	// Verify the file was NOT created outside
	if _, statErr := os.Stat(targetFile); statErr == nil {
		t.Error("File was created outside repository - security breach!")
		os.Remove(targetFile)
	}

	// Verify the file was NOT created inside either
	insidePath := filepath.Join(tmpDir, "should_not_be_written.txt")
	if _, statErr := os.Stat(insidePath); statErr == nil {
		t.Error("File was created inside repository with invalid path - should have been rejected")
	}
}
