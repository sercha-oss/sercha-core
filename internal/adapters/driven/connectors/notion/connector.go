package notion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure Connector implements the interface.
var _ driven.Connector = (*Connector)(nil)

// Connector fetches documents from Notion pages and databases.
type Connector struct {
	tokenProvider driven.TokenProvider
	containerID   string // Page or database ID (optional)
	client        *Client
	config        *Config
}

// NewConnector creates a Notion connector.
// containerID can be empty to index all accessible content, or a specific page/database ID.
func NewConnector(tokenProvider driven.TokenProvider, containerID string, config *Config) *Connector {
	if config == nil {
		config = DefaultConfig()
	}
	return &Connector{
		tokenProvider: tokenProvider,
		containerID:   containerID,
		client:        NewClient(tokenProvider, config),
		config:        config,
	}
}

// Type returns the provider type.
func (c *Connector) Type() domain.ProviderType {
	return domain.ProviderTypeNotion
}

// ValidateConfig validates source configuration.
func (c *Connector) ValidateConfig(config domain.SourceConfig) error {
	// No special validation needed for Notion
	return nil
}

// FetchChanges fetches document changes from Notion.
// For initial sync (empty cursor), it fetches all content.
// For incremental sync, it fetches changes since the cursor timestamp.
func (c *Connector) FetchChanges(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
	var changes []*domain.Change
	var lastModified time.Time

	// Parse cursor to get since timestamp
	var filter *SearchFilter
	if cursor != "" {
		parsed, err := time.Parse(time.RFC3339, cursor)
		if err == nil {
			// Filter by last_edited_time
			filter = &SearchFilter{
				Property: "last_edited_time",
				Value: map[string]interface{}{
					"after": parsed.Format(time.RFC3339),
				},
			}
		}
	}

	// Parse cursor timestamp for incremental sync check
	var sinceTime time.Time
	if cursor != "" {
		if parsed, err := time.Parse(time.RFC3339, cursor); err == nil {
			sinceTime = parsed
		}
	}

	// If containerID is specified, fetch only that page/database
	if c.containerID != "" {
		// First check if the page/database was modified since last sync
		page, err := c.client.GetPage(ctx, c.containerID)
		if err == nil {
			// It's a page
			if !sinceTime.IsZero() && !page.LastEditedTime.After(sinceTime) {
				// Page hasn't changed since last sync, return empty
				return nil, cursor, nil
			}

			// Get page content
			blocks, err := c.client.GetBlocksRecursive(ctx, c.containerID, c.config.MaxBlockDepth)
			if err != nil {
				return nil, "", fmt.Errorf("get blocks for page %s: %w", c.containerID, err)
			}
			content := c.formatPageContent(page, blocks)

			// If the page belongs to a database, treat it as a database entry
			// to maintain consistent document types and avoid duplicates
			var doc *domain.Document
			var externalID string
			if page.Parent.Type == "database_id" {
				var db *Database
				if page.Parent.DatabaseID != "" {
					db, _ = c.client.GetDatabase(ctx, page.Parent.DatabaseID)
				}
				doc = c.databaseEntryToDocument(page, db)
				externalID = fmt.Sprintf("database-entry-%s", page.ID)
			} else {
				doc = c.pageToDocument(page)
				externalID = fmt.Sprintf("page-%s", page.ID)
			}

			change := &domain.Change{
				Type:       domain.ChangeTypeModified,
				ExternalID: externalID,
				Document:   doc,
				Content:    content,
			}
			changes = append(changes, change)
			lastModified = page.LastEditedTime
		} else {
			// Try as database
			db, err := c.client.GetDatabase(ctx, c.containerID)
			if err != nil {
				return nil, "", fmt.Errorf("fetch container %s: not a page or database", c.containerID)
			}
			if !sinceTime.IsZero() && !db.LastEditedTime.After(sinceTime) {
				// Database hasn't changed since last sync, return empty
				return nil, cursor, nil
			}
			dbChanges, err := c.fetchDatabaseChanges(ctx, c.containerID)
			if err != nil {
				return nil, "", fmt.Errorf("fetch database %s: %w", c.containerID, err)
			}
			changes = append(changes, dbChanges...)
			lastModified = db.LastEditedTime
		}
	} else {
		// Search all accessible pages and databases
		searchCursor := ""
		for {
			resp, err := c.client.Search(ctx, filter, searchCursor)
			if err != nil {
				return nil, "", fmt.Errorf("search: %w", err)
			}

			for _, result := range resp.Results {
				if result.Archived {
					// Skip archived items
					continue
				}

				var itemChanges []*domain.Change
				var err error

				switch result.Object {
				case "page":
					// Skip pages that belong to a database - they'll be indexed
					// when we process the database to avoid duplicates
					if result.Parent.Type == "database_id" {
						continue
					}
					itemChanges, err = c.fetchPageChanges(ctx, result.ID)
				case "database":
					itemChanges, err = c.fetchDatabaseChanges(ctx, result.ID)
				}

				if err != nil {
					// Log error but continue with other items
					continue
				}

				changes = append(changes, itemChanges...)

				// Track latest modification time
				if result.LastEditedTime.After(lastModified) {
					lastModified = result.LastEditedTime
				}
			}

			if !resp.HasMore {
				break
			}
			searchCursor = resp.NextCursor
		}
	}

	// Update cursor to the latest modified time
	newCursor := ""
	if !lastModified.IsZero() {
		newCursor = lastModified.Format(time.RFC3339)
	} else if len(changes) > 0 {
		newCursor = time.Now().Format(time.RFC3339)
	}

	return changes, newCursor, nil
}

// fetchPageChanges fetches a page and its content as changes.
func (c *Connector) fetchPageChanges(ctx context.Context, pageID string) ([]*domain.Change, error) {
	page, err := c.client.GetPage(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("get page: %w", err)
	}

	// Get page content (blocks)
	blocks, err := c.client.GetBlocksRecursive(ctx, pageID, c.config.MaxBlockDepth)
	if err != nil {
		return nil, fmt.Errorf("get blocks: %w", err)
	}

	content := c.formatPageContent(page, blocks)
	doc := c.pageToDocument(page)

	change := &domain.Change{
		Type:       domain.ChangeTypeModified,
		ExternalID: fmt.Sprintf("page-%s", page.ID),
		Document:   doc,
		Content:    content,
	}

	return []*domain.Change{change}, nil
}

// fetchDatabaseChanges fetches a database and its entries as changes.
func (c *Connector) fetchDatabaseChanges(ctx context.Context, databaseID string) ([]*domain.Change, error) {
	db, err := c.client.GetDatabase(ctx, databaseID)
	if err != nil {
		return nil, fmt.Errorf("get database: %w", err)
	}

	var changes []*domain.Change

	// Query database entries (pages)
	queryCursor := ""
	for {
		resp, err := c.client.QueryDatabase(ctx, databaseID, nil, queryCursor)
		if err != nil {
			return nil, fmt.Errorf("query database: %w", err)
		}

		for _, page := range resp.Results {
			if page.Archived {
				continue
			}

			// Get page content
			blocks, err := c.client.GetBlocksRecursive(ctx, page.ID, c.config.MaxBlockDepth)
			if err != nil {
				// Log error but continue
				continue
			}

			content := c.formatPageContent(&page, blocks)
			doc := c.databaseEntryToDocument(&page, db)

			change := &domain.Change{
				Type:       domain.ChangeTypeModified,
				ExternalID: fmt.Sprintf("database-entry-%s", page.ID),
				Document:   doc,
				Content:    content,
			}

			changes = append(changes, change)
		}

		if !resp.HasMore {
			break
		}
		queryCursor = resp.NextCursor
	}

	return changes, nil
}

// FetchDocument fetches a single document by external ID.
func (c *Connector) FetchDocument(ctx context.Context, source *domain.Source, externalID string) (*domain.Document, string, error) {
	// Parse external ID format: "page-{uuid}" or "database-entry-{uuid}"
	var docType, id string
	if strings.HasPrefix(externalID, "database-entry-") {
		docType = "database-entry"
		id = strings.TrimPrefix(externalID, "database-entry-")
	} else if strings.HasPrefix(externalID, "page-") {
		docType = "page"
		id = strings.TrimPrefix(externalID, "page-")
	} else {
		return nil, "", fmt.Errorf("invalid external ID format: %s", externalID)
	}

	switch docType {
	case "page":
		page, err := c.client.GetPage(ctx, id)
		if err != nil {
			return nil, "", fmt.Errorf("fetch page: %w", err)
		}

		blocks, err := c.client.GetBlocksRecursive(ctx, id, c.config.MaxBlockDepth)
		if err != nil {
			return nil, "", fmt.Errorf("get blocks: %w", err)
		}

		content := c.formatPageContent(page, blocks)
		doc := c.pageToDocument(page)
		contentHash := computeContentHash(content)

		return doc, contentHash, nil

	case "database-entry":
		page, err := c.client.GetPage(ctx, id)
		if err != nil {
			return nil, "", fmt.Errorf("fetch database entry: %w", err)
		}

		// Get parent database
		var db *Database
		if page.Parent.Type == "database_id" && page.Parent.DatabaseID != "" {
			db, err = c.client.GetDatabase(ctx, page.Parent.DatabaseID)
			if err != nil {
				// Continue without database info
				db = nil
			}
		}

		blocks, err := c.client.GetBlocksRecursive(ctx, id, c.config.MaxBlockDepth)
		if err != nil {
			return nil, "", fmt.Errorf("get blocks: %w", err)
		}

		content := c.formatPageContent(page, blocks)
		doc := c.databaseEntryToDocument(page, db)
		contentHash := computeContentHash(content)

		return doc, contentHash, nil

	default:
		return nil, "", fmt.Errorf("unknown document type: %s", docType)
	}
}

// TestConnection tests the connection to Notion.
func (c *Connector) TestConnection(ctx context.Context, source *domain.Source) error {
	_, err := c.client.GetUser(ctx)
	return err
}

// pageToDocument converts a Notion page to a domain document.
func (c *Connector) pageToDocument(page *Page) *domain.Document {
	title := GetPageTitle(page.Properties)

	metadata := map[string]string{
		"page_id": page.ID,
		"type":    "page",
	}

	// Add parent info
	switch page.Parent.Type {
	case "workspace":
		metadata["parent"] = "workspace"
	case "page_id":
		metadata["parent"] = "page"
		metadata["parent_id"] = page.Parent.PageID
	case "database_id":
		metadata["parent"] = "database"
		metadata["parent_id"] = page.Parent.DatabaseID
	}

	return &domain.Document{
		Title:     title,
		Path:      page.URL,
		MimeType:  "application/x-notion-page",
		Metadata:  metadata,
		CreatedAt: page.CreatedTime,
		UpdatedAt: page.LastEditedTime,
	}
}

// databaseEntryToDocument converts a database entry (page) to a domain document.
func (c *Connector) databaseEntryToDocument(page *Page, db *Database) *domain.Document {
	title := GetPageTitle(page.Properties)

	metadata := map[string]string{
		"page_id": page.ID,
		"type":    "database_entry",
	}

	if db != nil {
		metadata["database_id"] = db.ID
		metadata["database_name"] = GetDatabaseTitle(db)
	}

	// Add property values to metadata
	for propName, prop := range page.Properties {
		if prop.Type == "title" {
			continue // Already in title
		}
		metadata[fmt.Sprintf("prop_%s", propName)] = c.formatPropertyValue(prop)
	}

	return &domain.Document{
		Title:     title,
		Path:      page.URL,
		MimeType:  "application/x-notion-database-entry",
		Metadata:  metadata,
		CreatedAt: page.CreatedTime,
		UpdatedAt: page.LastEditedTime,
	}
}

// formatPageContent formats page content for indexing.
func (c *Connector) formatPageContent(page *Page, blocks []Block) string {
	var sb strings.Builder

	// Add title
	title := GetPageTitle(page.Properties)
	sb.WriteString("# ")
	sb.WriteString(title)
	sb.WriteString("\n\n")

	// Add block content
	for _, block := range blocks {
		content := ExtractBlockContent(block)
		if content != "" {
			sb.WriteString(content)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// formatPropertyValue formats a property value as a string.
func (c *Connector) formatPropertyValue(prop Property) string {
	switch prop.Type {
	case "rich_text":
		return ExtractPlainText(prop.GetRichText())
	case "number":
		if num := prop.GetNumber(); num != nil {
			return fmt.Sprintf("%f", *num)
		}
	case "select":
		if sel := prop.GetSelect(); sel != nil {
			return sel.Name
		}
	case "date":
		if date := prop.GetDate(); date != nil {
			return date.Start
		}
	case "checkbox":
		if cb := prop.GetCheckbox(); cb != nil {
			return fmt.Sprintf("%t", *cb)
		}
	case "url":
		return prop.GetString(prop.URL)
	case "email":
		return prop.GetString(prop.Email)
	case "phone_number":
		return prop.GetString(prop.PhoneNumber)
	}
	return ""
}

// computeContentHash computes a SHA256 hash of content for change detection.
func computeContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}
