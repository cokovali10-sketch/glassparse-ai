package cleaner

import (
	"strings"
	"testing"
)

const sample = `<!DOCTYPE html><html><head><title>Acme</title>
<meta name="description" content="d">
</head><body>
<nav><a href="/x">menu</a></nav>
<h1>Acme Product</h1>
<p>It costs $99 for the deluxe edition, contact hi@acme.io</p>
<table><tr><th>K</th><th>V</th></tr><tr><td>A</td><td>B</td></tr></table>
<script>track()</script>
</body></html>`

func TestClean(t *testing.T) {
	c := New("https://acme.test")
	p := c.Clean([]byte(sample))

	if p.Meta.Title != "Acme" {
		t.Fatalf("title = %q, want Acme", p.Meta.Title)
	}
	if !strings.Contains(p.Markdown, "# Acme Product") {
		t.Fatalf("markdown missing heading: %q", p.Markdown)
	}
	if strings.Contains(p.Markdown, "track()") {
		t.Fatalf("javascript leaked into markdown")
	}
	if strings.Contains(p.Markdown, "menu") {
		t.Fatalf("nav leaked into markdown")
	}
	if p.Stats.InputBytes == 0 || p.Stats.OutputBytes == 0 {
		t.Fatalf("stats not populated: %+v", p.Stats)
	}
	if p.Stats.NodesAfter >= p.Stats.NodesBefore {
		t.Fatalf("expected DOM shrinkage, before=%d after=%d", p.Stats.NodesBefore, p.Stats.NodesAfter)
	}
}

func TestCleanRejectsJunk(t *testing.T) {
	c := New("")
	p := c.Clean([]byte("<div id=\"cookie-banner\">Accept</div><p>ok</p>"))
	if strings.Contains(p.Markdown, "Accept") {
		t.Fatalf("cookie banner survived: %q", p.Markdown)
	}
	if !strings.Contains(p.Markdown, "ok") {
		t.Fatalf("content missing: %q", p.Markdown)
	}
}

func TestCleanKeepsInlineSpacing(t *testing.T) {
	c := New("https://shop.test")
	html := `<p>The deluxe edition starts at <span class="price">$12,499.00</span> in
the flagship store &amp; ships <b>free</b>.</p>`
	p := c.Clean([]byte(html))
	md := p.Markdown
	if !strings.Contains(md, "starts at $12,499.00 in") {
		t.Fatalf("words fused across inline boundary: %q", md)
	}
	if !strings.Contains(md, "& ships free.") {
		t.Fatalf("entity / inline spacing broken: %q", md)
	}
}