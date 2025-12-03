// Package storage provides the BBolt database interface for lockenv.
//
// Database structure uses four buckets:
//   - config: KDF parameters (salt, iterations), timestamps (unencrypted)
//   - index: File paths, sizes, modification times (unencrypted, for ls/status)
//   - blobs: Encrypted file contents
//   - private: Encrypted checksums and detailed file metadata
//
// The unencrypted index bucket enables lockenv ls and lockenv status
// to work without requiring a password, improving UX for common operations.
//
// BBolt provides ACID transactions, file locking, and corruption detection.
package storage
