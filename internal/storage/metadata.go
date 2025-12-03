package storage

import (
	"time"
)

// Metadata represents the encrypted metadata for tracked files
type Metadata struct {
	Version  int         `json:"version"`
	Created  time.Time   `json:"created"`
	Modified time.Time   `json:"modified"`
	Checksum string      `json:"checksum"`
	Files    []FileEntry `json:"files"`
}

// FileEntry represents a file entry in the metadata
type FileEntry struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	Mode    uint32    `json:"mode"`
	ModTime time.Time `json:"modTime"`
	Hash    string    `json:"hash"`
}

// NewMetadata creates a new metadata structure
func NewMetadata() *Metadata {
	now := time.Now()
	return &Metadata{
		Version:  1,
		Created:  now,
		Modified: now,
		Files:    make([]FileEntry, 0),
	}
}

// AddFile adds or updates a file entry in the metadata
func (m *Metadata) AddFile(entry FileEntry) {
	if entry.Size < 0 {
		entry.Size = 0
	}
	// Update existing entry if present
	for i := range m.Files {
		if m.Files[i].Path == entry.Path {
			m.Files[i] = entry
			m.Modified = time.Now()
			return
		}
	}
	// Not found, append new entry
	m.Files = append(m.Files, entry)
	m.Modified = time.Now()
}

// RemoveFile removes a file entry from the metadata
func (m *Metadata) RemoveFile(path string) bool {
	for i, f := range m.Files {
		if f.Path == path {
			m.Files = append(m.Files[:i], m.Files[i+1:]...)
			m.Modified = time.Now()
			return true
		}
	}
	return false
}

// FindFile finds a file entry by path
func (m *Metadata) FindFile(path string) *FileEntry {
	for i := range m.Files {
		if m.Files[i].Path == path {
			return &m.Files[i]
		}
	}
	return nil
}

// GetTrackedPaths returns all tracked file paths
func (m *Metadata) GetTrackedPaths() []string {
	paths := make([]string, len(m.Files))
	for i, f := range m.Files {
		paths[i] = f.Path
	}
	return paths
}