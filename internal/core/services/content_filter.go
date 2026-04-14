package services

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure contentFilterService implements ContentFilter
var _ driven.ContentFilter = (*contentFilterService)(nil)

// contentFilterService implements the ContentFilter port.
// It provides centralized MIME type detection and content filtering logic,
// combining both path pattern matching and MIME type exclusions.
type contentFilterService struct{}

// NewContentFilterService creates a new ContentFilter service.
// This service is stateless and can be shared across components.
func NewContentFilterService() driven.ContentFilter {
	return &contentFilterService{}
}

// ShouldFetchContent determines if content should be fetched for a file.
// It performs two levels of filtering:
//  1. Path pattern matching (glob patterns, folder patterns, exact matches)
//  2. MIME type exclusion matching (wildcard and exact MIME patterns)
//
// Returns false (skip fetching) if:
//   - The path matches any exclusion pattern (e.g., *.png, .git/, node_modules/)
//   - The MIME type matches any exclusion pattern (e.g., image/*, font/*)
//
// Returns true (fetch content) otherwise.
func (s *contentFilterService) ShouldFetchContent(ctx context.Context, path string, settings *domain.SyncExclusionSettings) (shouldFetch bool, mimeType string) {
	// Get MIME type first
	mimeType = s.GetMimeType(path)

	// If no settings, allow everything
	if settings == nil {
		return true, mimeType
	}

	// Check path pattern exclusions
	if settings.HasPatterns() {
		activePatterns := settings.GetActivePatterns()
		if matchesExclusionPattern(path, activePatterns) {
			return false, mimeType
		}
	}

	// Check MIME type exclusions
	if settings.HasMimeExclusions() {
		activeMimeExclusions := settings.GetActiveMimeExclusions()
		if domain.ShouldExcludeMimeType(mimeType, activeMimeExclusions) {
			return false, mimeType
		}
	}

	return true, mimeType
}

// GetMimeType returns the MIME type for a file path based on its extension or filename.
// This is the single source of truth for MIME type detection, replacing the
// adapter-level GuessMimeType function.
//
// Returns:
//   - Matched MIME type for known extensions/filenames
//   - "text/plain" for unrecognized extensions (default fallback)
func (s *contentFilterService) GetMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}

	base := strings.ToLower(filepath.Base(path))
	if mime, ok := extensionlessNames[base]; ok {
		return mime
	}

	return "text/plain"
}

// matchesExclusionPattern checks if a path matches any exclusion pattern.
func matchesExclusionPattern(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPattern(path, pattern) {
			return true
		}
	}
	return false
}

// matchPattern matches a path against a pattern.
// Supports:
//   - Exact matches
//   - Glob patterns (*.txt, *.log)
//   - Folder patterns (.git/, node_modules/)
//   - Prefix matching for folder patterns
//
// This is extracted from sync.go to provide shared path matching logic.
func matchPattern(path, pattern string) bool {
	// Handle folder patterns (ending with /)
	if len(pattern) > 0 && pattern[len(pattern)-1] == '/' {
		// Prefix match for folder patterns
		// Check if path starts with pattern or contains it as a path component
		if len(path) >= len(pattern) && path[:len(pattern)] == pattern {
			return true
		}
		// Check if pattern appears as a path component
		// e.g., pattern ".git/" matches "foo/.git/bar"
		if len(path) > len(pattern) {
			for i := 0; i < len(path)-len(pattern); i++ {
				if path[i] == '/' && path[i+1:i+1+len(pattern)] == pattern {
					return true
				}
			}
		}
		return false
	}

	// Handle exact filename matches (e.g., ".DS_Store", "Thumbs.db")
	// Extract filename from path
	lastSlash := -1
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			lastSlash = i
			break
		}
	}
	filename := path
	if lastSlash >= 0 {
		filename = path[lastSlash+1:]
	}

	// Exact match on filename
	if filename == pattern {
		return true
	}

	// Glob pattern matching (*.txt, *.log, etc)
	if len(pattern) > 2 && pattern[0] == '*' && pattern[1] == '.' {
		// Extract extension from pattern
		ext := pattern[1:] // e.g., ".txt"
		// Check if filename ends with this extension
		if len(filename) >= len(ext) && filename[len(filename)-len(ext):] == ext {
			return true
		}
	}

	return false
}

// mimeTypes maps file extensions to MIME types.
// This is the single source of truth for MIME type mapping in the application.
var mimeTypes = map[string]string{
	// Markup / documentation
	".md":       "text/markdown",
	".markdown": "text/markdown",
	".mdx":      "text/markdown",
	".txt":      "text/plain",
	".text":     "text/plain",
	".rst":      "text/x-rst",
	".adoc":     "text/asciidoc",
	".asciidoc": "text/asciidoc",
	// Web
	".html":   "text/html",
	".htm":    "text/html",
	".css":    "text/css",
	".scss":   "text/x-scss",
	".sass":   "text/x-sass",
	".less":   "text/css",
	".js":     "application/javascript",
	".mjs":    "application/javascript",
	".cjs":    "application/javascript",
	".jsx":    "text/javascript-jsx",
	".ts":     "application/typescript",
	".mts":    "application/typescript",
	".cts":    "application/typescript",
	".tsx":    "text/typescript-jsx",
	".vue":    "text/html",
	".svelte": "text/html",
	// Data
	".json":  "application/json",
	".jsonc": "application/json",
	".yaml":  "text/yaml",
	".yml":   "text/yaml",
	".toml":  "text/x-toml",
	".xml":   "application/xml",
	".xsl":   "application/xml",
	".xslt":  "application/xml",
	".csv":   "text/csv",
	".ini":   "text/plain",
	".cfg":   "text/plain",
	".conf":  "text/plain",
	".env":   "text/plain",
	// Programming languages
	".go":    "text/x-go",
	".py":    "text/x-python",
	".pyi":   "text/x-python",
	".rs":    "text/x-rust",
	".java":  "text/x-java",
	".kt":    "text/x-kotlin",
	".kts":   "text/x-kotlin",
	".scala": "text/x-scala",
	".rb":    "text/x-ruby",
	".c":     "text/x-c",
	".h":     "text/x-c",
	".cpp":   "text/x-c++",
	".cc":    "text/x-c++",
	".cxx":   "text/x-c++",
	".hpp":   "text/x-c++",
	".cs":    "text/x-csharp",
	".swift": "text/x-swift",
	".php":   "text/x-php",
	".lua":   "text/x-lua",
	".r":     "text/x-r",
	".pl":    "text/x-perl",
	".pm":    "text/x-perl",
	".ex":    "text/x-elixir",
	".exs":   "text/x-elixir",
	".erl":   "text/x-erlang",
	".hrl":   "text/x-erlang",
	".hs":    "text/x-haskell",
	".clj":   "text/x-clojure",
	".cljs":  "text/x-clojure",
	".dart":  "text/x-dart",
	".zig":   "text/x-zig",
	// Shell / scripts
	".sh":   "text/x-shellscript",
	".bash": "text/x-shellscript",
	".zsh":  "text/x-shellscript",
	".fish": "text/x-shellscript",
	".ps1":  "text/x-powershell",
	// Build / config
	".dockerfile": "text/x-dockerfile",
	".makefile":   "text/x-makefile",
	".tf":         "text/x-hcl",
	".hcl":        "text/x-hcl",
	".proto":      "text/x-protobuf",
	".graphql":    "text/x-graphql",
	".gql":        "text/x-graphql",
	".sql":        "application/sql",
	// Images
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".ico":  "image/x-icon",
	".bmp":  "image/bmp",
	".tiff": "image/tiff",
	".tif":  "image/tiff",
	".heic": "image/heic",
	".heif": "image/heif",
	".avif": "image/avif",
	// Binary / archives
	".zip":   "application/zip",
	".tar":   "application/x-tar",
	".gz":    "application/gzip",
	".rar":   "application/vnd.rar",
	".7z":    "application/x-7z-compressed",
	".exe":   "application/x-msdownload",
	".dll":   "application/x-msdownload",
	".so":    "application/x-sharedlib",
	".dylib": "application/x-sharedlib",
	".wasm":  "application/wasm",
	// Fonts
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
	".otf":   "font/otf",
	".eot":   "application/vnd.ms-fontobject",
}

// extensionlessNames maps well-known filenames (without extensions) to MIME types.
var extensionlessNames = map[string]string{
	"dockerfile":    "text/x-dockerfile",
	"containerfile": "text/x-dockerfile",
	"makefile":      "text/x-makefile",
	"gnumakefile":   "text/x-makefile",
	"readme":        "text/plain",
	"license":       "text/plain",
	"licence":       "text/plain",
	"copying":       "text/plain",
	".gitignore":    "text/plain",
	".dockerignore": "text/plain",
	".editorconfig": "text/plain",
}
