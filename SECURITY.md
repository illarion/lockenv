# lockenv Security Design

## Overview

lockenv is a CLI tool for securely storing sensitive files in an encrypted container that can be safely committed to version control. This document describes the security architecture and implementation details.

## Threat Model

### What We Protect Against
- **Unauthorized access**: Files are encrypted with AES-256-GCM
- **Password brute-forcing**: PBKDF2 with 210,000 iterations
- **Data tampering**: Authenticated encryption prevents modification
- **Content confidentiality**: All file contents are encrypted with AES-256-GCM
- **Memory disclosure**: Sensitive data is cleared after use
- **Database corruption**: BBolt provides ACID guarantees

### What We Don't Protect Against
- **Weak passwords**: Users must choose strong passwords
- **Compromised system**: If the system is compromised during unlocking
- **Side-channel attacks**: Not designed for hostile local environments
- **Key loggers**: Password entry can be captured
- **Metadata visibility**: File paths, sizes, and modification times are visible without authentication. Use generic file names if this is a concern.

## Cryptographic Design

### Encryption Algorithm
**AES-256-GCM** (Galois/Counter Mode)
- Provides authenticated encryption with associated data (AEAD)
- 256-bit key size for strong security
- Built-in integrity protection
- Standard library support in Go

### Key Derivation
**PBKDF2-HMAC-SHA256**
- Derives encryption key from user password
- Parameters:
  - Salt: 32 bytes (unique per .lockenv file)
  - Iterations: 210,000 (OWASP minimum recommendation)
  - Key length: 32 bytes (256 bits)
- Slows down brute-force attacks

### Random Number Generation
- Uses `crypto/rand` for all random values
- Generates unique salts and nonces
- Cryptographically secure

## Storage Architecture

### BBolt Database Structure
lockenv uses BBolt (etcd's fork of BoltDB) as an embedded key-value database:

```
.lockenv (BBolt database)
├── config bucket (unencrypted)
│   ├── version     → "1"
│   ├── created     → timestamp
│   ├── modified    → timestamp
│   ├── salt        → 32 bytes (for PBKDF2)
│   └── iterations  → uint32 (PBKDF2 iterations)
├── index bucket (unencrypted)
│   └── [file_path] → {path, size, modTime} (JSON)
├── blobs bucket (encrypted values)
│   └── [file_path] → [nonce][ciphertext][tag]
└── private bucket (encrypted)
    ├── checksum    → encrypted password verification
    └── files       → encrypted file details (JSON)
```

### Security Properties of BBolt Storage

**Advantages over custom binary format:**
- ACID transactions prevent partial writes
- Built-in corruption detection via checksums
- File locking prevents concurrent access
- Efficient B+tree structure with O(log n) lookups
- Memory-mapped for performance without compromising security

### Encryption Format

Each encrypted value in BBolt:
```
[12-byte nonce][ciphertext][16-byte auth tag]
```
- Nonce: Unique per encryption operation
- Ciphertext: AES-256-GCM encrypted data
- Auth tag: Prevents tampering

## Security Properties

### Confidentiality

**File Contents:**
- All file contents encrypted with AES-256-GCM
- File hashes and detailed metadata encrypted in private bucket
- No information leakage about file contents

**Index (Unencrypted):**
- File paths, sizes, and modification times stored unencrypted in index bucket
- Anyone with repository read access can enumerate tracked files using `lockenv ls` or `lockenv status`
- If file paths are sensitive, use generic names (e.g., `config1.enc`)

### Integrity
- GCM mode provides authenticated encryption
- BBolt checksums detect database corruption
- SHA-256 hashes verify file integrity after decryption
- Any tampering detected during operations

### Authentication
- Password verification through encrypted checksum
- Prevents oracle attacks on file decryption
- Wrong password fails fast without attempting file decryption

### Forward Secrecy
- Each .lockenv file has unique salt
- Compromising one file doesn't affect others
- Password changes re-encrypt everything with new salt

## Memory Management
lockenv actively clears sensitive data from memory to minimize exposure:

**What gets cleared:**
- **Passwords**: Cleared immediately after key derivation using `crypto.ClearBytes`
- **Encryption keys**: Cleared when Encryptor is destroyed (via `defer`)
- **File contents**: Cleared after encryption (lock) or after writing to disk (unlock)
- **Decrypted data**: Cleared in all code paths including error paths
- **Merged data**: Cleared after conflict resolution
- **Local file contents**: Cleared after comparison during unlock

**When it's cleared:**
- After each file is processed in lock/unlock operations
- On error paths using explicit clearing before returns
- Using defer patterns to ensure cleanup on all exit paths
- Immediately after data is no longer needed

**Memory exposure window:**
- Lock: From file read to encryption completion (~milliseconds)
- Unlock: From decryption to disk write (~milliseconds)
- Conflict resolution: During user interaction (editor session can be longer)
- Change password: During re-encryption of all files

**Security guarantees:**
- No sensitive data persists after function completion
- Early clearing on error paths prevents leakage
- Reduced window for process memory dump attacks

## Implementation Guidelines

### Password Handling
```go
// CORRECT: Use []byte for passwords
password := []byte(readPassword())
defer crypto.ClearBytes(password)

// WRONG: Don't use strings
password := readPassword() // string
```

### Memory Safety
1. Clear all sensitive data after use
2. Use `defer` for cleanup
3. Avoid string types for sensitive data
4. Use `subtle.ConstantTimeCompare` for comparisons

### BBolt Transaction Safety
```go
// CORRECT: All operations in transaction
db.Update(func(tx *bolt.Tx) error {
    // All related operations here
    return nil
})

// WRONG: Multiple separate transactions
db.Update(func(tx *bolt.Tx) error { /* op1 */ })
db.Update(func(tx *bolt.Tx) error { /* op2 */ })
```

### Error Handling
- Don't leak information in error messages
- Fail early and securely
- Log security events appropriately

### Example: Secure Storage Operation
```go
func (s *Storage) StoreFile(path string, encData []byte) error {
    return s.db.Update(func(tx *bolt.Tx) error {
        blobs := tx.Bucket(BlobsBucket)
        if blobs == nil {
            return errors.New("blobs bucket not found")
        }
        return blobs.Put([]byte(path), encData)
    })
}
```

## Security Checklist

### Development
- [x] Use Go's `crypto` packages only
- [x] Never implement custom cryptography
- [x] Clear sensitive memory after use
- [x] Use constant-time comparisons
- [x] Validate all inputs
- [x] Handle errors securely
- [x] Use BBolt transactions for atomicity

### Operations
- [ ] Use strong passwords (recommend 20+ characters)
- [ ] Store .lockenv files in version control
- [ ] Never commit unencrypted files
- [ ] Regularly update lockenv tool
- [ ] Monitor for security advisories
- [ ] Use `bbolt check` to verify database integrity

### Testing
- [ ] Test with various file sizes
- [ ] Verify tamper detection works
- [ ] Check memory is cleared properly
- [ ] Test error conditions
- [ ] Verify no information leakage
- [ ] Test database corruption recovery

## BBolt Security Considerations

### Unencrypted Index

The index bucket stores file paths, sizes, and modification times **unencrypted** to enable:
- `lockenv ls` without password - useful for discovering what files are tracked
- `lockenv status` without password - shows which tracked files have changed
- Better user experience for routine operations

**Security implications:**
- Anyone with repository read access can run `lockenv ls` and `lockenv status`
- This reveals which files are being encrypted and their sizes
- File paths may expose information about your application structure
- Modification times may reveal when secrets were last updated

**What remains protected:**
- All file contents are encrypted
- File hashes stored in private bucket (encrypted)
- The actual secret values are never exposed

**Mitigation:**
Use generic file names (e.g., `config1.enc`, `config2.enc`) if file paths are sensitive.

**Comparison:** This is similar to `ls -la` output - you can see file names and sizes, but not the contents.

### Salt Storage
The salt is stored unencrypted in the config bucket because:
- Salt is not secret (only adds uniqueness)
- Required for key derivation before decryption
- Standard practice in all encryption systems

## References

1. [NIST SP 800-38D](https://nvlpubs.nist.gov/nistpubs/Legacy/SP/nistspecialpublication800-38d.pdf) - GCM Mode
2. [RFC 2898](https://tools.ietf.org/html/rfc2898) - PBKDF2
3. [Go Crypto Documentation](https://golang.org/pkg/crypto/)
4. [BBolt Documentation](https://github.com/etcd-io/bbolt)
5. [OWASP Cryptographic Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cryptographic_Storage_Cheat_Sheet.html)

## Version History

- **v2.0** - Migrated to BBolt storage backend
- **v1.0** - Initial design with custom binary format