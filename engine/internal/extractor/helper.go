// Structural and DOM helpers shared by the extractor's detectors.
package extractor

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// textOf returns concatenated text of a subtree, uncondensed.
func textOf(n *html.Node) string {
	var sb strings.Builder
	gather(n, &sb)
	return sb.String()
}

func gather(n *html.Node, sb *strings.Builder) {
	if n.Type == html.TextNode {
		sb.WriteString(n.Data)
		return
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		gather(ch, sb)
	}
}

// textFor is a thin alias used where a "display text" is wanted.
func textFor(n *html.Node) string { return textOf(n) }

// condense collapses whitespace runs into single spaces.
func condense(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// attribute is a small accessor for a node's attribute value.
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// noiseTokens are adjectives pointing at boilerplate blocks we should suppress.
var noiseTokens = []string{
	"cookie", "consent", "newsletter", "subscribe", "menu", "navigation",
	"footer", "header", "sidebar", "toolbar", "breadcrumb", "related",
}

// htmlFor is a best-effort human label derived from class/id or the tag name.
func htmlFor(n *html.Node) string {
	if v := attr(n, "id"); v != "" {
		return v
	}
	if v := attr(n, "class"); v != "" {
		return v
	}
	return n.Data
}

// classNoise reports whether the node's class/id strongly indicates boilerplate.
func classNoise(n *html.Node) bool {
	key := strings.ToLower(attr(n, "class") + " " + attr(n, "id"))
	for _, t := range noiseTokens {
		if strings.Contains(key, t) {
			return true
		}
	}
	return false
}

// blockScore measures how "content-like" a block is (text length, structure).
func blockScore(n *html.Node) int {
	score := 0
	txt := textOf(n)
	if len(txt) >= 120 {
		score += 4
	} else if len(txt) >= 40 {
		score += 2
	}
	// count nested block elements as a structural signal.
	stack := []*html.Node{n}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if cur == n {
			continue
		}
		if cur.Type == html.ElementNode {
			switch cur.DataAtom {
			case atom.P, atom.H1, atom.H2, atom.H3, atom.Li, atom.Td:
				score++
			}
		}
		for ch := cur.FirstChild; ch != nil; ch = ch.NextSibling {
			stack = append(stack, ch)
		}
	}
	return score
}

// blockConfidence turns a block's structural footprint into a 0..1 score.
func blockConfidence(n *html.Node) float64 {
	c := 0.5
	label := strings.ToLower(htmlFor(n))
	for _, tok := range contentTokens {
		if strings.Contains(label, tok) {
			c += 0.2
			break
		}
	}
	txt := textOf(n)
	if len(txt) > 200 {
		c += 0.2
	}
	if strings.ContainsAny(txt, ".,!?") && len(txt) > 80 {
		c += 0.1
	}
	if c > 1 {
		return 1
	}
	return c
}

// tableCells flattens a table into [[cell,cell],[cell,cell]] rows (header first).
func tableCells(n *html.Node) [][]string {
	var rows [][]string
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if cur.Type == html.ElementNode && cur.DataAtom == atom.Tr {
			var cells []string
			var cellsIn func(*html.Node)
			cellsIn = func(c *html.Node) {
				if c.Type == html.ElementNode && (c.DataAtom == atom.Th || c.DataAtom == atom.Td) {
					cells = append(cells, condense(textOf(c)))
				}
				for ch := c.FirstChild; ch != nil; ch = ch.NextSibling {
					cellsIn(ch)
				}
			}
			cellsIn(cur)
			if len(cells) > 0 {
				rows = append(rows, cells)
			}
			return
		}
		for ch := cur.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return rows
}

// joinedRows flattens the rows matrix into a compact summary string.
func joinedRows(rows [][]string) string {
	var sb strings.Builder
	for i, row := range rows {
		sb.WriteString(strings.Join(row, " | "))
		if i < len(rows)-1 {
			sb.WriteString(" // ")
		}
	}
	return sb.String()
}

// pathOf builds a stable structural path descriptor like "div.article" — the
// selector a dashboard / user can click to reproduce the extraction.
func pathOf(n *html.Node) string {
	if n == nil || n.Type != html.ElementNode {
		return ""
	}
	var parts []string
	for cur := n; cur != nil && cur.Type == html.ElementNode; cur = cur.Parent {
		name := cur.Data
		if i := indexInSiblings(cur); i > 0 {
			name += "[" + strconv.Itoa(i) + "]"
		}
		parts = append(parts, name)
	}
	// parts were built leaf-to-root; reverse to root-to-leaf.
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, " > ")
}

// indexInSiblings returns the 1-based index of n among its same-tag siblings.
func indexInSiblings(n *html.Node) int {
	idx := 1
	for p := n.Parent.FirstChild; p != nil; p = p.NextSibling {
		if p == n {
			return idx
		}
		if p.Type == html.ElementNode && p.DataAtom == n.DataAtom {
			idx++
		}
	}
	return 1
}