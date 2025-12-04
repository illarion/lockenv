package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/illarion/lockenv/cmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		runInit(ctx, os.Args[2:])
	case "lock":
		runLock(ctx, os.Args[2:])
	case "unlock":
		runUnlock(ctx, os.Args[2:])
	case "rm":
		runRm(ctx, os.Args[2:])
	case "ls":
		runLs(ctx, os.Args[2:])
	case "passwd":
		runPasswd(ctx, os.Args[2:])
	case "diff":
		runDiff(ctx, os.Args[2:])
	case "status":
		runStatus(ctx, os.Args[2:])
	case "compact":
		runCompact(ctx, os.Args[2:])
	case "completion":
		runCompletion(ctx, os.Args[2:])
	case "help", "-h", "--help":
		if len(os.Args) <= 2 {
			printUsage()
			return
		}
		printCommandHelp(os.Args[2])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func runInit(_ context.Context, args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	cmd.Init()
}

func runLock(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("lock", flag.ExitOnError)
	removeShort := fs.Bool("r", false, "Remove original files after locking")
	removeLong := fs.Bool("remove", false, "Remove original files after locking")
	force := fs.Bool("force", false, "Lock without confirmation")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	remove := *removeShort || *removeLong

	// If file arguments provided, lock those specific files
	if len(fs.Args()) > 0 {
		cmd.Lock(ctx, fs.Args(), remove)
		return
	}
	// Otherwise lock all tracked modified files
	cmd.LockAll(ctx, remove, *force)
}

func runUnlock(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("unlock", flag.ExitOnError)
	force := fs.Bool("force", false, "Overwrite local files without asking")
	keepLocal := fs.Bool("keep-local", false, "Skip all conflicts, keep local versions")
	keepBoth := fs.Bool("keep-both", false, "Keep both local and vault versions")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	cmd.Unlock(ctx, fs.Args(), *force, *keepLocal, *keepBoth)
}

func runRm(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("rm", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	cmd.Remove(ctx, fs.Args())
}

func runLs(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	cmd.Status(ctx)
}

func runPasswd(_ context.Context, args []string) {
	fs := flag.NewFlagSet("passwd", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	cmd.Passwd()
}

func runDiff(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	cmd.Diff(ctx)
}

func runStatus(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	cmd.Status(ctx)
}

func runCompact(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("compact", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	cmd.Compact(ctx)
}

func runCompletion(_ context.Context, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: lockenv completion <bash|zsh|fish>")
		os.Exit(1)
	}
	cmd.Completion(args[0])
}

func printUsage() {
	fmt.Println("lockenv - Simple, CLI-friendly secret storage")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  lockenv <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init        Create a .lockenv vault in current directory")
	fmt.Println("  lock        Encrypt and store files in the vault")
	fmt.Println("  unlock      Decrypt and restore files from the vault")
	fmt.Println("  rm          Remove files from the vault")
	fmt.Println("  ls, status  Show comprehensive vault status")
	fmt.Println("  passwd      Change vault password")
	fmt.Println("  diff        Compare vault contents with local files")
	fmt.Println("  compact     Compact vault to reclaim disk space")
	fmt.Println("  completion  Generate shell completions")
	fmt.Println("  help        Show help for a command")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  lockenv init                    # Create new vault")
	fmt.Println("  lockenv lock .env --rm          # Lock .env and remove original")
	fmt.Println("  lockenv unlock                  # Unlock all files")
	fmt.Println("  lockenv status                  # Check vault status")
	fmt.Println()
	fmt.Println("Use 'lockenv help <command>' for more information about a command.")
}

func printCommandHelp(command string) {
	switch command {
	case "init":
		fmt.Println("lockenv init")
		fmt.Println()
		fmt.Println("Creates a .lockenv vault file in the current directory.")
		fmt.Println("Prompts for a password that will be used for encryption.")
		fmt.Println("The password is not stored anywhere - you must remember it.")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  lockenv init                     # Create new vault")
	case "lock":
		fmt.Println("lockenv lock [--force] [-r|--remove] [<file> [file...]]")
		fmt.Println()
		fmt.Println("Encrypts and stores files in the vault.")
		fmt.Println("When run without file arguments, locks all tracked files that have been modified.")
		fmt.Println("Uses content hash comparison to detect changes.")
		fmt.Println("Supports glob patterns for multiple files.")
		fmt.Println()
		fmt.Println("Flags:")
		fmt.Println("  -r, --remove    Remove original files after locking")
		fmt.Println("  --force         Lock without confirmation (when no files specified)")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  lockenv lock                     # Lock all modified tracked files")
		fmt.Println("  lockenv lock --force             # Lock all modified files without asking")
		fmt.Println("  lockenv lock .env                # Lock specific .env file")
		fmt.Println("  lockenv lock .env --remove       # Lock and remove original")
		fmt.Println("  lockenv lock \"config/*.secret\"   # Lock multiple files with glob")
	case "unlock":
		fmt.Println("lockenv unlock [--force|--keep-local|--keep-both] [<file> [file...]]")
		fmt.Println()
		fmt.Println("Decrypts and restores files from the vault.")
		fmt.Println("When run without file arguments, unlocks all files.")
		fmt.Println("Supports glob patterns for specific files.")
		fmt.Println("Smart conflict resolution for files that exist locally.")
		fmt.Println()
		fmt.Println("Flags:")
		fmt.Println("  --force        Overwrite local files without asking")
		fmt.Println("  --keep-local   Skip all conflicts, keep local versions")
		fmt.Println("  --keep-both    Keep both versions (save vault as .from-vault)")
		fmt.Println()
		fmt.Println("Interactive mode (default):")
		fmt.Println("  - Skips unchanged files")
		fmt.Println("  - For conflicts, offers:")
		fmt.Println("    [l] Keep local version")
		fmt.Println("    [v] Use vault version (overwrite local)")
		fmt.Println("    [e] Edit merged (opens in $EDITOR, text files only)")
		fmt.Println("    [b] Keep both (save vault as .from-vault)")
		fmt.Println("    [x] Skip this file")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  lockenv unlock                   # Unlock all files")
		fmt.Println("  lockenv unlock .env              # Unlock specific file")
		fmt.Println("  lockenv unlock \"*.env\"           # Unlock files matching pattern")
		fmt.Println("  lockenv unlock --force           # Overwrite all")
		fmt.Println("  lockenv unlock --keep-both       # Keep both for all conflicts")
	case "rm":
		fmt.Println("lockenv rm <file> [file...]")
		fmt.Println()
		fmt.Println("Removes files from the vault.")
		fmt.Println("Supports glob patterns for multiple files.")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  lockenv rm .env")
		fmt.Println("  lockenv rm \"config/*.secret\"")
	case "ls":
		fmt.Println("lockenv ls")
		fmt.Println()
		fmt.Println("Alias for 'lockenv status'. Shows comprehensive vault status.")
	case "passwd":
		fmt.Println("lockenv passwd")
		fmt.Println()
		fmt.Println("Changes the vault password.")
		fmt.Println("Requires both the current and new passwords.")
		fmt.Println("Re-encrypts all files with the new password.")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  lockenv passwd")
	case "diff":
		fmt.Println("lockenv diff")
		fmt.Println()
		fmt.Println("Compares vault contents with local files.")
		fmt.Println("Shows which files have been modified locally.")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  lockenv diff")
	case "status":
		fmt.Println("lockenv status")
		fmt.Println()
		fmt.Println("Shows comprehensive vault status including:")
		fmt.Println("  - File count and total size")
		fmt.Println("  - Encryption details")
		fmt.Println("  - File states (locked, modified, unchanged)")
		fmt.Println("  - Detailed file list with status icons")
		fmt.Println()
		fmt.Println("Does not require a password.")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  lockenv status")
	case "compact":
		fmt.Println("lockenv compact")
		fmt.Println()
		fmt.Println("Compacts the .lockenv database to reclaim unused disk space.")
		fmt.Println("This is automatically done after 'rm' and 'passwd' commands,")
		fmt.Println("but can be run manually if needed.")
		fmt.Println()
		fmt.Println("Does not require a password.")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  lockenv compact")
	case "completion":
		fmt.Println("lockenv completion <bash|zsh|fish>")
		fmt.Println()
		fmt.Println("Outputs shell completion script for the specified shell.")
		fmt.Println()
		fmt.Println("Setup:")
		fmt.Println("  # Bash - add to ~/.bashrc")
		fmt.Println("  eval \"$(lockenv completion bash)\"")
		fmt.Println()
		fmt.Println("  # Zsh - add to ~/.zshrc")
		fmt.Println("  eval \"$(lockenv completion zsh)\"")
		fmt.Println()
		fmt.Println("  # Fish - add to ~/.config/fish/config.fish")
		fmt.Println("  lockenv completion fish | source")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
	}
}
