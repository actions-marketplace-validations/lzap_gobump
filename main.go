package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

func main() {
	InitConfig()

	if err := PrepareGoModWorkspace(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(ERR_READ)
	}

	if Config.Version {
		if info, ok := debug.ReadBuildInfo(); ok {
			fmt.Printf("%s %s\n", info.Main.Version, info.Main.Sum)
			os.Exit(0)
		}
		fmt.Println("(devel) HEAD")
		os.Exit(0)
	}

	InitLoggers()

	if err := ErrIfUnsafeGitWorktree(); err != nil {
		Fatal(err.Error(), ERR_GIT)
	}

	if Config.Format == "markdown" {
		PrintMarkdownHeader()
		defer PrintMarkdownFooter()
	}

	original, err := ParseMod(Config.GoModSrc)
	if err != nil {
		Fatal(err.Error(), ERR_PARSE)
	}
	originalSum, err := ReadGoSum(Config.GoModSrc)
	if err != nil {
		Fatal(err.Error(), ERR_READ)
	}

	defer func() {
		if Config.DryRun {
			if err := RestoreModuleState(Config.GoModDst, original, originalSum); err != nil {
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
