package mcp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	corehttp "github.com/sercha-oss/sercha-core/internal/adapters/driving/http"
	"github.com/sercha-oss/sercha-core/internal/adapters/driving/mcp/widgets"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
)

// CallerEnricher is an optional hook that lets a consumer extend the
// request context with a richer caller representation than Core's base
// *domain.Caller. The hook is opaque to Core — implementations receive the
// base caller plus the raw token claims and return a new context carrying
// whatever extended caller value downstream pipeline stages expect.
//
// The base caller is constructed by Core's MCP handler (Source=CallerSourceMCP,
// UserID from the token). Enrichment runs after that, before search or
// get_document executes, so any pipeline stage that reads from the
// context sees the enriched form.
type CallerEnricher interface {
	// EnrichContext returns a new context derived from ctx, augmented with
	// a richer caller value. baseCaller is never nil when called.
	EnrichContext(ctx context.Context, baseCaller *domain.Caller, tokenUserID, clientID, clientName string) context.Context
}

// DocumentSensitivityApplier is an optional hook for the get_document tool
// that lets a consumer apply post-processing to a single document's content
// before it is returned to the client. Typical use is policy-driven content
// rewriting (token replacement, redaction, drop) but the interface is
// general — implementations may rewrite, annotate, or refuse content for
// any reason.
//
// Apply is called after the document is fetched and before the response is
// assembled. The returned SensitivityInfo is surfaced as a structured field
// on the MCP response so the consuming client can observe what was applied.
//
// When the field is nil on MCPServerConfig the handler skips the hook and
// returns the document body verbatim.
type DocumentSensitivityApplier interface {
	// Apply returns the post-processed content plus an outcome summary.
	// docID identifies the source document; implementations that maintain
	// per-document caches use it as a key. A non-nil error surfaces to the
	// MCP client as a tool failure.
	Apply(ctx context.Context, docID, content string) (maskedContent string, info SensitivityInfo, err error)
}

// SensitivityInfo carries the structured outcome of a DocumentSensitivityApplier
// run. Included on GetDocumentOutput and SearchResult so consumers can see
// what categories were detected, which were rewritten, and under which
// policy.
type SensitivityInfo struct {
	// DetectedCategories lists entity categories found in the content.
	DetectedCategories []string `json:"detected_categories,omitempty"`

	// MaskedCategories lists categories that were masked or blocked.
	MaskedCategories []string `json:"masked_categories,omitempty"`

	// Result is "allowed", "masked", "blocked", or "error".
	Result string `json:"result,omitempty"`

	// PolicyID is the EffectivePolicy.ID used.
	PolicyID string `json:"policy_id,omitempty"`

	// PolicyVersion is the EffectivePolicy.Version used.
	PolicyVersion int `json:"policy_version,omitempty"`
}

// MCPServerConfig holds configuration for the MCP server
type MCPServerConfig struct {
	SearchService     driving.SearchService
	DocumentService   driving.DocumentService
	SourceService     driving.SourceService
	OAuthService      driving.OAuthServerService // For token verification
	MCPServerURL      string
	Version           string
	RetrievalObserver driven.RetrievalObserver // optional

	// CallerEnricher is called after Core constructs the base *domain.Caller
	// to allow consumers to attach an extended caller value to the context.
	// Optional — nil means no enrichment.
	CallerEnricher CallerEnricher // optional

	// DocumentSensitivityApplier applies sensitivity policy to get_document
	// responses.  Optional — nil means content is returned unmasked.
	DocumentSensitivityApplier DocumentSensitivityApplier // optional
}

// NewMCPServer creates a new MCP server with tools registered
func NewMCPServer(cfg MCPServerConfig) *mcpsdk.Server {
	impl := &mcpsdk.Implementation{
		Name:    "sercha",
		Title:   "Sercha",
		Version: cfg.Version,
		Icons:   serverIcons(),
	}

	server := mcpsdk.NewServer(impl, nil)

	// Register the MCP Apps widget resources (sandboxed iframe HTML served via
	// resources/read). Must be registered before tools so the _meta.ui on each
	// tool points at an available URI.
	widgets.RegisterAll(server)

	// Register search tool
	registerSearchTool(server, cfg.SearchService, cfg.SourceService, cfg.RetrievalObserver, cfg.CallerEnricher)

	// Register get_document tool
	registerGetDocumentTool(server, cfg.DocumentService, cfg.SourceService, cfg.RetrievalObserver, cfg.CallerEnricher, cfg.DocumentSensitivityApplier)

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
				"client_id":   tokenInfo.ClientID,
				"client_name": tokenInfo.ClientName,
				"audience":    tokenInfo.Audience,
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
	DocumentID   string  `json:"document_id" jsonschema_description:"Document ID"`
	Title        string  `json:"title" jsonschema_description:"Document title"`
	Content      string  `json:"content" jsonschema_description:"Relevant content snippet"`
	Score        float64 `json:"score" jsonschema_description:"Relevance score as percentage (0-100)"`
	SourceID     string  `json:"source_id" jsonschema_description:"Source ID"`
	ProviderType string  `json:"provider_type,omitempty" jsonschema_description:"Provider type of the containing source (e.g. github, notion, onedrive)"`

	// Sensitivity is populated when the sensitivity masker stage ran.
	// Nil when sensitivity masking is not active (e.g. HTTP search).
	Sensitivity *SensitivityInfo `json:"sensitivity,omitempty" jsonschema_description:"Sensitivity masking outcome for this result"`
}

// registerSearchTool registers the search tool with the MCP server
func registerSearchTool(server *mcpsdk.Server, searchService driving.SearchService, sourceService driving.SourceService, observer driven.RetrievalObserver, callerEnricher CallerEnricher) {
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "search",
		Description: "Search across all indexed documents using semantic and keyword search",
		Meta:        widgets.MetaForSearch(),
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, input SearchInput) (*mcpsdk.CallToolResult, SearchOutput, error) {
		start := time.Now()

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

		clientID, clientName := clientIdentityFromTokenInfo(tokenInfo)

		// Construct the base Core Caller (MCP source).
		baseCaller := &domain.Caller{
			Source: domain.CallerSourceMCP,
			UserID: tokenInfo.UserID,
		}

		// Inject the AuthContext so identity-resolving adapters that bridge
		// to Core's corehttp.GetAuthContext (e.g. the document-id-filter
		// stage's permission provider) work uniformly across HTTP and MCP
		// entry points.
		ctx = withAuthContextFromTokenInfo(ctx, tokenInfo)

		// Run the optional caller-enricher hook so downstream pipeline
		// stages see the extended caller value (ClientID, AgentMode, …)
		// alongside Core's base caller.
		if callerEnricher != nil {
			ctx = callerEnricher.EnrichContext(ctx, baseCaller, tokenInfo.UserID, clientID, clientName)
		}

		// Build search options — thread the base Caller so sensitivity-gated
		// stages can check CallerSource via SearchContext.Caller.
		opts := domain.SearchOptions{
			Mode:      domain.SearchModeHybrid,
			Limit:     limit,
			SourceIDs: input.SourceIDs,
			Caller:    baseCaller,
		}

		// Perform search
		searchResp, err := searchService.Search(ctx, input.Query, opts)
		if err != nil {
			return nil, SearchOutput{}, fmt.Errorf("search failed: %w", err)
		}

		// Build source_id -> provider_type map so the widget can pick the
		// right icon without a second round-trip. Cheap: one query.
		providerBySource := map[string]string{}
		if sourceService != nil {
			if sources, err := sourceService.List(ctx); err == nil {
				for _, s := range sources {
					providerBySource[s.ID] = string(s.ProviderType)
				}
			}
		}

		// Convert results to output format.  Read sensitivity metadata written
		// by the masker stage — available in r.Metadata (threaded through
		// pipeline.PresentedResult.Metadata → domain.SearchResultItem.Metadata).
		results := make([]SearchResult, len(searchResp.Results))
		documentIDs := make([]string, len(searchResp.Results))

		// Per-document outcome ledger for the observer event. Built only when
		// the masker stage ran (signalled by a "masked" key in r.Metadata);
		// non-masker pipelines leave docOutcomes nil and the observer treats
		// the absence as "no masking applied".
		var docOutcomes []map[string]any

		for i, r := range searchResp.Results {
			sr := SearchResult{
				DocumentID:   r.DocumentID,
				Title:        r.Title,
				Content:      r.Snippet,
				Score:        r.Score,
				SourceID:     r.SourceID,
				ProviderType: providerBySource[r.SourceID],
			}
			documentIDs[i] = r.DocumentID

			// Read per-result sensitivity metadata written by the masker stage.
			if r.Metadata != nil {
				if _, hasMasked := r.Metadata["masked"]; hasMasked {
					info := &SensitivityInfo{}
					var visible []string
					if cats, ok := r.Metadata["masked_categories"].([]string); ok {
						visible = filterSentinelCategories(cats)
						info.MaskedCategories = visible
					}
					result := "allowed"
					if len(visible) > 0 {
						result = "masked"
					}
					info.Result = result

					if pid, ok := r.Metadata["policy_id"].(string); ok && pid != "" {
						info.PolicyID = pid
					}
					if pver, ok := r.Metadata["policy_version"].(int); ok && pver != 0 {
						info.PolicyVersion = pver
					}
					sr.Sensitivity = info

					// Per-document outcome for the audit observer. before_hash
					// and after_hash are passed through verbatim when the
					// masker computed them; absent fields stay empty so the
					// observer's coercion drops them.
					outcome := map[string]any{
						"document_id":       r.DocumentID,
						"position":          i,
						"result":            result,
						"masked_categories": visible,
					}
					if bh, ok := r.Metadata["before_hash"].(string); ok && bh != "" {
						outcome["before_hash"] = bh
					}
					if ah, ok := r.Metadata["after_hash"].(string); ok && ah != "" {
						outcome["after_hash"] = ah
					}
					docOutcomes = append(docOutcomes, outcome)
				}
			}

			results[i] = sr
		}

		observerEvent := driven.SearchCompletedEvent{
			UserID:           tokenInfo.UserID,
			Query:            input.Query,
			DocumentIDs:      documentIDs,
			ResultCount:      searchResp.TotalCount,
			DurationNs:       time.Since(start).Nanoseconds(),
			ClientType:       "mcp",
			ClientID:         clientID,
			ClientName:       clientName,
			DocumentOutcomes: docOutcomes,
		}
		fireSearchObserver(observer, observerEvent)

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{
					Text: formatSearchResults(results),
				},
			},
		}, SearchOutput{Results: results}, nil
	})
}

// filterSentinelCategories removes the internal "__error__" sentinel used by
// markAllError so it is never surfaced to MCP clients in the SensitivityInfo
// or audit event fields.
func filterSentinelCategories(cats []string) []string {
	out := make([]string, 0, len(cats))
	for _, c := range cats {
		if c != "__error__" {
			out = append(out, c)
		}
	}
	return out
}

// clientIdentityFromTokenInfo extracts the OAuth client_id / client_name
// from tokenInfo.Extra for use in retrieval observer events. Both values
// are optional: missing keys, a nil tokenInfo, or non-string values yield
// empty strings rather than panicking. client_name is not populated by
// the current token verifier, so it will be empty until TokenInfo.Extra
// is enriched in a later change — that's intentional.
func clientIdentityFromTokenInfo(tokenInfo *auth.TokenInfo) (clientID, clientName string) {
	if tokenInfo == nil {
		return "", ""
	}
	if v, ok := tokenInfo.Extra["client_id"].(string); ok {
		clientID = v
	}
	if v, ok := tokenInfo.Extra["client_name"].(string); ok {
		clientName = v
	}
	return clientID, clientName
}

// withAuthContextFromTokenInfo attaches a *domain.AuthContext to ctx,
// derived from the OAuth2 tokenInfo, so downstream identity-resolving
// adapters that use corehttp.GetAuthContext (e.g. the document-id-filter
// stage's permission-store-document-ID-provider) can resolve a Principal
// from MCP requests in the same way they do from HTTP-bearer requests.
//
// The HTTP path constructs an AuthContext via AuthMiddleware after
// validating a bearer token; the MCP path validates an OAuth2 access
// token via the SDK's verifier and produces a TokenInfo. This helper
// bridges the two so consumers can stay entry-point agnostic.
//
// Fields not carried in the OAuth token (Email, Name, TeamID, SessionID)
// are left zero. Consumers that need them must look them up via the user
// store using UserID. Today the only consumer is the identity resolver,
// which only reads UserID.
func withAuthContextFromTokenInfo(ctx context.Context, tokenInfo *auth.TokenInfo) context.Context {
	if tokenInfo == nil || tokenInfo.UserID == "" {
		return ctx
	}
	return corehttp.WithAuthContext(ctx, &domain.AuthContext{
		UserID: tokenInfo.UserID,
	})
}

// fireSearchObserver invokes observer.OnSearchCompleted on a detached
// goroutine. Nil-guarded; errors are logged and swallowed.
func fireSearchObserver(observer driven.RetrievalObserver, event driven.SearchCompletedEvent) {
	if observer == nil {
		return
	}
	go func() {
		if err := observer.OnSearchCompleted(context.Background(), event); err != nil {
			log.Printf("retrieval observer: OnSearchCompleted failed: %v", err)
		}
	}()
}

// fireDocumentObserver invokes observer.OnDocumentRetrieved on a detached
// goroutine. Nil-guarded; errors are logged and swallowed.
func fireDocumentObserver(observer driven.RetrievalObserver, event driven.DocumentRetrievedEvent) {
	if observer == nil {
		return
	}
	go func() {
		if err := observer.OnDocumentRetrieved(context.Background(), event); err != nil {
			log.Printf("retrieval observer: OnDocumentRetrieved failed: %v", err)
		}
	}()
}

// GetDocumentInput represents the input for the get_document tool
type GetDocumentInput struct {
	DocumentID string `json:"document_id" jsonschema:"required" jsonschema_description:"Document ID to retrieve"`
}

// GetDocumentOutput represents the output for the get_document tool
type GetDocumentOutput struct {
	DocumentID   string            `json:"document_id" jsonschema_description:"Document ID"`
	Title        string            `json:"title" jsonschema_description:"Document title"`
	Content      string            `json:"content" jsonschema_description:"Full document content"`
	URL          string            `json:"url" jsonschema_description:"URL to open document in source system"`
	SourceID     string            `json:"source_id,omitempty" jsonschema_description:"ID of the source this document belongs to"`
	ProviderType string            `json:"provider_type,omitempty" jsonschema_description:"Provider type of the source (e.g. github, notion, onedrive)"`
	Metadata     map[string]string `json:"metadata" jsonschema_description:"Document metadata"`

	// Sensitivity is populated when the DocumentSensitivityApplier ran.
	// Nil when sensitivity masking is not configured.
	Sensitivity *SensitivityInfo `json:"sensitivity,omitempty" jsonschema_description:"Sensitivity masking outcome for this document"`
}

// registerGetDocumentTool registers the get_document tool with the MCP server
func registerGetDocumentTool(server *mcpsdk.Server, documentService driving.DocumentService, sourceService driving.SourceService, observer driven.RetrievalObserver, callerEnricher CallerEnricher, sensitivityApplier DocumentSensitivityApplier) {
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "get_document",
		Description: "Retrieve the full content and metadata of a specific document by ID",
		Meta:        widgets.MetaForDocument(),
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, input GetDocumentInput) (*mcpsdk.CallToolResult, GetDocumentOutput, error) {
		start := time.Now()

		// Get user context from token info
		tokenInfo := auth.TokenInfoFromContext(ctx)
		if tokenInfo == nil {
			return nil, GetDocumentOutput{}, fmt.Errorf("user context not found")
		}

		// Check scope
		if !hasScope(tokenInfo.Scopes, domain.ScopeMCPDocRead) {
			return nil, GetDocumentOutput{}, fmt.Errorf("insufficient scope: %s required", domain.ScopeMCPDocRead)
		}

		clientID, clientName := clientIdentityFromTokenInfo(tokenInfo)

		// Construct the base Core Caller (MCP source).
		baseCaller := &domain.Caller{
			Source: domain.CallerSourceMCP,
			UserID: tokenInfo.UserID,
		}

		// Inject the AuthContext so identity-resolving adapters that bridge
		// to Core's corehttp.GetAuthContext work uniformly across HTTP and
		// MCP entry points.
		ctx = withAuthContextFromTokenInfo(ctx, tokenInfo)

		// Run the optional caller-enricher hook (mirrors search-tool path).
		if callerEnricher != nil {
			ctx = callerEnricher.EnrichContext(ctx, baseCaller, tokenInfo.UserID, clientID, clientName)
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

		// Best-effort provider-type enrichment so the widget can render
		// the right icon without a second round-trip. Failures fall back
		// to an empty string (widget shows a letter chip).
		var providerType string
		if sourceService != nil && doc.SourceID != "" {
			if src, err := sourceService.Get(ctx, doc.SourceID); err == nil && src != nil {
				providerType = string(src.ProviderType)
			}
		}

		// Apply the optional document post-processor (typically policy-driven
		// masking) when one is wired.
		content := docContent.Body
		var sensitivityInfo *SensitivityInfo
		if sensitivityApplier != nil {
			masked, info, applyErr := sensitivityApplier.Apply(ctx, doc.ID, content)
			if applyErr != nil {
				return nil, GetDocumentOutput{}, fmt.Errorf("sensitivity masking failed: %w", applyErr)
			}
			// Blocked policy: the client must not learn the doc exists.
			// Mirrors the search-pipeline masker, which silently drops blocked
			// candidates from search results — confirming a doc by ID would
			// otherwise let a client enumerate the corpus to map what's
			// hidden from them. Return a not-found error so blocked and
			// non-existent are indistinguishable on the wire. The block is
			// still recorded server-side via the audit observer below.
			if info.Result == "blocked" {
				fireDocumentObserver(observer, driven.DocumentRetrievedEvent{
					UserID:     tokenInfo.UserID,
					DocumentID: doc.ID,
					DurationNs: time.Since(start).Nanoseconds(),
					ClientType: "mcp",
					ClientID:   clientID,
					ClientName: clientName,
				})
				return nil, GetDocumentOutput{}, fmt.Errorf("document not found")
			}
			content = masked
			sensitivityInfo = &info
		}

		output := GetDocumentOutput{
			DocumentID:   doc.ID,
			Title:        doc.Title,
			Content:      content,
			URL:          doc.Path,
			SourceID:     doc.SourceID,
			ProviderType: providerType,
			Metadata:     metadata,
			Sensitivity:  sensitivityInfo,
		}

		fireDocumentObserver(observer, driven.DocumentRetrievedEvent{
			UserID:     tokenInfo.UserID,
			DocumentID: doc.ID,
			DurationNs: time.Since(start).Nanoseconds(),
			ClientType: "mcp",
			ClientID:   clientID,
			ClientName: clientName,
		})

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
		fmt.Fprintf(&sb, "   Score: %.0f%%\n", r.Score)
		fmt.Fprintf(&sb, "   Content: %s\n", truncate(r.Content, 200))
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatDocument formats a document as text
func formatDocument(doc GetDocumentOutput) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Document: %s\n", doc.Title)
	fmt.Fprintf(&sb, "ID: %s\n", doc.DocumentID)
	if doc.URL != "" {
		fmt.Fprintf(&sb, "URL: %s\n", doc.URL)
	}
	sb.WriteString("\n")

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
