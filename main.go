package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/live-labs/lockenv/cmd"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		runInit(os.Args[2:])
	case "track":
		runTrack(os.Args[2:])
	case "forget":
		runForget(os.Args[2:])
	case "seal":
		runSeal(os.Args[2:])
	case "unseal":
		runUnseal(os.Args[2:])
	case "chpasswd":
		runChpasswd(os.Args[2:])
	case "diff":
		runDiff(os.Args[2:])
	case "status":
		runStatus(os.Args[2:])
	case "list":
		runList(os.Args[2:])
	case "help", "-h", "--help":
		if len(os.Args) > 2 {
			printCommandHelp(os.Args[2])
		} else {
			printUsage()
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	cmd.Init()
}

func runTrack(args []string) {
	fs := flag.NewFlagSet("track", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	cmd.Track(fs.Args())
}

func runForget(args []string) {
	fs := flag.NewFlagSet("forget", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	cmd.Forget(fs.Args())
}

func runSeal(args []string) {
	fs := flag.NewFlagSet("seal", flag.ExitOnError)
	removeShort := fs.Bool("r", false, "Remove original files after sealing")
	removeLong := fs.Bool("remove", false, "Remove original files after sealing")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	remove := *removeShort || *removeLong
	cmd.Seal(remove)
}

func runUnseal(args []string) {
	fs := flag.NewFlagSet("unseal", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	cmd.Unseal()
}

func runChpasswd(args []string) {
	fs := flag.NewFlagSet("chpasswd", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	cmd.Chpasswd()
}

func runDiff(args []string) {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	cmd.Diff()
}

func runStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	cmd.Status()
}

func runList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	cmd.List()
}

func printUsage() {
	fmt.Println("lockenv - Simple, CLI-friendly secret storage")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  lockenv <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init        Create a .lockenv file in current directory")
	fmt.Println("  track       Add a file to be tracked by lockenv")
	fmt.Println("  forget      Remove a file from list of tracked files")
	fmt.Println("  seal        Encrypt tracked files into .lockenv")
	fmt.Println("  unseal      Extract files from .lockenv")
	fmt.Println("  chpasswd    Change password for .lockenv")
	fmt.Println("  diff        Compare .lockenv contents with local files")
	fmt.Println("  status      Show tracked files and their status")
	fmt.Println("  list        List files stored in .lockenv")
	fmt.Println("  help        Show help for a command")
	fmt.Println()
	fmt.Println("Use 'lockenv help <command>' for more information about a command.")
}

func printCommandHelp(command string) {
	switch command {
	case "init":
		fmt.Println("lockenv init")
		fmt.Println()
		fmt.Println("Creates a .lockenv file in the current directory.")
		fmt.Println("Prompts for a password that will be used for encryption.")
		fmt.Println("The password is not stored anywhere - you must remember it.")
	case "track":
		fmt.Println("lockenv track <file> [file...]")
		fmt.Println()
		fmt.Println("Adds files to be tracked by lockenv.")
		fmt.Println("Supports glob patterns for multiple files.")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  lockenv track .env")
		fmt.Println("  lockenv track \"config/*.env\"")
	case "forget":
		fmt.Println("lockenv forget <file> [file...]")
		fmt.Println()
		fmt.Println("Removes files from the list of tracked files.")
	case "seal":
		fmt.Println("lockenv seal [flags]")
		fmt.Println()
		fmt.Println("Encrypts all tracked files into the .lockenv file.")
		fmt.Println("Requires the password associated with the .lockenv file.")
		fmt.Println()
		fmt.Println("Flags:")
		fmt.Println("  -r, --remove    Remove original files after sealing")
	case "unseal":
		fmt.Println("lockenv unseal")
		fmt.Println()
		fmt.Println("Extracts all files from .lockenv using your password.")
	case "chpasswd":
		fmt.Println("lockenv chpasswd")
		fmt.Println()
		fmt.Println("Changes the password for the .lockenv file.")
		fmt.Println("Requires both the current and new passwords.")
	case "diff":
		fmt.Println("lockenv diff")
		fmt.Println()
		fmt.Println("Compares the contents of files in .lockenv with local files.")
	case "status":
		fmt.Println("lockenv status")
		fmt.Println()
		fmt.Println("Shows which files are currently tracked and their status.")
	case "list":
		fmt.Println("lockenv list")
		fmt.Println()
		fmt.Println("Lists all files stored in .lockenv without extracting them.")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
	}
}
