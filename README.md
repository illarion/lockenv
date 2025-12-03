# lockenv

Simple, CLI-friendly secret storage that lets you safely commit encrypted secrets to version control.

## Overview

lockenv provides a secure way to store sensitive files (like `.env` files, configuration files, certificates) in an encrypted `.lockenv` file that can be safely committed to your repository. Files are encrypted using a password-derived key and can be easily extracted when needed.

## Installation

```bash
go install github.com/illarion/lockenv@latest
```

## Quick Start

```bash
# Initialize lockenv in your project
lockenv init

# Lock (encrypt and store) sensitive files
lockenv lock .env config/secrets.json

# Later, unlock (decrypt and restore) files with your password
lockenv unlock
```

## Commands

### `lockenv init`
Creates a `.lockenv` vault file in the current directory. Prompts for a password that will be used for encryption. The password is not stored anywhere - you must remember it.

```bash
$ lockenv init
Enter password:
Confirm password:
initialized: .lockenv
```

### `lockenv lock <file> [file...]`
Encrypts and stores files in the vault. Supports glob patterns for multiple files.

```bash
# Lock a single file
$ lockenv lock .env
Enter password:
locking: .env
encrypted: .env
locked: 1 files into .lockenv

# Lock multiple files with glob pattern
$ lockenv lock "config/*.env"
Enter password:
locking: config/dev.env
locking: config/prod.env
encrypted: config/dev.env
encrypted: config/prod.env
locked: 2 files into .lockenv
```

**Options:**
- `-r, --remove` - Remove original files after locking

```bash
$ lockenv lock .env --remove
Enter password:
locking: .env
encrypted: .env
removed: .env
locked: 1 files into .lockenv
```

### `lockenv unlock`
Decrypts and restores files from the vault with smart conflict resolution.

```bash
$ lockenv unlock
Enter password:
unlocked: .env
unlocked: config/database.yml

unlocked: 2 files
```

**Smart Conflict Resolution:**
When a file exists locally and differs from the vault version, you have multiple options:

**Interactive Mode (default):**
- `[l]` Keep local version
- `[v]` Use vault version (overwrite local)
- `[e]` Edit merged (opens in $EDITOR with git-style conflict markers, text files only)
- `[b]` Keep both (saves vault version as `.from-vault`)
- `[x]` Skip this file

**Non-Interactive Flags:**
- `--force` - Overwrite all local files with vault version
- `--keep-local` - Keep all local versions, skip conflicts
- `--keep-both` - Keep both versions for all conflicts (vault saved as `.from-vault`)

```bash
# Interactive mode example
$ lockenv unlock

warning: conflict detected: .env
   Local file exists and differs from vault version
   File type: text

Options:
  [l] Keep local version
  [v] Use vault version (overwrite local)
  [e] Edit merged (opens in $EDITOR)
  [b] Keep both (save vault as .from-vault)
  [x] Skip this file

Your choice: e

opening editor for merge...
# Editor opens with:
# <<<<<<< local
# API_KEY=old_value
# =======
# API_KEY=new_value
# DEBUG=true
# >>>>>>> vault

unlocked: .env

# Keep both versions example
$ lockenv unlock --keep-both
Enter password:
saved: .env.from-vault (vault version)
skipped: .env (kept local version)
saved: config/secrets.json.from-vault (vault version)
skipped: config/secrets.json (kept local version)
```

### `lockenv rm <file> [file...]`
Removes files from the vault. Supports glob patterns.

```bash
$ lockenv rm config/dev.env
Enter password:
🗑️ Removed config/dev.env from vault
```

### `lockenv ls`
Lists all files stored in the vault. Does not require a password.

```bash
$ lockenv ls
Files in .lockenv:
  .env (1.2 KB)
  config/dev.env (456 bytes)
  config/prod.env (523 bytes)
```

### `lockenv status`
Shows comprehensive vault status including statistics, file states, and detailed information. Does not require a password.

```bash
$ lockenv status

Vault Status
===========================================

Statistics:
   Files in vault: 3
   Total size:     2.17 KB
   Last locked:    2024-01-15 10:30:45
   Encryption:     AES-256-GCM (PBKDF2 iterations: 210000)
   Version:        1

Summary:
   .  2 unchanged
   *  1 modified

Files:
   * .env (modified)
   . config/dev.env (unchanged)
   * config/prod.env (vault only)

===========================================
```

### `lockenv passwd`
Changes the vault password. Requires both the current and new passwords. Re-encrypts all files with the new password.

```bash
$ lockenv passwd
Enter current password:
Enter new password:
Confirm new password:
password changed successfully
```

### `lockenv diff`
Shows actual content differences between vault and local files (like `git diff`). Uses the system `diff` command for professional unified diff output.

```bash
$ lockenv diff
Enter password:
--- a/.env
+++ b/.env
@@ -1,3 +1,4 @@
 API_KEY=secret123
-DATABASE_URL=localhost:5432
+DATABASE_URL=production:5432
+DEBUG=false
 PORT=3000

Binary file config/logo.png has changed

File not in working directory: config/prod.env
```

**Note:** `lockenv status` shows which files changed, `lockenv diff` shows what changed.

## Workflow Example

1. **Initial setup**
   ```bash
   # Initialize vault
   lockenv init

   # Lock your sensitive files (encrypt and store)
   lockenv lock .env config/database.yml certs/server.key --remove

   # Commit the encrypted vault
   git add .lockenv
   git commit -m "Add encrypted secrets"
   git push
   ```

2. **After cloning repository (new team member)**
   ```bash
   git clone <repo>
   cd <repo>

   # Unlock files to restore them
   lockenv unlock
   ```

3. **Updating secrets**
   ```bash
   # Make changes to your .env file
   echo "NEW_SECRET=value" >> .env

   # Check what changed
   lockenv status    # See file is modified
   lockenv diff      # See detailed changes

   # Lock the updated files
   lockenv lock .env

   # Commit the changes
   git add .lockenv
   git commit -m "Update secrets"
   git push
   ```

4. **Managing files**
   ```bash
   # Add new secret file
   lockenv lock new-secrets.json

   # Remove file from vault
   lockenv rm old-config.yml

   # Check vault status
   lockenv status

   # List all files without password
   lockenv ls
   ```

## Security Considerations

- **Password Management**: lockenv does not store your password. If you lose it, you cannot decrypt your files.
- **Encryption**: Uses industry-standard encryption (AES-256-GCM) with PBKDF2 key derivation for all file contents.
- **Metadata Visibility**: File paths, sizes, and modification times are visible without authentication via `lockenv ls` and `lockenv status`. If file paths themselves are sensitive, use generic names like `config1.enc`.
- **Memory Safety**: Sensitive data is cleared from memory after use.
- **Version Control**: Only commit the `.lockenv` file, never commit unencrypted sensitive files.

## Environment Variables

### LOCKENV_PASSWORD

For CI/CD environments, you can provide the password via environment variable:

```bash
export LOCKENV_PASSWORD="your-password"
lockenv unlock
```

**Security warning:** Environment variables may be visible to other processes on the system (via `/proc/<pid>/environ` on Linux or process inspection tools). Use this feature only in isolated CI/CD environments where process inspection by other users is not a concern. For interactive use, prefer the terminal prompt.

## Gitignore

Add these entries to your `.gitignore`:

```gitignore
# Sensitive files (add your specific files)
.env
*.key
*.pem

# Keep the encrypted file
!.lockenv
```
