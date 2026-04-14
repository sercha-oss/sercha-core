package domain

import "strings"

// MatchesMimePattern checks if a MIME type matches a pattern.
// Supports both exact matches (e.g., "image/png") and wildcard matches (e.g., "image/*").
// The pattern is case-insensitive.
//
// Examples:
//   - MatchesMimePattern("image/png", "image/*") returns true
//   - MatchesMimePattern("image/png", "image/png") returns true
//   - MatchesMimePattern("image/png", "text/*") returns false
//   - MatchesMimePattern("text/plain", "text/plain") returns true
//   - MatchesMimePattern("application/json", "application/*") returns true
func MatchesMimePattern(mimeType string, pattern string) bool {
	if mimeType == "" || pattern == "" {
		return false
	}

	// Normalize to lowercase for case-insensitive comparison
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	pattern = strings.ToLower(strings.TrimSpace(pattern))

	// Exact match
	if mimeType == pattern {
		return true
	}

	// Wildcard match (e.g., "image/*" matches "image/png")
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(mimeType, prefix+"/")
	}

	return false
}

// ShouldExcludeMimeType checks if a MIME type should be excluded based on a list of patterns.
// Returns true if the MIME type matches any of the exclusion patterns.
func ShouldExcludeMimeType(mimeType string, exclusionPatterns []string) bool {
	if mimeType == "" || len(exclusionPatterns) == 0 {
		return false
	}

	for _, pattern := range exclusionPatterns {
		if MatchesMimePattern(mimeType, pattern) {
			return true
		}
	}

	return false
}
