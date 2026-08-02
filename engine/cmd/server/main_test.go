package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glassparse/engine/internal/crawler"
	"github.com/glassparse/engine/internal/fetcher"
)

// fakeSite is a tiny deterministic upstream with a home page, a linked page
// and a robots.txt that forbids /secret.
func fakeSite(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "User-agent: *\nDisallow: /secret\nCrawl-delay: 0\n")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>Home</title></head><body>
<h1>Welcome</h1>
<p>GlassParse turns any website into clean structured data for AI pipelines. The engine strips the noise and keeps the prose, prices and contact points for downstream RAG systems and large language models.</p>
<a href="/docs">Docs</a>
</body></html>`)
	})
	mux.HandleFunc("/docs", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>Docs</title></head><body>
<h1>Docs</h1><p>Full documentation and an install guide with plenty of helpful text for readers to index.</p>
<a href="/">Home</a></body></html>`)
	})
	mux.HandleFunc("/secret", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>Secret</title></head><body><h1>Denied</h1><p>Do not crawl.</p></body></html>`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newHandler() *Handler {
	fc := fetcher.New("")
	return &Handler{client: fc, crawler: crawler.New(fc)}
}

func doJSON(t *testing.T, h func(http.ResponseWriter, *http.Request), method, url string) (*bytes.Buffer, int) {
	t.Helper()
	req := httptest.NewRequest(method, url, nil)
	w := httptest.NewRecorder()
	h(w, req)
	return w.Body, w.Code
}

func TestParseEndpoint(t *testing.T) {
	srv := fakeSite(t)
	hdl := newHandler()
	body, code := doJSON(t, hdl.Parse, http.MethodPost, "/api/parse?url="+srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("parse status = %d, body=%s", code, body.String())
	}
	var out struct {
		Page struct {
			Metadata struct {
				Title string `json:"title"`
			} `json:"metadata"`
			Markdown string `json:"markdown"`
		} `json:"page"`
		Chunks []struct {
			ID string `json:"id"`
		} `json:"chunks"`
	}
	if err := json.Unmarshal(body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Page.Metadata.Title == "" {
		t.Error("missing page title in parse response")
	}
	if !bytes.Contains([]byte(out.Page.Markdown), []byte("GlassParse")) {
		t.Errorf("markdown missing content: %q", out.Page.Markdown)
	}
	if len(out.Chunks) == 0 {
		t.Error("expected at least one RAG chunk")
	}
}

func TestParseEndpointRejectsMissing(t *testing.T) {
	hdl := newHandler()
	_, code := doJSON(t, hdl.Parse, http.MethodPost, "/api/parse")
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing url, got %d", code)
	}
}

func TestMainTextEndpoint(t *testing.T) {
	srv := fakeSite(t)
	hdl := newHandler()
	body, code := doJSON(t, hdl.MainText, http.MethodGet, "/api/main-text?url="+srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("main-text status=%d body=%s", code, body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["main_text"] == "" {
		t.Error("main_text empty")
	}
	if out["word_count"].(float64) <= 0 {
		t.Error("word_count not populated")
	}
}

func TestRobotsEndpoint(t *testing.T) {
	srv := fakeSite(t)
	hdl := newHandler()
	body, code := doJSON(t, hdl.Robots, http.MethodGet, "/api/robots?url="+srv.URL+"&path=/secret")
	if code != http.StatusOK {
		t.Fatalf("robots status=%d", code)
	}
	var out map[string]any
	if err := json.Unmarshal(body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["exists"] != true {
		t.Fatalf("robots exists = %v", out["exists"])
	}
	if out["allowed"] != false {
		t.Errorf("robots.allowed for /secret should be false, got %v", out["allowed"])
	}
}

func TestFingerprintEndpointStable(t *testing.T) {
	srv := fakeSite(t)
	hdl := newHandler()
	b1, c1 := doJSON(t, hdl.Fingerprint, http.MethodGet, "/api/fingerprint?url="+srv.URL+"/")
	if c1 != http.StatusOK {
		t.Fatalf("fingerprint status=%d", c1)
	}
	b2, _ := doJSON(t, hdl.Fingerprint, http.MethodGet, "/api/fingerprint?url="+srv.URL+"/")
	var a, b map[string]any
	_ = json.Unmarshal(b1.Bytes(), &a)
	_ = json.Unmarshal(b2.Bytes(), &b)
	if a["fingerprint"] != b["fingerprint"] {
		t.Error("fingerprint should be stable across identical fetches")
	}
	if a["short_fingerprint"] == "" {
		t.Error("short_fingerprint missing")
	}
}

func TestCrawlEndpointHonoursRobots(t *testing.T) {
	srv := fakeSite(t)
	hdl := newHandler()
	body, code := doJSON(t, hdl.Crawl, http.MethodPost, "/api/crawl?url="+srv.URL+"/&max_pages=4")
	if code != http.StatusOK {
		t.Fatalf("crawl status=%d body=%s", code, body.String())
	}
	var pages []map[string]any
	if err := json.Unmarshal(body.Bytes(), &pages); err != nil {
		t.Fatalf("decode crawl: %v", err)
	}
	if len(pages) < 2 {
		t.Fatalf("expected >=2 crawled pages, got %d", len(pages))
	}
	for _, p := range pages {
		if strings.Contains(p["url"].(string), "/secret") {
			t.Errorf("crawled forbidden /secret page: %v", p["url"])
		}
	}
}

func TestHealthz(t *testing.T) {
	handle := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": version})
	}
	body, code := doJSON(t, handle, http.MethodGet, "/api/healthz")
	if code != http.StatusOK || !bytes.Contains(body.Bytes(), []byte("ok")) {
		t.Fatalf("healthz: %d %s", code, body.String())
	}
}