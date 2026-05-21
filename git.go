package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func goModSumPathsForGit() []string {
	return []string{goModFile, goSumPath()}
}

func gitWorktreeDiffersFromHEAD() bool {
	paths := goModSumPathsForGit()
	args := append([]string{"diff", "--quiet", "HEAD", "--"}, paths...)
	err := gitRun(args...)
	if err == nil {
		return false
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return true
	}
	return false
}

// gitRelGoModSumPaths returns go.mod and go.sum paths relative to the repository root.
func gitRelGoModSumPaths() ([]string, error) {
	paths := goModSumPathsForGit()
	absPaths := make([]string, 0, len(paths))
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		absPaths = append(absPaths, abs)
	}
	topBytes, err := gitOutput("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	top := strings.TrimSpace(string(topBytes))
	relPaths := make([]string, len(absPaths))
	for i, ap := range absPaths {
		rel, err := filepath.Rel(top, ap)
		if err != nil {
			return nil, err
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("go.mod path %s is outside git top-level %s", ap, top)
		}
		relPaths[i] = filepath.ToSlash(rel)
	}
	return relPaths, nil
}

// gitDiscardGoModSumChanges restores go.mod and go.sum to the last commit without touching other files.
func gitDiscardGoModSumChanges() error {
	relPaths, err := gitRelGoModSumPaths()
	if err != nil {
		return err
	}
	args := append([]string{"checkout", "HEAD", "--"}, relPaths...)
	if err := gitRun(args...); err != nil {
		return fmt.Errorf("git checkout HEAD -- go.mod/go.sum: %w", err)
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
	for _, p := range goModSumPathsForGit() {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("git add: %w", err)
		}
	}
	relPaths, err := gitRelGoModSumPaths()
	if err != nil {
		return err
	}
	addArgs := append([]string{"add", "--"}, relPaths...)
	if err := gitRun(addArgs...); err != nil {
		return fmt.Errorf("git add: %w", err)
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
