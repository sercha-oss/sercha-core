package normalisers

import "strings"

// PlaintextNormaliser handles plain text content.
type PlaintextNormaliser struct{}

func (n *PlaintextNormaliser) Normalise(content string, mimeType string) string {
	// Basic cleanup: normalize line endings, trim whitespace
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	return strings.TrimSpace(content)
}

func (n *PlaintextNormaliser) SupportedTypes() []string {
	return []string{"text/plain", "*/*"} // Fallback for any type
}

func (n *PlaintextNormaliser) Priority() int {
	return 1 // Lowest priority - fallback
}
