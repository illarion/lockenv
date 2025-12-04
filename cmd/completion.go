package cmd

import (
	"fmt"
	"os"
)

// Completion outputs shell completion scripts
func Completion(shell string) {
	switch shell {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		fmt.Fprintf(os.Stderr, "Unknown shell: %s\nSupported: bash, zsh, fish\n", shell)
		os.Exit(1)
	}
}

const bashCompletion = `_lockenv() {
    local cur prev words cword
    _init_completion || return

    local commands="init lock unlock rm ls status passwd diff compact keyring help completion"

    if [[ $cword -eq 1 ]]; then
        COMPREPLY=($(compgen -W "$commands" -- "$cur"))
        return
    fi

    local cmd="${words[1]}"
    case "$cmd" in
        lock)
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "-r --remove --force" -- "$cur"))
            else
                _filedir
            fi
            ;;
        unlock)
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "--force --keep-local --keep-both" -- "$cur"))
            else
                # Complete with files from vault
                local files
                files=$(lockenv ls 2>/dev/null | grep -E '^\s+[.*]' | sed 's/^.*[.*] //' | sed 's/ (.*//')
                COMPREPLY=($(compgen -W "$files" -- "$cur"))
            fi
            ;;
        rm)
            # Complete with files from vault
            local files
            files=$(lockenv ls 2>/dev/null | grep -E '^\s+[.*]' | sed 's/^.*[.*] //' | sed 's/ (.*//')
            COMPREPLY=($(compgen -W "$files" -- "$cur"))
            ;;
        keyring)
            COMPREPLY=($(compgen -W "save delete status" -- "$cur"))
            ;;
        help)
            COMPREPLY=($(compgen -W "$commands" -- "$cur"))
            ;;
        completion)
            COMPREPLY=($(compgen -W "bash zsh fish" -- "$cur"))
            ;;
    esac
}

complete -F _lockenv lockenv
`

const zshCompletion = `#compdef lockenv

_lockenv() {
    local -a commands
    commands=(
        'init:Create a .lockenv vault in current directory'
        'lock:Encrypt and store files in the vault'
        'unlock:Decrypt and restore files from the vault'
        'rm:Remove files from the vault'
        'ls:Show comprehensive vault status'
        'status:Show comprehensive vault status'
        'passwd:Change vault password'
        'diff:Compare vault contents with local files'
        'compact:Compact vault to reclaim disk space'
        'keyring:Manage password in OS keyring'
        'help:Show help for a command'
        'completion:Generate shell completions'
    )

    _arguments -C \
        '1: :->command' \
        '*: :->args'

    case "$state" in
        command)
            _describe -t commands 'lockenv commands' commands
            ;;
        args)
            case "${words[2]}" in
                lock)
                    _arguments \
                        '-r[Remove original files after locking]' \
                        '--remove[Remove original files after locking]' \
                        '--force[Lock without confirmation]' \
                        '*:file:_files'
                    ;;
                unlock)
                    _arguments \
                        '--force[Overwrite local files without asking]' \
                        '--keep-local[Skip all conflicts, keep local versions]' \
                        '--keep-both[Keep both local and vault versions]' \
                        '*:vault file:_lockenv_vault_files'
                    ;;
                rm)
                    _arguments '*:vault file:_lockenv_vault_files'
                    ;;
                keyring)
                    _values 'subcommand' save delete status
                    ;;
                help)
                    _describe -t commands 'lockenv commands' commands
                    ;;
                completion)
                    _values 'shell' bash zsh fish
                    ;;
            esac
            ;;
    esac
}

_lockenv_vault_files() {
    local -a files
    files=(${(f)"$(lockenv ls 2>/dev/null | grep -E '^\s+[.*]' | sed 's/^.*[.*] //' | sed 's/ (.*//')"})
    _describe -t files 'vault files' files
}

_lockenv "$@"
`

const fishCompletion = `# lockenv fish completions

set -l commands init lock unlock rm ls status passwd diff compact keyring help completion

complete -c lockenv -f

# Commands
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a init -d 'Create a .lockenv vault'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a lock -d 'Encrypt and store files'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a unlock -d 'Decrypt and restore files'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a rm -d 'Remove files from vault'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a ls -d 'Show vault status'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a status -d 'Show vault status'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a passwd -d 'Change vault password'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a diff -d 'Compare vault with local'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a compact -d 'Compact vault'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a keyring -d 'Manage password in OS keyring'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a help -d 'Show help'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a completion -d 'Generate completions'

# lock flags and files
complete -c lockenv -n "__fish_seen_subcommand_from lock" -s r -d 'Remove original files'
complete -c lockenv -n "__fish_seen_subcommand_from lock" -l remove -d 'Remove original files'
complete -c lockenv -n "__fish_seen_subcommand_from lock" -l force -d 'Lock without confirmation'
complete -c lockenv -n "__fish_seen_subcommand_from lock" -F

# unlock flags
complete -c lockenv -n "__fish_seen_subcommand_from unlock" -l force -d 'Overwrite local files'
complete -c lockenv -n "__fish_seen_subcommand_from unlock" -l keep-local -d 'Keep local versions'
complete -c lockenv -n "__fish_seen_subcommand_from unlock" -l keep-both -d 'Keep both versions'

# keyring subcommands
complete -c lockenv -n "__fish_seen_subcommand_from keyring" -a "save delete status"

# help completions
complete -c lockenv -n "__fish_seen_subcommand_from help" -a "$commands"

# completion completions
complete -c lockenv -n "__fish_seen_subcommand_from completion" -a "bash zsh fish"
`
