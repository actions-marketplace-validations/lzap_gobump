package main

import (
	"testing"
)

func TestModuleProxyBaseURL(t *testing.T) {
	t.Setenv("GOPROXY", "")
	if got := moduleProxyBaseURL(""); got != "https://proxy.golang.org" {
		t.Errorf("empty GOPROXY: got %q", got)
	}
	if got := moduleProxyBaseURL("https://example/proxy"); got != "https://example/proxy" {
		t.Errorf("explicit: got %q", got)
	}

	t.Setenv("GOPROXY", "https://corp/proxy,direct")
	if got := moduleProxyBaseURL(""); got != "https://corp/proxy" {
		t.Errorf("GOPROXY list: got %q", got)
	}

	t.Setenv("GOPROXY", "direct")
	if got := moduleProxyBaseURL(""); got != "https://proxy.golang.org" {
		t.Errorf("GOPROXY direct only: got %q", got)
	}

	t.Setenv("GOPROXY", "off")
	if got := moduleProxyBaseURL(""); got != "https://proxy.golang.org" {
		t.Errorf("GOPROXY off: got %q", got)
	}
}
