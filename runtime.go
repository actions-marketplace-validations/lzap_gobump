package main

import "runtime/debug"

var (
	// Use linker flag to customize it: -X 'github.com/lzap/gobump.BuildCommit=1234567
	BuildCommit string = "HEAD"

	// Use linker flag to customize it: -X 'github.com/lzap/gobump.BuildTimestamp=2021-01-01T00:00:00Z'
	BuildTimestamp string
)

func init() {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, bs := range bi.Settings {
			switch bs.Key {
			case "vcs.revision":
				if len(bs.Value) > BuildCommitChars {
					BuildCommit = bs.Value[0:BuildCommitChars]
				}
			case "vcs.time":
				BuildTimestamp = bs.Value
			}
		}
	}
}

// buildID returns the build ID, typically a git commit but can be overriden via a linker flag.
// This is the short version, up to BuildCommitChars characters long.
func buildID() string {
	return BuildCommit
}
