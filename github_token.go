package main

import "os"

// GithubToken returns a token for GitHub API calls (changelog compare, gist).
// GH_TOKEN is set by the GitHub CLI and many CI setups; GITHUB_TOKEN is the Actions default.
func GithubToken() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("GH_TOKEN")
}
