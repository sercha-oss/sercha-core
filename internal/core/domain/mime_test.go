package domain

import "testing"

func TestMatchesMimePattern(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		pattern  string
		expected bool
	}{
		// Exact matches
		{
			name:     "exact match - image/png",
			mimeType: "image/png",
			pattern:  "image/png",
			expected: true,
		},
		{
			name:     "exact match - text/plain",
			mimeType: "text/plain",
			pattern:  "text/plain",
			expected: true,
		},
		{
			name:     "exact match - application/json",
			mimeType: "application/json",
			pattern:  "application/json",
			expected: true,
		},

		// Wildcard matches
		{
			name:     "wildcard match - image/*",
			mimeType: "image/png",
			pattern:  "image/*",
			expected: true,
		},
		{
			name:     "wildcard match - image/jpeg",
			mimeType: "image/jpeg",
			pattern:  "image/*",
			expected: true,
		},
		{
			name:     "wildcard match - text/*",
			mimeType: "text/plain",
			pattern:  "text/*",
			expected: true,
		},
		{
			name:     "wildcard match - application/*",
			mimeType: "application/json",
			pattern:  "application/*",
			expected: true,
		},
		{
			name:     "wildcard match - font/*",
			mimeType: "font/woff2",
			pattern:  "font/*",
			expected: true,
		},
		{
			name:     "wildcard match - audio/*",
			mimeType: "audio/mpeg",
			pattern:  "audio/*",
			expected: true,
		},
		{
			name:     "wildcard match - video/*",
			mimeType: "video/mp4",
			pattern:  "video/*",
			expected: true,
		},

		// Non-matches
		{
			name:     "different type",
			mimeType: "image/png",
			pattern:  "text/plain",
			expected: false,
		},
		{
			name:     "different wildcard",
			mimeType: "image/png",
			pattern:  "text/*",
			expected: false,
		},
		{
			name:     "different category",
			mimeType: "application/json",
			pattern:  "text/*",
			expected: false,
		},

		// Case insensitivity
		{
			name:     "uppercase MIME type",
			mimeType: "IMAGE/PNG",
			pattern:  "image/png",
			expected: true,
		},
		{
			name:     "uppercase pattern",
			mimeType: "image/png",
			pattern:  "IMAGE/PNG",
			expected: true,
		},
		{
			name:     "mixed case wildcard",
			mimeType: "IMAGE/PNG",
			pattern:  "image/*",
			expected: true,
		},
		{
			name:     "uppercase wildcard pattern",
			mimeType: "image/png",
			pattern:  "IMAGE/*",
			expected: true,
		},

		// Edge cases
		{
			name:     "empty MIME type",
			mimeType: "",
			pattern:  "image/*",
			expected: false,
		},
		{
			name:     "empty pattern",
			mimeType: "image/png",
			pattern:  "",
			expected: false,
		},
		{
			name:     "both empty",
			mimeType: "",
			pattern:  "",
			expected: false,
		},
		{
			name:     "whitespace in MIME type",
			mimeType: " image/png ",
			pattern:  "image/png",
			expected: true,
		},
		{
			name:     "whitespace in pattern",
			mimeType: "image/png",
			pattern:  " image/png ",
			expected: true,
		},
		{
			name:     "wildcard only match",
			mimeType: "image/png",
			pattern:  "image/*",
			expected: true,
		},

		// Special MIME types
		{
			name:     "svg+xml exact",
			mimeType: "image/svg+xml",
			pattern:  "image/svg+xml",
			expected: true,
		},
		{
			name:     "svg+xml wildcard",
			mimeType: "image/svg+xml",
			pattern:  "image/*",
			expected: true,
		},
		{
			name:     "x-icon exact",
			mimeType: "image/x-icon",
			pattern:  "image/x-icon",
			expected: true,
		},
		{
			name:     "x-icon wildcard",
			mimeType: "image/x-icon",
			pattern:  "image/*",
			expected: true,
		},

		// Invalid wildcards (should not match)
		{
			name:     "pattern without slash before wildcard",
			mimeType: "image/png",
			pattern:  "image*",
			expected: false,
		},
		{
			name:     "wildcard in middle",
			mimeType: "image/png",
			pattern:  "*/png",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchesMimePattern(tt.mimeType, tt.pattern)
			if result != tt.expected {
				t.Errorf("MatchesMimePattern(%q, %q) = %v, expected %v",
					tt.mimeType, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestShouldExcludeMimeType(t *testing.T) {
	tests := []struct {
		name               string
		mimeType           string
		exclusionPatterns  []string
		expected           bool
	}{
		{
			name:     "matches first pattern",
			mimeType: "image/png",
			exclusionPatterns: []string{
				"image/*",
				"font/*",
			},
			expected: true,
		},
		{
			name:     "matches second pattern",
			mimeType: "font/woff2",
			exclusionPatterns: []string{
				"image/*",
				"font/*",
			},
			expected: true,
		},
		{
			name:     "matches exact pattern in list",
			mimeType: "application/zip",
			exclusionPatterns: []string{
				"image/*",
				"application/zip",
				"font/*",
			},
			expected: true,
		},
		{
			name:     "no match",
			mimeType: "text/plain",
			exclusionPatterns: []string{
				"image/*",
				"font/*",
				"audio/*",
			},
			expected: false,
		},
		{
			name:     "empty patterns list",
			mimeType: "image/png",
			exclusionPatterns: []string{},
			expected: false,
		},
		{
			name:     "nil patterns list",
			mimeType: "image/png",
			exclusionPatterns: nil,
			expected: false,
		},
		{
			name:     "empty MIME type",
			mimeType: "",
			exclusionPatterns: []string{
				"image/*",
			},
			expected: false,
		},
		{
			name:     "matches wildcard in complex list",
			mimeType: "audio/mpeg",
			exclusionPatterns: []string{
				"image/*",
				"font/*",
				"audio/*",
				"video/*",
				"application/zip",
			},
			expected: true,
		},
		{
			name:     "matches exact in complex list",
			mimeType: "application/zip",
			exclusionPatterns: []string{
				"image/*",
				"font/*",
				"application/zip",
				"application/gzip",
			},
			expected: true,
		},
		{
			name:     "no match in complex list",
			mimeType: "text/markdown",
			exclusionPatterns: []string{
				"image/*",
				"font/*",
				"audio/*",
				"video/*",
				"application/zip",
			},
			expected: false,
		},
		{
			name:     "case insensitive exclusion",
			mimeType: "IMAGE/PNG",
			exclusionPatterns: []string{
				"image/*",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldExcludeMimeType(tt.mimeType, tt.exclusionPatterns)
			if result != tt.expected {
				t.Errorf("ShouldExcludeMimeType(%q, %v) = %v, expected %v",
					tt.mimeType, tt.exclusionPatterns, result, tt.expected)
			}
		})
	}
}
