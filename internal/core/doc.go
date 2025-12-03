// Package core provides the main lockenv vault operations.
//
// Core operations include:
//   - Init: Create new vault with password-derived encryption key
//   - LockFiles/FinalizeLock: Track and encrypt files into the vault
//   - Unlock: Decrypt and restore files with conflict resolution
//   - RemoveFiles: Remove files from vault tracking
//   - ChangePassword: Re-encrypt vault with new password
//
// Conflict resolution during unlock supports multiple strategies:
//   - Keep local version
//   - Use vault version (overwrite)
//   - Edit merged (opens $EDITOR with git-style conflict markers)
//   - Keep both (saves vault version as .from-vault)
package core
