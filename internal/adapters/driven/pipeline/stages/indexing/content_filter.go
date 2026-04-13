package indexing

// isLikelyNonText detects binary or encoded content that shouldn't be indexed as text.
// It samples up to the first 512 characters and checks for:
//   - High ratio of non-printable characters (>5%) indicating binary data
//   - Very low whitespace ratio (<2%) indicating compact encoded data like base64
func isLikelyNonText(content string) bool {
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

	// Binary: >5% non-printable characters
	if float64(nonPrintable)/float64(total) > 0.05 {
		return true
	}

	// Compact encoded data (base64): <2% whitespace
	if float64(whitespace)/float64(total) < 0.02 {
		return true
	}

	return false
}
