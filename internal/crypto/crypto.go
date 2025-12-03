package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"

	"golang.org/x/crypto/pbkdf2"
)

const (
	SaltSize      = 32     // Salt size in bytes
	KeySize       = 32     // AES-256 key size
	NonceSize     = 12     // GCM nonce size
	TagSize       = 16     // GCM authentication tag size
	DefaultIters  = 210000 // Default PBKDF2 iterations (OWASP minimum)
)

var (
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
	ErrAuthFailed        = errors.New("authentication failed")
)

// KDF handles key derivation from passwords
type KDF struct {
	Salt       []byte
	Iterations int
}

// NewKDF creates a new KDF with a random salt
func NewKDF() (*KDF, error) {
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	return &KDF{
		Salt:       salt,
		Iterations: DefaultIters,
	}, nil
}

// DeriveKey derives an encryption key from a password
func (k *KDF) DeriveKey(password []byte) []byte {
	key := pbkdf2.Key(password, k.Salt, k.Iterations, KeySize, sha256.New)
	return key
}

// Encryptor provides authenticated encryption
type Encryptor struct {
	key []byte
}

// NewEncryptor creates a new encryptor with the given key
func NewEncryptor(key []byte) *Encryptor {
	return &Encryptor{
		key: key,
	}
}

// Encrypt encrypts plaintext using AES-256-GCM
func (e *Encryptor) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Prepend nonce to ciphertext
	result := make([]byte, NonceSize+len(ciphertext))
	copy(result, nonce)
	copy(result[NonceSize:], ciphertext)

	return result, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func (e *Encryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < NonceSize+TagSize {
		return nil, ErrInvalidCiphertext
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce
	nonce := ciphertext[:NonceSize]
	ciphertext = ciphertext[NonceSize:]

	// Decrypt and verify
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrAuthFailed
	}

	return plaintext, nil
}

// Destroy clears the encryptor's key from memory
func (e *Encryptor) Destroy() {
	ClearBytes(e.key)
}

// ClearBytes securely clears a byte slice
func ClearBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// ConstantTimeCompare performs a constant-time comparison of two byte slices
func ConstantTimeCompare(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// GenerateRandom generates n random bytes
func GenerateRandom(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return b, nil
}