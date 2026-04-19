package normalisers

import (
	"strings"

	"github.com/heussd/pdftotext-go"
)

// PDFNormaliser extracts text content from PDF files using pdftotext-go.
// Requires poppler-utils >= 22.05.0 to be installed.
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

	// Concatenate page contents with double newlines
	var result strings.Builder
	for i, page := range pages {
		if page.Content == "" {
			continue
		}

		result.WriteString(strings.TrimSpace(page.Content))

		// Add separator between pages
		if i < len(pages)-1 {
			result.WriteString("\n\n")
		}
	}

	// Clean up whitespace
	text := result.String()

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
