package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func parse(s string) *url.URL { u, _ := url.Parse(s); return u }

// buildSite spins up a tiny same-host site with a robots.txt and 3 pages.
func buildSite(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "User-agent: *\nDisallow: /secret\nCrawl-delay: 0\n")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Home</title></head><body>
<h1>Home</h1>
<p>Welcome to the index page with a good amount of prose for parsing.</p>
<a href="/docs">Docs</a>
<a href="/about">About</a>
<a href="/secret">Secret</a>
</body></html>`)
	})
	mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Docs</title></head><body><h1>Docs</h1><p>Read the documentation here. Lots of fine text follows.</p><a href="/guide">Guide</a></body></html>`)
	})
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>About</title></head><body><h1>About</h1><p>An about page that talks about the people behind the project at length.</p></body></html>`)
	})
	mux.HandleFunc("/secret", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Secret</title></head><body><h1>Secret</h1><p>Denied content that robots forbid.</p></body></html>`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestCrawlDiscoversAndDedups(t *testing.T) {
	srv := buildSite(t)
	c := New(nil)
	pages := c.Crawl(context.Background(), srv.URL, Default())

	// seed + /docs + /about; /secret must be excluded by robots.
	urls := map[string]bool{}
	for _, p := range pages {
		urls[p.URL] = true
		if p.Err != nil {
			t.Fatalf("page %s errored: %v", p.URL, p.Err)
		}
		if p.Title == "" {
			t.Errorf("page %s missing title", p.URL)
		}
	}
	for _, want := range []string{srv.URL + "/", srv.URL + "/docs", srv.URL + "/about"} {
		if !urls[want] && !urls[want+"/"] && !urls[strings.TrimSuffix(want, "/")] {
			// accept either canonical form (trailing slash optional)
			t.Errorf("did not crawl expected page: %s", want)
		}
	}
	if urls[srv.URL+"/secret"] {
		t.Error("crawl visited a robots-disallowed path /secret")
	}
	if len(pages) > 4 {
		t.Errorf("crawl should be bounded; visited %d pages", len(pages))
	}
}

func TestCrawlBudgetAndDedup(t *testing.T) {
	srv := buildSite(t)
	c := New(nil)
	opts := Default()
	opts.MaxPages = 2
	pages := c.Crawl(context.Background(), srv.URL, opts)
	if len(pages) < 1 || len(pages) > 2 {
		t.Fatalf("MaxPages=2 budget  violated: got %d", len(pages))
	}
	// URL keys must be unique.
	seen := map[string]bool{}
	for _, p := range pages {
		if seen[p.URL] {
			t.Fatalf("duplicate URL crawled: %s", p.URL)
		}
		seen[p.URL] = true
	}
	if !strings.Contains(pages[0].Markdown, "Welcome") && pages[0].Err == nil {
		t.Errorf("seed page should parse to markdown: %q", pages[0].Markdown)
	}
}

func TestCanonical(t *testing.T) {
	cases := []struct{ in, want string }{
		{"HTTP://SITE.test/path#frag", "http://site.test/path"},
		{"https://site.test:443/a?q=1", "https://site.test/a?q=1"},
		{"https://site.test", "https://site.test/"},
	}
	for _, c := range cases {
		if got := canonical(parse(c.in)); got != c.want {
			t.Errorf("canonical(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}