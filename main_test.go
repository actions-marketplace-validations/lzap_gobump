package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFixtures(t *testing.T) {
	dirs, err := filepath.Glob("testdata/*/")
	if err != nil {
		t.Fatal(err)
	}

	for _, dir := range dirs {
		dir := dir
		if _, err := os.Stat(filepath.Join(dir, goModFile)); err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "go.sum")); err != nil {
			t.Fatalf("%s: missing go.sum", dir)
		}

		name := filepath.Base(strings.TrimSuffix(dir, string(filepath.Separator)))

		switch name {
		case "go-mod-positional":
			t.Run(name, func(t *testing.T) {
				runFixture(t, dir, "github.com/sirupsen/logrus")
			})
		case "go-mod-exclude", "go-mod-exclude-no-positional":
			t.Run(name, func(t *testing.T) {
				runFixture(t, dir, "-exclude", "github.com/sirupsen/logrus")
			})
		default:
			t.Run(name, func(t *testing.T) {
				runFixture(t, dir)
			})
		}
	}
}

func runFixture(t *testing.T, fixtureDir string, extraArgs ...string) {
	t.Helper()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(fixtureDir); err != nil {
		t.Fatal(err)
	}

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	args := []string{"test", "-dry-run", "-exec", "echo ok", "-format", "none"}
	args = append(args, extraArgs...)
	os.Args = args
	main()
}
