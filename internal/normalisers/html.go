package normalisers

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// HTMLNormaliser parses HTML into structured plain text. Headings keep their
// level as ATX-style markdown (`# `, `## `, …) so a section-aware chunker can
// split on `^#+ `; image alt and link title text are preserved inline; script,
// style, and other non-content elements are dropped wholesale.
type HTMLNormaliser struct{}

func (n *HTMLNormaliser) Normalise(content string, mimeType string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return ""
	}

	var b strings.Builder
	walkHTML(doc, &b)

	text := b.String()
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Collapse runs of spaces/tabs (but not newlines — those carry section
	// boundaries we just emitted).
	text = collapseInlineWhitespace(text)

	// Trim each line, then collapse 3+ blank lines to a single blank line.
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	text = strings.Join(lines, "\n")
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(text)
}

func (n *HTMLNormaliser) SupportedTypes() []string {
	return []string{"text/html", "application/xhtml+xml"}
}

func (n *HTMLNormaliser) Priority() int {
	return 50 // Medium priority - format-specific
}

// walkHTML traverses the parsed tree, emitting plain text plus markdown
// boundary markers. The output is meant for downstream tokenisation, not
// human reading: headings prefix with ATX (`## `), block elements emit a
// blank line before/after, image/link metadata folds into inline text.
func walkHTML(n *html.Node, b *strings.Builder) {
	if n == nil {
		return
	}

	switch n.Type {
	case html.TextNode:
		b.WriteString(n.Data)
		return
	case html.CommentNode, html.DoctypeNode:
		return
	}

	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Noscript, atom.Template, atom.Svg, atom.Iframe:
		return
	case atom.Img:
		if alt := getAttr(n, "alt"); alt != "" {
			b.WriteString(alt)
			b.WriteByte(' ')
		}
		return
	case atom.A:
		// Walk children for the anchor text. If the anchor has a title
		// attribute and it differs from the rendered text, append it once.
		var inner strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkHTML(c, &inner)
		}
		text := inner.String()
		b.WriteString(text)
		if title := getAttr(n, "title"); title != "" && !strings.Contains(text, title) {
			b.WriteByte(' ')
			b.WriteString(title)
		}
		return
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		level := headingLevel(n.DataAtom)
		var inner strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkHTML(c, &inner)
		}
		text := strings.TrimSpace(collapseInlineWhitespace(inner.String()))
		if text == "" {
			return
		}
		b.WriteString("\n\n")
		b.WriteString(strings.Repeat("#", level))
		b.WriteByte(' ')
		b.WriteString(text)
		b.WriteString("\n\n")
		return
	}

	// Block elements get a blank line before/after; inline elements pass
	// through unchanged.
	block := isBlockElement(n.DataAtom)
	if block {
		b.WriteByte('\n')
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkHTML(c, b)
	}
	if block {
		b.WriteByte('\n')
	}
}

func headingLevel(a atom.Atom) int {
	switch a {
	case atom.H1:
		return 1
	case atom.H2:
		return 2
	case atom.H3:
		return 3
	case atom.H4:
		return 4
	case atom.H5:
		return 5
	default:
		return 6
	}
}

func isBlockElement(a atom.Atom) bool {
	switch a {
	case atom.P, atom.Div, atom.Section, atom.Article, atom.Header, atom.Footer,
		atom.Main, atom.Aside, atom.Nav,
		atom.Ul, atom.Ol, atom.Li, atom.Dl, atom.Dt, atom.Dd,
		atom.Br, atom.Hr,
		atom.Blockquote, atom.Pre, atom.Figure, atom.Figcaption,
		atom.Table, atom.Thead, atom.Tbody, atom.Tfoot, atom.Tr, atom.Td, atom.Th:
		return true
	}
	return false
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// collapseInlineWhitespace collapses runs of spaces and tabs to a single
// space without touching newlines (those mark structural boundaries).
func collapseInlineWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return b.String()
}
