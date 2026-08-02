// Package extractor implements the Self-Healing extraction stage. Instead of
// brittle CSS selectors it reasons on a page with *structural and semantic
// signals*: text density, currency / email / phone patterns and DOM-path shape.
// Blocks are scored by content footprint rather than class name, so it keeps
// finding prices, titles and articles even when a site reworks its CSS — the
// self-healing property.
package extractor

import (
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/glassparse/engine/pkg/model"
)

// ------------------------------ lexical detectors ---------------------------

var (
	// reEmail matches common email addresses.
	reEmail = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	// rePhone matches loose international-ish phone numbers.
	rePhone = regexp.MustCompile(`(?:\+?\d[\d .\-()]{6,}\d)`)
	// Price patterns, both "$12.90" and "12,90 €" orders.
	rePriceSym  = regexp.MustCompile(`[$€£¥₹]\s?\d[\d\s.,]*`)
	rePriceTrav = regexp.MustCompile(`\d+([.,]\d{1,3})?\s?[$€£¥₹]`)

	// contentTokens are keywords that inflate a block's confidence.
	contentTokens = []string{"title", "price", "content", "article", "main",
		"description", "name", "detail", "product", "offer"}
)

func init() { _ = reEmail; _ = rePhone } // keep earlier detectors registered.

// Extractor pulls structured entities out of raw HTML.
type Extractor struct{}

// New returns a stateless Extractor safe to reuse across goroutines.
func New() *Extractor { return &Extractor{} }

// Extract runs the semantic block scan plus the lexical token sweep and returns
// a flat list of entities. Confidence is a 0..1 estimate of signal strength.
func (e *Extractor) Extract(input []byte) []model.Entity {
	root, _ := html.Parse(strings.NewReader(string(input)))
	var entities []model.Entity

	// (1) Semantic pass — pick out article-like / heading-like blocks.
	walk(root, func(n *html.Node) {
		switch n.DataAtom {
		case atom.Article, atom.Main:
			emitBlock(n, "article", &entities)
		case atom.Section:
			if blockScore(n) >= 8 {
				emitBlock(n, "section", &entities)
			}
		case atom.H1, atom.H2, atom.H3:
			emitBlock(n, "title", &entities)
		case atom.Table:
			emitTable(n, &entities)
		}
	})

	// (2) lexical pass — prices / phones / emails inside plain text.
	walk(root, func(n *html.Node) {
		if n.Type == html.TextNode {
			emitLeafSignals(n.Data, &entities)
		}
	})

	return entities
}

// walk is an iterative depth-first traversal, safe on deep documents.
func walk(root *html.Node, fn func(n *html.Node)) {
	stack := []*html.Node{root}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		fn(cur)
		for ch := cur.LastChild; ch != nil; ch = ch.PrevSibling {
			stack = append(stack, ch)
		}
	}
}

// emitBlock appends a text block as an entity when its content is non-trivial.
func emitBlock(n *html.Node, kind string, out *[]model.Entity) {
	txt := strings.TrimSpace(textOf(n))
	if len(txt) < 4 {
		return
	}
	if classNoise(n) {
		return
	}
	*out = append(*out, model.Entity{
		Type:       kind,
		Label:      htmlFor(n),
		Value:      condense(txt),
		Confidence: blockConfidence(n),
		Selector:   pathOf(n),
	})
}

// emitTable records tabular data. Rows become child entities.
func emitTable(n *html.Node, out *[]model.Entity) {
	rows := tableCells(n)
	if len(rows) < 2 {
		return
	}
	label := htmlFor(n)
	if label == "" {
		label = "table"
	}
	conf := 0.7
	if textFor(n) != "" { // non-empty enough to be meaningful.
		conf = 0.9
	}
	kids := make([]*model.Entity, 0, len(rows))
	for i, row := range rows {
		kids = append(kids, &model.Entity{
			Type:  "row",
			Label: "row_" + strconv.Itoa(i),
			Value: strings.Join(row, " | "),
		})
	}
	*out = append(*out, model.Entity{
		Type:       "table",
		Label:      label,
		Value:      joinedRows(rows),
		Confidence: conf,
		Selector:   pathOf(n),
		Children:   kids,
	})
}

// emitLeafSignals scatters entity hits found inside a plain-text leaf.
func emitLeafSignals(text string, out *[]model.Entity) {
	extractEmails(text, out)
	extractPhones(text, out)
	extractPrices(text, out)
}

func extractEmails(text string, out *[]model.Entity) {
	for _, m := range reEmail.FindAllString(text, -1) {
		*out = append(*out, model.Entity{Type: "email", Value: m, Confidence: 0.95})
	}
}

func extractPhones(text string, out *[]model.Entity) {
	for _, m := range rePhone.FindAllString(text, -1) {
		if len(strings.TrimSpace(m)) < 7 {
			continue
		}
		*out = append(*out, model.Entity{Type: "phone", Value: strings.TrimSpace(m), Confidence: 0.85})
	}
}

func extractPrices(text string, out *[]model.Entity) {
	seen := map[string]bool{}
	for _, pat := range []*regexp.Regexp{rePriceSym, rePriceTrav} {
		for _, m := range pat.FindAllString(text, -1) {
			m = strings.TrimSpace(m)
			if m == "" || seen[m] {
				continue
			}
			seen[m] = true
			*out = append(*out, model.Entity{Type: "price", Value: m, Confidence: 0.9})
		}
	}
}