// Package textfilter provides shared heuristics for detecting non-text content
// (binary blobs, base64, compressed payloads) in indexable documents.
//
// The same logic is used by the indexing pipeline (to skip chunking/embedding
// non-text bodies) and the search presenter (to skip them from snippets).
package textfilter

import "strings"

// IsLikelyNonText detects binary or encoded content that shouldn't be indexed
// or surfaced as a snippet. It samples up to the first 512 characters and
// flags content with:
//   - >5% non-printable characters (binary)
//   - <2% whitespace (compact encoded data like base64 or compressed payloads)
//
// Callers that already know the MIME type should prefer IsLikelyNonTextWithMime,
// which skips the whitespace check for known structured-text formats (minified
// JSON/JS/CSS would otherwise false-positive).
func IsLikelyNonText(content string) bool {
	return likelyNonText(content, false)
}

// IsLikelyNonTextWithMime is the MIME-aware variant. For known structured-text
// MIME types (JSON/JS/CSS/XML and their +json/+xml siblings) it skips the
// whitespace ratio check — those formats legitimately have near-zero whitespace
// when minified. The binary-control-character check still runs, so a corrupted
// payload with a misleading Content-Type still gets filtered.
//
// Free-form text/* formats (markdown, html, plain) go through the full heuristic
// because they can carry embedded base64/zlib payloads that must be filtered.
func IsLikelyNonTextWithMime(content, mimeType string) bool {
	return likelyNonText(content, isKnownStructuredTextMime(mimeType))
}

func likelyNonText(content string, skipWhitespaceCheck bool) bool {
	if len(content) < 64 {
		return false
	}

	sample := content
	if len(sample) > 512 {
		sample = sample[:512]
	}

	var whitespace, nonPrintable int
	for _, r := range sample {
		switch {
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			whitespace++
		case r < 32 || r == 0x7f:
			nonPrintable++
		}
	}

	total := len([]rune(sample))
	if total == 0 {
		return false
	}

	if float64(nonPrintable)/float64(total) > 0.05 {
		return true
	}
	if !skipWhitespaceCheck && float64(whitespace)/float64(total) < 0.02 {
		return true
	}
	return false
}

func isKnownStructuredTextMime(mimeType string) bool {
	if mimeType == "" {
		return false
	}
	mt := strings.ToLower(mimeType)
	if i := strings.Index(mt, ";"); i >= 0 {
		mt = strings.TrimSpace(mt[:i])
	}
	switch mt {
	case "application/json",
		"text/json",
		"application/javascript",
		"application/x-javascript",
		"text/javascript",
		"text/css",
		"application/xml",
		"text/xml":
		return true
	}
	return strings.HasSuffix(mt, "+json") || strings.HasSuffix(mt, "+xml")
}
