// Package crypto provides cryptographic operations for lockenv.
//
// Encryption uses AES-256-GCM with:
//   - 32-byte key derived from password via PBKDF2
//   - 12-byte random nonce per encryption operation
//   - Authenticated encryption prevents tampering
//
// Key derivation uses PBKDF2-HMAC-SHA256 with:
//   - 32-byte random salt (stored unencrypted)
//   - 210,000 iterations (OWASP minimum recommendation)
//
// Memory safety:
//   - Use ClearBytes() to zero sensitive data after use
//   - Call Encryptor.Destroy() when done with encryption operations
package crypto
