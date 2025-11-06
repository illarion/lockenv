package storage

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Bucket names
var (
	MetaBucket     = []byte("meta")
	ManifestBucket = []byte("manifest")
	FilesBucket    = []byte("files")
	MetadataBucket = []byte("metadata")
)

// Meta keys
var (
	MetaVersion  = []byte("version")
	MetaCreated  = []byte("created")
	MetaModified = []byte("modified")
	MetaSalt     = []byte("salt")
	MetaIters    = []byte("iterations")
)

// Storage provides BBolt-based storage for lockenv
type Storage struct {
	db *bolt.DB
}

// Open opens or creates a lockenv database
func Open(path string) (*Storage, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return &Storage{db: db}, nil
}

// Close closes the database
func (s *Storage) Close() error {
	return s.db.Close()
}

// Initialize creates the bucket structure for a new lockenv
func (s *Storage) Initialize() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		// Create all buckets
		for _, bucket := range [][]byte{MetaBucket, ManifestBucket, FilesBucket, MetadataBucket} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
			}
		}

		// Set version
		meta := tx.Bucket(MetaBucket)
		if err := meta.Put(MetaVersion, []byte("1")); err != nil {
			return err
		}

		// Set creation time
		now := time.Now()
		created, _ := now.MarshalBinary()
		if err := meta.Put(MetaCreated, created); err != nil {
			return err
		}
		if err := meta.Put(MetaModified, created); err != nil {
			return err
		}

		return nil
	})
}

// IsInitialized checks if the database has been initialized
func (s *Storage) IsInitialized() bool {
	var initialized bool
	s.db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket(MetaBucket)
		if meta != nil && meta.Get(MetaVersion) != nil {
			initialized = true
		}
		return nil
	})
	return initialized
}

// SetSalt stores the KDF salt
func (s *Storage) SetSalt(salt []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		meta := tx.Bucket(MetaBucket)
		return meta.Put(MetaSalt, salt)
	})
}

// GetSalt retrieves the KDF salt
func (s *Storage) GetSalt() ([]byte, error) {
	var salt []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket(MetaBucket)
		if meta == nil {
			return fmt.Errorf("meta bucket not found")
		}
		salt = meta.Get(MetaSalt)
		if salt == nil {
			return fmt.Errorf("salt not found")
		}
		// Make a copy since the slice is only valid during the transaction
		salt = append([]byte(nil), salt...)
		return nil
	})
	return salt, err
}

// SetIterations stores the KDF iterations
func (s *Storage) SetIterations(iterations uint32) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		meta := tx.Bucket(MetaBucket)
		iters := make([]byte, 4)
		iters[0] = byte(iterations >> 24)
		iters[1] = byte(iterations >> 16)
		iters[2] = byte(iterations >> 8)
		iters[3] = byte(iterations)
		return meta.Put(MetaIters, iters)
	})
}

// GetIterations retrieves the KDF iterations
func (s *Storage) GetIterations() (uint32, error) {
	var iterations uint32
	err := s.db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket(MetaBucket)
		if meta == nil {
			return fmt.Errorf("meta bucket not found")
		}
		iters := meta.Get(MetaIters)
		if iters == nil || len(iters) != 4 {
			return fmt.Errorf("iterations not found")
		}
		iterations = uint32(iters[0])<<24 | uint32(iters[1])<<16 | uint32(iters[2])<<8 | uint32(iters[3])
		return nil
	})
	return iterations, err
}

// UpdateModified updates the last modified timestamp
func (s *Storage) UpdateModified() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		meta := tx.Bucket(MetaBucket)
		now := time.Now()
		modified, _ := now.MarshalBinary()
		return meta.Put(MetaModified, modified)
	})
}

// GetModified retrieves the last modified timestamp
func (s *Storage) GetModified() (time.Time, error) {
	var modified time.Time
	err := s.db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket(MetaBucket)
		if meta == nil {
			return fmt.Errorf("meta bucket not found")
		}
		data := meta.Get(MetaModified)
		if data == nil {
			return fmt.Errorf("modified time not found")
		}
		return modified.UnmarshalBinary(data)
	})
	return modified, err
}

// ManifestEntry represents a file in the manifest
type ManifestEntry struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

// UpdateManifest updates a file entry in the manifest
func (s *Storage) UpdateManifest(path string, size int64, modTime time.Time) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		manifest := tx.Bucket(ManifestBucket)
		entry := ManifestEntry{
			Path:    path,
			Size:    size,
			ModTime: modTime,
		}
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		return manifest.Put([]byte(path), data)
	})
}

// RemoveFromManifest removes a file from the manifest
func (s *Storage) RemoveFromManifest(path string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		manifest := tx.Bucket(ManifestBucket)
		return manifest.Delete([]byte(path))
	})
}

// GetManifest returns all entries in the manifest
func (s *Storage) GetManifest() ([]ManifestEntry, error) {
	var entries []ManifestEntry
	err := s.db.View(func(tx *bolt.Tx) error {
		manifest := tx.Bucket(ManifestBucket)
		if manifest == nil {
			return nil
		}
		return manifest.ForEach(func(k, v []byte) error {
			var entry ManifestEntry
			if err := json.Unmarshal(v, &entry); err != nil {
				return err
			}
			entries = append(entries, entry)
			return nil
		})
	})
	return entries, err
}

// GetManifestEntry returns a single manifest entry
func (s *Storage) GetManifestEntry(path string) (*ManifestEntry, error) {
	var entry *ManifestEntry
	err := s.db.View(func(tx *bolt.Tx) error {
		manifest := tx.Bucket(ManifestBucket)
		if manifest == nil {
			return fmt.Errorf("manifest bucket not found")
		}
		data := manifest.Get([]byte(path))
		if data == nil {
			return nil // File not in manifest
		}
		entry = &ManifestEntry{}
		return json.Unmarshal(data, entry)
	})
	return entry, err
}

// StoreFile stores an encrypted file
func (s *Storage) StoreFile(path string, encryptedData []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		files := tx.Bucket(FilesBucket)
		return files.Put([]byte(path), encryptedData)
	})
}

// GetFile retrieves an encrypted file
func (s *Storage) GetFile(path string) ([]byte, error) {
	var data []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		files := tx.Bucket(FilesBucket)
		if files == nil {
			return fmt.Errorf("files bucket not found")
		}
		data = files.Get([]byte(path))
		if data == nil {
			return fmt.Errorf("file not found")
		}
		// Make a copy since the slice is only valid during the transaction
		data = append([]byte(nil), data...)
		return nil
	})
	return data, err
}

// RemoveFile removes a file from storage
func (s *Storage) RemoveFile(path string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		files := tx.Bucket(FilesBucket)
		return files.Delete([]byte(path))
	})
}

// StoreMetadata stores encrypted metadata
func (s *Storage) StoreMetadata(key string, encryptedData []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		metadata := tx.Bucket(MetadataBucket)
		return metadata.Put([]byte(key), encryptedData)
	})
}

// GetMetadata retrieves encrypted metadata
func (s *Storage) GetMetadata(key string) ([]byte, error) {
	var data []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		metadata := tx.Bucket(MetadataBucket)
		if metadata == nil {
			return fmt.Errorf("metadata bucket not found")
		}
		data = metadata.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("metadata not found")
		}
		// Make a copy since the slice is only valid during the transaction
		data = append([]byte(nil), data...)
		return nil
	})
	return data, err
}

// Transaction provides a way to perform multiple operations atomically
func (s *Storage) Transaction(writable bool, fn func(tx *Transaction) error) error {
	return s.db.Update(func(btx *bolt.Tx) error {
		tx := &Transaction{tx: btx}
		return fn(tx)
	})
}

// ViewTransaction provides a read-only transaction
func (s *Storage) ViewTransaction(fn func(tx *Transaction) error) error {
	return s.db.View(func(btx *bolt.Tx) error {
		tx := &Transaction{tx: btx}
		return fn(tx)
	})
}

// Transaction wraps a bolt transaction
type Transaction struct {
	tx *bolt.Tx
}

// GetTrackedFiles returns all tracked file paths from the manifest
func (s *Storage) GetTrackedFiles() ([]string, error) {
	var files []string
	err := s.db.View(func(tx *bolt.Tx) error {
		manifest := tx.Bucket(ManifestBucket)
		if manifest == nil {
			return nil
		}
		return manifest.ForEach(func(k, v []byte) error {
			files = append(files, string(k))
			return nil
		})
	})
	return files, err
}