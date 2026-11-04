package main

// alwaysTrustedProxies are the hops between a visitor and the panel on a normal install: the
// panel's own machine and private networks (the installer's Nginx reaches the container through
// the Docker network gateway, 172.18.0.1 and the like), plus Cloudflare's edge, which sits in
// front when the panel's domain is proxied (orange cloud).
//
// Trusting them is safe because Gin reads X-Forwarded-For from the right and stops at the first
// address it does not trust: a header only counts when the connection itself comes from one of
// these, and an address a visitor writes into the header is always to the left of the one the
// proxy saw. Without this every login, captcha and per-IP limit saw the Docker gateway instead
// of the visitor, so all of them shared one address.
var alwaysTrustedProxies = []string{
	// this machine and private networks (RFC 1918, RFC 4193, CGNAT for some Docker setups)
	"127.0.0.0/8", "::1/128",
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "100.64.0.0/10", "fc00::/7",

	// Cloudflare's edge: https://www.cloudflare.com/ips/
	"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
	"141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20",
	"197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
	"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
	"2400:cb00::/32", "2606:4700::/32", "2803:f800::/32", "2405:b500::/32",
	"2405:8100::/32", "2a06:98c0::/29", "2c0f:f248::/32",
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
