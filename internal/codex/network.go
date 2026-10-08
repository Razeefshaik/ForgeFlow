package codex

import (
	"net"
	"net/url"
	"strings"
)

// approvedNetworkEnvironment changes only the child's environment. A stale
// sandbox discard proxy must not defeat an explicit network approval. Normal
// user/enterprise proxies and the sandbox's own enforcement remain intact.
func approvedNetworkEnvironment(env []string, network bool) []string {
	result := make([]string, 0, len(env))
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		proxy := strings.EqualFold(key, "HTTP_PROXY") || strings.EqualFold(key, "HTTPS_PROXY") || strings.EqualFold(key, "ALL_PROXY")
		if network && proxy && discardProxy(value) {
			continue
		}
		result = append(result, entry)
	}
	return result
}

func discardProxy(value string) bool {
	u, err := url.Parse(value)
	if err != nil || u.User != nil || u.Port() != "9" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5" && u.Scheme != "socks5h" {
		return false
	}
	if strings.EqualFold(u.Hostname(), "localhost") {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return ip != nil && ip.IsLoopback()
}
