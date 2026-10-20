package agent

import (
	"net"
	"net/url"
	"regexp"
	"strings"
)

const maxFetchURLsPerMessage = 3

var messageURLPattern = regexp.MustCompile(`(?i)(https?://[^\s<>"']+|www\.[^\s<>"']+)`)

func findPublicURLs(message string) (bool, []url.URL) {
	matches := messageURLPattern.FindAllString(message, -1)
	if len(matches) == 0 {
		return false, nil
	}

	foundURL := false
	urls := make([]url.URL, 0, min(len(matches), maxFetchURLsPerMessage))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		match = strings.TrimRight(match, ".,!?;:)]}")
		if match == "" {
			continue
		}
		foundURL = true
		if strings.HasPrefix(strings.ToLower(match), "www.") {
			match = "https://" + match
		}
		parsed, err := url.ParseRequestURI(match)
		if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			continue
		}
		host := strings.ToLower(parsed.Hostname())
		if !isPublicFetchHost(host) {
			continue
		}
		parsed.Host = strings.ToLower(parsed.Host)
		canonical := parsed.String()
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		urls = append(urls, *parsed)
		if len(urls) == maxFetchURLsPerMessage {
			break
		}
	}
	return foundURL, urls
}

func isPublicFetchHost(host string) bool {
	if host == "" || strings.EqualFold(host, "localhost") || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return false
	}
	if address := net.ParseIP(host); address != nil {
		return address.IsGlobalUnicast() && !address.IsPrivate() && !address.IsLoopback() && !address.IsLinkLocalUnicast()
	}
	return strings.Contains(host, ".")
}
