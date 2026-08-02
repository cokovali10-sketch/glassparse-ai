// Readability & fingerprint helpers for the extractor. These let callers go
// from "a pile of DOM" to "the main prose", enumerate the page's own links and
// hash a page so a scheduler can detect changes or near-duplicates.
package extractor

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/glassparse/engine/pkg/model"
)

// MainText returns the dominant prose of the page (a readability-style pick).
// It scores block-level candidates by real text density and returns the best one
// as cleaned, single-spaced content. This is the drop-in "give me the article"
// helper SDKs and chunkers lean on.
func (e *Extractor) MainText(input []byte) string {
	root, _ := html.Parse(strings.NewReader(string(input)))

	best := ""
	bestScore := 0.0
	walk(root, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		switch n.DataAtom {
		case atom.Article, atom.Main, atom.Section, atom.Div, atom.Body, atom.P:
		default:
			return
		}
		if n.DataAtom == atom.Body && childCandidate(n) {
			return // avoid double-counting when a real <article> exists
		}
		if classNoise(n) {
			return
		}
		sc := densityScore(n)
		if sc > bestScore {
			bestScore = sc
			best = condense(textOf(n))
		}
	})
	return best
}

// childCandidate reports whether n (a <body>) contains a block-level candidate
// (article/main/section/div) so we let that narrower candidate win.
func childCandidate(n *html.Node) bool {
	var has bool
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if has {
			return
		}
		if cur.Type == html.ElementNode {
			switch cur.DataAtom {
			case atom.Article, atom.Main, atom.Section:
				has = true
				return
			}
		}
		for ch := cur.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	// Scan n's subtree; the first candidate (e.g. <article>) disables the body.
	walk(n)
	return has
}

// densityScore weights a candidate block by prose density: it rewards long
// paragraphs and punishes link/text fragments and noisy subtree labels.
func densityScore(n *html.Node) float64 {
	txt := condense(textOf(n))
	words := len(strings.Fields(txt))
	if words < 15 {
		return 0
	}
	links := linkWordCount(n)
	den := float64(words-links) / float64(words)
	if den < 0.35 {
		return 0
	}
	return float64(words) * den
}

// linkWordCount sums the words living inside <a> subtrees (nav/breadcrumb noise).
func linkWordCount(n *html.Node) int {
	total := 0
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if cur.Type == html.ElementNode && cur.DataAtom == atom.A {
			total += len(strings.Fields(textOf(cur)))
			return // don't double count nested links' text
		}
		for ch := cur.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return total
}

// Links returns the page's own hyperlinks (text + resolved-or-raw href) as
// entities — handy for crawl graph construction and sitemap building.
func (e *Extractor) Links(input []byte, baseURL string) []model.Entity {
	root, _ := html.Parse(strings.NewReader(string(input)))
	var out []model.Entity
	seen := map[string]bool{}
	walk(root, func(n *html.Node) {
		if n.Type != html.ElementNode || n.DataAtom != atom.A {
			return
		}
		href := strings.TrimSpace(attr(n, "href"))
		if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
			return
		}
		if strings.HasPrefix(href, "data:") || strings.HasPrefix(href, "mailto:") {
			return
		}
		name := condense(textOf(n))
		if href == "" {
			return
		}
		if baseURL != "" {
			if b, ok := resolve(baseURL, href); ok {
				href = b
			}
		}
		if seen[href] {
			return
		}
		seen[href] = true
		out = append(out, model.Entity{Type: "link", Value: href, Label: name, Confidence: 1})
	})
	return out
}

// resolve joins a possibly relative href against base and returns the absolute
// form, or false if it cannot be resolved.
func resolve(base, href string) (string, bool) {
	u, err := url.Parse(href)
	if err != nil {
		return "", false
	}
	if u.Scheme != "" && u.Host != "" {
		return u.String(), true
	}
	b, err := url.Parse(base)
	if err != nil {
		return "", false
	}
	return b.ResolveReference(u).String(), true
}

// Fingerprint hashes the *normalised* page text so layout-only changes (class
// renames, wrapper divs) produce an identical fingerprint, but content changes
// do not. Feed it the cleaned Markdown/HTML for best stability.
func (e *Extractor) Fingerprint(content string) string {
	norm := strings.Join(strings.Fields(content), " ")
	if len(norm) > 8192 {
		norm = norm[:8192]
	}
	h := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(h[:])
}

// ShortFingerprint returns a stable k-byte fragment of PageHash, useful as a
// dedupe key or change token without carrying the full digest around.
func (e *Extractor) ShortFingerprint(content string) string {
	return e.Fingerprint(content)[:16]
}

// CheapHash is a very fast (md5) content hash meant for large corpora sanity /
// near-duplicate pruning; not cryptographic.
func (e *Extractor) CheapHash(content string) string {
	sum := md5.Sum([]byte(content))
	return hex.EncodeToString(sum[:])
}