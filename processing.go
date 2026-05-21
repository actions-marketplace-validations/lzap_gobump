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
	if originalMod == nil || newMod == nil || originalMod.Go == nil || newMod.Go == nil {
		return fmt.Errorf("parsing error")
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
func UpgradeModule(proxy *GoProxy, r *modfile.Require, okMod *modfile.File) upgradeResult {
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

		if !runCommands(okMod, sumSnap) {
			continue
		}

		return upgradeResult{
			Mod:     newMod,
			Success: true,
		}
	}
	return upgradeResult{Mod: okMod}
}

func runCommands(revertTo *modfile.File, sumSnap []byte) bool {
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

func Process(original *modfile.File) []Result {
	var results []Result
	proxy := newGoProxy(Config.ModuleProxy)
	okMod, err := ParseMod(goModFile)
	if err != nil {
		Fatal(err.Error(), ERR_PARSE)
	}

	if err := validateRequestedDependencies(original); err != nil {
		Fatal(err.Error(), ERR_PARSE)
	}

	perDepGit := PerDependencyGitEnabled()

	dependencies := original.Require
	if len(Config.Dependencies) > 0 {
		dependencies = []*modfile.Require{}
		for _, r := range original.Require {
			for _, d := range Config.Dependencies {
				if r.Mod.Path == d {
					dependencies = append(dependencies, r)
				}
			}
		}
	}

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

		result := UpgradeModule(proxy, r, okMod)

		versionAfter := r.Mod.Version
		if result.Mod != nil {
			mi := slices.IndexFunc(result.Mod.Require, func(re *modfile.Require) bool {
				return re.Mod.Path == r.Mod.Path
			})
			if mi != -1 {
				versionAfter = result.Mod.Require[mi].Mod.Version
			}
		}

		if perDepGit {
			if !result.Success {
				Debug.Println("git discard go.mod/go.sum after failed bump for", r.Mod.Path)
				if err := gitDiscardGoModSumChanges(); err != nil {
					Err.Println("git discard go.mod/go.sum failed:", err.Error())
				}
			} else if versionAfter != r.Mod.Version && gitWorktreeDiffersFromHEAD() {
				Debug.Println("git commit bump for", r.Mod.Path, r.Mod.Version, "->", versionAfter)
				if err := GitCommitDependencyBump(r.Mod.Path, r.Mod.Version, versionAfter); err != nil {
					Err.Println("git commit failed:", err.Error())
					if err := gitDiscardGoModSumChanges(); err != nil {
						Err.Println("git discard go.mod/go.sum failed:", err.Error())
					}
					if okMod, err = ParseMod(goModFile); err != nil {
						Fatal(err.Error(), ERR_PARSE)
					}
					result.Success = false
					versionAfter = r.Mod.Version
				}
			}
		}

		if result.Success && versionAfter != r.Mod.Version {
			printConsoleUpdate(r.Mod.Path, versionAfter)
		}

		processResult := Result{
			ModulePath:      r.Mod.Path,
			VersionBefore:   r.Mod.Version,
			VersionAfter:    versionAfter,
			Success:         result.Success,
			NoProxyVersions: result.NoProxyVersions,
		}

		if result.Success {
			okMod = result.Mod
		}

		results = append(results, processResult)
	}

	slices.SortFunc(results, func(a, b Result) int {
		return strings.Compare(a.ModulePath, b.ModulePath)
	})

	return results
}
