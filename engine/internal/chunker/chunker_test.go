package chunker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChunkMarkdownByHeadings(t *testing.T) {
	md := "# Intro\nsome intro text.\n\n" +
		"## Install\nrun the installer.\n" +
		"### Advanced\nnested detail.\n" +
		"## Usage\nuse it.\n"

	res := ChunkMarkdown(md, "https://x.test", Default())
	if len(res.Chunks) != 4 {
		t.Fatalf("expected 4 chunks, got %d: %+v", len(res.Chunks), res)
	}
	if res.Chunks[0].Section != "Intro" {
		t.Fatalf("first section = %q", res.Chunks[0].Section)
	}
	// Heading-trail test: the "Advanced" chunk carries the full path.
	if !strings.Contains(res.Chunks[2].Markdown, "# Advanced") {
		t.Fatalf("advanced chunk lost its heading: %q", res.Chunks[2].Markdown)
	}
}

func TestChunkMarkdownNoHeadings(t *testing.T) {
	md := "just a single paragraph of plain text without any headings."
	res := ChunkMarkdown(md, "s", Default())
	if len(res.Chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(res.Chunks))
	}
	if res.Chunks[0].Meta["src"] != "s" {
		t.Fatalf("missing provenance: %+v", res.Chunks[0])
	}
	if res.Chunks[0].Tokens <= 0 {
		t.Fatal("token count not populated")
	}
}

func TestWriteAndExportJSONL(t *testing.T) {
	res := ChunkMarkdown("# One\nhello.\n## Two\nworld.\n", "https://s.test", Default())
	dir := t.TempDir()
	path := filepath.Join(dir, "chunks.jsonl")

	n, err := ExportJSONL(res, path)
	if err != nil {
		t.Fatalf("ExportJSONL: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if int64(len(raw)) != n {
		t.Fatalf("byte count mismatch: reported=%d file=%d", n, len(raw))
	}
	// one JSON object per line, each with id + markdown.
	lines := strings.Fields(string(raw))
	_ = lines
	lineCount := 0
	for _, ln := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if strings.TrimSpace(ln) != "" {
			lineCount++
			if !strings.Contains(ln, `"id"`) || !strings.Contains(ln, `"markdown"`) {
				t.Fatalf("line lacks chunk shape: %s", ln)
			}
		}
	}
	if lineCount != len(res.Chunks) {
		t.Fatalf("expected %d JSONL lines, got %d", len(res.Chunks), lineCount)
	}
}