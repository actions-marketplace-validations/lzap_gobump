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
	if DebugEnabled() {
		Debug = Err
	} else {
		Debug = log.New(io.Discard, "", 0)
	}

	outWriter := io.Writer(os.Stdout)
	if Config.Format == "none" {
		outWriter = io.Discard
	}
	Out = log.New(outWriter, "", 0)
}

func Fatal(msg string, code int) {
	Err.Println(msg)
	os.Exit(code)
}

func DebugEnabled() bool {
	return Config != nil && Config.Verbose
}

func StrOrDash(str string) string {
	if str == "" {
		return "-"
	}
	return str
}

func PrintResults(results []Result) {
	switch Config.Format {
	case "markdown":
		PrintMarkdownResults(results)
	case "console":
		PrintConsoleResults(results)
	}
}

func PrintMarkdownHeader() {
	Out.Println("## Pinned Go version dependency update")
}

func PrintMarkdownFooter() {
	Out.Printf("\n:pretzel: *Created with [gobump](https://github.com/lzap/gobump) (%s)* :pretzel:\n", BuildID())
}

func MarkdownTableRow(cells ...string) string {
	return "| " + strings.Join(cells, " | ") + " |"
}

func MarkdownStatus(r Result) string {
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

func PrintMarkdownResults(results []Result) {
	Out.Println("| Module | Status | Version |")
	Out.Println("| --- | --- | --- |")
	for _, r := range results {
		Out.Println(MarkdownTableRow(
			r.ModulePath,
			MarkdownStatus(r),
			StrOrDash(r.VersionBefore)+" > "+StrOrDash(r.VersionAfter),
		))
	}
	Out.Println("Status: **U** updated, **E** error, **X** excluded, **N** no newer versions, **-** unchanged.")
}

func ConsoleStatus(r Result) string {
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

func PrintConsoleUpdate(path, version string) {
	if Config.Format != "console" {
		return
	}
	Out.Printf("go get %s@%s\n", path, version)
}

func PrintConsoleResults(results []Result) {
	Out.Println("summary:")
	for _, r := range results {
		action := ConsoleStatus(r)
		if r.VersionAfter != "" && r.VersionAfter != r.VersionBefore {
			Out.Println(r.ModulePath, action, r.VersionBefore, "->", r.VersionAfter)
		} else {
			Out.Println(r.ModulePath, action)
		}
	}
}
