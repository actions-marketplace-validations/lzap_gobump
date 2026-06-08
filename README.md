# gobump

Pins Go version when bumping dependencies.

A simple tool that upgrades all direct dependencies one by one, ensuring the `go` version statement in `go.mod` is never touched. It runs all `go` commands with the bundled Go binary (via `GOTOOLCHAIN`) and does not honor a `toolchain` line in the project's `go.mod`. This is useful if your build infrastructure lags behind the latest Go version and you are unable to upgrade, for example, when using Go from Linux distribution packages or a container runtime like the Red Hat Go Toolset for UBI.

## The problem

When `go get -u ./...` is issued, at some point `go X.YY` in `go.mod` will be upgraded with the following message:

```
$ go get -u ./...
go: upgraded go 1.21.0 => 1.22.0
```

Using an explicit version of the Go binary does not change anything:

```
$ go1.21.0 get -u ./...
go: upgraded go 1.21.0 => 1.22.0
```

Starting from Go 1.21, the toolchain feature was added to solve some problems with tool versioning. It also skips upgrades when the toolchain version is explicitly set, but it has a different problem. When a single dependency cannot be upgraded, it skips the entire upgrade transaction, leading to no upgrades.

In the following scenario, the `github.com/google/go-cmp` package could have been upgraded as it was working on Go 1.21 at the time; however, nothing was upgraded:

```
$ GOTOOLCHAIN=go1.21.0 go get -u ./...
go: golang.org/x/mod@v0.24.0 requires go >= 1.23.0 (running go 1.21.0; GOTOOLCHAIN=go1.21.0)
go: golang.org/x/sys@v0.33.0 requires go >= 1.23.0 (running go 1.21.0; GOTOOLCHAIN=go1.21.0)
go: golang.org/x/term@v0.32.0 requires go >= 1.23.0 (running go 1.21.0; GOTOOLCHAIN=go1.21.0)
```

Only when dependencies are upgraded one by one, it works:

```
$ GOTOOLCHAIN=go1.21.0 go get -u github.com/google/go-cmp
go: upgraded github.com/google/go-cmp v0.3.0 => v0.7.0
```

This utility upgrades dependencies one by one, optionally running `go build` or `go test` when configured to ensure the project builds. This is useful for mass-upgrading dependencies to isolate those that break tests.

When a dependency cannot be upgraded (or optional commands fail to execute, e.g., `go test`), it retries several times (configurable) with older versions of the module until it succeeds. By default, it goes back up to 5 versions.

## Installation

```
go install github.com/lzap/gobump@latest
```

This repository’s `go.mod` targets a recent Go release; use a `gobump` binary built with a Go toolchain that matches your environment if you need to run the tool itself on an older Go version.

## Usage

```
  -changelog
    	fetch upstream git changelog for each updated module (embedded in per-dependency commit messages when git integration is enabled; otherwise aggregated at end per -changelog-dest)
  -changelog-dest string
    	with -changelog and -no-git (or no usable git work tree): write aggregated changelogs to stdout (default), a file path, or "gist"; ignored when changelogs are committed per dependency (default "stdout")
  -dry-run
    	revert to original go.mod and go.sum after running
  -exclude value
    	comma-separated list of modules to exclude from update
  -exec value
    	exec command for each individual bump, can be used multiple times
  -fail-on-error
    	exit with status 1 if any non-excluded module failed to update
  -format string
    	output format (console, markdown, none) (default "console")
  -proxy string
    	module proxy base URL (default: first usable $GOPROXY entry, else https://proxy.golang.org)
  -retries int
    	number of downgrade retries for each module (default: 5) (default 5)
  -no-git
    	if true, skip all git operations (no per-dependency commits or reset/clean on failure)
  -user-email string
    	git user.email for per-dependency commits (local repo config) (default "schutzbot@gmail.com")
  -user-name string
    	git user.name for per-dependency commits (local repo config) (default "Schutzbot")
  -verbose
    	print more information including stderr of executed commands
  -version
    	print gobump version and module checksum
```

With `-changelog`, upstream commits between the old and new module versions are fetched (via the module proxy and GitHub). By default, each successful bump commit includes that module’s changelog in the message body. Use `-no-git` if you prefer a single aggregated changelog at the end instead.

`-changelog-dest` only applies together with `-changelog` when per-dependency git commits are **not** used (`-no-git`, `-dry-run`, or no clean git work tree):

* `stdout` (default) — print the aggregated changelogs after the summary
* a file path — write the aggregated changelogs to that file (the GitHub Action uses `/tmp/changelog.txt` this way when `no_git` is true)
* `gist` — create one private GitHub Gist with all changelogs; the URL is printed (requires `GITHUB_TOKEN` or `GH_TOKEN`; never pass a token on the command line)

The utility can also take one or more module paths as positional arguments. When provided, only those dependencies will be updated, ignoring others. This is useful for targeting specific dependency updates.

When no arguments are provided, `gobump` updates all direct dependencies. Run it with the Go toolchain you intend to use (for example the Red Hat Go Toolset or a specific `go` on `PATH`). Gobump sets `GOTOOLCHAIN` from that binary's `go env GOVERSION` for every `go` subprocess, ignoring any `toolchain` line in the project's `go.mod`. With `-verbose`, a warning is printed when such a `toolchain` line is present.

```
go1.22.0 get github.com/lzap/gobump@latest
go1.22.0 gobump
```

Or set `GOVERSION=go1.22.0` so the `go` command resolves to that binary.

By default, the module version list is fetched from the first usable URL in `GOPROXY` (same as the `go` command), or from `https://proxy.golang.org` when that is unset or only `direct`/`off` is configured. Override with `-proxy` if needed.

For automation (for example CI), use `-fail-on-error` so the process exits with status 1 when any dependency that was attempted ends in `err` in the summary (excluded modules do not affect the exit code).

Example output:

```
go get github.com/google/go-cmp@latest
go get golang.org/x/mod@latest
go: golang.org/x/mod@latest: golang.org/x/mod@v0.24.0 requires go >= 1.23.0 (running go 1.22.0; GOTOOLCHAIN=go1.22.0)
upgrade unsuccessful, reverting go.mod
go get golang.org/x/term@latest
go: golang.org/x/term@latest: golang.org/x/term@v0.32.0 requires go >= 1.23.0 (running go 1.22.0; GOTOOLCHAIN=go1.22.0)
upgrade unsuccessful, reverting go.mod

Summary:
github.com/google/go-cmp keep
golang.org/x/mod err
golang.org/x/term err
```

Summary legend:

* `keep`: current version is already the best candidate tried from the proxy list (or upgrade left the version unchanged)
* `noop`: the module proxy returned no newer versions than the one in `go.mod`, so no `go get` was run
* `update`: module updated to a newer version
* `err`: there was an error during the update; either the required Go version is too high, one of the `exec` commands failed, fetching the version list failed, or another error occurred
* `excluded`: module was excluded from update

## GitHub Action

The GitHub Action executes `gobump` and opens an update PR. Example PR: https://github.com/lzap/gobump/pull/7

Example action:

```
name: "Weekly gobump"
on:
  schedule:
    - cron: '13 13 * * SUN'
  workflow_dispatch:

jobs:
  bump-deps-ubuntu:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Run gobump-deps action
        uses: lzap/gobump@main
        with:
          go_version: "1.22.0"
          token: ${{ secrets.GITHUB_TOKEN }}
```

Action inputs:

* `go_version`: **The version to use** and pin the project to (defaults to stable, but always set this).
* `setup_go`: Set to `false` to avoid the `setup-go` action (e.g., when a container with a specific Go version is used).
* `exec`: An optional command to execute for each dependency update.
* `exec2`: A second optional command to execute for each dependency update.
* `exclude`: A comma-separated list of modules to exclude from the update.
* `tidy` *(deprecated, ignored)*: Kept for backward compatibility. gobump runs `go mod tidy` before each per-dependency commit in git mode.
* `exec_pr` *(deprecated, ignored)*: Kept for backward compatibility. No longer runs a command before the PR step.
* `pr`: Set to `false` to avoid the creation of a PR.
* `token`: The GitHub token (used for pull requests and, when changelog is enabled with `gist` output, for creating the Gist; the tool reads `GITHUB_TOKEN` or `GH_TOKEN`).
* `labels`: Comma-separated GitHub PR labels.
* `no_git`: When `true`, passes `-no-git` so gobump does not run any git commands (per-dependency commits or reset/clean).
* `user_name` / `user_email`: Git author identity for per-dependency commits (defaults: `Schutzbot` / `schutzbot@gmail.com`). CI runners often have no global `user.name` / `user.email`; gobump sets these in the local repository before each commit.

Tip: When building or testing in a container, use `-buildvcs=false` to avoid `git: detected dubious ownership in repository` permissions errors. Alternatively, set the `git config --system --add safe.directory /path` config option.

## How it works

* Loads the project's `go.mod` and stores it in memory.
* For each direct dependency, it asks the configured module proxy for `@v/list`, then runs `go get MODULE@V` for up to `-retries` newer versions (newest first). This is not the same as `go get MODULE@latest` in one shot, but it walks backward through recent releases when an upgrade fails.
* All `go` subprocesses use `GOTOOLCHAIN` from the bundled `go` binary (`go env GOVERSION`); the project's `toolchain` line in `go.mod` is not used (see `-verbose` for a warning when it is set).
* If the `go get` command fails or modifies the `go` or `toolchain` lines in `go.mod`, it reverts `go.mod` and `go.sum` and tries again with the next lower version until it succeeds or runs out of attempts.
* In a git repository, unless `-no-git` or `-dry-run` is set, gobump exits before doing any work if there are uncommitted changes (`git status --porcelain` is non-empty), so local edits are not mixed with automatic commits or `git reset --hard` / `git clean -fdx` on failed bumps. After a failed bump or failed `-exec`, gobump restores the work tree to the last commit and removes untracked and ignored files (for example `vendor/` from `go mod vendor`). Use `-no-git` when you intentionally want only `go.mod` / `go.sum` updates with no git integration.
* When per-dependency git commits are enabled, each successful bump runs `go mod tidy`, then `git add -A` and commits all staged changes as `chore(deps): update MODULE to VERSION` (including files produced by `-exec`, such as `vendor/`). With `-changelog`, the upstream git changelog for that module is appended to the commit message body (`-changelog-dest` is not used in this mode).
* If and only if a module succeeds in updating to a newer version and one or more optional `exec` arguments are passed, it executes them for that candidate. If the proxy had no newer versions, `exec` is skipped for that module. If any `exec` fails, it reverts to the last good `go.mod` and tries the next older candidate version, up to the retry limit. The same applies when `go get` fails or the `go` / `toolchain` directives would change.
* Repeats for every other direct dependency.

## Custom commands

For every updated dependency, it is possible to run one or more commands to ensure the project builds or tests are passing. Use the `-exec` option multiple times to do that. When such a command returns a non-zero value, that candidate version is rolled back and an older version is tried, up to `-retries` times (same as when `go get` fails).

```
gobump -exec "go build ./..." -exec "go test ./..."
```

Commands are not executed via a shell. `go` commands in `-exec` receive the same bundled `GOTOOLCHAIN` as `go get` and `go mod tidy`.

## Ambiguous imports

Sometimes, even `gobump` does not help, specifically with ambiguous imports in transient dependencies:

```
go: mypackage imports
 cloud.google.com/go/storage imports
  google.golang.org/grpc/stats/opentelemetry: ambiguous import: found package google.golang.org/grpc/stats/opentelemetry in multiple modules:
  google.golang.org/grpc v1.67.3 (/home/lzap/go/pkg/mod/google.golang.org/grpc@v1.67.3/stats/opentelemetry)
  google.golang.org/grpc/stats/opentelemetry v0.0.0-20240907200651-3ffb98b2c93a (/home/lzap/go/pkg/mo
```

In this case, use this trick:

```
go get google.golang.org/grpc/stats/opentelemetry@none
```

## Configuration

It is possible to use a different binary than `go`; set the `GOVERSION=go1.21.0` environment variable to use a different Go version on `PATH`. Gobump derives `GOTOOLCHAIN` from that binary and does not read the project's `toolchain` line in `go.mod`.

## Limitations

`gobump` only considers versions returned by the module proxy’s version list (subject to `-retries`). It does not perform an exhaustive search of the module graph beyond that list, and it does not intentionally downgrade a module to an older major line to satisfy constraints (that remains manual).

## Testing

Integration tests under `main_test.go` run `-dry-run` inside each `testdata/<case>/` directory, which contains a self-contained `go.mod` and `go.sum`. The repository root module is not modified.

## Discussion

I created a post on Reddit if you need to reach out: https://www.reddit.com/r/golang/comments/1kfypws/gobump_update_dependencies_with_pinned_go_version/

Alternatively, create an issue if you find a problem or need a new feature.
