package main

import (
	"fmt"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
)

// upgradeResult captures the outcome of attempting to upgrade a module.
type upgradeResult struct {
	Mod             *modfile.File // The modfile after upgrade attempt (may be unchanged)
	Success         bool          // Whether the upgrade succeeded
	NoProxyVersions bool          // Whether the proxy returned no newer versions
}

func attemptUpgrade(modulePath, version string) (*modfile.File, error) {
	err := Cmd(Config.GoBinary, "get", modulePath+"@"+version)
	if err != nil {
		return nil, fmt.Errorf("failed to get module: %w", err)
	}
	return ParseMod(goModFile)
}

// ValidateUpgrade checks if the upgrade is valid.
func ValidateUpgrade(originalMod, newMod *modfile.File) error {
	if originalMod == nil || originalMod.Go == nil {
		return fmt.Errorf("original go.mod is missing or has no go directive")
	}
	if newMod == nil || newMod.Go == nil {
		return fmt.Errorf("upgraded go.mod is missing or has no go directive")
	}
	if normalizeVersion(originalMod.Go.Version) != normalizeVersion(newMod.Go.Version) {
		return fmt.Errorf("upgrade changes required Go version %s => %s", originalMod.Go.Version, newMod.Go.Version)
	}
	if toolchainIdentity(originalMod) != toolchainIdentity(newMod) {
		return fmt.Errorf("upgrade changes toolchain %s => %s", toolchainLabel(originalMod), toolchainLabel(newMod))
	}
	return nil
}

// UpgradeModule attempts to upgrade a single module.
func UpgradeModule(proxy *GoProxy, r *modfile.Require, okMod *modfile.File, perDepGit bool) upgradeResult {
	Debug.Println("bump module", r.Mod.Path, "from", r.Mod.Version)

	versions, err := proxy.FetchVersions(r.Mod.Path, r.Mod.Version)
	if err != nil {
		Debug.Println("failed to fetch versions:", err.Error())
		return upgradeResult{Mod: okMod}
	}
	Debug.Println("proxy returned", len(versions), "newer version(s) for", r.Mod.Path)
	if len(versions) == 0 {
		return upgradeResult{
			Mod:             okMod,
			Success:         true,
			NoProxyVersions: true,
		}
	}

	for vi, version := range versions {
		if vi >= Config.Retries {
			Debug.Println("too many failed attempts, giving up")
			break
		}

		Debug.Println("attempt", vi+1, "of", min(Config.Retries, len(versions)), r.Mod.Path+"@"+version.Version)
		sumSnap, err := ReadGoSum()
		if err != nil {
			Fatal(err.Error(), ERR_READ)
		}

		newMod, err := attemptUpgrade(r.Mod.Path, version.Version)
		if err != nil {
			Debug.Println("upgrade unsuccessful, reverting go.mod and go.sum")
			if err := RestoreModuleState(okMod, sumSnap); err != nil {
				Debug.Println("failed to revert module state:", err.Error())
			}
			continue
		}

		if err := ValidateUpgrade(okMod, newMod); err != nil {
			Debug.Printf("%s; reverting go.mod and go.sum\n", err.Error())
			if err := RestoreModuleState(okMod, sumSnap); err != nil {
				Debug.Println("failed to revert module state:", err.Error())
			}
			continue
		}

		Debug.Println("compare go directive:", okMod.Go.Version, "=>", newMod.Go.Version)

		if !runCommands(okMod, sumSnap, perDepGit) {
			continue
		}

		return upgradeResult{
			Mod:     newMod,
			Success: true,
		}
	}
	return upgradeResult{Mod: okMod}
}

func runCommands(revertTo *modfile.File, sumSnap []byte, perDepGit bool) bool {
	for _, c := range Config.Commands {
		if c == "" {
			continue
		}
		Debug.Println("running -exec:", c)
		if err := Cmds(c); err != nil {
			Debug.Println("exec failed, reverting go.mod and go.sum")
			if err := RestoreModuleState(revertTo, sumSnap); err != nil {
				Debug.Println("failed to revert module state:", err.Error())
			}
			if perDepGit {
				Debug.Println("git reset worktree after failed -exec")
				if err := gitResetWorktreeClean(); err != nil {
					Debug.Println("git reset worktree failed:", err.Error())
				}
			}
			return false
		}
		Debug.Println("-exec succeeded:", c)
	}
	return true
}

func validateRequestedDependencies(original *modfile.File) error {
	if len(Config.Dependencies) == 0 {
		return nil
	}
	direct := make(map[string]*modfile.Require)
	indirect := make(map[string]bool)
	for _, r := range original.Require {
		if r.Indirect {
			indirect[r.Mod.Path] = true
			continue
		}
		direct[r.Mod.Path] = r
	}
	var problems []string
	for _, name := range Config.Dependencies {
		if _, ok := direct[name]; ok {
			continue
		}
		if indirect[name] {
			problems = append(problems, fmt.Sprintf("%q is an indirect dependency", name))
			continue
		}
		problems = append(problems, fmt.Sprintf("%q is not a direct dependency in go.mod", name))
	}
	if len(problems) > 0 {
		return fmt.Errorf("unknown positional module(s): %s", strings.Join(problems, "; "))
	}
	return nil
}

// selectDependencies returns the list of dependencies to process based on
// Config.Dependencies (positional arguments) or all direct dependencies if none specified.
func selectDependencies(original *modfile.File) []*modfile.Require {
	if len(Config.Dependencies) == 0 {
		return original.Require
	}

	selected := []*modfile.Require{}
	for _, r := range original.Require {
		for _, d := range Config.Dependencies {
			if r.Mod.Path == d {
				selected = append(selected, r)
			}
		}
	}
	return selected
}

// extractVersionAfter finds the new version of a module after an upgrade attempt.
func extractVersionAfter(mod *modfile.File, modulePath, fallback string) string {
	if mod == nil {
		return fallback
	}
	mi := slices.IndexFunc(mod.Require, func(re *modfile.Require) bool {
		return re.Mod.Path == modulePath
	})
	if mi != -1 {
		return mod.Require[mi].Mod.Version
	}
	return fallback
}

// handleGitIntegration performs per-dependency git operations (commit or discard).
// Returns the final success status and version after git operations.
func handleGitIntegration(result upgradeResult, modulePath, versionBefore, versionAfter string) (bool, string) {
	if !result.Success {
		Debug.Println("git reset worktree after failed bump for", modulePath)
		if err := gitResetWorktreeClean(); err != nil {
			Err.Println("git reset worktree failed:", err.Error())
		}
		return false, versionBefore
	}

	if versionAfter == versionBefore || !gitWorktreeDiffersFromHEAD() {
		return true, versionAfter
	}

	Debug.Println("git commit bump for", modulePath, versionBefore, "->", versionAfter)
	if err := GitCommitDependencyBump(modulePath, versionBefore, versionAfter); err != nil {
		Err.Println("git commit failed:", err.Error())
		if err := gitResetWorktreeClean(); err != nil {
			Err.Println("git reset worktree failed:", err.Error())
		}
		return false, versionBefore
	}
	return true, versionAfter
}

func Process(original *modfile.File) []Result {
	if err := validateRequestedDependencies(original); err != nil {
		Fatal(err.Error(), ERR_PARSE)
	}

	var results []Result
	proxy := newGoProxy(Config.ModuleProxy)
	okMod, err := ParseMod(goModFile)
	if err != nil {
		Fatal(err.Error(), ERR_PARSE)
	}

	perDepGit := PerDependencyGitEnabled()
	dependencies := selectDependencies(original)

	for _, r := range dependencies {
		if r.Indirect {
			continue
		}

		excluded := false
		if slices.Contains(Config.Exclude, r.Mod.Path) {
			results = append(results, Result{
				ModulePath:    r.Mod.Path,
				VersionBefore: r.Mod.Version,
				VersionAfter:  r.Mod.Version,
				Success:       false,
				Excluded:      true,
			})
			excluded = true
		}
		if excluded {
			continue
		}

		result := UpgradeModule(proxy, r, okMod, perDepGit)
		versionAfter := extractVersionAfter(result.Mod, r.Mod.Path, r.Mod.Version)
		success := result.Success

		if perDepGit {
			success, versionAfter = handleGitIntegration(result, r.Mod.Path, r.Mod.Version, versionAfter)
			if !success {
				// Git commit failed; reload the current state
				if okMod, err = ParseMod(goModFile); err != nil {
					Fatal(err.Error(), ERR_PARSE)
				}
			}
		}

		if success && versionAfter != r.Mod.Version {
			printConsoleUpdate(r.Mod.Path, versionAfter)
		}

		processResult := Result{
			ModulePath:      r.Mod.Path,
			VersionBefore:   r.Mod.Version,
			VersionAfter:    versionAfter,
			Success:         success,
			NoProxyVersions: result.NoProxyVersions,
		}

		if success {
			okMod = result.Mod
		}

		results = append(results, processResult)
	}

	slices.SortFunc(results, func(a, b Result) int {
		return strings.Compare(a.ModulePath, b.ModulePath)
	})

	return results
}
