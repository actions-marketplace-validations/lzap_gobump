package main

import "time"

// File names
const (
	goModFile = "go.mod"
)

// Exit codes
const (
	ERR_READ  = 2
	ERR_WRITE = 3
	ERR_PARSE = 4
	ERR_CMD   = 5
	ERR_GIT   = 6
)

// HTTP client configuration
const (
	HTTPUserAgent = "gobump (https://github.com/lzap/gobump)"
	// HTTPClientTimeout bounds how long outbound HTTP (module proxy, GitHub API) may block.
	HTTPClientTimeout = 90 * time.Second
)

// Build metadata
const (
	// BuildCommitChars is the number of characters to show in the build commit.
	BuildCommitChars = 5
)
