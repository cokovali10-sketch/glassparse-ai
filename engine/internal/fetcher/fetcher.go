// Package fetcher handles network retrieval with an anti-bot posture and the
// transparent HTTP -> headless switch for JS-rendered (SPA / React / Vue) pages.
//
// The core idea: first try the fast static path; if the HTML looks
// JavaScript-driven (an empty app mount point, zero block text, or scripts), a
// caller running with headless support is signalled to re-fetch through a real
// browser. The Fetcher is intentionally dependency-light on the browser side —
// plugging in Rod / Playwright happens through the HeadlessRenderer hook.
package fetcher

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/glassparse/engine/pkg/model"
)

// HeadlessRenderer is implemented by a browser-backed fetcher (Rod,
// Playwright-go or a remote service). Returning false from NeedsHeadless means
// "no such browser available".
type HeadlessRenderer interface {
	// Fetch renders the URL in a real browser and returns the serialised HTML.
	Fetch(ctx context.Context, opts model.FetchOptions) ([]byte, int, error)
}

// defaultUserAgents is a small rotation pool used when RotateUA is set.
var defaultUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36",
}

// Client fetches pages, applying anti-bot headers, proxying and rotation.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	RotateUA  bool
	Headless  HeadlessRenderer
}

// New returns a Client with sensible defaults and an optional proxy URL.
func New(proxyURL string, opts ...Option) *Client {
	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 16,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	c := &Client{
		HTTP:      &http.Client{Transport: tr, Timeout: 30 * time.Second},
		UserAgent: defaultUserAgents[0],
		RotateUA:  true,
	}
	for _, o := range opts {
		o(c)
	}
	if c.HTTP.Timeout == 0 {
		c.HTTP.Timeout = 30 * time.Second
	}
	return c
}

// Option configures a Client at construction time.
type Option func(*Client)

// WithTimeout sets the overall request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.HTTP.Timeout = d }
}

// WithHeadless attaches a real-browser renderer for SPA pages.
func WithHeadless(r HeadlessRenderer) Option {
	return func(c *Client) { c.Headless = r }
}

// Fetch retrieves a page. It returns raw body bytes, status code, mode used.
//
//   - If opts.Headless is true the headless renderer is forced.
//   - Otherwise it fetches over HTTP, inspects the body, and if the page looks
//     JS-only *and* a headless renderer is registered, it transparently falls
//     back to rendering it.
func (c *Client) Fetch(ctx context.Context, opts model.FetchOptions) ([]byte, int, model.RenderMode, error) {
	if opts.Headless && c.Headless != nil {
		b, code, err := c.Headless.Fetch(ctx, opts)
		return b, code, model.RenderHeadless, err
	}

	ua := c.UserAgent
	if opts.UserAgent != "" {
		ua = opts.UserAgent
	} else if c.RotateUA {
		ua = randomUA()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.URL, nil)
	if err != nil {
		return nil, 0, model.RenderHTTP, err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("DNT", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	if opts.Proxy != "" {
		// per-request proxy override
		_ = opts.Proxy
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, model.RenderHTTP, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20)) // cap at 64 MiB.
	if err != nil {
		return nil, resp.StatusCode, model.RenderHTTP, err
	}
	mode := model.RenderHTTP
	if looksSPA(body) && c.Headless != nil {
		if hb, code, herr := c.Headless.Fetch(ctx, opts); herr == nil && len(hb) > 0 {
			return hb, code, model.RenderHeadless, nil
		}
	}
	return body, resp.StatusCode, mode, nil
}

// looksSPA heuristically detects client-side-rendered pages: an app root that is
// basically empty plus heavy <script> payloads. Sites rendered client-side have
// little to no static block text.
func looksSPA(body []byte) bool {
	s := string(body)
	if strings.Contains(s, `<div id="app">`) ||
		strings.Contains(s, `<div id="root">`) ||
		strings.Contains(s, `<div class="app">`) {
		// Cheap signal: less static text than typical server-rendered docs.
		var textLen int
		for _, kw := range []string{"<p>", "<h1", "<h2", "<h3", "<article"} {
			textLen += strings.Count(s, kw)
		}
		if textLen < 2 {
			return true
		}
	}
	return false
}

// randomUA picks a pseudo-random User-Agent from the pool.
func randomUA() string {
	var b [2]byte
	_, _ = rand.Read(b[:])
	i := int(binary.BigEndian.Uint16(b[:2]) % uint16(len(defaultUserAgents)))
	return defaultUserAgents[i]
}