package main

import (
	"net/http"
	"time"
)

const (
	HTTPUserAgent = "gobump (https://github.com/lzap/gobump)"
	// HTTPClientTimeout bounds how long outbound HTTP (module proxy, GitHub API) may block.
	HTTPClientTimeout = 90 * time.Second
)

func NewHTTPClient() *http.Client {
	return &http.Client{Timeout: HTTPClientTimeout}
}

func SetDefaultHTTPHeaders(req *http.Request) {
	req.Header.Set("User-Agent", HTTPUserAgent)
}
