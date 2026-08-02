// Command server runs GlassParse AI's HTTP API, the production transport that
// the Python SDK (and the dashboard) talk to. It exposes:
//
//	POST /api/parse   -> clean a page, extract entities, produce chunks (JSON)
//	GET  /api/healthz -> liveness probe
//	GET  /api/version -> engine + protocol version
//
// It is dependency-light (stdlib net/http) but is built to serve concurrent
// parses: the pipeline is stateless and goroutine-safe — cleaner/extractor
// instances are created per request.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/glassparse/engine/internal/chunker"
	"github.com/glassparse/engine/internal/cleaner"
	"github.com/glassparse/engine/internal/crawler"
	"github.com/glassparse/engine/internal/extractor"
	"github.com/glassparse/engine/internal/fetcher"
	"github.com/glassparse/engine/pkg/model"
)

const version = "0.1.0"

func main() {
	var (
		addr  = flag.String("addr", "127.0.0.1:8080", "listen address")
		proxy = flag.String("proxy", "", "optional upstream proxy URL")
	)
	flag.Parse()

	client := fetcher.New(*proxy, fetcher.WithTimeout(45*time.Second))
	hdl := &Handler{client: client, crawler: crawler.New(client)}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/parse", hdl.Parse)
	mux.HandleFunc("POST /api/crawl", hdl.Crawl)
	mux.HandleFunc("GET /api/main-text", hdl.MainText)
	mux.HandleFunc("GET /api/robots", hdl.Robots)
	mux.HandleFunc("GET /api/fingerprint", hdl.Fingerprint)
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": version})
	})
	mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"engine": "glassparse", "version": version})
	})

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown: drain in-flight requests, then exit cleanly on a
	// SIGINT / SIGTERM (Ctrl-C, docker stop, service manager).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("glassparse engine %s listening on %s", version, *addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	stop()
	log.Println("shutting down: draining in-flight requests")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Printf("forced shutdown: %v", err)
	}
	log.Println("bye")
}

// Handler wires the engine pipeline behind HTTP.
type Handler struct {
	client  *fetcher.Client
	crawler *crawler.Crawler
}

// ParseResponse is the HTTP envelope the SDK deserialises directly into
// glassparse.models.ParseResult / Page / Entity / Chunk.
type ParseResponse struct {
	Page     model.Page      `json:"page"`
	Entities []model.Entity  `json:"entities"`
	Chunks   []chunker.Chunk `json:"chunks"`
}

// Parse handles POST /api/parse?url=...&headless=...&proxy=...
func (h *Handler) Parse(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	q := r.Form

	target := q.Get("url")
	if target == "" {
		writeError(w, http.StatusBadRequest, "missing url")
		return
	}

	opts := model.FetchOptions{
		URL:       target,
		UserAgent: q.Get("user_agent"),
		Proxy:     q.Get("proxy"),
		RotateUA:  q.Has("rotate_ua"),
		Headless:  q.Has("headless"),
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	start := time.Now()

	// Fetch (fast HTTP first, upgrading to headless for JS-rendered pages).
	body, status, mode, err := h.client.Fetch(ctx, opts)
	if err != nil {
		writeError(w, http.StatusBadGateway, "fetch failed: "+err.Error())
		return
	}
	if status >= 400 {
		writeError(w, http.StatusBadGateway, "upstream status "+strconv.Itoa(status))
		return
	}

	// Pipeline: clean -> extract -> chunk.
	page := cleaner.New(target).Clean(body)
	page.URL = target
	page.StatusCode = status
	page.Mode = mode
	page.Duration = time.Since(start)

	entities := extractor.New().Extract(body)
	chunks := chunker.ChunkMarkdown(page.Markdown, target, chunker.Default()).Chunks

	writeJSON(w, http.StatusOK, ParseResponse{Page: page, Entities: entities, Chunks: chunks})
}

// Crawl handles POST /api/crawl?url=...&max_pages=...&concurrency=...
// It walks a site's pages (honouring robots.txt) and returns the parsed results.
func (h *Handler) Crawl(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	q := r.Form
	target := q.Get("url")
	if target == "" {
		writeError(w, http.StatusBadRequest, "missing url")
		return
	}
	opts := crawler.Default()
	if v, e := strconv.Atoi(q.Get("max_pages")); e == nil && v > 0 {
		opts.MaxPages = v
	}
	if v, e := strconv.Atoi(q.Get("concurrency")); e == nil && v > 0 {
		opts.Concurrency = v
	}
	opts.UserAgent = q.Get("user_agent")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	pages := h.crawler.Crawl(ctx, target, opts)
	writeJSON(w, http.StatusOK, pages)
}

// MainText handles GET /api/main-text?url=...
// It returns the dominant prose of the page — the "give me the article" helper.
func (h *Handler) MainText(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	target := q.Get("url")
	if target == "" {
		writeError(w, http.StatusBadRequest, "missing url")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	body, status, _, err := h.client.Fetch(ctx, model.FetchOptions{URL: target, UserAgent: q.Get("user_agent")})
	if err != nil {
		writeError(w, http.StatusBadGateway, "fetch failed: "+err.Error())
		return
	}
	if status >= 400 {
		writeError(w, http.StatusBadGateway, "upstream status "+strconv.Itoa(status))
		return
	}
	ex := extractor.New()
	page := cleaner.New(target).Clean(body)
	writeJSON(w, http.StatusOK, map[string]any{
		"url":      target,
		"title":    page.Meta.Title,
		"main_text": ex.MainText(body),
		"word_count": len(splitWords(ex.MainText(body))),
	})
}

// Robots handles GET /api/robots?url=...&path=/...
// It fetches robots.txt and answers whether a path is allowed for a UA.
func (h *Handler) Robots(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	target := q.Get("url")
	if target == "" {
		writeError(w, http.StatusBadRequest, "missing url")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	rb, err := h.client.FetchRobots(ctx, target, q.Get("user_agent"))
	if err != nil {
		writeError(w, http.StatusBadGateway, "robots fetch failed: "+err.Error())
		return
	}
	if rb == nil {
		writeJSON(w, http.StatusOK, map[string]any{"url": target, "exists": false, "allowed": true})
		return
	}
	ua := q.Get("user_agent")
	if ua == "" {
		ua = "glassparse"
	}
	pathArg := q.Get("path")
	if pathArg == "" {
		pathArg = "/"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url":          target,
		"exists":       true,
		"allowed":      rb.Allowed(pathArg, ua),
		"path":         pathArg,
		"crawl_delay":  rb.CrawlDelay(ua).String(),
		"sitemaps":     rb.Sitemaps(),
	})
}

// Fingerprint handles GET /api/fingerprint?url=... — a stable content hash of
// the cleaned page, useful for change detection and dedup.
func (h *Handler) Fingerprint(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	target := q.Get("url")
	if target == "" {
		writeError(w, http.StatusBadRequest, "missing url")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	body, status, _, err := h.client.Fetch(ctx, model.FetchOptions{URL: target, UserAgent: q.Get("user_agent")})
	if err != nil {
		writeError(w, http.StatusBadGateway, "fetch failed: "+err.Error())
		return
	}
	if status >= 400 {
		writeError(w, http.StatusBadGateway, "upstream status "+strconv.Itoa(status))
		return
	}
	page := cleaner.New(target).Clean(body)
	ex := extractor.New()
	writeJSON(w, http.StatusOK, map[string]any{
		"url":               target,
		"fingerprint":       ex.Fingerprint(page.Markdown),
		"short_fingerprint": ex.ShortFingerprint(page.Markdown),
		"input_bytes":       page.Stats.InputBytes,
		"output_bytes":      page.Stats.OutputBytes,
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// splitWords splits on any whitespace — small helper for word counts.
func splitWords(s string) []string {
	return strings.Fields(s)
}