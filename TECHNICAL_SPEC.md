# lockenv Technical Specification

## Architecture Overview

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   CLI       │────▶│   Core       │────▶│   Crypto    │
│  Commands   │     │   Logic      │     │   Module    │
└─────────────┘     └──────────────┘     └─────────────┘
                            │
                            ▼
                    ┌──────────────┐
                    │   BBolt DB   │
                    │  (.lockenv)  │
                    └──────────────┘
```

## Package Structure

```
lockenv/
├── main.go              # CLI entry point
├── cmd/                 # CLI command implementations
│   ├── init.go
│   ├── lock.go
│   ├── unlock.go
│   ├── rm.go
│   ├── ls.go
│   ├── passwd.go
│   ├── status.go
│   ├── diff.go
│   └── common.go
├── internal/
│   ├── core/           # Core business logic
│   │   ├── lockenv.go  # Main lockenv operations
│   │   └── password.go # Password handling
│   ├── crypto/         # Cryptographic operations
│   │   └── crypto.go   # Encryption/decryption
│   └── storage/        # BBolt database interface
│       ├── bbolt.go    # BBolt storage implementation
│       └── metadata.go # Metadata structures
```

## Core Components

### 1. Storage Module (`internal/storage`)

Uses BBolt (etcd's fork of BoltDB) as an embedded key-value database.

#### Database Structure
```
.lockenv (BBolt database)
├── config bucket (unencrypted)
│   ├── version      → "1"
│   ├── created      → timestamp (binary)
│   ├── modified     → timestamp (binary)
│   ├── salt         → 32 bytes
│   └── iterations   → uint32
├── index bucket (unencrypted)
│   └── [file_path]  → ManifestEntry (JSON)
├── blobs bucket (encrypted values)
│   └── [file_path]  → encrypted file content
└── private bucket (encrypted)
    ├── checksum     → encrypted password verification
    └── files        → encrypted Metadata struct (JSON)
```

#### Key Types
```go
// ManifestEntry - stored unencrypted in manifest bucket
type ManifestEntry struct {
    Path    string    `json:"path"`
    Size    int64     `json:"size"`
    ModTime time.Time `json:"modTime"`
}
// ManifestEntry data is visible without password.
// Anyone with repository read access can see file paths, sizes, and times
// by running `lockenv ls` or `lockenv status`.

// Metadata - stored encrypted with full file details
type Metadata struct {
    Version  int         `json:"version"`
    Created  time.Time   `json:"created"`
    Modified time.Time   `json:"modified"`
    Checksum string      `json:"checksum"`
    Files    []FileEntry `json:"files"`
}

// FileEntry - detailed file information
type FileEntry struct {
    Path    string    `json:"path"`
    Size    int64     `json:"size"`
    Mode    uint32    `json:"mode"`
    ModTime time.Time `json:"modTime"`
    Hash    string    `json:"hash"`
}
```

### 2. Crypto Module (`internal/crypto`)

#### Key Derivation Function
```go
type KDF struct {
    Salt       []byte  // 32 bytes, randomly generated
    Iterations int     // 210,000 (PBKDF2)
}

func (k *KDF) DeriveKey(password []byte) []byte {
    return pbkdf2.Key(password, k.Salt, k.Iterations, 32, sha256.New)
}
```

#### Encryption
- Algorithm: AES-256-GCM
- Key size: 32 bytes (256 bits)
- Nonce size: 12 bytes (unique per encryption)
- Tag size: 16 bytes (authentication)

Each encrypted value is stored as:
```
[12-byte nonce][ciphertext + 16-byte auth tag]
```

### 3. Core Module (`internal/core`)

Main operations flow:

#### Init
```
1. Create BBolt database file
2. Initialize bucket structure
3. Generate random salt (32 bytes)
4. Store salt and iterations in meta bucket
5. Derive key using PBKDF2
6. Encrypt and store password checksum
7. Create empty metadata
```

#### Lock
```
1. Open database and verify password
2. Read current metadata (encrypted)
3. Add file entries to metadata
4. For each file:
   - Read file contents
   - Calculate SHA-256 hash
   - Encrypt contents with unique nonce
   - Store in files bucket
   - Update manifest with file info (JSON)
5. Update metadata with hashes
6. Update modification timestamp
7. Optionally remove original files
```

#### Unlock
```
1. Open database and verify password
2. Read metadata to get file list
3. For each file:
   - Read encrypted data from files bucket
   - Decrypt contents
   - Verify SHA-256 hash
   - Check if file exists locally
   - If exists and differs, handle conflict (keep local, use vault, or prompt)
   - Write to disk with original permissions
   - Restore modification time
4. Return result with extracted, skipped, and error counts
```

## Security Model

### Password Verification
- A known string is encrypted with the user's key
- Stored in metadata bucket as "checksum"
- Used to verify correct password on operations

### Encryption Details
- Each file encrypted independently
- Unique nonce per encryption operation
- Authentication tag prevents tampering
- File hashes verify integrity after decryption

### BBolt Benefits
- ACID transactions ensure consistency
- File locking prevents concurrent access
- Built-in corruption detection
- Efficient B+tree structure

## Error Handling

### Security Errors
- Invalid password: Decryption fails on checksum
- Tampered data: GCM authentication fails
- Corrupt database: BBolt consistency checks

### Operational Errors
- File permissions: Checked before operations
- Missing files: Warning printed, operation continues
- Database locked: BBolt handles with timeout

## Performance Considerations

### BBolt Advantages
- Memory-mapped file for fast access
- Efficient key/value lookups
- Minimal overhead for small files
- Scales well with many files

### Memory Usage
- Files loaded entirely for encryption
- Sensitive data cleared after use
- BBolt manages page cache efficiently

## Testing Strategy

### Unit Tests
```go
// storage_test.go
func TestBBoltOperations(t *testing.T)
func TestManifestOperations(t *testing.T)

// crypto_test.go
func TestEncryption(t *testing.T)
func TestKeyDerivation(t *testing.T)

// lockenv_test.go
func TestInitCommand(t *testing.T)
func TestLockUnlock(t *testing.T)
```

### Integration Tests
- Full workflow tests
- Password change scenarios
- Error condition handling
- Large file handling

## CLI Examples

```bash
# Initialize
$ lockenv init
Enter password:
Confirm password:
initialized: .lockenv

# Lock files (encrypt and store)
$ lockenv lock .env config/*.json --remove
Enter password:
locking: .env
locking: config/app.json
locking: config/db.json
encrypted: .env
encrypted: config/app.json
encrypted: config/db.json
removed: .env
removed: config/app.json
removed: config/db.json
locked: 3 files into .lockenv

# List files (no password required)
$ lockenv ls
Files in .lockenv:
  .env (45 bytes)
  config/app.json (1.2 KB)
  config/db.json (523 bytes)

# Unlock (decrypt and restore)
$ lockenv unlock
Enter password:
unlocked: .env
unlocked: config/app.json
unlocked: config/db.json

# Inspect with BBolt CLI
$ bbolt buckets .lockenv
blobs
config
index
private

$ bbolt keys .lockenv index
.env
config/app.json
config/db.json
```

## Future Enhancements

1. **Compression**
   - Compress before encryption
   - Configurable compression levels

2. **Selective Operations**
   - Unlock specific files only
   - Pattern-based unlocking

3. **Cloud Backup**
   - Export/import commands
   - Remote storage integration

4. **Team Features**
   - Multiple passwords (key wrapping)
   - Access control per file