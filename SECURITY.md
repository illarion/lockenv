# lockenv Security Design

## Overview

lockenv is a CLI tool for securely storing sensitive files in an encrypted container that can be safely committed to version control. This document describes the security architecture and implementation details.

## Threat Model

### What We Protect Against
- **Unauthorized access**: Files are encrypted with AES-256-GCM
- **Password brute-forcing**: PBKDF2 with 100,000 iterations
- **Data tampering**: Authenticated encryption prevents modification
- **Information leakage**: Sensitive metadata is encrypted
- **Memory disclosure**: Sensitive data is cleared after use
- **Database corruption**: BBolt provides ACID guarantees

### What We Don't Protect Against
- **Weak passwords**: Users must choose strong passwords
- **Compromised system**: If the system is compromised during unsealing
- **Side-channel attacks**: Not designed for hostile local environments
- **Key loggers**: Password entry can be captured

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
  - Iterations: 100,000 (default)
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
├── meta bucket (unencrypted)
│   ├── version     → "1"
│   ├── created     → timestamp
│   ├── modified    → timestamp
│   ├── salt        → 32 bytes (for PBKDF2)
│   └── iterations  → uint32 (PBKDF2 iterations)
├── manifest bucket (unencrypted)
│   └── [file_path] → {path, size, modTime} (JSON)
├── files bucket (encrypted values)
│   └── [file_path] → [nonce][ciphertext][tag]
└── metadata bucket (encrypted)
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
- All file contents encrypted with AES-256-GCM
- Sensitive metadata encrypted in metadata bucket
- Basic file list in manifest bucket for usability
- No information leakage about file contents

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
        files := tx.Bucket(FilesBucket)
        if files == nil {
            return errors.New("files bucket not found")
        }
        return files.Put([]byte(path), encData)
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

### Why Unencrypted Manifest?
The manifest bucket stores basic file information unencrypted to enable:
- `lockenv list` without password
- `lockenv status` without password
- Better user experience

This is safe because:
- Only paths and sizes are exposed
- No file contents or sensitive metadata
- Similar to `ls -la` information

### Salt Storage
The salt is stored unencrypted in the meta bucket because:
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