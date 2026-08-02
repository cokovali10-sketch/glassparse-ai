package extractor

import (
	"strings"
	"testing"
)

const articleHTML = `<html><head><title>T</title></head><body>
<nav><a href="/home">Home</a> <a href="/about">About</a></nav>
<article>
  <h1>Open-Source Web Parsing</h1>
  <p>Modern parsing should resist layout churn. Websites are redesigned every
few months, yet the information we need stays the same: titles, prices,
contact points and the prose itself.</p>
  <p>We score blocks by text density and DOM shape instead of brittle CSS
selectors, so the engine keeps extracting after a redesign. That is the
self-healing crate that pipelines and large language models rely on for
stable, token-efficient data.</p>
</article>
<footer>© 2026 GlassParse</footer>
</body></html>`

func extract() *Extractor { return New() }

func TestMainTextPicksArticle(t *testing.T) {
	main := New().MainText([]byte(articleHTML))
	if main == "" {
		t.Fatal("MainText returned empty")
	}
	if !strings.Contains(main, "self-healing") {
		t.Fatalf("MainText should include article prose, got: %q", truncate(main))
	}
	if strings.Contains(main, "About") {
		t.Fatalf("MainText leaked navigation: %q", truncate(main))
	}
}

func TestLinksExtraction(t *testing.T) {
	links := New().Links([]byte(articleHTML), "https://site.test/a/b")
	var hrefs []string
	for _, l := range links {
		hrefs = append(hrefs, l.Value)
	}
	if len(hrefs) == 0 {
		t.Fatal("expected links")
	}
	found := false
	for _, h := range hrefs {
		if strings.HasSuffix(h, "/about") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected relative href resolved; links=%v", hrefs)
	}
}

func TestFingerprintStableAndSensitive(t *testing.T) {
	ex := New()
	a := ex.Fingerprint("# Hello\nworld and more")
	b := ex.Fingerprint("# Hello\nworld and more")
	if a != b {
		t.Fatal("fingerprint should be stable for identical content")
	}
	c := ex.Fingerprint("# Hello\nworld and MORE")
	if a == c {
		t.Fatal("fingerprint should change when content changes")
	}
	if len(ex.ShortFingerprint("# Hello\nworld")) != 16 {
		t.Fatal("ShortFingerprint should be 16 hex chars")
	}
}

func truncate(s string) string {
	if len(s) > 90 {
		return s[:90] + "..."
	}
	return s
}