// Package cleaner implements stage one of the GlassParse AI pipeline: turning
// raw, noisy HTML into a semantically clean DOM tree and optimised Markdown.
//
// Goals:
//   - robustness: never panic on malformed or hostile markup,
//   - token economy: drop anything a downstream LLM does not need,
//   - metrology: track how much DOM was removed so the dashboard can render
//     live compression & throughput charts.
package cleaner

import (
	"bytes"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/glassparse/engine/pkg/model"
)

// maxAttrLen guards against pathological attribute values (data-URIs, minified
// JS jammed into attributes) that would inflate token counts.
const maxAttrLen = 512

// bannedTags are removed outright, including their whole subtree. This is the
// "scripts, styles, svg, ads, navigation" hygiene pass.
var bannedTags = map[atom.Atom]bool{
	atom.Script: true, atom.Style: true, atom.Svg: true, atom.Noscript: true,
	atom.Template: true, atom.Iframe: true, atom.Object: true, atom.Embed: true,
	atom.Video: true, atom.Audio: true, atom.Canvas: true,
	atom.Nav: true, atom.Footer: true, atom.Header: true, atom.Aside: true,
	atom.Form: true, atom.Input: true, atom.Button: true, atom.Select: true,
	atom.Textarea: true, atom.Option: true, atom.Optgroup: true,
	atom.Link: true, atom.Meta: true, atom.Map: true, atom.Area: true,
	atom.Noembed: true, atom.Noframes: true, atom.Frameset: true, atom.Frame: true,
	// Keep the text but drop time/semantic wrappers whose meaning varies per site.
	atom.Time: true,
}

// noiseSubstrings match ad-bot generated class/id names. A node whose class or
// id contains any of these is treated as noise and pruned.
var noiseSubstrings = []string{
	"advert", "ads", "banner", "cookie", "consent", "popup", "modal",
	"newsletter", "subscribe", "social", "share", "menu", "sidebar",
	"comment", "related", "sponsor", "promo", "tracker", "analytics",
	"widget", "promotions", "recommend", "carousel", "slider",
}

// Cleaner runs the cleaning pipeline against one HTML document.
type Cleaner struct {
	baseURL *url.URL

	stats    model.Stats
	metadata model.Metadata

	buf         bytes.Buffer // reusable markdown output buffer.
	htmlOut     bytes.Buffer // reusable clean-html output buffer.
	nodesBefore int
	nodesRemoved int
	links        int
}

// New returns a Cleaner ready to process one document whose navigation-relative
// links should resolve against baseURL.
func New(baseURL string) *Cleaner {
	u, _ := url.Parse(baseURL)
	return &Cleaner{baseURL: u}
}

// Clean runs the full pipeline on input and returns a populated model.Page.
// It never returns an error for malformed HTML — it degrades gracefully instead,
// which keeps long-running crawls alive.
func (c *Cleaner) Clean(input []byte) model.Page {
	c.reset()

	root, err := html.Parse(bytes.NewReader(input))
	if err != nil {
		return model.Page{Stats: c.stats, Meta: c.metadata}
	}

	// Pass 1: collect <head>/<meta> facts + count nodes.
	c.collect(root)
	// Pass 2: prune noise, producing a "clean" DOM.
	kept := c.sanitize(root)
	// Pass 3: flatten the clean DOM into Markdown.
	c.renderMarkdown(kept, 0)

	c.stats.OutputBytes = int64(c.buf.Len())
	c.stats.InputBytes = int64(len(input))
	c.stats.CompressionRatio = ratio(c.stats.OutputBytes, c.stats.InputBytes)
	c.stats.RemovedNodes = c.nodesRemoved
	c.stats.Headings = countHeadings(kept)
	c.stats.Links = c.links
	c.stats.NodesBefore = c.nodesBefore
	c.stats.NodesAfter = countNodes(kept)

	c.htmlOut.WriteString(renderNode(kept))
	c.stats.Tables = countTag(kept, atom.Table)
	c.stats.Images = countTag(kept, atom.Img)

	return model.Page{
		Meta:      c.metadata,
		Markdown:  c.buf.String(),
		CleanHTML: c.htmlOut.String(),
		Stats:     c.stats,
	}
}

// reset zeroes per-run mutable state so a Cleaner instance can be reused.
func (c *Cleaner) reset() {
	c.stats = model.Stats{}
	c.metadata = model.Metadata{OG: map[string]string{}}
	c.buf.Reset()
	c.htmlOut.Reset()
	c.nodesBefore, c.nodesRemoved, c.links = 0, 0, 0
}

// ---------------------------------------------------------------------------
// Pass 1 — metadata & counting
// ---------------------------------------------------------------------------

// collect walks the untouched DOM reading facts and counting nodes.
func (c *Cleaner) collect(n *html.Node) {
	if n.Type == html.ElementNode {
		c.nodesBefore++
		switch n.DataAtom {
		case atom.Html:
			if v := getAttr(n, "lang"); v != "" {
				c.metadata.Lang = v
			}
		case atom.Title:
			c.metadata.Title = strings.TrimSpace(textOf(n))
		case atom.Meta:
			c.readMeta(n)
		case atom.Link:
			if strings.Contains(strings.ToLower(getAttr(n, "rel")), "canonical") {
				c.metadata.Canonical = c.absHref(getAttr(n, "href"))
			}
		}
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		c.collect(ch)
	}
}

// readMeta fans out the various <meta> conventions into Metadata.
func (c *Cleaner) readMeta(n *html.Node) {
	prop := getAttr(n, "property")
	name := getAttr(n, "name")
	content := strings.TrimSpace(getAttr(n, "content"))
	httpEquiv := getAttr(n, "http-equiv")
	if content == "" {
		return
	}
	key := strings.ToLower(firstNonEmpty(prop, name))
	if key == "" || strings.HasPrefix(key, "twitter") {
		return
	}
	switch {
	case strings.Contains(key, "title"):
		if c.metadata.Title == "" {
			c.metadata.Title = content
		}
	case strings.Contains(key, "description"):
		c.metadata.Description = content
	case strings.Contains(key, "site_name"):
		c.metadata.SiteName = content
	case strings.Contains(key, "author"):
		c.metadata.Author = content
	case strings.Contains(key, "image"):
		c.metadata.Image = content
	case strings.Contains(key, "keywords"):
		for _, k := range strings.Split(content, ",") {
			if k = strings.TrimSpace(k); k != "" {
				c.metadata.Keywords = append(c.metadata.Keywords, k)
			}
		}
	default:
		if key == "og" || strings.Contains(prop, "og:") {
			if c.metadata.OG == nil {
				c.metadata.OG = map[string]string{}
			}
			c.metadata.OG[prop] = content
		} else if strings.HasPrefix(prop, "og:") {
			if c.metadata.OG == nil {
				c.metadata.OG = map[string]string{}
			}
			c.metadata.OG[prop] = content
		}
	}
	// http-equiv refresh / meta can be ignored — only count.
	_ = httpEquiv
}

// ---------------------------------------------------------------------------
// Pass 2 — deep prune
// ---------------------------------------------------------------------------

// sanitize prunes banned + noisy subtrees, returning the roots of kept nodes.
// It walks the tree using a manual stack to avoid unbounded recursion on
// deeply nested attacker-controlled documents.
func (c *Cleaner) sanitize(root *html.Node) *html.Node {
	var stack []*html.Node
	stack = append(stack, root)
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if cur.Type == html.ElementNode {
			if shouldDrop(cur) {
				c.nodesRemoved += countNodes(cur)
				unlink(cur)
				continue
			}
			cur.Attr = cleanAttr(cur.Attr)
		}
		for ch := cur.LastChild; ch != nil; ch = ch.PrevSibling {
			stack = append(stack, ch)
		}
	}
	return root
}

// shouldDrop decides whether a node (and its subtree) is noise.
func shouldDrop(n *html.Node) bool {
	if bannedTags[n.DataAtom] {
		return true
	}
	key := strings.ToLower(getAttr(n, "class") + " " + getAttr(n, "id"))
	if key == "" {
		return false
	}
	for _, sub := range noiseSubstrings {
		if strings.Contains(key, sub) {
			return true
		}
	}
	return false
}

// cleanAttr strips tracking/logging attributes and over-long values.
func cleanAttr(attrs []html.Attribute) []html.Attribute {
	out := attrs[:0]
	for _, a := range attrs {
		switch strings.ToLower(a.Key) {
		case "onclick", "onload", "onerror", "onmouseover", "onfocus",
			"onblur", "onchange", "onsubmit", "onkeydown", "onkeyup",
			"formaction", "autoplay", "ping", "style", "sizes", "srcset":
			continue
		}
		if len(a.Val) > maxAttrLen {
			a.Val = a.Val[:maxAttrLen]
		}
		out = append(out, a)
	}
	return out
}

// unlink removes a node from the tree, tracking how many nodes were removed.
func unlink(n *html.Node) {
	if n.Parent != nil {
		n.Parent.RemoveChild(n)
	}
}