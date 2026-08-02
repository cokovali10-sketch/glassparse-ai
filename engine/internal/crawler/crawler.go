// Package crawler performs a polite, concurrent single-site crawl: it fetches
// the seed URL, discovers same-host links, respects robots.txt, dedups by URL,
// fingerprints pages and parses each into clean Markdown + entities. Think of it
// as "build a sitemap from a seed URL" — the tool a docs / PDF / knowledge base
// pipeline needs before chunking and vectorising a whole site.
package crawler

import (
	"context"
	"net/url"
	"strings"
	"sync"

	"github.com/glassparse/engine/internal/cleaner"
	"github.com/glassparse/engine/internal/extractor"
	"github.com/glassparse/engine/internal/fetcher"
	"github.com/glassparse/engine/pkg/model"
)

// Options controls the crawl.
type Options struct {
	// MaxPages is the global page budget (default 10).
	MaxPages int
	// Concurrency is the parallel-fetch limit (default 4).
	Concurrency int
	// UserAgent names the crawler; used for UA and robots evaluation.
	UserAgent string
	// RespectRobots disables robots.txt evaluation when false (default true).
	RespectRobots bool
	// SameHostOnly restricts discovery to the seed's host (default true).
	SameHostOnly bool
	// Allow is an optional include filter on each discovered URL.
	Allow func(*url.URL) bool
}

// Default returns safe zero-value options.
func Default() Options {
	return Options{MaxPages: 10, Concurrency: 4, RespectRobots: true, SameHostOnly: true}
}

// Page is the parsed result of one crawled URL.
type Page struct {
	URL         string          `json:"url"`
	StatusCode  int             `json:"status_code"`
	Mode        model.RenderMode `json:"mode"`
	Title       string          `json:"title"`
	Markdown    string          `json:"markdown"`
	Fingerprint string          `json:"fingerprint"`
	Entities    []model.Entity  `json:"entities"`
	Links       []string        `json:"links"`
	Err         error           `json:"error,omitempty"`
}

// Crawler crawls a site. Use New to optionally share a fetcher client.
type Crawler struct {
	Client    *fetcher.Client
	Extractor *extractor.Extractor
}

// New returns a Crawler backed by a fetcher Client (proxied is fine).
func New(fc *fetcher.Client) *Crawler {
	if fc == nil {
		fc = fetcher.New("")
	}
	return &Crawler{Client: fc, Extractor: extractor.New()}
}

// Crawl performs a bounded BFS crawl of root using a concurrency-limited worker
// pool. It returns every visited page (including the seed) in discovery order.
// Per-page errors are recorded so one 404 / timeout never aborts the crawl.
func (c *Crawler) Crawl(ctx context.Context, root string, opts Options) []Page {
	if opts.MaxPages <= 0 {
		opts.MaxPages = 10
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}
	seed, err := url.Parse(root)
	if err != nil {
		return []Page{{URL: root, Err: err}}
	}

	var mu sync.Mutex
	visited := map[string]bool{}
	pages := make([]Page, 0, opts.MaxPages)

	var wg sync.WaitGroup
	sem := make(chan struct{}, opts.Concurrency)
	var process func(string)

	// robots decision is per-host and cached once.
	allow := func(u *url.URL) bool { return true }
	if opts.RespectRobots {
		if rb, rerr := c.Client.FetchRobots(ctx, root, opts.UserAgent); rerr == nil && rb != nil {
			ua := opts.UserAgent
			if ua == "" {
				ua = "glassparse-crawler"
			}
			allow = func(u *url.URL) bool { return rb.Allowed(u.Path, ua) }
		}
	}

	shouldVisit := func(k string) bool {
		mu.Lock()
		defer mu.Unlock()
		if visited[k] {
			return false
		}
		if len(visited) >= opts.MaxPages {
			return false
		}
		visited[k] = true
		return true
	}

	enqueue := func(raw string) {
		u, perr := url.Parse(raw)
		if perr != nil {
			return
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return
		}
		if opts.SameHostOnly && !strings.EqualFold(u.Host, seed.Host) {
			return
		}
		if opts.Allow != nil && !opts.Allow(u) {
			return
		}
		if !allow(u) {
			return
		}
		k := canonical(u)
		if shouldVisit(k) {
			wg.Add(1)
			go process(u.String())
		}
	}

	process = func(rawURL string) {
		defer wg.Done()
		sem <- struct{}{}
		defer func() { <-sem }()

		res := Page{URL: rawURL}
		pr, herr := c.fetchParse(ctx, rawURL, opts.UserAgent)
		if herr != nil {
			res.Err = herr
		} else {
			res.StatusCode = pr.statusCode
			res.Mode = pr.mode
			res.Title = pr.meta.Title
			res.Markdown = pr.markdown
			res.Fingerprint = c.Extractor.Fingerprint(pr.markdown)
			res.Entities = pr.entities
			res.Links = pr.links
		}

		mu.Lock()
		pages = append(pages, res)
		mu.Unlock()

		// Best-effort worth protecting the page graph.
		for _, link := range res.Links {
			enqueue(link)
		}
	}

	enqueue(root)
	wg.Wait()
	return pages
}

// crawlPage is the parsed + link-extracted product of a single fetch.
type crawlPage struct {
	statusCode int
	mode       model.RenderMode
	meta       model.Metadata
	markdown   string
	cleanHTML  string
	entities   []model.Entity
	links      []string
}

func (c *Crawler) fetchParse(ctx context.Context, rawURL, optUA string) (crawlPage, error) {
	opts := model.FetchOptions{URL: rawURL}
	if optUA != "" {
		opts.UserAgent = optUA
	}
	body, code, mode, err := c.Client.Fetch(ctx, opts)
	if err != nil {
		return crawlPage{}, err
	}
	cl := cleaner.New(rawURL)
	p := cl.Clean(body)
	cp := crawlPage{
		statusCode: code,
		mode:       mode,
		meta:       p.Meta,
		markdown:   p.Markdown,
		cleanHTML:  p.CleanHTML,
	}
	cp.entities = append(cp.entities, c.Extractor.Extract(body)...)
	cp.entities = append(cp.entities, c.Extractor.Links(body, rawURL)...)
	for _, e := range cp.entities {
		if e.Type == "link" {
			cp.links = append(cp.links, e.Value)
		}
	}
	return cp, nil
}

// canonical normalises a URL to a stable visit key: lowercase scheme/host,
// stripped fragment, empty path becomes "/".
func canonical(u *url.URL) string {
	host := strings.ToLower(u.Host)
	if strings.HasSuffix(u.Host, ":80") || strings.HasSuffix(u.Host, ":443") {
		host = strings.Split(host, ":")[0]
	}
	path := u.Path
	if path == "" {
		path = "/"
	}
	key := strings.ToLower(u.Scheme) + "://" + host + path
	if u.RawQuery != "" {
		key += "?" + u.RawQuery
	}
	return key
}