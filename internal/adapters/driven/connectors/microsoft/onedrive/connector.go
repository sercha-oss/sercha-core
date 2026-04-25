package onedrive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/connectors/microsoft"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure Connector implements the interface.
var _ driven.Connector = (*Connector)(nil)

// Connector fetches documents from Microsoft OneDrive.
type Connector struct {
	tokenProvider driven.TokenProvider
	containerID   string // Folder ID (optional, format: "id:name")
	containerName string // Folder name for path matching
	client        *microsoft.Client
	config        *Config
}

// NewConnector creates a OneDrive connector.
// containerID can be empty to index all accessible content, or format "id:name" for a specific folder.
func NewConnector(tokenProvider driven.TokenProvider, containerID string, config *Config) *Connector {
	if config == nil {
		config = DefaultConfig()
	}

	clientConfig := &microsoft.ClientConfig{
		BaseURL:        "https://graph.microsoft.com/v1.0",
		RateLimitRPS:   config.RateLimitRPS,
		RequestTimeout: config.RequestTimeout,
		MaxRetries:     config.MaxRetries,
	}

	// Parse containerID format "id:name"
	var folderID, folderName string
	if containerID != "" {
		parts := strings.SplitN(containerID, ":", 2)
		folderID = parts[0]
		if len(parts) > 1 {
			folderName = parts[1]
		}
	}

	return &Connector{
		tokenProvider: tokenProvider,
		containerID:   folderID,
		containerName: folderName,
		client:        microsoft.NewClient(tokenProvider, clientConfig),
		config:        config,
	}
}

// Type returns the provider type.
func (c *Connector) Type() domain.ProviderType {
	return domain.ProviderTypeOneDrive
}

// ValidateConfig validates source configuration.
func (c *Connector) ValidateConfig(config domain.SourceConfig) error {
	// No special validation needed for OneDrive
	return nil
}

// FetchChanges fetches document changes from OneDrive using delta queries.
// For initial sync (empty cursor), it fetches all content.
// For incremental sync, it uses the delta link from the cursor.
//
// Microsoft Graph paginates large delta responses with @odata.nextLink
// for in-cycle continuation and ends the cycle with @odata.deltaLink
// (the cursor to store for the next tick). Items dropped by our
// filters — folders, oversize files, container-mismatches, content
// fetch errors — are not re-emitted by Graph on subsequent ticks, so
// the entire cycle MUST be drained inside one FetchChanges call. The
// loop below paginates until DeltaLink is provided, switching the
// stored cursor to that final value before returning.
func (c *Connector) FetchChanges(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
	var changes []*domain.Change
	pageCursor := cursor
	newCursor := cursor
	resynced := false

	for {
		select {
		case <-ctx.Done():
			return changes, newCursor, ctx.Err()
		default:
		}

		deltaResp, err := c.client.GetDelta(ctx, pageCursor)
		if err != nil {
			// 410 resyncRequired: Microsoft has aged out the stored
			// delta token and we must restart from scratch. Drop any
			// in-flight changes (they came from a stale cursor that
			// the next-cycle DeltaLink will not honour anyway), reset
			// the page cursor to empty so GetDelta begins a fresh
			// cycle, and continue. Bound recovery to one resync per
			// call to avoid pathological loops if Graph repeatedly
			// 410s an empty token.
			if errors.Is(err, microsoft.ErrResyncRequired) && !resynced {
				slog.Warn("onedrive: delta cursor invalidated; starting fresh delta cycle",
					"prior_cursor_present", cursor != "",
					"error", err,
				)
				resynced = true
				changes = nil
				pageCursor = ""
				newCursor = ""
				continue
			}
			return nil, "", fmt.Errorf("get delta: %w", err)
		}

		for _, item := range deltaResp.Value {
			// Skip folders (we only index files)
			if item.IsFolder() {
				continue
			}

			// Skip deleted items — these are emitted as ChangeTypeDeleted
			// directly because Graph natively signals deletion via the
			// @removed facet, and the delete must reach the orchestrator
			// before the cursor advances past it.
			if item.IsDeleted() {
				change := &domain.Change{
					Type:       domain.ChangeTypeDeleted,
					ExternalID: fmt.Sprintf("file-%s", item.ID),
				}
				changes = append(changes, change)
				continue
			}

			// Skip if not a file
			if !item.IsFile() {
				continue
			}

			// Skip large files
			if item.Size > c.config.MaxFileSize {
				continue
			}

			// If containerID is specified, filter by folder
			if c.containerID != "" {
				if !c.isInContainer(item) {
					continue
				}
			}

			// Fetch file content
			content, err := c.client.GetDriveItemContent(ctx, item.ID)
			if err != nil {
				slog.Warn("onedrive: content fetch failed; skipping",
					"item_id", item.ID,
					"name", item.Name,
					"error", err,
				)
				continue
			}

			doc := c.driveItemToDocument(&item)
			change := &domain.Change{
				Type:       domain.ChangeTypeModified,
				ExternalID: fmt.Sprintf("file-%s", item.ID),
				Document:   doc,
				Content:    string(content),
			}

			changes = append(changes, change)
		}

		// Microsoft documents two pagination terminators: DeltaLink ends
		// the cycle (this is the cursor for the next tick); NextLink
		// continues the same cycle (must be drained now). If both are
		// empty, treat the cycle as ended at the current page.
		if deltaResp.DeltaLink != "" {
			newCursor = deltaResp.DeltaLink
			break
		}
		if deltaResp.NextLink == "" {
			break
		}
		pageCursor = deltaResp.NextLink
	}

	return changes, newCursor, nil
}

// FetchDocument fetches a single document by external ID.
func (c *Connector) FetchDocument(ctx context.Context, source *domain.Source, externalID string) (*domain.Document, string, error) {
	// Parse external ID format: "file-{id}"
	if !strings.HasPrefix(externalID, "file-") {
		return nil, "", fmt.Errorf("invalid external ID format: %s", externalID)
	}

	itemID := strings.TrimPrefix(externalID, "file-")

	// Get item metadata
	item, err := c.client.GetDriveItem(ctx, itemID)
	if err != nil {
		return nil, "", fmt.Errorf("get drive item: %w", err)
	}

	// Check if it's a file
	if !item.IsFile() {
		return nil, "", fmt.Errorf("item is not a file: %s", externalID)
	}

	// Fetch content
	content, err := c.client.GetDriveItemContent(ctx, itemID)
	if err != nil {
		return nil, "", fmt.Errorf("get content: %w", err)
	}

	doc := c.driveItemToDocument(item)
	contentHash := computeContentHash(string(content))

	return doc, contentHash, nil
}

// TestConnection tests the connection to OneDrive.
func (c *Connector) TestConnection(ctx context.Context, source *domain.Source) error {
	_, err := c.client.GetMe(ctx)
	return err
}

// driveItemToDocument converts a OneDrive item to a domain document.
func (c *Connector) driveItemToDocument(item *microsoft.DriveItem) *domain.Document {
	metadata := map[string]string{
		"file_id": item.ID,
		"type":    "file",
		"size":    fmt.Sprintf("%d", item.Size),
	}

	if item.File != nil {
		metadata["mime_type"] = item.File.MimeType
	}

	if item.ParentReference != nil {
		if item.ParentReference.Path != "" {
			metadata["path"] = item.ParentReference.Path
		}
		if item.ParentReference.ID != "" {
			metadata["parent_id"] = item.ParentReference.ID
		}
	}

	// Determine MIME type
	mimeType := "application/octet-stream"
	if item.File != nil && item.File.MimeType != "" {
		mimeType = item.File.MimeType
	}

	return &domain.Document{
		Title:     item.Name,
		Path:      item.WebURL,
		MimeType:  mimeType,
		Metadata:  metadata,
		CreatedAt: item.CreatedDateTime,
		UpdatedAt: item.LastModifiedDateTime,
	}
}

// isInContainer checks if an item is within the specified container (folder).
func (c *Connector) isInContainer(item microsoft.DriveItem) bool {
	if c.containerID == "" && c.containerName == "" {
		return true
	}

	// Check if immediate parent matches container ID
	if item.ParentReference != nil && item.ParentReference.ID == c.containerID {
		return true
	}

	// Check if path contains the container folder name (handles recursive matching)
	// Path format: /drive/root:/FolderName/SubFolder
	if c.containerName != "" && item.ParentReference != nil && item.ParentReference.Path != "" {
		// Match :/FolderName or :/FolderName/ in path
		pathPattern := ":/" + c.containerName
		return strings.Contains(item.ParentReference.Path, pathPattern)
	}

	return false
}

// computeContentHash computes a SHA256 hash of content for change detection.
func computeContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// ReconciliationScopes declares which canonical-ID prefixes this connector
// snapshot-enumerates for delete detection. OneDrive's delta API natively
// emits @removed tombstones for deleted items (see FetchChanges at lines
// 90-96), so snapshot reconciliation is redundant and would be wasteful.
func (c *Connector) ReconciliationScopes() []string {
	return nil
}

// Inventory is structurally unreachable because ReconciliationScopes is
// empty — the orchestrator's phase-1 loop iterates over zero scopes. The
// explicit error defends against future callers that might invoke it
// directly and makes the design intent loud.
func (c *Connector) Inventory(ctx context.Context, source *domain.Source, scope string) ([]string, error) {
	return nil, driven.ErrInventoryNotSupported
}
