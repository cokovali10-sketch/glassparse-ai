// Markdown rendering pass for the cleaner. This flattens the pruned DOM into a
// token-efficient CommonMark-ish document: headings become # … ####, lists keep
// nesting, links and images collapse to inline forms, tables stay readable.
package cleaner

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// headingLevel maps an h1..h6 atom to its markdown # depth.
var headingLevel = map[atom.Atom]int{
	atom.H1: 1, atom.H2: 2, atom.H3: 3,
	atom.H4: 4, atom.H5: 5, atom.H6: 6,
}

// renderMarkdown recursively writes markdown for node n at the given list depth.
func (c *Cleaner) renderMarkdown(n *html.Node, listDepth int) {
	// The parser returns a DocumentNode as the root; unwrap it.
	if n.Type == html.DocumentNode {
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			c.renderMarkdown(ch, listDepth)
		}
		return
	}
	if n.Type == html.TextNode {
		// Keep boundary spacing so adjacent inline nodes stay separated
		// (fixes "starts at$12,499.00in"); only collapse interior whitespace.
		t := collapseSpace(n.Data)
		if strings.TrimSpace(t) != "" {
			c.buf.WriteString(t)
		}
		return
	}
	if n.Type != html.ElementNode {
		return
	}
	switch {
	case n.DataAtom == atom.Img:
		c.renderImg(n)
	case n.DataAtom == atom.Br || n.DataAtom == atom.Hr:
		c.buf.WriteString("\n")
	case n.DataAtom == atom.P || n.DataAtom == atom.Div:
		c.renderBlock(n, listDepth, true)
	case n.DataAtom == atom.Ul || n.DataAtom == atom.Ol:
		c.renderList(n, n.DataAtom == atom.Ol, listDepth)
	case n.DataAtom == atom.Li:
		c.renderListItem(n, listDepth)
	case n.DataAtom == atom.Table:
		c.renderTable(n)
	case n.DataAtom == atom.Blockquote:
		c.renderBlockquote(n, listDepth)
	case n.DataAtom == atom.Code || n.DataAtom == atom.Pre:
		c.renderCode(n)
	case headingLevel[n.DataAtom] > 0:
		c.renderHeading(n)
	case n.DataAtom == atom.A:
		c.renderLink(n, listDepth)
	default:
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			c.renderMarkdown(ch, listDepth)
		}
	}
}

// inline renderers -----------------------------------------------------------

func (c *Cleaner) renderLink(n *html.Node, d int) {
	var sb strings.Builder
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		sb.WriteString(textOf(ch))
	}
	txt := strings.TrimSpace(sb.String())
	if txt == "" {
		return
	}
	c.buf.WriteString(fmt.Sprintf("[%s](%s)", txt, c.absHref(getAttr(n, "href"))))
}

func (c *Cleaner) renderImg(n *html.Node) {
	alt := strings.TrimSpace(getAttr(n, "alt"))
	if alt == "" {
		alt = getAttr(n, "title")
	}
	src := c.absHref(getAttr(n, "src"))
	if alt == "" {
		c.buf.WriteString(fmt.Sprintf("![image](%s)", src))
		return
	}
	c.buf.WriteString(fmt.Sprintf("![%s](%s)", alt, src))
}

// block renderers ------------------------------------------------------------

func (c *Cleaner) renderBlock(n *html.Node, d int, spacing bool) {
	text := textOf(n)
	if strings.TrimSpace(text) == "" {
		return
	}
	if spacing {
		c.buf.WriteString("\n\n")
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		c.renderMarkdown(ch, d)
	}
	if spacing {
		c.buf.WriteString("\n")
	}
}

func (c *Cleaner) renderHeading(n *html.Node) {
	text := strings.TrimSpace(textOf(n))
	if text == "" {
		return
	}
	level := headingLevel[n.DataAtom]
	c.buf.WriteString("\n\n" + strings.Repeat("#", level) + " " + text + "\n")
}

func (c *Cleaner) renderList(n *html.Node, ordered bool, d int) {
	c.buf.WriteString("\n")
	// Use a growing depth so nested lists keep their relative form.
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.ElementNode && ch.DataAtom == atom.Li {
			c.renderListItem(ch, d+1)
		}
	}
	c.buf.WriteString("\n")
}

func (c *Cleaner) renderListItem(n *html.Node, d int) {
	c.buf.WriteString(strings.Repeat("  ", max(0, d-1)))
	// Simplified bullet glyph; ordered numbering could be derived from index.
	c.buf.WriteString("- ")
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.ElementNode && (ch.DataAtom == atom.Ul || ch.DataAtom == atom.Ol) {
			c.buf.WriteString("\n")
			for sub := ch.FirstChild; sub != nil; sub = sub.NextSibling {
				if sub.Type == html.ElementNode && sub.Data == "li" {
					c.renderListItem(sub, d+1)
				}
			}
			continue
		}
		c.renderMarkdown(ch, d)
	}
	c.buf.WriteString("\n")
}

func (c *Cleaner) renderTable(n *html.Node) {
	c.buf.WriteString("\n")
	// Extract the grid of cells, then lay out a markdown table.
	rows := tableRows(n)
	if len(rows) == 0 {
		return
	}
	cols := len(rows[0])
	if cols == 0 {
		return
	}
	header := rows[0]
	c.buf.WriteString("| " + strings.Join(header, " | ") + " |\n")
	c.buf.WriteString("|" + strings.Repeat(" --- |", cols) + "\n")
	for _, row := range rows[1:] {
		padded := row
		for len(padded) < cols {
			padded = append(padded, "")
		}
		c.buf.WriteString("| " + strings.Join(padded, " | ") + " |\n")
	}
	c.buf.WriteString("\n")
}

func (c *Cleaner) renderBlockquote(n *html.Node, d int) {
	text := strings.TrimSpace(textOf(n))
	if text == "" {
		return
	}
	// Prefix every line with ">". Simplify: single-block emission.
	c.buf.WriteString("\n> " + text + "\n")
}

// renderCode emits <code>/<pre> content as fenced code blocks.
func (c *Cleaner) renderCode(n *html.Node) {
	text := strings.TrimSpace(textOf(n))
	if text == "" {
		return
	}
	c.buf.WriteString("\n```\n" + text + "\n```\n")
}

// helpers --------------------------------------------------------------------

// absHref resolves a possibly-relative href/src against the page base URL.
func (c *Cleaner) absHref(href string) string {
	if href == "" || c.baseURL == nil {
		return href
	}
	if u, err := c.baseURL.Parse(href); err == nil {
		if u.Scheme != "" {
			return u.String()
		}
	}
	return href
}

// textOf returns the trimmed concatenated text of a subtree.
func textOf(n *html.Node) string {
	var sb strings.Builder
	gatherText(n, &sb)
	return sb.String()
}

func gatherText(n *html.Node, sb *strings.Builder) {
	if n.Type == html.TextNode {
		sb.WriteString(n.Data)
		return
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		gatherText(ch, sb)
	}
}

// renderNode serialises a DOM subtree back to HTML (used for clean_html).
func renderNode(n *html.Node) string {
	var sb strings.Builder
	writeNode(&sb, n)
	return sb.String()
}

func writeNode(sb *strings.Builder, n *html.Node) {
	if n.Type != html.ElementNode && n.Type != html.DocumentNode {
		return
	}
	sb.WriteString("<" + n.Data)
	for _, a := range n.Attr {
		sb.WriteString(" " + a.Key + "=\"" + html.EscapeString(a.Val) + "\"")
	}
	sb.WriteString(">")
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.TextNode {
			sb.WriteString(html.EscapeString(ch.Data))
			continue
		}
		writeNode(sb, ch)
	}
	sb.WriteString("</" + n.Data + ">")
}

// counting helpers -----------------------------------------------------------

// countNodes counts element nodes in a subtree (before/after/removed metrics).
func countNodes(n *html.Node) int {
	total := 0
	var stack = []*html.Node{n}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if cur.Type == html.ElementNode {
			total++
		}
		for ch := cur.FirstChild; ch != nil; ch = ch.NextSibling {
			stack = append(stack, ch)
		}
	}
	return total
}

func countTag(n *html.Node, tag atom.Atom) int {
	total := 0
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if cur.Type == html.ElementNode && cur.DataAtom == tag {
			total++
		}
		for ch := cur.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return total
}

func countHeadings(n *html.Node) int {
	return countTag(n, atom.H1) + countTag(n, atom.H2) + countTag(n, atom.H3) +
		countTag(n, atom.H4) + countTag(n, atom.H5) + countTag(n, atom.H6)
}

// tableRows flattens <table> into header + body rows of cell text.
func tableRows(n *html.Node) [][]string {
	var rows [][]string
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if cur.Type == html.ElementNode && cur.DataAtom == atom.Tr {
			var cells []string
			var cellWalk func(*html.Node)
			cellWalk = func(c *html.Node) {
				if c.Type == html.ElementNode && (c.DataAtom == atom.Th || c.DataAtom == atom.Td) {
					cells = append(cells, strings.TrimSpace(textOf(c)))
				}
				for ch := c.FirstChild; ch != nil; ch = ch.NextSibling {
					cellWalk(ch)
				}
			}
			cellWalk(cur)
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

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func ratio(out, in int64) float64 {
	if in <= 0 {
		return 0
	}
	return float64(out) / float64(in)
}

// firstNonEmpty returns the first non-empty argument.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// collapseSpace collapses any run of whitespace (spaces, tabs, newlines) to a
// single space while preserving whether the string *starts/ends* in whitespace.
// This keeps inline nodes (spans, em, links) from fusing into one word.
func collapseSpace(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	lead := s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r'
	trail := false
	if n := len(s); n > 0 {
		trail = s[n-1] == ' ' || s[n-1] == '\t' || s[n-1] == '\n' || s[n-1] == '\r'
	}
	s = strings.Join(strings.Fields(s), " ")
	if lead {
		s = " " + s
	}
	if trail {
		s = s + " "
	}
	return s
}