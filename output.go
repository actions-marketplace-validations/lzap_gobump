package main

import (
	"fmt"
	"os"
	"strings"
)

type Output interface {
	Begin(text ...any)
	Header(text string)
	BeginPreformatted(text ...any)
	EndPreformatted(text ...any)
	EndPreformattedCond(render bool, text ...any)
	End(text ...any)
	Error(str ...string)
	Debug(text ...any)
	Debugf(format string, args ...any)
	Fatal(msg string, code ...int)
	Write(buf []byte) (int, error)
	Println(text ...string)
	PrintSummary(results []Result)
}

func joinAny(text ...any) string {
	if len(text) == 0 {
		return ""
	}

	var str []string
	for _, t := range text {
		str = append(str, fmt.Sprint(t))
	}
	return strings.Join(str, " ")
}

func strOrDash(str string) string {
	if str == "" {
		return "-"
	}
	return str
}

func debugEnabled() bool {
	return config != nil && config.Verbose
}

func writeDebugStderr(text ...any) {
	if len(text) == 0 {
		return
	}
	writeDebugfStderr("%s", joinAny(text...))
}

func writeDebugfStderr(format string, args ...any) {
	if !debugEnabled() {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
