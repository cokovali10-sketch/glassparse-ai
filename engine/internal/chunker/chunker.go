// Package chunker splits structured page content into semantic chunks for RAG
// ingestion. It uses the Markdown heading outline (H1..H4) as natural section
// boundaries, enforces a token budget, and records provenance so downstream
// vector DBs (ChromaDB, Qdrant) and JSONL exports can trace chunks to source.
package chunker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

// Options tunes chunk sizing and enrichment.
type Options struct {
	// MaxTokens is the approximate upper bound of each chunk.
	MaxTokens int
	// MaxChars is a hard fallback cap on chunk size in characters.
	MaxChars int
}

// Default returns recommended chunking options.
func Default() Options { return Options{MaxTokens: 1000, MaxChars: 4000} }

// Chunk is one RAG-ready unit.
type Chunk struct {
	ID       string            `json:"id"`
	Tokens   int               `json:"tokens"`
	Level    int               `json:"level"`   // heading depth of the section.
	Section  string            `json:"section"` // heading trail, e.g. "Docs > Install".
	Markdown string            `json:"markdown"`
	Meta     map[string]string `json:"meta"`
}

// ChunkResult bundles produced chunks with a provenance header.
type ChunkResult struct {
	SourceURL     string    `json:"source_url"`
	EngineVersion string    `json:"engine_version"`
	CreatedAt     time.Time `json:"created_at"`
	Chunks        []Chunk   `json:"chunks"`
}

var (
	reHeading  = regexp.MustCompile(`(?m)^(#{1,4})\s+(.+)$`)
	reNewlines = regexp.MustCompile(`\n{3,}`)
)

// approxCharsPerToken is the chars-per-token heuristic for plain text.
const approxCharsPerToken = 4

// section is an internal heading-delimited slice of the document.
type section struct {
	level   int
	title   string
	content string
}

// ChunkMarkdown splits Markdown into semantic chunks. src is the originating
// URL; it is recorded in metadata so chunks stay attributable.
func ChunkMarkdown(md, src string, opts Options) ChunkResult {
	res := ChunkResult{
		SourceURL:     src,
		EngineVersion: "0.1.0",
		CreatedAt:     time.Now().UTC(),
		Chunks:        []Chunk{},
	}
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 1000
	}
	if opts.MaxChars <= 0 {
		opts.MaxChars = 4000
	}

	sections := sliceSections(md)
	if len(sections) == 0 {
		if t := strings.TrimSpace(md); t != "" {
			res.Chunks = append(res.Chunks, chunkOf(0, "root", 0, t, src))
		}
		return res
	}

	var trail []string
	var buf strings.Builder
	trailStr := ""

	// flush closes the buffered section as one chunk (id uses the *previous*
	// section index, so ids stay stable when a chunk carries a whole section).
	flush := func(prevIdx int, section string, level int) {
		text := strings.TrimSpace(buf.String())
		buf.Reset()
		if text == "" {
			return
		}
		for _, piece := range splitLong(text, opts.MaxChars) {
			res.Chunks = append(res.Chunks, chunkOf(prevIdx, section, level, piece, src))
		}
	}

	for i, s := range sections {
		updateTrail(&trail, i, s)
		nextTrail := strings.Join(trail, " > ")

		// Close the previous section before starting the next heading, so each
		// heading owns its own chunk (semantic, heading-aligned splitting).
		if buf.Len() > 0 {
			flush(i-1, trailStr, headingLevel(i-1, sections))
		}
		trailStr = nextTrail

		buf.WriteString(strings.Repeat("#", s.level) + " " + s.title + "\n\n")
		buf.WriteString(s.content)
		buf.WriteString("\n")
	}
	flush(len(sections)-1, trailStr, headingLevel(len(sections)-1, sections))
	return res
}

// splitLong breaks an over-budget section into pieces at line boundaries.
func splitLong(text string, maxChars int) []string {
	if len(text) <= maxChars {
		return []string{text}
	}
	var pieces []string
	rest := text
	for len(rest) > maxChars {
		cut := -1
		if idx := strings.LastIndexByte(rest[:maxChars], '\n'); idx > maxChars/2 {
			cut = idx
		}
		if cut < 0 {
			cut = maxChars
		}
		pieces = append(pieces, strings.TrimSpace(rest[:cut]))
		rest = rest[cut:]
	}
	if rest = strings.TrimSpace(rest); rest != "" {
		pieces = append(pieces, rest)
	}
	return pieces
}

// sliceSections splits the document at heading lines.
func sliceSections(md string) []section {
	idx := reHeading.FindAllStringSubmatchIndex(md, -1)
	out := make([]section, 0, len(idx))
	for i, m := range idx {
		end := len(md)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		out = append(out, section{
			level:   len(md[m[2]:m[3]]), // count of '#'.
			title:   strings.TrimSpace(md[m[4]:m[5]]),
			content: reNewlines.ReplaceAllString(md[m[0]:end], "\n\n"),
		})
	}
	return out
}

// updateTrail keeps a heading stack pruned to the current section's depth.
func updateTrail(trail *[]string, i int, s section) {
	for len(*trail) >= s.level {
		*trail = (*trail)[:len(*trail)-1]
	}
	*trail = append(*trail, s.title)
}

// headingLevel looks up the depth for a given section index.
func headingLevel(i int, sections []section) int {
	if i >= 0 && i < len(sections) {
		return sections[i].level
	}
	return 0
}

// chunkOf assembles a Chunk for the given section context.
func chunkOf(i int, section string, level int, text, src string) Chunk {
	return Chunk{
		ID:       fmt.Sprintf("chunk-%d", i),
		Section:  section,
		Level:    level,
		Markdown: text,
		Tokens:   tokensOf(text),
		Meta:     map[string]string{"src": src},
	}
}

// tokensOf estimates token count from character count.
func tokensOf(s string) int {
	if s == "" {
		return 0
	}
	return len(s)/approxCharsPerToken + 1
}

// WriteJSONL streams a ChunkResult to w as one JSON object per line — the format
// loaders for Qdrant / ChromaDB / LangChain / LlamaIndex ingest directly.
func (r ChunkResult) WriteJSONL(w io.Writer) error {
	enc := json.NewEncoder(w)
	for _, ch := range r.Chunks {
		if err := enc.Encode(ch); err != nil {
			return err
		}
	}
	return nil
}

// ExportJSONL writes a ChunkResult to a file in JSONL form and returns bytes
// flushed. Convenience wrapper around WriteJSONL for CLIs and pipelines.
func ExportJSONL(res ChunkResult, path string) (int64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	bw := bufio.NewWriter(f)
	counter := &countingWriter{w: bw}
	if err := res.WriteJSONL(counter); err != nil {
		_ = f.Close()
		return counter.n, err
	}
	if err := bw.Flush(); err != nil {
		_ = f.Close()
		return counter.n, err
	}
	return counter.n, f.Close()
}

// countingWriter wraps an io.Writer and records how many bytes passed through.
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}