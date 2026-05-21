package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

func main() {
	InitConfig()

	if config.Version {
		if info, ok := debug.ReadBuildInfo(); ok {
			fmt.Printf("%s %s\n", info.Main.Version, info.Main.Sum)
			os.Exit(0)
		}
		fmt.Println("(devel) HEAD")
		os.Exit(0)
	}

	InitLoggers()

	if err := errIfUnsafeGitWorktree(); err != nil {
		fatal(err.Error(), ERR_GIT)
	}

	if config.Format == "markdown" {
		printMarkdownHeader()
		defer printMarkdownFooter()
	}

	original, err := parseMod(config.GoModSrc)
	if err != nil {
		fatal(err.Error(), ERR_PARSE)
	}

	defer func() {
		if config.DryRun {
			if err := saveMod(config.GoModDst, original); err != nil {
				fatal(err.Error(), ERR_WRITE)
			}
		}
	}()

	results := process(original)

	printResults(results)
	if config.Changelog && !perDependencyGitEnabled() {
		PrintChangelogs(results)
	}
	if config.FailOnError && resultsHaveErrors(results) {
		os.Exit(1)
	}
}
