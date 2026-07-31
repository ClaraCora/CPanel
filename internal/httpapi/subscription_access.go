package httpapi

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"unicode/utf8"
)

var errInvalidUAWhitelist = errors.New("invalid user agent whitelist")

func normalizeUAWhitelist(value string) (string, error) {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	entries := make([]string, 0, len(lines))
	seen := make(map[string]bool, len(lines))
	for _, line := range lines {
		entry := strings.TrimSpace(line)
		if entry == "" {
			continue
		}
		if utf8.RuneCountInString(entry) > 256 {
			return "", errInvalidUAWhitelist
		}
		key := strings.ToLower(entry)
		if seen[key] {
			continue
		}
		seen[key] = true
		entries = append(entries, entry)
		if len(entries) > 100 {
			return "", errInvalidUAWhitelist
		}
	}
	return strings.Join(entries, "\n"), nil
}

func uaWhitelistMatches(userAgent, configured string) bool {
	userAgent = strings.ToLower(userAgent)
	if userAgent == "" {
		return false
	}
	for _, entry := range strings.Split(configured, "\n") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry != "" && strings.Contains(userAgent, entry) {
			return true
		}
	}
	return false
}

func isConventionalBrowserUA(userAgent string) bool {
	value := strings.ToLower(userAgent)
	if !strings.Contains(value, "mozilla/") {
		return false
	}
	for _, marker := range []string{
		"chrome/", "chromium/", "crios/", "firefox/", "fxios/",
		"safari/", "edg/", "opr/", "opera/", "msie ", "trident/",
	} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func sanitizeUserAgent(userAgent string) string {
	userAgent = strings.TrimSpace(userAgent)
	if utf8.RuneCountInString(userAgent) <= 512 {
		return userAgent
	}
	return string([]rune(userAgent)[:512])
}

func resolvedClientIP(r *http.Request, trustedCIDRs string) string {
	remote, ok := parseRequestAddress(r.RemoteAddr)
	if !ok {
		return ""
	}
	prefixes := parseTrustedProxyPrefixes(trustedCIDRs)
	if !isTrustedProxy(remote, prefixes) {
		return remote.String()
	}

	forwarded := make([]netip.Addr, 0)
	for _, value := range strings.Split(r.Header.Get("X-Forwarded-For"), ",") {
		if address, valid := parseRequestAddress(value); valid {
			forwarded = append(forwarded, address)
		}
	}
	current := remote
	for index := len(forwarded) - 1; index >= 0; index-- {
		if !isTrustedProxy(current, prefixes) {
			break
		}
		current = forwarded[index]
	}
	if current != remote {
		return current.String()
	}
	if realIP, valid := parseRequestAddress(r.Header.Get("X-Real-IP")); valid {
		return realIP.String()
	}
	return remote.String()
}

func parseRequestAddress(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	value = strings.Trim(value, "[]")
	address, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, false
	}
	return address.Unmap(), true
}

func parseTrustedProxyPrefixes(value string) []netip.Prefix {
	parts := strings.FieldsFunc(value, func(character rune) bool {
		return character == ',' || character == ';' || character == '\n' || character == '\r' || character == '\t' || character == ' '
	})
	prefixes := make([]netip.Prefix, 0, len(parts))
	for _, part := range parts {
		if prefix, err := netip.ParsePrefix(part); err == nil {
			prefixes = append(prefixes, prefix.Masked())
			continue
		}
		if address, err := netip.ParseAddr(part); err == nil {
			prefixes = append(prefixes, netip.PrefixFrom(address.Unmap(), address.Unmap().BitLen()))
		}
	}
	return prefixes
}

func isTrustedProxy(address netip.Addr, prefixes []netip.Prefix) bool {
	if address.IsLoopback() {
		return true
	}
	for _, prefix := range prefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}
