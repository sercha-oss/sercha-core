package normalisers

import "strings"

// HTMLNormaliser handles HTML content.
type HTMLNormaliser struct{}

func (n *HTMLNormaliser) Normalise(content string, mimeType string) string {
	// Basic HTML text extraction
	// This is a simple implementation - production would use a proper HTML parser

	// Remove script and style blocks
	content = removeHTMLBlocks(content, "script")
	content = removeHTMLBlocks(content, "style")

	// Remove HTML tags (simple approach)
	content = stripHTMLTags(content)

	// Decode common HTML entities
	content = decodeHTMLEntities(content)

	// Clean up whitespace
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	// Collapse multiple spaces
	for strings.Contains(content, "  ") {
		content = strings.ReplaceAll(content, "  ", " ")
	}

	// Remove excessive blank lines
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(content)
}

func (n *HTMLNormaliser) SupportedTypes() []string {
	return []string{"text/html", "application/xhtml+xml"}
}

func (n *HTMLNormaliser) Priority() int {
	return 50 // Medium priority - format-specific
}

// Helper functions for HTML processing

func removeHTMLBlocks(content, tagName string) string {
	result := content

	for {
		startTag := "<" + strings.ToLower(tagName)
		endTag := "</" + strings.ToLower(tagName) + ">"

		startIdx := strings.Index(strings.ToLower(result), startTag)
		if startIdx == -1 {
			break
		}

		endIdx := strings.Index(strings.ToLower(result[startIdx:]), endTag)
		if endIdx == -1 {
			break
		}

		result = result[:startIdx] + result[startIdx+endIdx+len(endTag):]
	}

	return result
}

func stripHTMLTags(content string) string {
	var result strings.Builder
	inTag := false

	for _, r := range content {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			result.WriteRune(' ') // Replace tag with space
		case !inTag:
			result.WriteRune(r)
		}
	}

	return result.String()
}

func decodeHTMLEntities(content string) string {
	// Common HTML entities
	replacements := map[string]string{
		"&nbsp;":   " ",
		"&amp;":    "&",
		"&lt;":     "<",
		"&gt;":     ">",
		"&quot;":   "\"",
		"&apos;":   "'",
		"&#39;":    "'",
		"&mdash;":  "—",
		"&ndash;":  "–",
		"&hellip;": "...",
		"&copy;":   "©",
		"&reg;":    "®",
		"&trade;":  "™",
	}

	for entity, replacement := range replacements {
		content = strings.ReplaceAll(content, entity, replacement)
	}

	return content
}
