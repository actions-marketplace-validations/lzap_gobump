package main

import (
	"fmt"
	"strings"
)

func PerDependencyGitEnabled() bool {
	if Config.NoGit || Config.DryRun {
		return false
	}
	if !gitInsideWorkTree() {
		return false
	}
	if gitHasUncommittedChanges() {
		return false
	}
	return true
}

// ErrIfUnsafeGitWorktree returns an error when the repository has local changes
// and gobump would use git, so the user can opt out explicitly with -no-git.
func ErrIfUnsafeGitWorktree() error {
	if Config.NoGit || Config.DryRun {
		return nil
	}
	if !gitInsideWorkTree() {
		return nil
	}
	if !gitHasUncommittedChanges() {
		return nil
	}
	return fmt.Errorf("refusing to run: git work tree has uncommitted changes. Commit or stash your changes first, or pass -no-git to skip all git integration (no commits, reset, or clean)")
}

func gitHasUncommittedChanges() bool {
	outBytes, err := gitOutput("status", "--porcelain")
	if err != nil {
		// If status fails inside a work tree, treat as unsafe.
		return true
	}
	return strings.TrimSpace(string(outBytes)) != ""
}

func gitInsideWorkTree() bool {
	outBytes, err := gitOutput("rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(outBytes)) == "true"
}

func gitRun(args ...string) error {
	return RunCmd("git", args, true)
}

func gitOutput(args ...string) ([]byte, error) {
	return RunCmdOutput("git", args, true)
}

func gitWorktreeDiffersFromHEAD() bool {
	return gitHasUncommittedChanges()
}

// gitResetWorktreeClean restores the work tree to HEAD, discarding all tracked
// changes and removing untracked and ignored files (e.g. vendor/ from go mod vendor).
// Only call when the tree was verified clean at start (ErrIfUnsafeGitWorktree).
func gitResetWorktreeClean() error {
	if err := gitRun("reset", "--hard", "HEAD"); err != nil {
		return fmt.Errorf("git reset --hard HEAD: %w", err)
	}
	if err := gitRun("clean", "-fdx"); err != nil {
		return fmt.Errorf("git clean -fdx: %w", err)
	}
	return nil
}

func gitEnsureUserIdentity() error {
	if err := gitRun("config", "user.name", Config.GitUserName); err != nil {
		return fmt.Errorf("git config user.name: %w", err)
	}
	if err := gitRun("config", "user.email", Config.GitUserEmail); err != nil {
		return fmt.Errorf("git config user.email: %w", err)
	}
	return nil
}

func goModTidy() error {
	if err := Cmd(Config.GoBinary, "mod", "tidy"); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}
	return nil
}

func GitCommitDependencyBump(modulePath, versionBefore, versionAfter string) error {
	if err := gitEnsureUserIdentity(); err != nil {
		return err
	}
	if err := goModTidy(); err != nil {
		return err
	}
	if err := gitRun("add", "-A"); err != nil {
		return fmt.Errorf("git add -A: %w", err)
	}
	msg := fmt.Sprintf("chore(deps): update %s to %s", modulePath, versionAfter)
	if Config.Changelog {
		msg += formatModuleChangelog(modulePath, versionBefore, versionAfter)
	}
	if err := gitRun("commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}
