// Command glassparse is the runnable demo of the GlassParse AI engine core.
// It drives the fetcher -> cleaner -> extractor -> chunker pipeline end-to-end
// and prints a human or machine-readable report.
//
// Usage:
//
//		go run ./cmd/glassparse -url https://example.com
//		go run ./cmd/glassparse -file ./sample.html
//		go run ./cmd/glassparse -url ... -json
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"github.com/glassparse/engine/internal/chunker"
	"github.com/glassparse/engine/internal/cleaner"
	"github.com/glassparse/engine/internal/extractor"
	"github.com/glassparse/engine/internal/fetcher"
	"github.com/glassparse/engine/pkg/model"
)

func main() {
	var (
		sourceURL = flag.String("url", "", "url to parse")
		file      = flag.String("file", "", "parse a local .html file instead of a URL")
		asJSON    = flag.Bool("json", false, "emit the full result as JSON")
	)
	flag.Parse()
	if *sourceURL == "" && *file == "" {
		log.Fatal("provide -url or -file")
	}

	htmlBytes, status, renderMode, err := acquire(*sourceURL, *file)
	if err != nil {
		log.Fatalf("fetch: %v", err)
	}

	// Stage 1 — clean & convert to Markdown / semantic HTML.
	page := cleaner.New(*sourceURL).Clean(htmlBytes)
	page.URL = *sourceURL
	page.StatusCode = status
	page.Mode = renderMode

	// Stage 2 — self-healing extraction.
	entities := extractor.New().Extract(htmlBytes)

	// Stage 3 — RAG chunking of the cleaned Markdown.
	chunks := chunker.ChunkMarkdown(page.Markdown, *sourceURL, chunker.Default()).Chunks

	if *asJSON {
		b, _ := json.MarshalIndent(struct {
			model.Page
			Entities []model.Entity  `json:"entities"`
			Chunks   []chunker.Chunk `json:"chunks"`
		}{page, entities, chunks}, "", "  ")
		os.Stdout.Write(b)
		os.Stdout.Write([]byte("\n"))
		return
	}

	log.Printf("== parsed %q mode=%s status=%d", *sourceURL, renderMode, status)
	log.Printf("metadata : %q | %s", page.Meta.Title, page.Meta.Description)
	log.Printf("stats    : in=%dB out=%dB (%.0f%%) nodes %d->%d removed=%d",
		page.Stats.InputBytes, page.Stats.OutputBytes, page.Stats.CompressionRatio*100,
		page.Stats.NodesBefore, page.Stats.NodesAfter, page.Stats.RemovedNodes)
	log.Printf("entities : %d", len(entities))
	for _, e := range entities {
		log.Printf("   [%s] %q conf=%.2f @ %s", e.Type, trunc(e.Value, 60), e.Confidence, e.Selector)
	}
	log.Printf("chunks   : %d", len(chunks))
	for _, c := range chunks {
		log.Printf("   #%s level=%d tokens=%d %s", c.ID, c.Level, c.Tokens, trunc(c.Section, 40))
	}
}

// acquire loads raw HTML from a URL (through the anti-bot fetcher) or a file.
// It returns body bytes, the HTTP status code, and the render mode used.
func acquire(pageURL, file string) ([]byte, int, model.RenderMode, error) {
	if file != "" {
		b, err := os.ReadFile(file)
		return b, 0, model.RenderHTTP, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client := fetcher.New("")
	body, status, mode, err := client.Fetch(ctx, model.FetchOptions{URL: pageURL})
	if err != nil {
		return nil, status, "", err
	}
	return body, status, mode, nil
}

// trunc shortens a string for compact logging.
func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}