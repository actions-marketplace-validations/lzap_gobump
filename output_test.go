package main

import (
	"bytes"
	"io"
	"log"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func WithLoggers(t *testing.T, stdout, stderr io.Writer, verbose bool, format string) {
	t.Helper()
	Config = &AppConfig{Verbose: verbose, Format: format}
	InitLoggers()
	if stdout != nil {
		Out = log.New(stdout, "", 0)
	}
	if stderr != nil {
		Err = log.New(stderr, "", 0)
		Debug = newVerboseLogger(stderr)
	}
}

func TestPrintMarkdownResults(t *testing.T) {
	var buf bytes.Buffer
	WithLoggers(t, &buf, nil, false, "markdown")

	PrintMarkdownResults([]Result{
		{
			ModulePath:    "example.com/mod",
			Success:       true,
			VersionBefore: "v1.0.0",
			VersionAfter:  "v2.0.0",
		},
		{
			ModulePath:    "example.com/unchanged",
			Success:       true,
			VersionBefore: "v1.0.0",
			VersionAfter:  "v1.0.0",
		},
	})

	expected := `| Module | Status | Version |
| --- | --- | --- |
| example.com/mod | U | v1.0.0 > v2.0.0 |
| example.com/unchanged | - | v1.0.0 > v1.0.0 |
Status: **U** updated, **E** error, **X** excluded, **N** no newer versions, **-** unchanged.
`
	if diff := cmp.Diff(expected, buf.String()); diff != "" {
		t.Errorf("markdown results mismatch (-want +got):\n%s", diff)
	}
}

func TestPrintMarkdownMinimalStdout(t *testing.T) {
	var buf bytes.Buffer
	WithLoggers(t, &buf, nil, false, "markdown")

	PrintMarkdownHeader()
	PrintMarkdownResults([]Result{
		{
			ModulePath:    "example.com/mod",
			Success:       true,
			VersionBefore: "v1.0.0",
			VersionAfter:  "v2.0.0",
		},
	})
	PrintMarkdownFooter()

	expected := `## Pinned Go version dependency update
| Module | Status | Version |
| --- | --- | --- |
| example.com/mod | U | v1.0.0 > v2.0.0 |
Status: **U** updated, **E** error, **X** excluded, **N** no newer versions, **-** unchanged.

:pretzel: *Created with [gobump](https://github.com/lzap/gobump) (HEAD)* :pretzel:
`
	if diff := cmp.Diff(expected, buf.String()); diff != "" {
		t.Errorf("minimal markdown stdout mismatch (-want +got):\n%s", diff)
	}
}

func TestPrintConsoleResults(t *testing.T) {
	var buf bytes.Buffer
	WithLoggers(t, &buf, nil, false, "console")

	PrintConsoleResults([]Result{
		{
			ModulePath:    "example.com/mod",
			Success:       true,
			VersionBefore: "v1.0.0",
			VersionAfter:  "v2.0.0",
		},
		{
			ModulePath:    "example.com/broken",
			Success:       false,
			VersionBefore: "v1.0.0",
			VersionAfter:  "v1.0.0",
		},
	})

	if !strings.Contains(buf.String(), "summary:") {
		t.Fatalf("expected summary header, got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "example.com/mod update v1.0.0 -> v2.0.0") {
		t.Fatalf("expected update line, got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "example.com/broken err") {
		t.Fatalf("expected err line, got:\n%s", buf.String())
	}
}

func TestDebugDiscardedWhenNotVerbose(t *testing.T) {
	var stderr bytes.Buffer
	WithLoggers(t, nil, &stderr, false, "none")

	Debug.Println("should not appear")
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got:\n%s", stderr.String())
	}
}

func TestErrAlwaysWrites(t *testing.T) {
	var stderr bytes.Buffer
	WithLoggers(t, nil, &stderr, false, "none")

	Err.Println("visible error")
	if !strings.Contains(stderr.String(), "visible error") {
		t.Fatalf("expected error on stderr, got:\n%s", stderr.String())
	}
}
