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

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/connectors"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure Connector implements the interface.
var _ driven.Connector = (*Connector)(nil)

// Connector fetches documents from a local filesystem directory.
type Connector struct {
	rootPath    string // Full path to directory
	containerID string // Relative path (for metadata)
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

// unsupportedRESTClient is the RESTClient implementation returned by
// connectors with no HTTP surface. Every Do call returns ErrRESTUnsupported
// so callers can detect the capability gap without nil-checking.
type unsupportedRESTClient struct{}

func (unsupportedRESTClient) Do(_ context.Context, _, _ string, _, _ any) error {
	return driven.ErrRESTUnsupported
}

// RESTClient implements driven.Connector. The local filesystem connector has
// no REST surface; callers receive a sentinel client whose Do always returns
// driven.ErrRESTUnsupported.
func (c *Connector) RESTClient() driven.RESTClient {
	return unsupportedRESTClient{}
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
// Since external IDs are SHA256 hashes of relative paths, we walk the directory
// to find the file whose path matches the given ID.
func (c *Connector) FetchDocument(ctx context.Context, source *domain.Source, externalID string) (*domain.Document, string, error) {
	// Validate external ID format
	if !strings.HasPrefix(externalID, "file-") {
		return nil, "", fmt.Errorf("invalid external ID format: %s", externalID)
	}

	var foundDoc *domain.Document
	var foundContent string

	err := filepath.WalkDir(c.rootPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // Skip inaccessible files
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() {
			if c.shouldExcludeDir(path) {
				return filepath.SkipDir
			}
			return nil
		}

		if !c.shouldIncludeFile(path) {
			return nil
		}

		relPath, err := filepath.Rel(c.rootPath, path)
		if err != nil {
			return nil
		}

		// Check if this file's external ID matches
		if c.generateExternalID(relPath) != externalID {
			return nil
		}

		// Found the file - read it
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("get file info: %w", err)
		}

		content, err := c.readFileContent(path)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		foundDoc = c.fileToDocument(path, relPath, info)
		foundContent = content

		// Use a sentinel error to stop walking
		return filepath.SkipAll
	})

	if err != nil && err != context.Canceled {
		return nil, "", fmt.Errorf("walk directory: %w", err)
	}

	if foundDoc == nil {
		return nil, "", fmt.Errorf("document not found: %s", externalID)
	}

	// Compute content hash for change detection
	contentHash := computeContentHash(foundContent)
	return foundDoc, contentHash, nil
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
	defer func() { _ = file.Close() }()

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
	mimeType := connectors.GuessMimeType(fullPath)

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

// computeContentHash computes a SHA256 hash of content for change detection.
func computeContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// ReconciliationScopes declares which canonical-ID prefixes this connector
// snapshot-enumerates for delete detection. Local filesystem walks would
// naturally support this (a directory walk is a snapshot), but the
// implementation lands in a follow-up; returning nil here keeps the
// orchestrator's phase-1 loop a no-op for localfs until then.
func (c *Connector) ReconciliationScopes() []string {
	return nil
}

// Inventory is a stub until the directory-walk implementation lands.
func (c *Connector) Inventory(ctx context.Context, source *domain.Source, scope string) ([]string, error) {
	return nil, driven.ErrInventoryNotSupported
}
