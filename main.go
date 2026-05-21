package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

func main() {
	InitConfig()

	if Config.Version {
		if info, ok := debug.ReadBuildInfo(); ok {
			fmt.Printf("%s %s\n", info.Main.Version, info.Main.Sum)
			os.Exit(0)
		}
		fmt.Println("(devel) HEAD")
		os.Exit(0)
	}

	InitLoggers()

	if err := initBundledToolchain(); err != nil {
		Fatal(err.Error(), ERR_CMD)
	}

	if err := ErrIfUnsafeGitWorktree(); err != nil {
		Fatal(err.Error(), ERR_GIT)
	}

	if Config.Format == "markdown" {
		printMarkdownHeader()
		defer printMarkdownFooter()
	}

	original, err := ParseMod(goModFile)
	if err != nil {
		Fatal(err.Error(), ERR_PARSE)
	}
	warnIgnoredGoModToolchain(original)
	originalSum, err := ReadGoSum()
	if err != nil {
		Fatal(err.Error(), ERR_READ)
	}

	defer func() {
		if Config.DryRun {
			if err := RestoreModuleState(original, originalSum); err != nil {
				Fatal(err.Error(), ERR_WRITE)
			}
		}
	}()

	results := Process(original)

	PrintResults(results)
	if Config.Changelog && !PerDependencyGitEnabled() {
		PrintChangelogs(results)
	}
	if Config.FailOnError && ResultsHaveErrors(results) {
		os.Exit(1)
	}
}
