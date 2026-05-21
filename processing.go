package main

import (
	"fmt"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
)

// AttemptUpgrade tries to upgrade a module to a specific version.
func AttemptUpgrade(modulePath, version string) (*modfile.File, error) {
	err := Cmd(Config.GoBinary, "get", modulePath+"@"+version)
	if err != nil {
		return nil, fmt.Errorf("failed to get module: %w", err)
	}
	return ParseMod(Config.GoModSrc)
}

// ValidateUpgrade checks if the upgrade is valid.
func ValidateUpgrade(originalMod, newMod *modfile.File) error {
	if newMod == nil || newMod.Go == nil {
		return fmt.Errorf("parsing error")
	}
	if strings.TrimSuffix(originalMod.Go.Version, ".0") != strings.TrimSuffix(newMod.Go.Version, ".0") {
		return fmt.Errorf("upgrade changes required Go version %s => %s", originalMod.Go.Version, newMod.Go.Version)
	}
	return nil
}

// UpgradeModule attempts to upgrade a single module.
// The bool is success; the third return is true when the proxy listed no newer versions (no go get run).
func UpgradeModule(proxy *GoProxy, r *modfile.Require, okMod *modfile.File) (*modfile.File, bool, bool) {
	var success bool
	var noProxyVersions bool
	Debug.Println("bump module", r.Mod.Path, "from", r.Mod.Version)

	versions, err := proxy.FetchVersions(r.Mod.Path, r.Mod.Version)
	if err != nil {
		Debug.Println("failed to fetch versions:", err.Error())
		return okMod, success, false
	}
	Debug.Println("proxy returned", len(versions), "newer version(s) for", r.Mod.Path)
	if len(versions) == 0 {
		success = true
		noProxyVersions = true
		return okMod, success, noProxyVersions
	}

	for vi, version := range versions {
		if vi >= Config.Retries {
			Debug.Println("too many failed attempts, giving up")
			break
		}

		Debug.Println("attempt", vi+1, "of", min(Config.Retries, len(versions)), r.Mod.Path+"@"+version.Version)
		newMod, err := AttemptUpgrade(r.Mod.Path, version.Version)
		if err != nil {
			Debug.Println("upgrade unsuccessful, reverting go.mod")
			if err := SaveMod(Config.GoModDst, okMod); err != nil {
				Debug.Println("failed to revert go.mod:", err.Error())
			}
			continue
		}

		if err := ValidateUpgrade(okMod, newMod); err != nil {
			Debug.Printf("%s; reverting go.mod\n", err.Error())
			if err := SaveMod(Config.GoModDst, okMod); err != nil {
				Debug.Println("failed to revert go.mod:", err.Error())
			}
			continue
		}

		Debug.Println("compare go directive:", okMod.Go.Version, "=>", newMod.Go.Version)

		if !RunCommands(okMod) {
			continue
		}

		success = true
		return newMod, success, false
	}
	return okMod, success, false
}

// RunCommands executes post-upgrade commands against the current go.mod on disk
// (expected to match a successful upgrade). On failure it restores revertTo.
func RunCommands(revertTo *modfile.File) bool {
	for _, c := range Config.Commands {
		if c == "" {
			continue
		}
		Debug.Println("running -exec:", c)
		if err := Cmds(c); err != nil {
			Debug.Println("exec failed, reverting go.mod")
			if err := SaveMod(Config.GoModDst, revertTo); err != nil {
				Debug.Println("failed to revert go.mod:", err.Error())
			}
			return false
		}
		Debug.Println("-exec succeeded:", c)
	}
	return true
}

func Process(original *modfile.File) []Result {
	var results []Result
	proxy := NewGoProxy(Config.ModuleProxy)
	okMod, err := ParseMod(Config.GoModSrc)
	if err != nil {
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

		newMod, upgradeSuccess, noProxyVersions := UpgradeModule(proxy, r, okMod)

		versionAfter := r.Mod.Version
		if newMod != nil {
			mi := slices.IndexFunc(newMod.Require, func(re *modfile.Require) bool {
				return re.Mod.Path == r.Mod.Path
			})
			if mi != -1 {
				versionAfter = newMod.Require[mi].Mod.Version
			}
		}

		if perDepGit {
			if !upgradeSuccess {
				Debug.Println("git reset/clean after failed bump for", r.Mod.Path)
				if err := GitResetHardHEAD(); err != nil {
					Err.Println("git reset/clean failed:", err.Error())
				}
			} else if versionAfter != r.Mod.Version && GitWorktreeDiffersFromHEAD() {
				Debug.Println("git commit bump for", r.Mod.Path, r.Mod.Version, "->", versionAfter)
				if err := GitCommitDependencyBump(r.Mod.Path, r.Mod.Version, versionAfter); err != nil {
					Err.Println("git commit failed:", err.Error())
				}
			}
		}

		if upgradeSuccess && versionAfter != r.Mod.Version {
			PrintConsoleUpdate(r.Mod.Path, versionAfter)
		}

		result := Result{
			ModulePath:      r.Mod.Path,
			VersionBefore:   r.Mod.Version,
			VersionAfter:    versionAfter,
			NoProxyVersions: noProxyVersions,
		}

		if upgradeSuccess {
			okMod = newMod
			result.Success = true
		} else {
			result.Success = false
		}

		results = append(results, result)
	}

	slices.SortFunc(results, func(a, b Result) int {
		return strings.Compare(a.ModulePath, b.ModulePath)
	})

	return results
}
