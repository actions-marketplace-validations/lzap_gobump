package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// GitHubToken returns a token for GitHub API calls (changelog compare, gist).
// GH_TOKEN is set by the GitHub CLI and many CI setups; GITHUB_TOKEN is the Actions default.
func GitHubToken() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("GH_TOKEN")
}

// GitHubCommit represents a single commit in the GitHub API response.
type GitHubCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"commit"`
}

// GitHubCompareResponse represents the response from the GitHub compare API.
type GitHubCompareResponse struct {
	Commits []GitHubCommit `json:"commits"`
}

// GistFile represents a file in a Gist.
type GistFile struct {
	Content string `json:"content"`
}

// GistRequest represents the request to create a Gist.
type GistRequest struct {
	Description string              `json:"description"`
	Public      bool                `json:"public"`
	Files       map[string]GistFile `json:"files"`
}

// GistResponse represents the response from creating a Gist.
type GistResponse struct {
	HTMLURL string `json:"html_url"`
}

func ShortCommitSHA(sha string) string {
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}

// GitHubRepoFromOriginURL maps a module VCS origin URL to a GitHub owner/repo for compare API calls.
func GitHubRepoFromOriginURL(originURL string) (owner, repo string, ok bool) {
	originURL = strings.TrimSuffix(strings.TrimSpace(originURL), "/")
	switch {
	case strings.HasPrefix(originURL, "https://github.com/"):
		rest := strings.TrimPrefix(originURL, "https://github.com/")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return parts[0], parts[1], true
		}
	case strings.HasPrefix(originURL, "http://github.com/"):
		rest := strings.TrimPrefix(originURL, "http://github.com/")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return parts[0], parts[1], true
		}
	case strings.HasPrefix(originURL, "https://go.googlesource.com/"):
		name := strings.TrimPrefix(originURL, "https://go.googlesource.com/")
		if name != "" && !strings.Contains(name, "/") {
			return "golang", name, true
		}
	}
	return "", "", false
}

func FormatGitHubCompareCommits(commits []GitHubCommit) string {
	var changelog strings.Builder
	for _, commit := range commits {
		firstLine := strings.Split(commit.Commit.Message, "\n")[0]
		changelog.WriteString(fmt.Sprintf("* %s: %s (%s)\n", ShortCommitSHA(commit.SHA), firstLine, commit.Commit.Author.Name))
	}
	return changelog.String()
}

func FetchGitHubCompare(owner, repo, compareRange string) (GitHubCompareResponse, int, error) {
	var compareResp GitHubCompareResponse
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/compare/%s", owner, repo, compareRange)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
	if err != nil {
		return compareResp, 0, fmt.Errorf("failed to build GitHub request: %w", err)
	}
	SetDefaultHTTPHeaders(req)
	if tok := GitHubToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := NewHTTPClient().Do(req)
	if err != nil {
		return compareResp, 0, fmt.Errorf("failed to fetch changelog from GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		DiscardBody(resp)
		return compareResp, resp.StatusCode, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(&compareResp); err != nil {
		return compareResp, resp.StatusCode, fmt.Errorf("failed to decode GitHub API response: %w", err)
	}
	return compareResp, resp.StatusCode, nil
}

// GetChangelog fetches upstream commits between two module versions via the module proxy and GitHub.
func GetChangelog(modulePath, fromVersion, toVersion string) (string, error) {
	proxy := NewGoProxy(Config.ModuleProxy)
	fromInfo, err := proxy.FetchVersionInfo(modulePath, fromVersion)
	if err != nil {
		return "", err
	}
	toInfo, err := proxy.FetchVersionInfo(modulePath, toVersion)
	if err != nil {
		return "", err
	}
	owner, repo, ok := GitHubRepoFromOriginURL(toInfo.Origin.URL)
	if !ok {
		owner, repo, ok = GitHubRepoFromOriginURL(fromInfo.Origin.URL)
	}
	if !ok {
		return "", nil
	}

	compareRange := fromVersion + "..." + toVersion
	compareResp, status, err := FetchGitHubCompare(owner, repo, compareRange)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK && fromInfo.Origin.Hash != "" && toInfo.Origin.Hash != "" {
		compareRange = fromInfo.Origin.Hash + "..." + toInfo.Origin.Hash
		compareResp, status, err = FetchGitHubCompare(owner, repo, compareRange)
		if err != nil {
			return "", err
		}
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned non-200 status: %d", status)
	}
	return FormatGitHubCompareCommits(compareResp.Commits), nil
}

// FormatModuleChangelog returns a changelog section for one module bump.
func FormatModuleChangelog(modulePath, versionBefore, versionAfter string) string {
	var sb strings.Builder
	sb.WriteString("\n\nCHANGELOG:\n")
	changelog, err := GetChangelog(modulePath, versionBefore, versionAfter)
	if err != nil {
		fmt.Fprintf(&sb, "Failed to get changelog: %s\n", err.Error())
	} else if changelog == "" {
		sb.WriteString("No commits found between versions.\n")
	} else {
		sb.WriteString(changelog)
	}
	return sb.String()
}

// CreateGist creates a new GitHub Gist with the provided content.
func CreateGist(token, description, content string) (string, error) {
	gistRequest := GistRequest{
		Description: description,
		Public:      false,
		Files: map[string]GistFile{
			"changelog.md": {
				Content: content,
			},
		},
	}

	requestBody, err := json.Marshal(gistRequest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Gist request: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://api.github.com/gists", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create Gist request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	SetDefaultHTTPHeaders(req)

	client := NewHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send Gist request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("GitHub API returned non-201 status for Gist creation: %s", resp.Status)
	}

	var gistResponse GistResponse
	if err := json.NewDecoder(resp.Body).Decode(&gistResponse); err != nil {
		return "", fmt.Errorf("failed to decode Gist response: %w", err)
	}

	return gistResponse.HTMLURL, nil
}

// PrintChangelogs prints the changelogs for all updated modules.
func PrintChangelogs(results []Result) {

	if Config.ChangelogDest == "gist" {
		var fullChangelog strings.Builder
		fullChangelog.WriteString("# GoBump Changelog\n\n")
		for _, result := range results {
			if result.Success && result.VersionBefore != result.VersionAfter {
				fullChangelog.WriteString(fmt.Sprintf("## %s\n\n", result.ModulePath))
				fullChangelog.WriteString(fmt.Sprintf("Updated from `%s` to `%s`\n\n", result.VersionBefore, result.VersionAfter))
				changelog, err := GetChangelog(result.ModulePath, result.VersionBefore, result.VersionAfter)
				if err != nil {
					fullChangelog.WriteString(fmt.Sprintf("Failed to get changelog: %s\n\n", err.Error()))
				} else if changelog == "" {
					fullChangelog.WriteString("No commits found between versions.\n\n")
				} else {
					fullChangelog.WriteString(changelog + "\n")
				}
			}
		}
		token := GitHubToken()
		if token == "" {
			Err.Println("Failed to create Gist: no GITHUB_TOKEN or GH_TOKEN set")
			return
		}
		gistURL, err := CreateGist(token, "GoBump Dependency Changelog", fullChangelog.String())
		if err != nil {
			Err.Println("Failed to create Gist:", err.Error())
		} else {
			Debug.Println("Changelog Gist created:", gistURL)
		}
	} else {
		sb := strings.Builder{}
		for _, result := range results {
			if result.Success && result.VersionBefore != result.VersionAfter {
				sb.WriteString(FormatModuleChangelog(result.ModulePath, result.VersionBefore, result.VersionAfter))
			}
		}

		if Config.ChangelogDest == "stdout" {
			if s := sb.String(); s != "" {
				Out.Print(s)
			}
		} else if Config.ChangelogDest != "" {
			err := os.WriteFile(Config.ChangelogDest, []byte(sb.String()), 0644)
			if err != nil {
				Err.Println("Failed to write changelog to file:", err.Error())
			}
		}
	}
}
