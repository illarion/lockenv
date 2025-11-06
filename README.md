# lockenv

Simple, CLI-friendly secret storage that lets you safely commit encrypted secrets to version control.

## Overview

lockenv provides a secure way to store sensitive files (like `.env` files, configuration files, certificates) in an encrypted `.lockenv` file that can be safely committed to your repository. Files are encrypted using a password-derived key and can be easily extracted when needed.

## Installation

```bash
go install github.com/live-labs/lockenv@latest
```

## Quick Start

```bash
# Initialize lockenv in your project
lockenv init

# Track sensitive files
lockenv track .env
lockenv track config/secrets.json

# Seal (encrypt) tracked files into .lockenv
lockenv seal

# Later, extract files with your password
lockenv unseal
```

## Commands

### `lockenv init`
Creates a `.lockenv` file in the current directory. Prompts for a password that will be used for encryption. The password is not stored anywhere - you must remember it.

```bash
$ lockenv init
Enter password: 
Confirm password: 
✓ Initialized .lockenv
```

### `lockenv track <file>`
Adds a file to be tracked by lockenv. Supports glob patterns for multiple files.

```bash
# Track a single file
$ lockenv track .env
✓ Tracking .env

# Track multiple files with glob pattern
$ lockenv track "config/*.env"
✓ Tracking config/dev.env
✓ Tracking config/prod.env
```

### `lockenv forget <file>`
Removes a file from the list of tracked files.

```bash
$ lockenv forget config/dev.env
✓ No longer tracking config/dev.env
```

### `lockenv seal`
Encrypts all tracked files into the `.lockenv` file. Requires the password associated with the `.lockenv` file.

```bash
$ lockenv seal
Enter password: 
✓ Sealed 3 files into .lockenv
```

**Options:**
- `-r, --remove` - Remove original files after sealing

```bash
$ lockenv seal -r
Enter password: 
✓ Sealed 3 files into .lockenv
✓ Removed original files
```

### `lockenv unseal`
Extracts all files from `.lockenv` using your password.

```bash
$ lockenv unseal
Enter password: 
✓ Extracted 3 files from .lockenv
```

### `lockenv chpasswd`
Changes the password for the `.lockenv` file. Requires both the current and new passwords.

```bash
$ lockenv chpasswd
Enter current password: 
Enter new password: 
Confirm new password: 
✓ Password changed successfully
```

### `lockenv diff`
Compares the contents of files in `.lockenv` with local files (if they exist).

```bash
$ lockenv diff
Enter password: 
.env: modified
config/prod.env: not in working directory
config/dev.env: unchanged
```

### `lockenv status`
Shows which files are currently tracked and their status.

```bash
$ lockenv status
Tracked files:
  .env (modified)
  config/dev.env (unchanged)
  config/prod.env (sealed only)
  
.lockenv: present (last sealed: 2024-01-15 10:30:45)
```

### `lockenv list`
Lists all files stored in `.lockenv` without extracting them.

```bash
$ lockenv list
Files in .lockenv:
  .env (1.2 KB)
  config/dev.env (456 bytes)
  config/prod.env (523 bytes)
```

## Workflow Example

1. **Initial setup**
   ```bash
   # Initialize lockenv
   lockenv init
   
   # Track your sensitive files
   lockenv track .env
   lockenv track config/database.yml
   lockenv track certs/server.key
   ```

2. **Before committing**
   ```bash
   # Seal files into .lockenv
   lockenv seal -r
   
   # Commit the encrypted .lockenv file
   git add .lockenv
   git commit -m "Add encrypted secrets"
   ```

3. **After cloning/pulling**
   ```bash
   # Extract your files
   lockenv unseal
   ```

4. **Updating secrets**
   ```bash
   # Make changes to your .env file
   echo "NEW_SECRET=value" >> .env
   
   # Check what changed
   lockenv diff
   
   # Seal the updated files
   lockenv seal
   ```

## Security Considerations

- **Password Management**: lockenv does not store your password. If you lose it, you cannot decrypt your files.
- **Encryption**: Uses industry-standard encryption (AES-256-GCM) with PBKDF2 key derivation.
- **Memory Safety**: Sensitive data is cleared from memory after use.
- **Version Control**: Only commit the `.lockenv` file, never commit unencrypted sensitive files.

## Environment Variables

For CI/CD environments, you can provide the password via environment variable:

```bash
export LOCKENV_PASSWORD="your-password"
lockenv unseal
```

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
