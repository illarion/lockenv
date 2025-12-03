# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

lockenv is a CLI tool for encrypting and storing sensitive files (`.env`, config files, certificates) in a `.lockenv` vault file that can be safely committed to git. Files are encrypted using AES-256-GCM with PBKDF2 key derivation.

## Development Commands

```bash
go build              # Build the project
go test ./...         # Run all tests
go test -run TestName ./...  # Run specific test
go test -cover ./...  # Run with coverage
go fmt ./...          # Format code
```

## Architecture

```
lockenv/
├── main.go              # CLI entry, flag parsing, command routing
├── cmd/                 # CLI command implementations
│   ├── common.go        # Shared helpers (password prompts, vault access)
│   ├── init.go, lock.go, unlock.go, rm.go, ls.go, passwd.go, status.go, diff.go
├── internal/
│   ├── core/            # Business logic
│   │   ├── lockenv.go   # Main LockEnv struct and operations
│   │   ├── merge.go     # Conflict resolution for unlock
│   │   └── password.go  # Password handling
│   ├── crypto/          # AES-256-GCM encryption, PBKDF2 key derivation
│   ├── storage/         # BBolt database interface
│   │   ├── bbolt.go     # Database operations
│   │   └── metadata.go  # Data structures (Metadata, FileEntry, ManifestEntry)
│   └── git/             # Git integration checks
```

### Key Components

**Storage (BBolt database structure):**
- `config` bucket: version, salt, iterations, timestamps (unencrypted)
- `index` bucket: file paths, sizes, mod times (for `ls`/`status` without password)
- `blobs` bucket: encrypted file contents
- `private` bucket: encrypted checksums and full file metadata

**Core Operations:**
- `Init()` → creates vault, generates salt, stores encrypted checksum
- `LockFiles()` + `FinalizeLock()` → track files then encrypt and store
- `Unlock()` → decrypt files with conflict resolution (keep local/vault/both/edit merged)
- `GetChangedFiles()` → hash comparison to detect modifications

**Crypto:**
- Key derivation: PBKDF2 with SHA-256, 210k iterations, 32-byte salt
- Encryption: AES-256-GCM with 12-byte random nonce per operation
- Memory cleanup: `crypto.ClearBytes()` and `enc.Destroy()` after use

### Code Patterns

- All operations use context for cancellation
- Password verification via encrypted checksum decryption
- File paths stored as forward-slash normalized relative paths

## Security Notes

- Use `crypto.ClearBytes()` to zero sensitive data after use
- Call `enc.Destroy()` on encryptor when done
- Store relative paths only (no absolute paths in vault)
