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
	if !GitInsideWorkTree() {
		return false
	}
	if GitHasUncommittedChanges() {
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
	if !GitInsideWorkTree() {
		return nil
	}
	if !GitHasUncommittedChanges() {
		return nil
	}
	return fmt.Errorf("refusing to run: git work tree has uncommitted changes. Commit or stash your changes first, or pass -no-git to skip all git integration (no commits, reset, or clean)")
}

func GitHasUncommittedChanges() bool {
	outBytes, err := GitOutput("status", "--porcelain")
	if err != nil {
		// If status fails inside a work tree, treat as unsafe.
		return true
	}
	return strings.TrimSpace(string(outBytes)) != ""
}

func GitInsideWorkTree() bool {
	outBytes, err := GitOutput("rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(outBytes)) == "true"
}

func GitRun(args ...string) error {
	return RunCmd("git", args, true)
}

func GitOutput(args ...string) ([]byte, error) {
	return RunCmdOutput("git", args, true)
}

func GoModSumPathsForGit() []string {
	modPath := Config.GoModDst
	sumPath := strings.TrimSuffix(modPath, ".mod") + ".sum"
	paths := []string{modPath}
	if st, err := os.Stat(sumPath); err == nil && !st.IsDir() {
		paths = append(paths, sumPath)
	}
	return paths
}

func GitWorktreeDiffersFromHEAD() bool {
	paths := GoModSumPathsForGit()
	args := append([]string{"diff", "--quiet", "HEAD", "--"}, paths...)
	err := GitRun(args...)
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
	paths := GoModSumPathsForGit()
	absPaths := make([]string, 0, len(paths))
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		absPaths = append(absPaths, abs)
	}
	topBytes, err := GitOutput("rev-parse", "--show-toplevel")
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

// GitDiscardGoModSumChanges restores go.mod and go.sum to the last commit without touching other files.
func GitDiscardGoModSumChanges() error {
	relPaths, err := gitRelGoModSumPaths()
	if err != nil {
		return err
	}
	args := append([]string{"checkout", "HEAD", "--"}, relPaths...)
	if err := GitRun(args...); err != nil {
		return fmt.Errorf("git checkout HEAD -- go.mod/go.sum: %w", err)
	}
	return nil
}

func GitResetHardHEAD() error {
	if err := GitRun("reset", "--hard", "HEAD"); err != nil {
		return fmt.Errorf("git reset --hard HEAD: %w", err)
	}
	if err := GitRun("clean", "-fdq"); err != nil {
		return fmt.Errorf("git clean -fdq: %w", err)
	}
	return nil
}

func GitEnsureUserIdentity() error {
	if err := GitRun("config", "user.name", Config.GitUserName); err != nil {
		return fmt.Errorf("git config user.name: %w", err)
	}
	if err := GitRun("config", "user.email", Config.GitUserEmail); err != nil {
		return fmt.Errorf("git config user.email: %w", err)
	}
	return nil
}

func GoModTidy() error {
	if err := Cmd(Config.GoBinary, "mod", "tidy"); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}
	return nil
}

func GitCommitDependencyBump(modulePath, versionBefore, versionAfter string) error {
	if err := GitEnsureUserIdentity(); err != nil {
		return err
	}
	if err := GoModTidy(); err != nil {
		return err
	}
	for _, p := range GoModSumPathsForGit() {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("git add: %w", err)
		}
	}
	relPaths, err := gitRelGoModSumPaths()
	if err != nil {
		return err
	}
	addArgs := append([]string{"add", "--"}, relPaths...)
	if err := GitRun(addArgs...); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	msg := fmt.Sprintf("chore(deps): update %s to %s", modulePath, versionAfter)
	if Config.Changelog {
		msg += FormatModuleChangelog(modulePath, versionBefore, versionAfter)
	}
	if err := GitRun("commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}
