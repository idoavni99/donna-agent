package agent

import "testing"

func TestFindPublicURLsRecognizesLinksAndStripsPunctuation(t *testing.T) {
	found, urls := findPublicURLs("Please summarize https://example.com/article?id=2.")
	if !found || len(urls) != 1 || urls[0].Host != "example.com" || urls[0].Path != "/article" {
		t.Fatalf("unexpected URL detection: found=%v urls=%+v", found, urls)
	}
}

func TestFindPublicURLsRejectsLocalHostsAndCapsResults(t *testing.T) {
	found, urls := findPublicURLs("http://127.0.0.1/admin https://service.local/page https://one.example/a https://two.example/b https://three.example/c https://four.example/d")
	if !found || len(urls) != maxFetchURLsPerMessage {
		t.Fatalf("expected up to %d public URLs, got found=%v urls=%+v", maxFetchURLsPerMessage, found, urls)
	}
	for _, parsed := range urls {
		if parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "service.local" {
			t.Fatalf("local host was accepted for fetch: %s", parsed.Host)
		}
	}
}

func TestFindPublicURLsRecognizesWwwURL(t *testing.T) {
	found, urls := findPublicURLs("Read www.example.org/page")
	if !found || len(urls) != 1 || urls[0].Scheme != "https" {
		t.Fatalf("expected www URL to be normalized: found=%v urls=%+v", found, urls)
	}
}
