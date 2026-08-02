package fetcher

import (
	"testing"
	"time"
)

const robotsSample = `User-agent: Googlebot
Disallow: /nogoogle
Disallow: /nogoogle/*.pdf
Crawl-delay: 10
User-agent: *
Allow: /
Disallow: /private
Disallow: /tmp
Crawl-delay: 2
Sitemap: https://site.test/sitemap.xml
`

func TestParseRobotsAllowed(t *testing.T) {
	r := ParseRobots(robotsSample)

	// "*" group—verifying both Allow all and the specific Disallow paths.
	if !r.Allowed("/public/page", "glassparse") {
		t.Error("/public/page should be allowed")
	}
	if r.Allowed("/private/secret", "glassparse") {
		t.Error("/private/secret should be disallowed")
	}
	if r.Allowed("/tmp/cache", "glassparse") {
		t.Error("/tmp/cache should be disallowed")
	}
	if r.Allowed("/tmp/x", "glassparse") {
		t.Error("wildcard /tmp/* must block /tmp/x")
	}
}

func TestParseRobotsUserAgentSpecific(t *testing.T) {
	r := ParseRobots(robotsSample)
	if r.Allowed("/nogoogle/x", "Googlebot") {
		t.Error("Googlebot should be blocked under its group")
	}
	if !r.Allowed("/nogoogle", "glassparse") {
		t.Error("generic UA should be allowed /nogoogle (only Googlebot blocked)")
	}
}

func TestParseRobotsCrawlDelayAndSitemap(t *testing.T) {
	r := ParseRobots(robotsSample)
	if got := r.CrawlDelay("Googlebot"); got < time.Second {
		t.Fatalf("expected google crawl-delay ~10s, got %v", got)
	}
	if got := r.CrawlDelay("glassparse"); got < time.Second {
		t.Fatalf("expected * crawl-delay ~2s, got %v", got)
	}
	sms := r.Sitemaps()
	if len(sms) != 1 || sms[0] != "https://site.test/sitemap.xml" {
		t.Fatalf("sitemaps = %v", sms)
	}
}

func TestRobotsWildcardEndAnchor(t *testing.T) {
	r := ParseRobots("User-agent: *\nDisallow: /admin$\nDisallow: /report/*.pdf$")
	if !r.Allowed("/admin/dashboard", "*") {
		t.Error("/admin/dashboard should be allowed (only exact /admin blocked)")
	}
	if r.Allowed("/admin", "*") {
		t.Error("/admin must be blocked")
	}
	if r.Allowed("/report/q1.pdf", "*") {
		t.Error("/report/*.pdf$ must match the file")
	}
}