# lockenv

Simple, CLI-friendly secret storage that lets you safely commit encrypted secrets to version control.

> For small teams who want something simpler than sops/git-crypt for .env and infra secrets.

## Overview

lockenv provides a secure way to store sensitive files (like `.env` files, configuration files, certificates) in an encrypted `.lockenv` file that can be safely committed to your repository. Files are encrypted using a password-derived key and can be easily extracted when needed.

## How is this different?

| Feature | lockenv | git-crypt | sops |
|---------|---------|-----------|------|
| Format | Single vault file | Transparent per-file | YAML/JSON native |
| Auth | Password (PBKDF2) | GPG keys | KMS/PGP/age |
| Git integration | Manual (lock/unlock) | Transparent (git filter) | Manual |
| Setup | `lockenv init` | GPG key exchange | KMS/key config |
| Best for | Simple .env/config | Large teams, many devs | Cloud infra, key rotation |

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap illarion/tap
brew install lockenv
```

### Debian/Ubuntu

Download the `.deb` file from the [latest release](https://github.com/illarion/lockenv/releases/latest):

```bash
sudo dpkg -i lockenv_*_linux_amd64.deb
```

### Fedora/RHEL

Download the `.rpm` file from the [latest release](https://github.com/illarion/lockenv/releases/latest):

```bash
sudo rpm -i lockenv_*_linux_amd64.rpm
```

### Binary Download

Download pre-built binaries from [GitHub Releases](https://github.com/illarion/lockenv/releases/latest).

Available for:
- Linux (amd64, arm64)
- macOS (amd64, arm64)

### Go Install

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

### `lockenv unlock [file...]`
Decrypts and restores files from the vault with smart conflict resolution.

```bash
# Unlock all files
$ lockenv unlock
Enter password:
unlocked: .env
unlocked: config/database.yml

unlocked: 2 files

# Unlock specific file
$ lockenv unlock .env
Enter password:
unlocked: .env

unlocked: 1 files

# Unlock files matching pattern
$ lockenv unlock "config/*.env"
Enter password:
unlocked: config/dev.env
unlocked: config/prod.env

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
removed: config/dev.env from vault
```

### `lockenv ls`
Alias for `lockenv status`. Shows comprehensive vault status.

### `lockenv status`
Shows comprehensive vault status including statistics, file states, and detailed information. Does not require a password.

```bash
$ lockenv status

Vault Status
===========================================

Statistics:
   Files in vault: 3
   Total size:     2.17 KB
   Last locked:    2025-01-15 10:30:45
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
Shows actual content differences between vault and local files (like `git diff`).

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

### `lockenv compact`

Compacts the vault database to reclaim unused disk space. Runs automatically after `rm` and `passwd`, but can be run manually.

```bash
$ lockenv compact
Compacted: 45.2 KB -> 12.1 KB
```

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
   ```

## Security Considerations

- **Password Management**: lockenv does not store your password. If you lose it, you cannot decrypt your files.
- **Encryption**: Uses industry-standard encryption (AES-256-GCM) with PBKDF2 key derivation for all file contents.
- **Metadata Visibility**: File paths, sizes, and modification times are visible without authentication via `lockenv status`. If file paths themselves are sensitive, use generic names like `config1.enc`.
- **Memory Safety**: Sensitive data is cleared from memory after use.
- **Version Control**: Only commit the `.lockenv` file, never commit unencrypted sensitive files.

## Threat Model

**lockenv protects against:**
- Secrets exposed in git history or repository leaks
- Unauthorized repository access without the password
- Dev laptops without the password

**lockenv does NOT protect against:**
- Compromised CI runner (sees plaintext after unlock)
- Attacker who has the password

## Environment Variables

### LOCKENV_PASSWORD

For CI/CD environments, you can provide the password via environment variable:

```bash
export LOCKENV_PASSWORD="your-password"
lockenv unlock
```

**Security warning:** Environment variables may be visible to other processes on the system (via `/proc/<pid>/environ` on Linux or process inspection tools). Use this feature only in isolated CI/CD environments where process inspection by other users is not a concern. For interactive use, prefer the terminal prompt.

## CI/CD Integration

### GitHub Actions

```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install lockenv
        run: |
          curl -sL https://github.com/illarion/lockenv/releases/latest/download/lockenv_linux_amd64.tar.gz | tar xz
          sudo mv lockenv /usr/local/bin/
      - name: Unlock secrets
        env:
          LOCKENV_PASSWORD: ${{ secrets.LOCKENV_PASSWORD }}
        run: lockenv unlock
```

### GitLab CI

```yaml
deploy:
  before_script:
    - curl -sL https://github.com/illarion/lockenv/releases/latest/download/lockenv_linux_amd64.tar.gz | tar xz
    - mv lockenv /usr/local/bin/
    - lockenv unlock
  variables:
    LOCKENV_PASSWORD: $LOCKENV_PASSWORD
```

## Shell Completions

Shell completions are **automatically installed** when using Homebrew, deb, or rpm packages.

For manual installation (binary download or `go install`):

```bash
# Bash - add to ~/.bashrc
eval "$(lockenv completion bash)"

# Zsh - add to ~/.zshrc
eval "$(lockenv completion zsh)"

# Fish - add to ~/.config/fish/config.fish
lockenv completion fish | source
```

## Git Integration

lockenv is designed for version control: ignore your sensitive files, commit only the encrypted `.lockenv` vault.

### Basic Setup

Add to your `.gitignore`:

```gitignore
# Sensitive files - these are stored encrypted in .lockenv
.env
.env.*
*.key
*.pem
secrets/

# Keep the encrypted vault (negation pattern)
!.lockenv
```

The `!.lockenv` negation ensures the vault is tracked even if broader patterns (like `.*`) would exclude it.

### Project-Specific Examples

**Node.js:**
```gitignore
.env
.env.local
.env.production
config/secrets.json
!.lockenv
```

**Python:**
```gitignore
.env
*.pem
secrets.yaml
config/credentials.py
!.lockenv
```

**Go:**
```gitignore
.env
config/secrets.yaml
*.key
!.lockenv
```

**Ruby/Rails:**
```gitignore
.env
config/master.key
config/credentials.yml.enc
config/secrets.yml
!.lockenv
```

**Terraform:**
```gitignore
*.tfvars
terraform.tfstate
terraform.tfstate.backup
.terraform/
!.lockenv
```

## Limitations

- **File size**: Files are loaded entirely into memory for encryption. Not recommended for large binary files (>100MB).
- **Single password**: One password for the entire vault. No per-user or per-file access control.

For feature requests or issues, see [GitHub Issues](https://github.com/illarion/lockenv/issues).
