package localfs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
)

// Ensure Connector implements the interface.
var _ driven.Connector = (*Connector)(nil)

// Connector fetches documents from a local filesystem directory.
type Connector struct {
	rootPath    string  // Full path to directory
	containerID string  // Relative path (for metadata)
	config      *Config
}

// NewConnector creates a LocalFS connector scoped to a directory.
func NewConnector(rootPath, containerID string, config *Config) *Connector {
	if config == nil {
		config = DefaultConfig()
	}
	return &Connector{
		rootPath:    rootPath,
		containerID: containerID,
		config:      config,
	}
}

// Type returns the provider type.
func (c *Connector) Type() domain.ProviderType {
	return domain.ProviderTypeLocalFS
}

// ValidateConfig validates source configuration.
func (c *Connector) ValidateConfig(config domain.SourceConfig) error {
	return nil
}

// FetchChanges walks the directory and returns files as changes.
// Cursor format: RFC3339 timestamp of last sync.
func (c *Connector) FetchChanges(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
	var changes []*domain.Change
	var latestMod time.Time

	// Parse cursor (last sync timestamp)
	var since time.Time
	if cursor != "" {
		parsed, err := time.Parse(time.RFC3339, cursor)
		if err == nil {
			since = parsed
		}
	}

	err := filepath.WalkDir(c.rootPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // Skip inaccessible files
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories
		if d.IsDir() {
			if c.shouldExcludeDir(path) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file should be included
		if !c.shouldIncludeFile(path) {
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return nil
		}

		// Check size limit
		if info.Size() > c.config.MaxFileSize {
			return nil
		}

		// Track latest modification
		modTime := info.ModTime()
		if modTime.After(latestMod) {
			latestMod = modTime
		}

		// Determine change type
		changeType := domain.ChangeTypeAdded
		if !since.IsZero() {
			if modTime.Before(since) || modTime.Equal(since) {
				return nil // No change since last sync
			}
			changeType = domain.ChangeTypeModified
		}

		// Read content
		content, err := c.readFileContent(path)
		if err != nil {
			return nil // Skip unreadable files
		}

		// Generate external ID from path hash
		relPath, _ := filepath.Rel(c.rootPath, path)
		externalID := c.generateExternalID(relPath)

		// Create document
		doc := c.fileToDocument(path, relPath, info)

		changes = append(changes, &domain.Change{
			Type:       changeType,
			ExternalID: externalID,
			Document:   doc,
			Content:    content,
		})

		return nil
	})

	if err != nil && err != context.Canceled {
		return nil, "", fmt.Errorf("walk directory: %w", err)
	}

	// Update cursor
	newCursor := ""
	if !latestMod.IsZero() {
		newCursor = latestMod.Format(time.RFC3339)
	} else if cursor != "" {
		newCursor = cursor // Keep existing cursor if no changes
	}

	return changes, newCursor, nil
}

// FetchDocument fetches a single document by external ID.
func (c *Connector) FetchDocument(ctx context.Context, source *domain.Source, externalID string) (*domain.Document, string, error) {
	// External ID format: "file-<path-hash>"
	// We'd need to store path mapping to resolve this
	return nil, "", fmt.Errorf("single document fetch not implemented for localfs")
}

// TestConnection tests if the directory is accessible.
func (c *Connector) TestConnection(ctx context.Context, source *domain.Source) error {
	info, err := os.Stat(c.rootPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", c.rootPath)
		}
		return fmt.Errorf("access directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", c.rootPath)
	}
	return nil
}

// shouldExcludeDir checks if a directory should be skipped.
func (c *Connector) shouldExcludeDir(path string) bool {
	base := filepath.Base(path)

	// Always exclude hidden directories
	if strings.HasPrefix(base, ".") {
		return true
	}

	for _, exclude := range c.config.ExcludePaths {
		if strings.HasSuffix(exclude, "/") {
			dirPattern := strings.TrimSuffix(exclude, "/")
			if base == dirPattern {
				return true
			}
		}
	}
	return false
}

// shouldIncludeFile checks if a file should be indexed.
func (c *Connector) shouldIncludeFile(path string) bool {
	base := filepath.Base(path)

	// Skip hidden files
	if strings.HasPrefix(base, ".") {
		return false
	}

	// Check excluded patterns
	for _, exclude := range c.config.ExcludePaths {
		if !strings.HasSuffix(exclude, "/") {
			matched, _ := filepath.Match(exclude, base)
			if matched {
				return false
			}
		}
	}

	// Check file extensions if configured
	if len(c.config.FileExtensions) > 0 {
		ext := strings.ToLower(filepath.Ext(path))
		found := false
		for _, allowedExt := range c.config.FileExtensions {
			if ext == "."+allowedExt || ext == allowedExt {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	} else {
		// Default: check against supported extensions
		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" {
			// Check for extensionless files like Makefile, Dockerfile
			supported := false
			for _, s := range SupportedExtensions() {
				if !strings.HasPrefix(s, ".") && strings.EqualFold(base, s) {
					supported = true
					break
				}
			}
			if !supported {
				return false
			}
		} else {
			supported := false
			for _, s := range SupportedExtensions() {
				if strings.HasPrefix(s, ".") && ext == s {
					supported = true
					break
				}
			}
			if !supported {
				return false
			}
		}
	}

	return true
}

// readFileContent reads file content as string.
func (c *Connector) readFileContent(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// generateExternalID creates a stable ID from file path.
func (c *Connector) generateExternalID(relPath string) string {
	hash := sha256.Sum256([]byte(relPath))
	return "file-" + hex.EncodeToString(hash[:8])
}

// fileToDocument converts file info to domain document.
func (c *Connector) fileToDocument(fullPath, relPath string, info os.FileInfo) *domain.Document {
	mimeType := guessMimeType(fullPath)

	metadata := map[string]string{
		"file_path": relPath,
		"size":      fmt.Sprintf("%d", info.Size()),
		"full_path": fullPath,
	}

	if c.containerID != "" {
		metadata["container"] = c.containerID
	}

	return &domain.Document{
		Title:     filepath.Base(fullPath),
		Path:      relPath,
		MimeType:  mimeType,
		Metadata:  metadata,
		CreatedAt: info.ModTime(), // Use mod time as we can't get creation time portably
		UpdatedAt: info.ModTime(),
	}
}

// guessMimeType guesses MIME type from file extension.
func guessMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	mimeTypes := map[string]string{
		".md":         "text/markdown",
		".markdown":   "text/markdown",
		".txt":        "text/plain",
		".text":       "text/plain",
		".go":         "text/x-go",
		".py":         "text/x-python",
		".js":         "application/javascript",
		".jsx":        "application/javascript",
		".mjs":        "application/javascript",
		".ts":         "application/typescript",
		".tsx":        "application/typescript",
		".json":       "application/json",
		".yaml":       "text/yaml",
		".yml":        "text/yaml",
		".toml":       "text/x-toml",
		".html":       "text/html",
		".htm":        "text/html",
		".css":        "text/css",
		".scss":       "text/x-scss",
		".sass":       "text/x-sass",
		".rs":         "text/x-rust",
		".java":       "text/x-java",
		".rb":         "text/x-ruby",
		".sh":         "text/x-shellscript",
		".bash":       "text/x-shellscript",
		".zsh":        "text/x-shellscript",
		".sql":        "application/sql",
		".xml":        "application/xml",
		".c":          "text/x-c",
		".h":          "text/x-c",
		".cpp":        "text/x-c++",
		".hpp":        "text/x-c++",
		".cc":         "text/x-c++",
		".cs":         "text/x-csharp",
		".swift":      "text/x-swift",
		".kt":         "text/x-kotlin",
		".kts":        "text/x-kotlin",
		".scala":      "text/x-scala",
		".php":        "text/x-php",
		".lua":        "text/x-lua",
		".r":          "text/x-r",
		".dockerfile": "text/x-dockerfile",
		".makefile":   "text/x-makefile",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}

	// Check extensionless files
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "dockerfile":
		return "text/x-dockerfile"
	case "makefile":
		return "text/x-makefile"
	case "readme":
		return "text/plain"
	}

	return "text/plain"
}
