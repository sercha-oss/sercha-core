package normalisers

import (
	"fmt"
	"strings"

	"github.com/heussd/pdftotext-go"
)

// PDFNormaliser extracts text content from PDF files using pdftotext-go.
// Requires poppler-utils >= 22.05.0 to be installed.
//
// Pages are emitted as `## Page N` headings between blocks of content. PDFs
// don't expose section structure to pdftotext (font sizes / styles are gone
// by the time the text comes back), so the page boundary is the only
// reliable structural signal we have. Encoding it as an ATX heading lets a
// section-aware chunker keep page-coherent windows and lets a presenter cite
// "from page 7" later.
type PDFNormaliser struct{}

func (n *PDFNormaliser) Normalise(content string, mimeType string) string {
	if content == "" {
		return ""
	}

	// Extract text from PDF bytes
	pages, err := pdftotext.Extract([]byte(content))
	if err != nil {
		return ""
	}

	if len(pages) == 0 {
		return ""
	}

	text := assemblePages(pages)

	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Collapse multiple spaces
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}

	// Remove excessive blank lines
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(text)
}

func (n *PDFNormaliser) SupportedTypes() []string {
	return []string{"application/pdf"}
}

func (n *PDFNormaliser) Priority() int {
	return 50 // Medium priority - format-specific
}

// assemblePages joins page contents with `## Page N` markers. Empty pages are
// skipped (some PDFs have intentional blanks); falls back to the slice index
// when poppler doesn't report a page number.
func assemblePages(pages []pdftotext.PdfPage) string {
	var b strings.Builder
	for i, page := range pages {
		body := strings.TrimSpace(page.Content)
		if body == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		num := page.Number
		if num <= 0 {
			num = i + 1
		}
		fmt.Fprintf(&b, "## Page %d\n\n%s", num, body)
	}
	return b.String()
}
