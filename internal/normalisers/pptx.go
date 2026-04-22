package normalisers

import (
	"strings"

	"code.sajari.com/docconv"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Verify interface compliance
var _ driven.Normaliser = (*PptxNormaliser)(nil)

// PptxNormaliser extracts text content from PPTX files using docconv.
type PptxNormaliser struct{}

func (n *PptxNormaliser) Normalise(content string, mimeType string) string {
	if content == "" {
		return ""
	}

	// Extract text from PPTX content
	text, _, err := docconv.ConvertPptx(strings.NewReader(content))
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

func (n *PptxNormaliser) SupportedTypes() []string {
	return []string{"application/vnd.openxmlformats-officedocument.presentationml.presentation"}
}

func (n *PptxNormaliser) Priority() int {
	return 50 // Medium priority - format-specific
}
