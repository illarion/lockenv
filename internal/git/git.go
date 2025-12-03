package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitStatus contains git integration status information
type GitStatus struct {
	IsRepo              bool
	LockEnvTracked      bool
	UntrackedSecrets    []string // Secrets not tracked by git (good)
	TrackedSecrets      []string // Secrets tracked by git (bad)
	IgnoredSecrets      []string // Secrets in .gitignore (good)
	UnignoredSecrets    []string // Secrets not in .gitignore (warning)
}

// IsGitRepo checks if the working directory is inside a git repository
func IsGitRepo(workDir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = workDir
	err := cmd.Run()
	return err == nil
}

// IsTracked checks if a file is tracked by git
func IsTracked(workDir, path string) bool {
	cmd := exec.Command("git", "ls-files", "--", path)
	cmd.Dir = workDir
	output, err := cmd.Output()

	if err != nil {
		return false
	}

	return len(strings.TrimSpace(string(output))) > 0
}

// IsIgnored checks if a file is ignored by git (handles all .gitignore files)
func IsIgnored(workDir, path string) bool {
	cmd := exec.Command("git", "check-ignore", "-q", "--", path)
	cmd.Dir = workDir
	err := cmd.Run()

	// git check-ignore returns exit code 0 if file is ignored
	return err == nil
}

// CheckGitIntegration checks git integration status for lockenv
func CheckGitIntegration(workDir string, trackedFiles []string) (*GitStatus, error) {
	status := &GitStatus{}

	// Check if this is a git repository
	if !IsGitRepo(workDir) {
		status.IsRepo = false
		return status, nil
	}
	status.IsRepo = true

	// Check if .lockenv is tracked
	status.LockEnvTracked = IsTracked(workDir, ".lockenv")

	// Check status of each tracked file
	for _, file := range trackedFiles {
		tracked := IsTracked(workDir, file)
		ignored := IsIgnored(workDir, file)

		if tracked {
			status.TrackedSecrets = append(status.TrackedSecrets, file)
		} else {
			status.UntrackedSecrets = append(status.UntrackedSecrets, file)
		}

		if ignored {
			status.IgnoredSecrets = append(status.IgnoredSecrets, file)
		} else {
			status.UnignoredSecrets = append(status.UnignoredSecrets, file)
		}
	}

	return status, nil
}

// FormatGitStatus formats git status for display
func FormatGitStatus(status *GitStatus) string {
	if !status.IsRepo {
		return ""
	}

	var result strings.Builder
	result.WriteString("\nGit Integration:\n")

	// Check .lockenv status
	if status.LockEnvTracked {
		result.WriteString("   ok: .lockenv is tracked by git\n")
	} else {
		result.WriteString("   error: .lockenv not tracked (run: git add .lockenv)\n")
	}

	// Check if any secrets are tracked by git (critical issue)
	if len(status.TrackedSecrets) > 0 {
		result.WriteString(fmt.Sprintf("   error: %d secret file(s) tracked by git:\n", len(status.TrackedSecrets)))
		for _, file := range status.TrackedSecrets {
			result.WriteString(fmt.Sprintf("      - %s (run: git rm --cached %s)\n", file, file))
		}
	} else if len(status.UntrackedSecrets) > 0 {
		result.WriteString("   ok: no secret files tracked by git\n")
	}

	// Check if secrets are properly ignored
	if len(status.UnignoredSecrets) > 0 {
		// Build map for O(1) lookup
		trackedSet := make(map[string]bool, len(status.TrackedSecrets))
		for _, f := range status.TrackedSecrets {
			trackedSet[f] = true
		}

		hasTrackedSecrets := len(status.TrackedSecrets) > 0
		for _, file := range status.UnignoredSecrets {
			// Only show warning if file is not already flagged as tracked
			if !trackedSet[file] {
				if !hasTrackedSecrets {
					result.WriteString(fmt.Sprintf("   warning: %s not in .gitignore (add to .gitignore)\n", file))
				} else {
					result.WriteString(fmt.Sprintf("   warning: %s not in .gitignore\n", file))
				}
			}
		}
	} else if len(status.IgnoredSecrets) > 0 {
		result.WriteString(fmt.Sprintf("   ok: %d secret file(s) in .gitignore\n", len(status.IgnoredSecrets)))
	}

	return result.String()
}
