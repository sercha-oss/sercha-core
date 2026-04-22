package normalisers

import (
	"strings"

	"code.sajari.com/docconv"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Verify interface compliance
var _ driven.Normaliser = (*DocxNormaliser)(nil)

// DocxNormaliser extracts text content from DOCX files using docconv.
type DocxNormaliser struct{}

func (n *DocxNormaliser) Normalise(content string, mimeType string) string {
	if content == "" {
		return ""
	}

	// Extract text from DOCX content
	text, _, err := docconv.ConvertDocx(strings.NewReader(content))
	if err != nil {
		return ""
	}

	// Clean up whitespace

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

func (n *DocxNormaliser) SupportedTypes() []string {
	return []string{"application/vnd.openxmlformats-officedocument.wordprocessingml.document"}
}

func (n *DocxNormaliser) Priority() int {
	return 50 // Medium priority - format-specific
}
