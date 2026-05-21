package main

import (
	"strings"
	"testing"
)

func TestGitHubRepoFromOriginURL(t *testing.T) {
	tests := []struct {
		url       string
		wantOwner string
		wantRepo  string
		wantOK    bool
	}{
		{"https://github.com/google/go-cmp", "google", "go-cmp", true},
		{"https://github.com/google/go-cmp/", "google", "go-cmp", true},
		{"https://go.googlesource.com/mod", "golang", "mod", true},
		{"https://example.com/foo", "", "", false},
	}
	for _, tt := range tests {
		owner, repo, ok := GitHubRepoFromOriginURL(tt.url)
		if ok != tt.wantOK || owner != tt.wantOwner || repo != tt.wantRepo {
			t.Errorf("GitHubRepoFromOriginURL(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.url, owner, repo, ok, tt.wantOwner, tt.wantRepo, tt.wantOK)
		}
	}
}

func TestGetChangelogGolangOrgModule(t *testing.T) {
	if testing.Short() {
		t.Skip("network")
	}
	Config = &AppConfig{}
	changelog, err := GetChangelog("golang.org/x/mod", "v0.30.0", "v0.33.0")
	if err != nil {
		t.Fatal(err)
	}
	if changelog == "" {
		t.Fatal("expected non-empty changelog for golang.org/x/mod")
	}
	if !strings.Contains(changelog, "* ") {
		t.Fatalf("expected bullet commits: %q", changelog)
	}
}

func TestShortCommitSHA(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"abc", "abc"},
		{"abcdefg", "abcdefg"},
		{"abcdefgh", "abcdefg"},
	}
	for _, tt := range tests {
		if got := ShortCommitSHA(tt.in); got != tt.want {
			t.Errorf("ShortCommitSHA(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
