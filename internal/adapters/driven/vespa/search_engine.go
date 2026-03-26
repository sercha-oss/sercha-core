package vespa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
)

// Verify interface compliance
var _ driven.SearchEngine = (*SearchEngine)(nil)

// SearchEngine implements driven.SearchEngine using Vespa
type SearchEngine struct {
	baseURL    string
	httpClient *http.Client
}

// Config holds Vespa connection configuration
type Config struct {
	// BaseURL is the Vespa endpoint (e.g., http://localhost:19071)
	BaseURL string

	// Timeout for HTTP requests
	Timeout time.Duration
}

// DefaultConfig returns sensible defaults
func DefaultConfig(baseURL string) Config {
	return Config{
		BaseURL: baseURL,
		Timeout: 30 * time.Second,
	}
}

// NewSearchEngine creates a new Vespa-backed SearchEngine
func NewSearchEngine(cfg Config) *SearchEngine {
	return &SearchEngine{
		baseURL: strings.TrimSuffix(cfg.BaseURL, "/"),
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// vespaDocument represents a document in Vespa format
type vespaDocument struct {
	Fields vespaFields `json:"fields"`
}

type vespaFields struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	SourceID   string    `json:"source_id"`
	Content    string    `json:"content"`
	Embedding  []float32 `json:"embedding,omitempty"`
	Position   int       `json:"chunk_position"`
}

// Index indexes chunks for a document
func (s *SearchEngine) Index(ctx context.Context, chunks []*domain.Chunk) error {
	for _, chunk := range chunks {
		if err := s.indexChunk(ctx, chunk); err != nil {
			return fmt.Errorf("failed to index chunk %s: %w", chunk.ID, err)
		}
	}
	return nil
}

func (s *SearchEngine) indexChunk(ctx context.Context, chunk *domain.Chunk) error {
	doc := vespaDocument{
		Fields: vespaFields{
			ID:         chunk.ID,
			DocumentID: chunk.DocumentID,
			SourceID:   chunk.SourceID,
			Content:    chunk.Content,
			Embedding:  chunk.Embedding,
			Position:   chunk.Position,
		},
	}

	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	// Vespa document API: POST /document/v1/{namespace}/{doctype}/docid/{docid}
	url := fmt.Sprintf("%s/document/v1/sercha/chunk/docid/%s", s.baseURL, chunk.ID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("vespa index failed: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// Search performs a search query with retry logic for transient failures
func (s *SearchEngine) Search(ctx context.Context, query string, queryEmbedding []float32, opts domain.SearchOptions) ([]*domain.RankedChunk, int, error) {
	var lastErr error
	maxRetries := 3
	retryDelay := 500 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		results, count, err := s.doSearch(ctx, query, queryEmbedding, opts)
		if err == nil {
			return results, count, nil
		}

		lastErr = err

		// Don't retry on context cancellation or deadline exceeded
		if ctx.Err() != nil {
			return nil, 0, ctx.Err()
		}

		// Retry on transient errors (connection refused, timeouts, 503s)
		if !isRetryableError(err) {
			return nil, 0, err
		}

		// Wait before retrying (with exponential backoff)
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-time.After(retryDelay):
			retryDelay *= 2 // exponential backoff
		}
	}

	return nil, 0, fmt.Errorf("search failed after %d attempts: %w", maxRetries, lastErr)
}

// isRetryableError checks if an error is transient and worth retrying
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Retry on connection errors and server errors
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "EOF")
}

// doSearch performs the actual search request
func (s *SearchEngine) doSearch(ctx context.Context, query string, queryEmbedding []float32, opts domain.SearchOptions) ([]*domain.RankedChunk, int, error) {
	// Build YQL query based on search mode
	yql := s.buildYQL(query, opts)

	// Build search request
	searchReq := map[string]interface{}{
		"yql":    yql,
		"hits":   opts.Limit,
		"offset": opts.Offset,
	}

	// Add embedding and ranking profile based on search mode
	switch opts.Mode {
	case domain.SearchModeTextOnly:
		searchReq["ranking.profile"] = "bm25"
	case domain.SearchModeSemanticOnly:
		if len(queryEmbedding) > 0 {
			searchReq["input.query(embedding)"] = queryEmbedding
			searchReq["ranking.profile"] = "semantic"
		}
	default: // Hybrid
		if len(queryEmbedding) > 0 {
			searchReq["input.query(embedding)"] = queryEmbedding
			searchReq["ranking.profile"] = "hybrid"
		} else {
			// Fall back to BM25 if no embedding provided
			searchReq["ranking.profile"] = "bm25"
		}
	}

	body, err := json.Marshal(searchReq)
	if err != nil {
		return nil, 0, err
	}

	// Vespa search API
	url := fmt.Sprintf("%s/search/", s.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("vespa search failed: %s - %s", resp.Status, string(respBody))
	}

	var searchResp vespaSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, 0, err
	}

	// Convert to domain objects
	results := make([]*domain.RankedChunk, 0, len(searchResp.Root.Children))
	for _, hit := range searchResp.Root.Children {
		chunk := &domain.Chunk{
			ID:         hit.Fields.ID,
			DocumentID: hit.Fields.DocumentID,
			SourceID:   hit.Fields.SourceID,
			Content:    hit.Fields.Content,
			Position:   hit.Fields.Position,
		}

		ranked := &domain.RankedChunk{
			Chunk: chunk,
			Score: hit.Relevance,
		}
		results = append(results, ranked)
	}

	totalCount := int(searchResp.Root.Fields.TotalCount)
	return results, totalCount, nil
}

func (s *SearchEngine) buildYQL(query string, opts domain.SearchOptions) string {
	var conditions []string

	// Text search condition using content contains for BM25
	if query != "" {
		// Escape quotes in query for YQL
		escapedQuery := strings.ReplaceAll(query, "\"", "\\\"")
		switch opts.Mode {
		case domain.SearchModeTextOnly:
			conditions = append(conditions, fmt.Sprintf("content contains \"%s\"", escapedQuery))
		case domain.SearchModeSemanticOnly:
			conditions = append(conditions, "({targetHits:100}nearestNeighbor(embedding,embedding))")
		default: // Hybrid
			conditions = append(conditions, fmt.Sprintf("content contains \"%s\" or ({targetHits:100}nearestNeighbor(embedding,embedding))", escapedQuery))
		}
	}

	// Source filter
	if len(opts.SourceIDs) > 0 {
		sourceConditions := make([]string, len(opts.SourceIDs))
		for i, sourceID := range opts.SourceIDs {
			sourceConditions[i] = fmt.Sprintf("source_id contains \"%s\"", sourceID)
		}
		conditions = append(conditions, "("+strings.Join(sourceConditions, " or ")+")")
	}

	// Build final YQL
	whereClause := "true"
	if len(conditions) > 0 {
		whereClause = strings.Join(conditions, " and ")
	}

	return fmt.Sprintf("select * from chunk where %s", whereClause)
}

// vespaSearchResponse represents Vespa's search response format
type vespaSearchResponse struct {
	Root struct {
		Fields struct {
			TotalCount int64 `json:"totalCount"`
		} `json:"fields"`
		Children []struct {
			Relevance float64     `json:"relevance"`
			Fields    vespaFields `json:"fields"`
		} `json:"children"`
	} `json:"root"`
}

// Delete deletes chunks by IDs
func (s *SearchEngine) Delete(ctx context.Context, chunkIDs []string) error {
	for _, id := range chunkIDs {
		if err := s.deleteChunk(ctx, id); err != nil {
			return fmt.Errorf("failed to delete chunk %s: %w", id, err)
		}
	}
	return nil
}

func (s *SearchEngine) deleteChunk(ctx context.Context, id string) error {
	url := fmt.Sprintf("%s/document/v1/sercha/chunk/docid/%s", s.baseURL, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 404 is OK - document already deleted
	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("vespa delete failed: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// DeleteByDocument deletes all chunks for a document
func (s *SearchEngine) DeleteByDocument(ctx context.Context, documentID string) error {
	// Use Vespa's delete-by-query via visit API
	// This deletes all documents matching the selection
	selection := fmt.Sprintf("chunk.document_id==\"%s\"", documentID)
	return s.deleteBySelection(ctx, selection)
}

// DeleteBySource deletes all chunks for a source
func (s *SearchEngine) DeleteBySource(ctx context.Context, sourceID string) error {
	selection := fmt.Sprintf("chunk.source_id==\"%s\"", sourceID)
	return s.deleteBySelection(ctx, selection)
}

func (s *SearchEngine) deleteBySelection(ctx context.Context, selection string) error {
	// Vespa delete by selection using document/v1 API with selection parameter
	url := fmt.Sprintf("%s/document/v1/sercha/chunk/docid/?selection=%s&cluster=sercha",
		s.baseURL, url.QueryEscape(selection))

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("vespa delete by selection failed: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// HealthCheck verifies the search engine is available
func (s *SearchEngine) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/state/v1/health", s.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vespa health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vespa unhealthy: %s", resp.Status)
	}

	return nil
}

// Count returns the total number of indexed chunks in Vespa
func (s *SearchEngine) Count(ctx context.Context) (int64, error) {
	// Use Vespa search with hits=0 to just get the totalCount
	searchReq := map[string]interface{}{
		"yql":  "select * from chunk where true",
		"hits": 0,
	}

	body, err := json.Marshal(searchReq)
	if err != nil {
		return 0, err
	}

	url := fmt.Sprintf("%s/search/", s.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("vespa count query failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("vespa count query failed: %s - %s", resp.Status, string(respBody))
	}

	var searchResp vespaSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return 0, err
	}

	return searchResp.Root.Fields.TotalCount, nil
}
