package main

import (
	"io"
	"log"
	"os"
	"strings"
)

var (
	Out   *log.Logger
	Debug *log.Logger
	Err   *log.Logger
)

func InitLoggers() {
	Err = log.New(os.Stderr, "", 0)
	if debugEnabled() {
		Debug = Err
	} else {
		Debug = log.New(io.Discard, "", 0)
	}

	outWriter := io.Writer(os.Stdout)
	if config.Format == "none" {
		outWriter = io.Discard
	}
	Out = log.New(outWriter, "", 0)
}

func fatal(msg string, code int) {
	Err.Println(msg)
	os.Exit(code)
}

func debugEnabled() bool {
	return config != nil && config.Verbose
}

func strOrDash(str string) string {
	if str == "" {
		return "-"
	}
	return str
}

func printResults(results []Result) {
	switch config.Format {
	case "markdown":
		printMarkdownResults(results)
	case "console":
		printConsoleResults(results)
	}
}

func printMarkdownHeader() {
	Out.Println("## Pinned Go version dependency update")
}

func printMarkdownFooter() {
	Out.Printf("\n:pretzel: *Created with [gobump](https://github.com/lzap/gobump) (%s)* :pretzel:\n", BuildID())
}

func markdownTableRow(cells ...string) string {
	return "| " + strings.Join(cells, " | ") + " |"
}

func markdownStatus(r Result) string {
	if r.Excluded {
		return "X"
	}
	if r.NoProxyVersions {
		return "N"
	}
	if r.Success {
		if r.VersionAfter == r.VersionBefore {
			return "-"
		}
		return "U"
	}
	return "E"
}

func printMarkdownResults(results []Result) {
	Out.Println("| Module | Status | Version |")
	Out.Println("| --- | --- | --- |")
	for _, r := range results {
		Out.Println(markdownTableRow(
			r.ModulePath,
			markdownStatus(r),
			strOrDash(r.VersionBefore)+" > "+strOrDash(r.VersionAfter),
		))
	}
	Out.Println("Status: **U** updated, **E** error, **X** excluded, **N** no newer versions, **-** unchanged.")
}

func consoleStatus(r Result) string {
	if r.Excluded {
		return "excluded"
	}
	if r.NoProxyVersions {
		return "noop"
	}
	if r.Success {
		if r.VersionAfter == r.VersionBefore {
			return "keep"
		}
		return "update"
	}
	return "err"
}

func printConsoleUpdate(path, version string) {
	if config.Format != "console" {
		return
	}
	Out.Printf("go get %s@%s\n", path, version)
}

func printConsoleResults(results []Result) {
	Out.Println(color("summary:", ColorBold))
	for _, r := range results {
		action := consoleStatus(r)
		if r.VersionAfter != "" && r.VersionAfter != r.VersionBefore {
			Out.Println(r.ModulePath, action, r.VersionBefore, "->", r.VersionAfter)
		} else {
			Out.Println(r.ModulePath, action)
		}
	}
}
