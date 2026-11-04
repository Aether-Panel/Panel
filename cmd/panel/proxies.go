package main

import (
	_ "embed"
	"strings"
)

// trustedProxiesList holds the hops between a visitor and the panel on a normal install: the
// panel's own machine, private networks (the installer's Nginx reaches the container through the
// Docker network gateway, 172.18.0.1 and the like) and Cloudflare's edge, which sits in front
// when the panel's domain is proxied (orange cloud). It is a data file so the ranges can be
// read and updated as a list.
//
// Trusting them is safe because Gin reads X-Forwarded-For from the right and stops at the first
// address it does not trust: a header only counts when the connection itself comes from one of
// these, and an address a visitor writes into the header is always to the left of the one the
// proxy saw. Without this every login, captcha and per-IP limit saw the Docker gateway instead
// of the visitor, so all of them shared one address.
//
//go:embed trusted_proxies.txt
var trustedProxiesList string

// alwaysTrustedProxies is trustedProxiesList without comments and blank lines
var alwaysTrustedProxies = parseProxyList(trustedProxiesList)

func parseProxyList(list string) []string {
	var proxies []string
	for _, line := range strings.Split(list, "\n") {
		line, _, _ = strings.Cut(line, "#")
		if line = strings.TrimSpace(line); line != "" {
			proxies = append(proxies, line)
		}
	}
	return proxies
}

// trustedProxyList is what the panel trusts: alwaysTrustedProxies plus what the admin listed in
// security.trustedProxies, each once
func trustedProxyList(configured []string) []string {
	list := make([]string, 0, len(alwaysTrustedProxies)+len(configured))
	seen := make(map[string]bool, cap(list))
	for _, proxy := range append(append([]string{}, alwaysTrustedProxies...), configured...) {
		if proxy == "" || seen[proxy] {
			continue
		}
		seen[proxy] = true
		list = append(list, proxy)
	}
	return list
}
