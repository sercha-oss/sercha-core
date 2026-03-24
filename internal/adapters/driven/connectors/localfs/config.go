package localfs

// Config contains configuration for the local filesystem connector.
type Config struct {
	// FileExtensions is a list of file extensions to index (without dot).
	// Empty means all text-based files.
	FileExtensions []string

	// ExcludePaths is a list of path patterns to exclude.
	// Patterns ending with "/" match directories.
	ExcludePaths []string

	// MaxFileSize is the maximum file size in bytes to fetch.
	MaxFileSize int64

	// FollowSymlinks determines whether to follow symbolic links.
	FollowSymlinks bool
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		FileExtensions: []string{}, // Empty means all supported text files
		ExcludePaths: []string{
			".git/",
			"node_modules/",
			"vendor/",
			"__pycache__/",
			".venv/",
			"venv/",
			".idea/",
			".vscode/",
			"*.min.js",
			"*.min.css",
			"*.lock",
			"package-lock.json",
			"yarn.lock",
			"go.sum",
		},
		MaxFileSize:    1 << 20, // 1MB
		FollowSymlinks: false,
	}
}

// SupportedExtensions returns the list of file extensions that are
// considered indexable text files.
func SupportedExtensions() []string {
	return []string{
		".md", ".markdown",
		".txt", ".text",
		".go",
		".py",
		".js", ".jsx", ".mjs",
		".ts", ".tsx",
		".json",
		".yaml", ".yml",
		".toml",
		".html", ".htm",
		".css", ".scss", ".sass",
		".rs",
		".java",
		".rb",
		".sh", ".bash", ".zsh",
		".sql",
		".xml",
		".c", ".h",
		".cpp", ".hpp", ".cc",
		".cs",
		".swift",
		".kt", ".kts",
		".scala",
		".php",
		".lua",
		".r", ".R",
		".dockerfile", "Dockerfile",
		".makefile", "Makefile",
		".gitignore",
		".env.example",
	}
}
