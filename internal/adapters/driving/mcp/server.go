package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
)

// MCPServerConfig holds configuration for the MCP server
type MCPServerConfig struct {
	SearchService   driving.SearchService
	DocumentService driving.DocumentService
	SourceService   driving.SourceService
	OAuthService    driving.OAuthServerService // For token verification
	MCPServerURL    string
	Version         string
}

// NewMCPServer creates a new MCP server with tools registered
func NewMCPServer(cfg MCPServerConfig) *mcpsdk.Server {
	impl := &mcpsdk.Implementation{
		Name:    "sercha",
		Version: cfg.Version,
	}

	server := mcpsdk.NewServer(impl, nil)

	// Register search tool
	registerSearchTool(server, cfg.SearchService)

	// Register get_document tool
	registerGetDocumentTool(server, cfg.DocumentService)

	// Register list_sources tool
	registerListSourcesTool(server, cfg.SourceService)

	return server
}

// NewHTTPHandler creates the StreamableHTTPHandler for the MCP server with bearer token auth
func NewHTTPHandler(server *mcpsdk.Server, oauthService driving.OAuthServerService, mcpServerURL string) http.Handler {
	// Create token verifier using the OAuth server service
	verifier := func(ctx context.Context, token string, req *http.Request) (*auth.TokenInfo, error) {
		tokenInfo, err := oauthService.ValidateAccessToken(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", auth.ErrInvalidToken, err)
		}

		return &auth.TokenInfo{
			UserID:     tokenInfo.UserID,
			Scopes:     tokenInfo.Scopes,
			Expiration: time.Now().Add(domain.AccessTokenTTL), // Token expiration time
			Extra: map[string]any{
				"client_id": tokenInfo.ClientID,
				"audience":  tokenInfo.Audience,
			},
		}, nil
	}

	// Create bearer token middleware
	bearerTokenMiddleware := auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: mcpServerURL + "/.well-known/oauth-protected-resource",
		Scopes:              domain.DefaultMCPScopes,
	})

	// Create the MCP HTTP handler
	mcpHandler := mcpsdk.NewStreamableHTTPHandler(
		func(r *http.Request) *mcpsdk.Server {
			return server
		},
		nil, // default options
	)

	// Wrap with bearer token auth middleware
	return bearerTokenMiddleware(mcpHandler)
}

// Tool input/output types

// SearchInput represents the input for the search tool
type SearchInput struct {
	Query     string   `json:"query" jsonschema:"required" jsonschema_description:"Search query text"`
	SourceIDs []string `json:"source_ids,omitempty" jsonschema_description:"Optional source IDs to filter by"`
	Limit     int      `json:"limit,omitempty" jsonschema_description:"Maximum number of results (default 10)"`
}

// SearchOutput represents the output for the search tool
type SearchOutput struct {
	Results []SearchResult `json:"results" jsonschema_description:"Search results"`
}

// SearchResult represents a single search result
type SearchResult struct {
	DocumentID string  `json:"document_id" jsonschema_description:"Document ID"`
	Title      string  `json:"title" jsonschema_description:"Document title"`
	Content    string  `json:"content" jsonschema_description:"Relevant content snippet"`
	Score      float64 `json:"score" jsonschema_description:"Relevance score"`
	SourceID   string  `json:"source_id" jsonschema_description:"Source ID"`
}

// registerSearchTool registers the search tool with the MCP server
func registerSearchTool(server *mcpsdk.Server, searchService driving.SearchService) {
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "search",
		Description: "Search across all indexed documents using semantic and keyword search",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, input SearchInput) (*mcpsdk.CallToolResult, SearchOutput, error) {
		// Get user context from token info
		tokenInfo := auth.TokenInfoFromContext(ctx)
		if tokenInfo == nil {
			return nil, SearchOutput{}, fmt.Errorf("user context not found")
		}

		// Check scope
		if !hasScope(tokenInfo.Scopes, domain.ScopeMCPSearch) {
			return nil, SearchOutput{}, fmt.Errorf("insufficient scope: %s required", domain.ScopeMCPSearch)
		}

		// Set default limit
		limit := input.Limit
		if limit <= 0 {
			limit = 10
		}

		// Build search options
		opts := domain.SearchOptions{
			Mode:      domain.SearchModeHybrid,
			Limit:     limit,
			SourceIDs: input.SourceIDs,
		}

		// Perform search
		// Note: For Phase 1, we're using the service without user-level filtering
		// The bearer token already provides scope-based access control
		searchResp, err := searchService.Search(ctx, input.Query, opts)
		if err != nil {
			return nil, SearchOutput{}, fmt.Errorf("search failed: %w", err)
		}

		// Convert results to output format
		results := make([]SearchResult, len(searchResp.Results))
		for i, r := range searchResp.Results {
			results[i] = SearchResult{
				DocumentID: r.DocumentID,
				Title:      r.Title,
				Content:    r.Snippet, // Use Snippet field instead of Content
				Score:      r.Score,
				SourceID:   r.SourceID,
			}
		}

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{
					Text: formatSearchResults(results),
				},
			},
		}, SearchOutput{Results: results}, nil
	})
}

// GetDocumentInput represents the input for the get_document tool
type GetDocumentInput struct {
	DocumentID string `json:"document_id" jsonschema:"required" jsonschema_description:"Document ID to retrieve"`
}

// GetDocumentOutput represents the output for the get_document tool
type GetDocumentOutput struct {
	DocumentID string            `json:"document_id" jsonschema_description:"Document ID"`
	Title      string            `json:"title" jsonschema_description:"Document title"`
	Content    string            `json:"content" jsonschema_description:"Full document content"`
	Metadata   map[string]string `json:"metadata" jsonschema_description:"Document metadata"`
}

// registerGetDocumentTool registers the get_document tool with the MCP server
func registerGetDocumentTool(server *mcpsdk.Server, documentService driving.DocumentService) {
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "get_document",
		Description: "Retrieve the full content and metadata of a specific document by ID",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, input GetDocumentInput) (*mcpsdk.CallToolResult, GetDocumentOutput, error) {
		// Get user context from token info
		tokenInfo := auth.TokenInfoFromContext(ctx)
		if tokenInfo == nil {
			return nil, GetDocumentOutput{}, fmt.Errorf("user context not found")
		}

		// Check scope
		if !hasScope(tokenInfo.Scopes, domain.ScopeMCPDocRead) {
			return nil, GetDocumentOutput{}, fmt.Errorf("insufficient scope: %s required", domain.ScopeMCPDocRead)
		}

		// Get document with content
		docContent, err := documentService.GetContent(ctx, input.DocumentID)
		if err != nil {
			return nil, GetDocumentOutput{}, fmt.Errorf("get document failed: %w", err)
		}

		// Get document metadata
		doc, err := documentService.Get(ctx, input.DocumentID)
		if err != nil {
			return nil, GetDocumentOutput{}, fmt.Errorf("get document metadata failed: %w", err)
		}

		// Convert metadata to string map
		metadata := make(map[string]string)
		for k, v := range doc.Metadata {
			metadata[k] = fmt.Sprintf("%v", v)
		}

		output := GetDocumentOutput{
			DocumentID: doc.ID,
			Title:      doc.Title,
			Content:    docContent.Body,
			Metadata:   metadata,
		}

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{
					Text: formatDocument(output),
				},
			},
		}, output, nil
	})
}

// ListSourcesInput represents the input for the list_sources tool
type ListSourcesInput struct{}

// ListSourcesOutput represents the output for the list_sources tool
type ListSourcesOutput struct {
	Sources []SourceInfo `json:"sources" jsonschema_description:"Available sources"`
}

// SourceInfo represents information about a source
type SourceInfo struct {
	ID            string `json:"id" jsonschema_description:"Source ID"`
	Name          string `json:"name" jsonschema_description:"Source name"`
	Type          string `json:"type" jsonschema_description:"Source type"`
	Enabled       bool   `json:"enabled" jsonschema_description:"Whether source is enabled"`
	DocumentCount int    `json:"document_count" jsonschema_description:"Number of documents"`
}

// registerListSourcesTool registers the list_sources tool with the MCP server
func registerListSourcesTool(server *mcpsdk.Server, sourceService driving.SourceService) {
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "list_sources",
		Description: "List all available document sources",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, input ListSourcesInput) (*mcpsdk.CallToolResult, ListSourcesOutput, error) {
		// Get user context from token info
		tokenInfo := auth.TokenInfoFromContext(ctx)
		if tokenInfo == nil {
			return nil, ListSourcesOutput{}, fmt.Errorf("user context not found")
		}

		// Check scope
		if !hasScope(tokenInfo.Scopes, domain.ScopeMCPSourcesList) {
			return nil, ListSourcesOutput{}, fmt.Errorf("insufficient scope: %s required", domain.ScopeMCPSourcesList)
		}

		// List sources with summary (includes document counts)
		sources, err := sourceService.ListWithSummary(ctx)
		if err != nil {
			return nil, ListSourcesOutput{}, fmt.Errorf("list sources failed: %w", err)
		}

		// Convert to output format
		sourceInfos := make([]SourceInfo, len(sources))
		for i, s := range sources {
			sourceInfos[i] = SourceInfo{
				ID:            s.Source.ID,
				Name:          s.Source.Name,
				Type:          string(s.Source.ProviderType),
				Enabled:       s.Source.Enabled,
				DocumentCount: s.DocumentCount,
			}
		}

		output := ListSourcesOutput{Sources: sourceInfos}

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{
					Text: formatSources(sourceInfos),
				},
			},
		}, output, nil
	})
}

// Helper functions

// hasScope checks if the given scope is present in the scopes list
func hasScope(scopes []string, required string) bool {
	for _, s := range scopes {
		if s == required {
			return true
		}
	}
	return false
}

// formatSearchResults formats search results as text
func formatSearchResults(results []SearchResult) string {
	if len(results) == 0 {
		return "No results found."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d results:\n\n", len(results))

	for i, r := range results {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, r.Title)
		fmt.Fprintf(&sb, "   Document ID: %s\n", r.DocumentID)
		fmt.Fprintf(&sb, "   Source ID: %s\n", r.SourceID)
		fmt.Fprintf(&sb, "   Score: %.2f\n", r.Score)
		fmt.Fprintf(&sb, "   Content: %s\n", truncate(r.Content, 200))
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatDocument formats a document as text
func formatDocument(doc GetDocumentOutput) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Document: %s\n", doc.Title)
	fmt.Fprintf(&sb, "ID: %s\n\n", doc.DocumentID)

	if len(doc.Metadata) > 0 {
		sb.WriteString("Metadata:\n")
		for k, v := range doc.Metadata {
			fmt.Fprintf(&sb, "  %s: %s\n", k, v)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Content:\n")
	sb.WriteString(doc.Content)

	return sb.String()
}

// formatSources formats sources as text
func formatSources(sources []SourceInfo) string {
	if len(sources) == 0 {
		return "No sources available."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Available sources (%d):\n\n", len(sources))

	for i, s := range sources {
		status := "enabled"
		if !s.Enabled {
			status = "disabled"
		}

		fmt.Fprintf(&sb, "%d. %s (%s)\n", i+1, s.Name, status)
		fmt.Fprintf(&sb, "   ID: %s\n", s.ID)
		fmt.Fprintf(&sb, "   Type: %s\n", s.Type)
		fmt.Fprintf(&sb, "   Documents: %d\n", s.DocumentCount)
		sb.WriteString("\n")
	}

	return sb.String()
}

// truncate truncates a string to the given length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
