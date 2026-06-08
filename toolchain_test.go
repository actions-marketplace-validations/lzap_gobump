package main

import (
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

func TestValidateUpgradeRejectsToolchainChange(t *testing.T) {
	orig, err := modfile.Parse("go.mod", []byte("module m\n\ngo 1.22\n\ntoolchain go1.22.0\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	newMod, err := modfile.Parse("go.mod", []byte("module m\n\ngo 1.22\n\ntoolchain go1.23.0\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	err = ValidateUpgrade(orig, newMod)
	if err == nil {
		t.Fatal("expected error when toolchain changes")
	}
	if !strings.Contains(err.Error(), "toolchain") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateUpgradeAllowsUnchangedToolchain(t *testing.T) {
	orig, err := modfile.Parse("go.mod", []byte("module m\n\ngo 1.22\n\ntoolchain go1.22.0\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateUpgrade(orig, orig); err != nil {
		t.Fatal(err)
	}
}

func TestSetEnvVarReplacesGOTOOLCHAIN(t *testing.T) {
	env := []string{"PATH=/bin", "GOTOOLCHAIN=local", "HOME=/tmp"}
	got := setEnvVar(env, "GOTOOLCHAIN", "go1.22.0")
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3: %v", len(got), got)
	}
	found := false
	for _, e := range got {
		if e == "GOTOOLCHAIN=go1.22.0" {
			found = true
		}
		if e == "GOTOOLCHAIN=local" {
			t.Fatal("old GOTOOLCHAIN still present")
		}
	}
	if !found {
		t.Fatalf("GOTOOLCHAIN not set: %v", got)
	}
}

func TestInitBundledToolchain(t *testing.T) {
	Config = &AppConfig{GoBinary: "go"}
	if err := initBundledToolchain(); err != nil {
		t.Skip("go not available:", err)
	}
	if !strings.HasPrefix(bundledToolchain, "go") {
		t.Fatalf("bundledToolchain = %q, want go-prefixed version", bundledToolchain)
	}
}

func TestWarnIgnoredGoModToolchainVerbose(t *testing.T) {
	var stderr strings.Builder
	withLoggers(t, nil, &stderr, true, "none")
	bundledToolchain = "go1.22.0"

	mod, err := modfile.Parse("go.mod", []byte("module m\n\ngo 1.22\n\ntoolchain go1.99.0\n"), nil)
	if err != nil {
		t.Fatal(err)
	}

	warnIgnoredGoModToolchain(mod)
	if !strings.Contains(stderr.String(), "ignoring toolchain") {
		t.Fatalf("expected warning on stderr, got: %q", stderr.String())
	}
}
