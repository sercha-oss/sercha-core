package notion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
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

// RESTClient implements driven.Connector. Returns the embedded Notion Client,
// which satisfies driven.RESTClient natively.
func (c *Connector) RESTClient() driven.RESTClient {
	return c.client
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

	// Note: Notion Search API only supports filtering by object type, not by timestamp.
	// We do client-side filtering based on last_edited_time after fetching results.
	// The filter parameter is left nil to fetch all pages and databases.

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
		// Note: No filter passed - Notion Search API only supports object type filter
		searchCursor := ""
		for {
			resp, err := c.client.Search(ctx, nil, searchCursor)
			if err != nil {
				return nil, "", fmt.Errorf("search: %w", err)
			}

			for _, result := range resp.Results {
				if result.Archived {
					// Skip archived items
					continue
				}

				// Client-side filtering for incremental sync
				if !sinceTime.IsZero() && !result.LastEditedTime.After(sinceTime) {
					// Item hasn't changed since last sync, skip it
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
					// One item's fetch failure must not stop the rest of
					// the workspace from syncing. Log so operators can
					// spot it; the cursor doesn't advance for this item
					// (the continue below skips the lastModified update),
					// so the next tick will retry naturally.
					slog.Warn("notion: per-item fetch failed; skipping",
						"object", result.Object,
						"id", result.ID,
						"error", err,
					)
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

	// Update cursor to the latest modified time. Use nanosecond precision:
	// Notion's last_edited_time is millisecond-precision, and RFC3339's
	// second-only output combined with the ! After comparison above would
	// drop any item edited in the same wall-clock second as the cursor.
	// RFC3339Nano is wire-compatible — time.Parse(time.RFC3339, ...) reads
	// the nano-suffixed form just fine, so legacy cursors keep working.
	newCursor := ""
	if !lastModified.IsZero() {
		newCursor = lastModified.Format(time.RFC3339Nano)
	} else if len(changes) > 0 {
		newCursor = time.Now().Format(time.RFC3339Nano)
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

// ReconciliationScopes declares which canonical-ID prefixes this connector
// snapshot-enumerates for delete detection. Notion has no native delete
// signal — archived pages and deleted databases simply stop appearing in
// Search/QueryDatabase responses — so every prefix the connector emits
// must be covered by phase-1 reconciliation.
//
// Caveat (#100 finding 7): Notion's Search API does not document stable
// ordering across paginated calls. A multi-page Inventory walk therefore
// admits a small risk that an item shifts pages mid-enumeration and is
// reported missing, leading reconciliation to delete a live page or
// database. Single-page walks are implicitly consistent. The Inventory
// implementations log a warning when they paginate so operators can
// correlate any spurious deletions with the actual risk window.
// Reconciliation is best-effort under that caveat.
func (c *Connector) ReconciliationScopes() []string {
	return []string{"page-", "database-entry-"}
}

// Inventory enumerates every canonical ID currently present upstream
// within the given scope. Pagination is "complete-or-error" — any page
// failure aborts the walk so the orchestrator never deletes against a
// partial snapshot.
//
// When the source is scoped to a single container (page or database),
// the inventory is restricted to that container so reconciliation can't
// reach across to delete items in unrelated parts of the workspace.
func (c *Connector) Inventory(ctx context.Context, source *domain.Source, scope string) ([]string, error) {
	switch scope {
	case "page-":
		return c.inventoryPages(ctx)
	case "database-entry-":
		return c.inventoryDatabaseEntries(ctx)
	default:
		return nil, fmt.Errorf("notion: unknown reconciliation scope %q", scope)
	}
}

// inventoryPages enumerates standalone pages — those whose parent is not
// a database. Database entries are inventoried separately under the
// "database-entry-" scope so the prefix sets stay disjoint.
func (c *Connector) inventoryPages(ctx context.Context) ([]string, error) {
	// containerID points at a single page or database; in the page case
	// reconciliation against "page-" should only consider that one page,
	// since the connector emits no other standalone pages for this source.
	if c.containerID != "" {
		page, err := c.client.GetPage(ctx, c.containerID)
		if err != nil {
			// If it's actually a database, there are no standalone pages
			// in scope — return an empty inventory rather than erroring.
			return nil, nil
		}
		if page.Archived || page.Parent.Type == "database_id" {
			return nil, nil
		}
		return []string{fmt.Sprintf("page-%s", page.ID)}, nil
	}

	var ids []string
	cursor := ""
	pages := 0
	filter := &SearchFilter{Property: "object", Value: "page"}
	for {
		resp, err := c.client.Search(ctx, filter, cursor)
		if err != nil {
			return nil, fmt.Errorf("inventory pages: %w", err)
		}
		pages++
		for _, result := range resp.Results {
			if result.Object != "page" {
				continue
			}
			if result.Archived {
				continue
			}
			if result.Parent.Type == "database_id" {
				continue
			}
			ids = append(ids, fmt.Sprintf("page-%s", result.ID))
		}
		if !resp.HasMore {
			break
		}
		cursor = resp.NextCursor
	}
	// Notion's Search API does not guarantee stable ordering across
	// paginated calls. A single-page walk is implicitly consistent;
	// multi-page walks open a small window for an item to shift pages
	// mid-enumeration and be falsely reported as missing — which would
	// cause reconciliation to delete a live page. Surface this so
	// operators can correlate any spurious deletions with the warning.
	if pages > 1 {
		slog.Warn("notion: inventory walk paginated; Search ordering is not stable across pages, deletes are best-effort",
			"scope", "page-",
			"pages_fetched", pages,
			"items_collected", len(ids),
		)
	}
	return ids, nil
}

// inventoryDatabaseEntries enumerates pages whose parent is a database.
// Notion has no global "list every entry across all databases" endpoint,
// so we must walk Search → list databases, then QueryDatabase per database.
func (c *Connector) inventoryDatabaseEntries(ctx context.Context) ([]string, error) {
	// Single-container case: if the source is scoped to one database,
	// only enumerate that database's entries.
	if c.containerID != "" {
		// Resolve whether containerID is a database; if it's a page,
		// there are no database entries in scope.
		if _, err := c.client.GetDatabase(ctx, c.containerID); err != nil {
			return nil, nil
		}
		return c.enumerateDatabaseEntries(ctx, c.containerID)
	}

	// Workspace-wide: find every accessible database, then walk each.
	var databaseIDs []string
	cursor := ""
	pages := 0
	filter := &SearchFilter{Property: "object", Value: "database"}
	for {
		resp, err := c.client.Search(ctx, filter, cursor)
		if err != nil {
			return nil, fmt.Errorf("inventory: list databases: %w", err)
		}
		pages++
		for _, result := range resp.Results {
			if result.Object != "database" {
				continue
			}
			if result.Archived {
				continue
			}
			databaseIDs = append(databaseIDs, result.ID)
		}
		if !resp.HasMore {
			break
		}
		cursor = resp.NextCursor
	}
	// Same ordering caveat as inventoryPages: Search is not stable
	// across paginated calls, so a multi-page walk for the database
	// list could legitimately miss a database and miss every entry
	// inside it. Per-database QueryDatabase walks below are
	// deterministic, so this is the only Search-instability site here.
	if pages > 1 {
		slog.Warn("notion: database-list inventory walk paginated; Search ordering is not stable across pages, deletes are best-effort",
			"scope", "database-entry-",
			"pages_fetched", pages,
			"databases_found", len(databaseIDs),
		)
	}

	var ids []string
	for _, dbID := range databaseIDs {
		entries, err := c.enumerateDatabaseEntries(ctx, dbID)
		if err != nil {
			// A single missing database (deleted between Search and
			// QueryDatabase, or permission lost mid-walk) shouldn't
			// poison the entire inventory. The whole point of phase-1
			// is best-effort cleanup; return what we found and let the
			// next tick try again.
			continue
		}
		ids = append(ids, entries...)
	}
	return ids, nil
}

// enumerateDatabaseEntries walks one database's entries with
// complete-or-error semantics for that single walk.
func (c *Connector) enumerateDatabaseEntries(ctx context.Context, databaseID string) ([]string, error) {
	var ids []string
	cursor := ""
	for {
		resp, err := c.client.QueryDatabase(ctx, databaseID, nil, cursor)
		if err != nil {
			return nil, fmt.Errorf("query database %s: %w", databaseID, err)
		}
		for _, page := range resp.Results {
			if page.Archived {
				continue
			}
			ids = append(ids, fmt.Sprintf("database-entry-%s", page.ID))
		}
		if !resp.HasMore {
			break
		}
		cursor = resp.NextCursor
	}
	return ids, nil
}
