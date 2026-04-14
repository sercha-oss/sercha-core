package normalisers

import "strings"

// GitHubPRNormaliser handles GitHub pull request content.
// It preserves Markdown formatting while cleaning up PR-specific artifacts.
type GitHubPRNormaliser struct{}

func (n *GitHubPRNormaliser) Normalise(content string, mimeType string) string {
	// Normalize line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	// Remove HTML comments
	content = removeHTMLComments(content)

	// Remove excessive blank lines
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(content)
}

func (n *GitHubPRNormaliser) SupportedTypes() []string {
	return []string{"application/x-github-pr"}
}

func (n *GitHubPRNormaliser) Priority() int {
	return 90 // High priority - connector-specific
}
