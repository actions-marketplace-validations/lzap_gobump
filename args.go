package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

type CommaSeparatedStringSlice []string

func (i *CommaSeparatedStringSlice) String() string {
	return strings.Join(*i, ",")
}

func (i *CommaSeparatedStringSlice) Set(value string) error {
	if value != "" {
		parts := strings.Split(value, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		*i = out
	}
	return nil
}

type StringSlice []string

func (i *StringSlice) String() string {
	return fmt.Sprintf("%v", *i)
}

func (i *StringSlice) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// AppConfig holds the application configuration
type AppConfig struct {
	Version       bool
	DryRun        bool
	Verbose       bool
	Format        string
	Retries       int
	Commands      StringSlice
	GoBinary      string
	Changelog     bool
	ChangelogDest string
	Dependencies  []string
	Exclude       CommaSeparatedStringSlice
	NoGit         bool
	GitUserName   string
	GitUserEmail  string
	ModuleProxy   string
	FailOnError   bool
}

var Config *AppConfig

func IsCI() bool {
	return os.Getenv("GITHUB_ACTIONS")+os.Getenv("GITLAB_CI")+os.Getenv("CIRCLECI") != ""
}

// InitConfig initializes the global configuration object.
func InitConfig() {
	Config = &AppConfig{}

	goBinary := os.Getenv("GOVERSION")
	if goBinary == "" {
		goBinary = "go"
	}
	Config.GoBinary = goBinary

	defaultFormat := "console"
	defaultVerbose := false
	if IsCI() {
		defaultFormat = "markdown"
		defaultVerbose = true
	}

	var commands StringSlice
	var exclude CommaSeparatedStringSlice
	flag.BoolVar(&Config.Version, "version", false, "print gobump version and module checksum")
	flag.BoolVar(&Config.DryRun, "dry-run", false, "revert to original go.mod and go.sum after running")
	flag.BoolVar(&Config.Verbose, "verbose", defaultVerbose, "log go get, -exec, and git commands (command line, stdout, stderr, exit code) to stderr")
	flag.Var(&commands, "exec", "exec command for each individual bump, can be used multiple times")
	flag.Var(&exclude, "exclude", "comma-separated list of modules to exclude from update")
	flag.StringVar(&Config.Format, "format", defaultFormat, "output format (console, markdown, none)")
	flag.IntVar(&Config.Retries, "retries", 5, "number of downgrade retries for each module (default: 5)")
	flag.BoolVar(&Config.Changelog, "changelog", false, "fetch upstream git changelog for each updated module (embedded in per-dependency commit messages when git integration is enabled; otherwise aggregated at end per -changelog-dest)")
	flag.StringVar(&Config.ChangelogDest, "changelog-dest", "stdout", "with -changelog and -no-git (or no usable git work tree): write aggregated changelogs to stdout (default), a file path, or \"gist\"; ignored when changelogs are committed per dependency")
	flag.BoolVar(&Config.NoGit, "no-git", false, "if true, skip all git operations (no per-dependency commits or go.mod/go.sum discard on failure)")
	flag.StringVar(&Config.GitUserName, "user-name", "Schutzbot", "git user.name for per-dependency commits (local repo config)")
	flag.StringVar(&Config.GitUserEmail, "user-email", "schutzbot@gmail.com", "git user.email for per-dependency commits (local repo config)")
	flag.StringVar(&Config.ModuleProxy, "proxy", "", "module proxy base URL (default: first usable $GOPROXY entry, else https://proxy.golang.org)")
	flag.BoolVar(&Config.FailOnError, "fail-on-error", false, "exit with status 1 if any non-excluded module failed to update")
	flag.Parse()

	Config.Commands = commands
	Config.Dependencies = flag.Args()
	Config.Exclude = exclude
}
