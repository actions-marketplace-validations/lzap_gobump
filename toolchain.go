package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

// bundledToolchain is GOTOOLCHAIN for go subprocesses (from Config.GoBinary's GOVERSION).
var bundledToolchain string

func initBundledToolchain() error {
	out, err := exec.Command(Config.GoBinary, "env", "GOVERSION").Output()
	if err != nil {
		return fmt.Errorf("%s env GOVERSION: %w", Config.GoBinary, err)
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return fmt.Errorf("%s env GOVERSION: empty output", Config.GoBinary)
	}
	bundledToolchain = v
	return nil
}

func usesGoToolchain(name string) bool {
	if name == Config.GoBinary {
		return true
	}
	return filepath.Base(name) == "go"
}

func subprocessEnv(name string) []string {
	env := os.Environ()
	if !usesGoToolchain(name) || bundledToolchain == "" {
		return env
	}
	return setEnvVar(env, "GOTOOLCHAIN", bundledToolchain)
}

func setEnvVar(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			if !replaced {
				out = append(out, prefix+value)
				replaced = true
			}
			continue
		}
		out = append(out, e)
	}
	if !replaced {
		out = append(out, prefix+value)
	}
	return out
}

func toolchainIdentity(mod *modfile.File) string {
	if mod == nil || mod.Toolchain == nil {
		return ""
	}
	return strings.TrimSuffix(mod.Toolchain.Name, ".0")
}

func toolchainLabel(mod *modfile.File) string {
	if mod == nil || mod.Toolchain == nil || mod.Toolchain.Name == "" {
		return "(none)"
	}
	return mod.Toolchain.Name
}

func warnIgnoredGoModToolchain(mod *modfile.File) {
	if mod == nil || mod.Toolchain == nil || mod.Toolchain.Name == "" {
		return
	}
	Debug.Printf(
		"warning: ignoring toolchain %s in go.mod; using GOTOOLCHAIN=%s\n",
		mod.Toolchain.Name,
		bundledToolchain,
	)
}
