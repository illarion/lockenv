package core

import (
	"testing"
)

func TestDetectFileType_Text(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    bool
	}{
		{
			name:    "plain ASCII text",
			content: []byte("Hello, World!\nThis is a test."),
			want:    true,
		},
		{
			name:    "UTF-8 with special chars",
			content: []byte("Hello 世界! Ñoño café"),
			want:    true,
		},
		{
			name:    "empty file",
			content: []byte(""),
			want:    true,
		},
		{
			name:    "newlines and spaces",
			content: []byte("\n\n  \t  \n"),
			want:    true,
		},
		{
			name:    "JSON content",
			content: []byte(`{"key": "value", "number": 123}`),
			want:    true,
		},
		{
			name:    "code with symbols",
			content: []byte("func main() {\n\tfmt.Println(\"test\")\n}"),
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFileType(tt.content)
			if got != tt.want {
				t.Errorf("DetectFileType() for %s = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestDetectFileType_Binary(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    bool
	}{
		{
			name:    "content with null bytes",
			content: []byte("Hello\x00World"),
			want:    false,
		},
		{
			name:    "random binary data",
			content: []byte{0xFF, 0xFE, 0x00, 0x01, 0xAB, 0xCD},
			want:    false,
		},
		{
			name:    "non-UTF-8 sequences",
			content: []byte{0x80, 0x81, 0x82, 0x83, 0x84},
			want:    false,
		},
		{
			name:    "binary with lots of non-printable",
			content: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFileType(tt.content)
			if got != tt.want {
				t.Errorf("DetectFileType() for %s = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestCompareFiles_Identical(t *testing.T) {
	tests := []struct {
		name     string
		content1 []byte
		content2 []byte
	}{
		{
			name:     "identical text",
			content1: []byte("Hello, World!"),
			content2: []byte("Hello, World!"),
		},
		{
			name:     "identical empty files",
			content1: []byte(""),
			content2: []byte(""),
		},
		{
			name:     "identical binary data",
			content1: []byte{0x00, 0x01, 0x02, 0xFF},
			content2: []byte{0x00, 0x01, 0x02, 0xFF},
		},
		{
			name:     "identical multiline text",
			content1: []byte("line1\nline2\nline3"),
			content2: []byte("line1\nline2\nline3"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !CompareFiles(tt.content1, tt.content2) {
				t.Errorf("CompareFiles() for %s should return true for identical content", tt.name)
			}
		})
	}
}

func TestCompareFiles_Different(t *testing.T) {
	tests := []struct {
		name     string
		content1 []byte
		content2 []byte
	}{
		{
			name:     "different text",
			content1: []byte("data1"),
			content2: []byte("data2"),
		},
		{
			name:     "different length",
			content1: []byte("short"),
			content2: []byte("much longer content"),
		},
		{
			name:     "empty vs non-empty",
			content1: []byte(""),
			content2: []byte("content"),
		},
		{
			name:     "different binary data",
			content1: []byte{0x00, 0x01},
			content2: []byte{0x00, 0x02},
		},
		{
			name:     "case difference",
			content1: []byte("Hello"),
			content2: []byte("hello"),
		},
		{
			name:     "whitespace difference",
			content1: []byte("Hello World"),
			content2: []byte("Hello  World"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if CompareFiles(tt.content1, tt.content2) {
				t.Errorf("CompareFiles() for %s should return false for different content", tt.name)
			}
		})
	}
}

func TestCreateLineDiff_SingleLineChange(t *testing.T) {
	local := []byte("line1\nline2\nline3\n")
	vault := []byte("line1\nmodified\nline3\n")

	result := createLineDiff(local, vault)

	// Should contain unchanged lines and only the difference in markers
	resultStr := string(result)
	if !contains(resultStr, "line1\n") {
		t.Error("Result should contain unchanged line1")
	}
	if !contains(resultStr, "line3\n") {
		t.Error("Result should contain unchanged line3")
	}
	if !contains(resultStr, "<<<<<<< local") {
		t.Error("Result should contain local marker")
	}
	if !contains(resultStr, "=======") {
		t.Error("Result should contain separator")
	}
	if !contains(resultStr, ">>>>>>> vault") {
		t.Error("Result should contain vault marker")
	}
	if !contains(resultStr, "line2") {
		t.Error("Result should contain local line2")
	}
	if !contains(resultStr, "modified") {
		t.Error("Result should contain vault modified line")
	}
}

func TestCreateLineDiff_IdenticalFiles(t *testing.T) {
	content := []byte("line1\nline2\nline3\n")

	result := createLineDiff(content, content)

	resultStr := string(result)
	// Identical files should have no conflict markers
	if contains(resultStr, "<<<<<<<") {
		t.Error("Identical files should not have conflict markers")
	}
	if contains(resultStr, "=======") {
		t.Error("Identical files should not have separator")
	}
	if contains(resultStr, ">>>>>>>") {
		t.Error("Identical files should not have vault marker")
	}
	// Content should be preserved
	if resultStr != string(content) {
		t.Errorf("Identical files should return same content.\nGot: %q\nWant: %q", resultStr, string(content))
	}
}

func TestCreateLineDiff_MultipleChanges(t *testing.T) {
	local := []byte("line1\nline2\nline3\nline4\nline5\n")
	vault := []byte("line1\nchanged2\nline3\nchanged4\nline5\n")

	result := createLineDiff(local, vault)
	resultStr := string(result)

	// Should have multiple conflict sections
	count := countOccurrences(resultStr, "<<<<<<< local")
	if count != 2 {
		t.Errorf("Expected 2 conflict sections, got %d", count)
	}
}

func TestCreateLineDiff_AddedLines(t *testing.T) {
	local := []byte("line1\nline2\n")
	vault := []byte("line1\nline2\nline3\n")

	result := createLineDiff(local, vault)
	resultStr := string(result)

	// Should show added line in conflict
	if !contains(resultStr, "<<<<<<< local") {
		t.Error("Result should contain conflict markers for addition")
	}
	if !contains(resultStr, "line3") {
		t.Error("Result should contain added line3")
	}
}

func TestCreateLineDiff_RemovedLines(t *testing.T) {
	local := []byte("line1\nline2\nline3\n")
	vault := []byte("line1\nline3\n")

	result := createLineDiff(local, vault)
	resultStr := string(result)

	// Should show removed line in conflict
	if !contains(resultStr, "<<<<<<< local") {
		t.Error("Result should contain conflict markers for removal")
	}
	if !contains(resultStr, "line2") {
		t.Error("Result should contain removed line2 in local section")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func countOccurrences(s, substr string) int {
	count := 0
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			count++
		}
	}
	return count
}
