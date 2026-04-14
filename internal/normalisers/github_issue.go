package normalisers

import "strings"

// GitHubIssueNormaliser handles GitHub issue content.
// It preserves Markdown formatting while cleaning up issue-specific artifacts.
type GitHubIssueNormaliser struct{}

func (n *GitHubIssueNormaliser) Normalise(content string, mimeType string) string {
	// Normalize line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	// Remove GitHub-specific artifacts

	// Remove "<!-- ... -->" HTML comments (task lists, etc.)
	content = removeHTMLComments(content)

	// Clean up checkbox markers for cleaner text
	content = strings.ReplaceAll(content, "- [ ]", "- [ ]")
	content = strings.ReplaceAll(content, "- [x]", "- [x]")
	content = strings.ReplaceAll(content, "- [X]", "- [x]")

	// Remove excessive blank lines
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(content)
}

func (n *GitHubIssueNormaliser) SupportedTypes() []string {
	return []string{"application/x-github-issue"}
}

func (n *GitHubIssueNormaliser) Priority() int {
	return 90 // High priority - connector-specific
}

// removeHTMLComments removes HTML comments from content.
func removeHTMLComments(content string) string {
	result := content

	for {
		startIdx := strings.Index(result, "<!--")
		if startIdx == -1 {
			break
		}

		endIdx := strings.Index(result[startIdx:], "-->")
		if endIdx == -1 {
			break
		}

		result = result[:startIdx] + result[startIdx+endIdx+3:]
	}

	return result
}
