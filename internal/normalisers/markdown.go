package normalisers

import "strings"

// MarkdownNormaliser handles Markdown content.
type MarkdownNormaliser struct{}

func (n *MarkdownNormaliser) Normalise(content string, mimeType string) string {
	// Basic Markdown cleanup
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	// Remove excessive blank lines (more than 2 consecutive)
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(content)
}

func (n *MarkdownNormaliser) SupportedTypes() []string {
	return []string{"text/markdown", "text/x-markdown"}
}

func (n *MarkdownNormaliser) Priority() int {
	return 50 // Medium priority - format-specific
}
