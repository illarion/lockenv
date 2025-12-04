package storage

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Bucket names
var (
	ConfigBucket  = []byte("config")  // KDF params (salt, iterations), timestamps - unencrypted
	IndexBucket   = []byte("index")   // Public file list for ls/status - unencrypted
	BlobsBucket   = []byte("blobs")   // Encrypted file contents
	PrivateBucket = []byte("private") // Encrypted checksum + file details
)

// Config keys
var (
	ConfigVersion  = []byte("version")
	ConfigCreated  = []byte("created")
	ConfigModified = []byte("modified")
	ConfigSalt     = []byte("salt")
	ConfigIters    = []byte("iterations")
	ConfigVaultID  = []byte("vault_id")
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
		for _, bucket := range [][]byte{ConfigBucket, IndexBucket, BlobsBucket, PrivateBucket} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
			}
		}

		// Set version
		config := tx.Bucket(ConfigBucket)
		if err := config.Put(ConfigVersion, []byte("1")); err != nil {
			return err
		}

		// Set creation time
		now := time.Now()
		created, _ := now.MarshalBinary()
		if err := config.Put(ConfigCreated, created); err != nil {
			return err
		}
		if err := config.Put(ConfigModified, created); err != nil {
			return err
		}

		return nil
	})
}

// IsInitialized checks if the database has been initialized
func (s *Storage) IsInitialized() (bool, error) {
	var initialized bool
	err := s.db.View(func(tx *bolt.Tx) error {
		config := tx.Bucket(ConfigBucket)
		if config != nil && config.Get(ConfigVersion) != nil {
			initialized = true
		}
		return nil
	})
	return initialized, err
}

// SetSalt stores the KDF salt
func (s *Storage) SetSalt(salt []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		config := tx.Bucket(ConfigBucket)
		return config.Put(ConfigSalt, salt)
	})
}

// GetSalt retrieves the KDF salt
func (s *Storage) GetSalt() ([]byte, error) {
	var salt []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		config := tx.Bucket(ConfigBucket)
		if config == nil {
			return fmt.Errorf("config bucket not found")
		}
		salt = config.Get(ConfigSalt)
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
		config := tx.Bucket(ConfigBucket)
		iters := make([]byte, 4)
		binary.BigEndian.PutUint32(iters, iterations)
		return config.Put(ConfigIters, iters)
	})
}

// GetIterations retrieves the KDF iterations
func (s *Storage) GetIterations() (uint32, error) {
	var iterations uint32
	err := s.db.View(func(tx *bolt.Tx) error {
		config := tx.Bucket(ConfigBucket)
		if config == nil {
			return fmt.Errorf("config bucket not found")
		}
		iters := config.Get(ConfigIters)
		if iters == nil || len(iters) != 4 {
			return fmt.Errorf("iterations not found")
		}
		iterations = binary.BigEndian.Uint32(iters)
		return nil
	})
	return iterations, err
}

// UpdateModified updates the last modified timestamp
func (s *Storage) UpdateModified() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		config := tx.Bucket(ConfigBucket)
		now := time.Now()
		modified, _ := now.MarshalBinary()
		return config.Put(ConfigModified, modified)
	})
}

// GetModified retrieves the last modified timestamp
func (s *Storage) GetModified() (time.Time, error) {
	var modified time.Time
	err := s.db.View(func(tx *bolt.Tx) error {
		config := tx.Bucket(ConfigBucket)
		if config == nil {
			return fmt.Errorf("config bucket not found")
		}
		data := config.Get(ConfigModified)
		if data == nil {
			return fmt.Errorf("modified time not found")
		}
		return modified.UnmarshalBinary(data)
	})
	return modified, err
}

// GetVaultID retrieves the vault ID from config bucket
func (s *Storage) GetVaultID() (string, error) {
	var vaultID string
	err := s.db.View(func(tx *bolt.Tx) error {
		config := tx.Bucket(ConfigBucket)
		if config == nil {
			return fmt.Errorf("config bucket not found")
		}
		data := config.Get(ConfigVaultID)
		if data == nil {
			return fmt.Errorf("vault_id not found")
		}
		vaultID = string(data)
		return nil
	})
	return vaultID, err
}

// GetOrCreateVaultID retrieves existing vault ID or generates a new one
func (s *Storage) GetOrCreateVaultID() (string, error) {
	vaultID, err := s.GetVaultID()
	if err == nil {
		return vaultID, nil
	}

	// Generate new UUID
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate vault ID: %w", err)
	}
	vaultID = hex.EncodeToString(b)

	// Store it
	err = s.db.Update(func(tx *bolt.Tx) error {
		config := tx.Bucket(ConfigBucket)
		return config.Put(ConfigVaultID, []byte(vaultID))
	})
	if err != nil {
		return "", err
	}

	return vaultID, nil
}

// ManifestEntry represents a file in the manifest
type ManifestEntry struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	Hash    string    `json:"hash"` // Content hash for change detection
}

// UpdateManifest updates a file entry in the manifest
func (s *Storage) UpdateManifest(path string, size int64, modTime time.Time, hash string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		manifest := tx.Bucket(IndexBucket)
		entry := ManifestEntry{
			Path:    path,
			Size:    size,
			ModTime: modTime,
			Hash:    hash,
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
		manifest := tx.Bucket(IndexBucket)
		return manifest.Delete([]byte(path))
	})
}

// GetManifest returns all entries in the manifest
func (s *Storage) GetManifest() ([]ManifestEntry, error) {
	var entries []ManifestEntry
	err := s.db.View(func(tx *bolt.Tx) error {
		manifest := tx.Bucket(IndexBucket)
		if manifest == nil {
			return fmt.Errorf("index bucket not found")
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
		manifest := tx.Bucket(IndexBucket)
		if manifest == nil {
			return fmt.Errorf("index bucket not found")
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

// StoreFileData stores encrypted file data
func (s *Storage) StoreFileData(path string, encryptedData []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		blobs := tx.Bucket(BlobsBucket)
		return blobs.Put([]byte(path), encryptedData)
	})
}

// GetFileData retrieves encrypted file data
func (s *Storage) GetFileData(path string) ([]byte, error) {
	var data []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		blobs := tx.Bucket(BlobsBucket)
		if blobs == nil {
			return fmt.Errorf("blobs bucket not found")
		}
		data = blobs.Get([]byte(path))
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
		blobs := tx.Bucket(BlobsBucket)
		return blobs.Delete([]byte(path))
	})
}

// StoreMetadataBytes stores encrypted metadata bytes
func (s *Storage) StoreMetadataBytes(key string, encryptedData []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		private := tx.Bucket(PrivateBucket)
		return private.Put([]byte(key), encryptedData)
	})
}

// GetMetadataBytes retrieves encrypted metadata bytes
func (s *Storage) GetMetadataBytes(key string) ([]byte, error) {
	var data []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		private := tx.Bucket(PrivateBucket)
		if private == nil {
			return fmt.Errorf("private bucket not found")
		}
		data = private.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("metadata not found")
		}
		// Make a copy since the slice is only valid during the transaction
		data = append([]byte(nil), data...)
		return nil
	})
	return data, err
}

// GetTrackedFiles returns all tracked file paths from the manifest
func (s *Storage) GetTrackedFiles() ([]string, error) {
	var files []string
	err := s.db.View(func(tx *bolt.Tx) error {
		manifest := tx.Bucket(IndexBucket)
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

// Compact creates a compacted copy of the database, removing unused space.
// This is useful after deleting files to reclaim disk space.
func (s *Storage) Compact() error {
	srcPath := s.db.Path()
	tmpPath := srcPath + ".compact"

	// Create new database
	dst, err := bolt.Open(tmpPath, 0600, nil)
	if err != nil {
		return fmt.Errorf("failed to create compact database: %w", err)
	}

	// Copy all buckets
	err = s.db.View(func(srcTx *bolt.Tx) error {
		return dst.Update(func(dstTx *bolt.Tx) error {
			return srcTx.ForEach(func(name []byte, srcBucket *bolt.Bucket) error {
				dstBucket, err := dstTx.CreateBucketIfNotExists(name)
				if err != nil {
					return err
				}
				return srcBucket.ForEach(func(k, v []byte) error {
					return dstBucket.Put(k, v)
				})
			})
		})
	})

	if err != nil {
		dst.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to copy data: %w", err)
	}

	if err := dst.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close compact database: %w", err)
	}

	if err := s.db.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close source database: %w", err)
	}

	// Atomic replace
	backupPath := srcPath + ".backup"
	if err := os.Rename(srcPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup original: %w", err)
	}
	if err := os.Rename(tmpPath, srcPath); err != nil {
		os.Rename(backupPath, srcPath) // rollback
		return fmt.Errorf("failed to replace database: %w", err)
	}
	os.Remove(backupPath)

	// Reopen database
	s.db, err = bolt.Open(srcPath, 0600, nil)
	if err != nil {
		return fmt.Errorf("failed to reopen database: %w", err)
	}

	return nil
}
