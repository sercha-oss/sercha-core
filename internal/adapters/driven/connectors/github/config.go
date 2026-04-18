package github

// Config contains configuration for the GitHub connector.
type Config struct {
	// APIBaseURL is the base URL for GitHub API.
	// Defaults to https://api.github.com for github.com.
	// For GitHub Enterprise, use https://<hostname>/api/v3
	APIBaseURL string

	// PerPage is the number of items to fetch per page.
	// Maximum is 100.
	PerPage int

	// MaxRetries is the maximum number of retry attempts for rate-limited requests.
	MaxRetries int

	// IncludeFiles enables indexing of repository files.
	IncludeFiles bool

	// IncludeIssues enables indexing of issues.
	IncludeIssues bool

	// IncludePRs enables indexing of pull requests.
	IncludePRs bool

	// IncludeDiscussions enables indexing of discussions.
	IncludeDiscussions bool

	// IncludeWiki enables indexing of wiki pages.
	IncludeWiki bool

	// FileExtensions is a list of file extensions to index.
	// Empty means all text-based files.
	FileExtensions []string

	// ExcludePaths is a list of path patterns to exclude.
	ExcludePaths []string

	// MaxFileSize is the maximum file size in bytes to fetch.
	// Default is 1MB.
	MaxFileSize int64

	// Concurrency is the number of concurrent file content fetches.
	// Default is 10.
	Concurrency int
}

// DefaultConfig returns the default GitHub connector configuration.
func DefaultConfig() *Config {
	return &Config{
		APIBaseURL:         "https://api.github.com",
		PerPage:            100,
		MaxRetries:         3,
		IncludeFiles:       true,
		IncludeIssues:      true,
		IncludePRs:         true,
		IncludeDiscussions: false,      // Requires GraphQL API
		IncludeWiki:        false,      // Wiki has separate API
		FileExtensions:     []string{}, // All text files
		ExcludePaths: []string{
			"vendor/",
			"node_modules/",
			".git/",
			"*.min.js",
			"*.min.css",
			"package-lock.json",
			"yarn.lock",
		},
		MaxFileSize: 1 << 20, // 1MB
		Concurrency: 10,
	}
}

// OAuthConfig contains OAuth configuration for GitHub.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
}
