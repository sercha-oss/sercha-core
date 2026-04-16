package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	driven "github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure interface compliance
var _ driven.SearchEngine = (*SearchEngine)(nil)

// Index indexes chunks for searching
func (s *SearchEngine) Index(ctx context.Context, chunks []*domain.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Ensure index exists with correct mapping
	if err := s.ensureIndex(ctx); err != nil {
		return fmt.Errorf("failed to ensure index: %w", err)
	}

	// Build bulk request body
	var buf bytes.Buffer
	for _, chunk := range chunks {
		// Index action
		action := map[string]any{
			"index": map[string]any{
				"_index": s.indexName,
				"_id":    chunk.ID,
			},
		}
		if err := json.NewEncoder(&buf).Encode(action); err != nil {
			return fmt.Errorf("failed to encode action: %w", err)
		}

		// Document
		doc := map[string]any{
			"id":             chunk.ID,
			"document_id":    chunk.DocumentID,
			"source_id":      chunk.SourceID,
			"content":        chunk.Content,
			"chunk_position": chunk.Position,
		}
		if err := json.NewEncoder(&buf).Encode(doc); err != nil {
			return fmt.Errorf("failed to encode document: %w", err)
		}
	}

	// Execute bulk request
	req := opensearchapi.BulkReq{
		Body: bytes.NewReader(buf.Bytes()),
	}

	resp, err := s.client.Bulk(ctx, req)
	if err != nil {
		return fmt.Errorf("bulk index failed: %w", err)
	}

	// Check for HTTP errors via Inspect
	if httpResp := resp.Inspect().Response; httpResp != nil && httpResp.StatusCode != 200 {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("bulk index failed with status %d: %s", httpResp.StatusCode, string(body))
	}

	// Check for item-level errors using typed response
	if resp.Errors {
		var errs []string
		for i, item := range resp.Items {
			for _, v := range item {
				if v.Error != nil && v.Error.Type != "" {
					errs = append(errs, fmt.Sprintf("item %d: %s - %s", i, v.Error.Type, v.Error.Reason))
					if len(errs) >= 5 {
						break
					}
				}
			}
			if len(errs) >= 5 {
				break
			}
		}
		return fmt.Errorf("bulk index had errors: %s", strings.Join(errs, "; "))
	}

	return nil
}

// IndexDocument indexes a full document for BM25 text search.
// Uses document_id as the OpenSearch document _id for upsert semantics.
func (s *SearchEngine) IndexDocument(ctx context.Context, doc *domain.DocumentContent) error {
	if err := s.ensureIndex(ctx); err != nil {
		return fmt.Errorf("failed to ensure index: %w", err)
	}

	body := map[string]any{
		"document_id": doc.DocumentID,
		"source_id":   doc.SourceID,
		"title":       doc.Title,
		"content":     doc.Body,
		"path":        doc.Path,
		"mime_type":   doc.MimeType,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal document: %w", err)
	}

	req := opensearchapi.IndexReq{
		Index:      s.indexName,
		DocumentID: doc.DocumentID,
		Body:       bytes.NewReader(bodyBytes),
	}

	resp, err := s.client.Index(ctx, req)
	if err != nil {
		return fmt.Errorf("index document failed: %w", err)
	}

	if httpResp := resp.Inspect().Response; httpResp != nil {
		if httpResp.StatusCode != 200 && httpResp.StatusCode != 201 {
			respBody, _ := io.ReadAll(httpResp.Body)
			return fmt.Errorf("index document failed with status %d: %s", httpResp.StatusCode, string(respBody))
		}
	}

	return nil
}

// SearchDocuments performs a BM25 text search returning document-level results.
func (s *SearchEngine) SearchDocuments(ctx context.Context, query string, opts domain.SearchOptions) ([]driven.DocumentResult, int, error) {
	// Build search query against both title and content fields
	searchQuery := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": []any{
					map[string]any{
						"multi_match": map[string]any{
							"query":  query,
							"fields": []string{"title^3", "content"},
							"type":   "most_fields",
						},
					},
				},
			},
		},
		"from": opts.Offset,
		"size": opts.Limit,
		"highlight": map[string]any{
			"fields": map[string]any{
				"content": map[string]any{
					"fragment_size":       200,
					"number_of_fragments": 3,
				},
				"title": map[string]any{
					"fragment_size":       200,
					"number_of_fragments": 1,
				},
			},
		},
	}

	// Add source filter if specified
	if len(opts.SourceIDs) > 0 {
		boolQuery := searchQuery["query"].(map[string]any)["bool"].(map[string]any)
		boolQuery["filter"] = []any{
			map[string]any{
				"terms": map[string]any{
					"source_id": opts.SourceIDs,
				},
			},
		}
	}

	queryBody, err := json.Marshal(searchQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal query: %w", err)
	}

	req := &opensearchapi.SearchReq{
		Indices: []string{s.indexName},
		Body:    bytes.NewReader(queryBody),
	}

	resp, err := s.client.Search(ctx, req)
	if err != nil {
		// Index doesn't exist yet — return empty results instead of 500
		if isIndexNotFoundError(err) {
			return []driven.DocumentResult{}, 0, nil
		}
		return nil, 0, fmt.Errorf("search failed: %w", err)
	}

	results := make([]driven.DocumentResult, 0, len(resp.Hits.Hits))
	for _, hit := range resp.Hits.Hits {
		var source map[string]any
		if err := json.Unmarshal(hit.Source, &source); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal source: %w", err)
		}

		results = append(results, driven.DocumentResult{
			DocumentID: getString(source, "document_id"),
			SourceID:   getString(source, "source_id"),
			Title:      getString(source, "title"),
			Content:    getString(source, "content"),
			Score:      float64(hit.Score),
		})
	}

	return results, resp.Hits.Total.Value, nil
}

// Search performs a BM25 text search
func (s *SearchEngine) Search(ctx context.Context, query string, queryEmbedding []float32, opts domain.SearchOptions) ([]*domain.RankedChunk, int, error) {
	// Build search query
	searchQuery := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": []any{
					map[string]any{
						"match": map[string]any{
							"content": query,
						},
					},
				},
			},
		},
		"from": opts.Offset,
		"size": opts.Limit,
		"highlight": map[string]any{
			"fields": map[string]any{
				"content": map[string]any{
					"fragment_size":       200,
					"number_of_fragments": 3,
				},
			},
		},
	}

	// Add source filter if specified
	if len(opts.SourceIDs) > 0 {
		boolQuery := searchQuery["query"].(map[string]any)["bool"].(map[string]any)
		boolQuery["filter"] = []any{
			map[string]any{
				"terms": map[string]any{
					"source_id": opts.SourceIDs,
				},
			},
		}
	}

	// Marshal query
	queryBody, err := json.Marshal(searchQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal query: %w", err)
	}

	// Execute search
	req := &opensearchapi.SearchReq{
		Indices: []string{s.indexName},
		Body:    bytes.NewReader(queryBody),
	}

	resp, err := s.client.Search(ctx, req)
	if err != nil {
		// Index doesn't exist yet — return empty results instead of 500
		if isIndexNotFoundError(err) {
			return []*domain.RankedChunk{}, 0, nil
		}
		return nil, 0, fmt.Errorf("search failed: %w", err)
	}

	// Convert typed response to domain objects
	results := make([]*domain.RankedChunk, 0, len(resp.Hits.Hits))
	for _, hit := range resp.Hits.Hits {
		// Parse source
		var source map[string]any
		if err := json.Unmarshal(hit.Source, &source); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal source: %w", err)
		}

		chunk := &domain.Chunk{
			ID:         hit.ID,
			DocumentID: getString(source, "document_id"),
			SourceID:   getString(source, "source_id"),
			Content:    getString(source, "content"),
			Position:   getInt(source, "chunk_position"),
		}

		// Extract highlights
		var highlights []string
		if hl, ok := hit.Highlight["content"]; ok {
			highlights = hl
		}

		rankedChunk := &domain.RankedChunk{
			Chunk:      chunk,
			Score:      float64(hit.Score),
			Highlights: highlights,
		}

		results = append(results, rankedChunk)
	}

	return results, resp.Hits.Total.Value, nil
}

// Delete deletes chunks by IDs
func (s *SearchEngine) Delete(ctx context.Context, chunkIDs []string) error {
	if len(chunkIDs) == 0 {
		return nil
	}

	// Build bulk delete request
	var buf bytes.Buffer
	for _, id := range chunkIDs {
		action := map[string]any{
			"delete": map[string]any{
				"_index": s.indexName,
				"_id":    id,
			},
		}
		if err := json.NewEncoder(&buf).Encode(action); err != nil {
			return fmt.Errorf("failed to encode delete action: %w", err)
		}
	}

	// Execute bulk request
	req := opensearchapi.BulkReq{
		Body: bytes.NewReader(buf.Bytes()),
	}

	resp, err := s.client.Bulk(ctx, req)
	if err != nil {
		return fmt.Errorf("bulk delete failed: %w", err)
	}

	// Check for HTTP errors
	if httpResp := resp.Inspect().Response; httpResp != nil && httpResp.StatusCode != 200 {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("bulk delete failed with status %d: %s", httpResp.StatusCode, string(body))
	}

	return nil
}

// DeleteByDocument deletes all chunks for a document
func (s *SearchEngine) DeleteByDocument(ctx context.Context, documentID string) error {
	return s.deleteByQuery(ctx, map[string]any{
		"query": map[string]any{
			"term": map[string]any{
				"document_id": documentID,
			},
		},
	})
}

// DeleteByDocuments deletes all chunks for multiple documents in a single operation
func (s *SearchEngine) DeleteByDocuments(ctx context.Context, documentIDs []string) error {
	if len(documentIDs) == 0 {
		return nil
	}
	return s.deleteByQuery(ctx, map[string]any{
		"query": map[string]any{
			"terms": map[string]any{
				"document_id": documentIDs,
			},
		},
	})
}

// DeleteBySource deletes all chunks for a source
func (s *SearchEngine) DeleteBySource(ctx context.Context, sourceID string) error {
	return s.deleteByQuery(ctx, map[string]any{
		"query": map[string]any{
			"term": map[string]any{
				"source_id": sourceID,
			},
		},
	})
}

// DeleteBySourceAndContainer deletes all indexed data for a specific container within a source
// IMPLEMENTATION NOTE: Since container_id is not indexed in OpenSearch (chunks/documents only have
// source_id and document_id fields), we cannot filter directly by container_id.
// This is a limitation of the current schema - container_id lives in PostgreSQL document metadata only.
//
// For a proper implementation, the service layer should:
// 1. Query PostgreSQL for document IDs matching (source_id, container_id)
// 2. Call DeleteByDocuments() with those document IDs
//
// For now, this is a no-op placeholder. The service layer (deleteContainerData in source.go)
// will need to orchestrate the deletion by first querying document IDs, then calling
// DeleteByDocuments instead of this method.
func (s *SearchEngine) DeleteBySourceAndContainer(ctx context.Context, sourceID, containerID string) error {
	// TODO: This method cannot be properly implemented without container_id indexed in OpenSearch
	// The service layer must handle this by:
	// 1. Querying DocumentStore for document IDs where source_id=$1 AND metadata->>'container_id'=$2
	// 2. Calling SearchEngine.DeleteByDocuments(ctx, documentIDs)
	// For now, return nil (no-op) to satisfy the interface
	return nil
}

// HealthCheck verifies the search engine is available
func (s *SearchEngine) HealthCheck(ctx context.Context) error {
	resp, err := s.client.Cluster.Health(ctx, nil)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	// Check for HTTP errors
	if httpResp := resp.Inspect().Response; httpResp != nil && httpResp.StatusCode != 200 {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("health check failed with status %d: %s", httpResp.StatusCode, string(body))
	}

	return nil
}

// Count returns the total number of indexed chunks
func (s *SearchEngine) Count(ctx context.Context) (int64, error) {
	req := opensearchapi.IndicesCountReq{
		Indices: []string{s.indexName},
	}

	resp, err := s.client.Indices.Count(ctx, &req)
	if err != nil {
		return 0, fmt.Errorf("count failed: %w", err)
	}

	// Check for HTTP errors
	if httpResp := resp.Inspect().Response; httpResp != nil {
		if httpResp.StatusCode == 404 {
			// Index doesn't exist yet
			return 0, nil
		}
		if httpResp.StatusCode != 200 {
			body, _ := io.ReadAll(httpResp.Body)
			return 0, fmt.Errorf("count failed with status %d: %s", httpResp.StatusCode, string(body))
		}
	}

	return int64(resp.Count), nil
}

// ensureIndex creates the index with the correct mapping if it doesn't exist.
// The index stores full documents (not chunks) for BM25 text search.
func (s *SearchEngine) ensureIndex(ctx context.Context) error {
	// Check if index exists using low-level Do to avoid v4 client treating 404 as error.
	// IndicesExists returns 200 if the index exists, 404 if it doesn't — both are valid.
	existsResp, err := s.client.Client.Do(ctx, opensearchapi.IndicesExistsReq{
		Indices: []string{s.indexName},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to check index existence: %w", err)
	}
	defer func() { _ = existsResp.Body.Close() }()

	// Index exists (200)
	if existsResp.StatusCode == 200 {
		return nil
	}

	// Create index with document-level mapping
	mapping := map[string]any{
		"mappings": map[string]any{
			"properties": map[string]any{
				"document_id": map[string]any{
					"type": "keyword",
				},
				"source_id": map[string]any{
					"type": "keyword",
				},
				"title": map[string]any{
					"type":     "text",
					"analyzer": "standard",
				},
				"content": map[string]any{
					"type":     "text",
					"analyzer": "standard",
				},
				"path": map[string]any{
					"type": "keyword",
				},
				"mime_type": map[string]any{
					"type": "keyword",
				},
			},
		},
	}

	mappingBody, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("failed to marshal mapping: %w", err)
	}

	createReq := opensearchapi.IndicesCreateReq{
		Index: s.indexName,
		Body:  bytes.NewReader(mappingBody),
	}

	createResp, err := s.client.Indices.Create(ctx, createReq)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	// Check for HTTP errors
	if httpResp := createResp.Inspect().Response; httpResp != nil && httpResp.StatusCode != 200 {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("failed to create index with status %d: %s", httpResp.StatusCode, string(body))
	}

	return nil
}

// deleteByQuery is a helper for deleting documents matching a query.
// Uses wait_for_completion=false so OpenSearch processes deletes asynchronously,
// avoiding blocking the caller while the index is being cleaned up.
func (s *SearchEngine) deleteByQuery(ctx context.Context, query map[string]any) error {
	queryBody, err := json.Marshal(query)
	if err != nil {
		return fmt.Errorf("failed to marshal query: %w", err)
	}

	waitForCompletion := false
	req := opensearchapi.DocumentDeleteByQueryReq{
		Indices: []string{s.indexName},
		Body:    bytes.NewReader(queryBody),
		Params: opensearchapi.DocumentDeleteByQueryParams{
			WaitForCompletion: &waitForCompletion,
			Conflicts:         "proceed",
		},
	}

	resp, err := s.client.Document.DeleteByQuery(ctx, req)
	if err != nil {
		return fmt.Errorf("delete by query failed: %w", err)
	}

	// Check for HTTP errors
	if httpResp := resp.Inspect().Response; httpResp != nil {
		// 404 is OK - index doesn't exist, so nothing to delete
		if httpResp.StatusCode == 404 {
			return nil
		}
		// 200 (sync) and 202 (async task accepted) are both OK
		if httpResp.StatusCode != 200 && httpResp.StatusCode != 202 {
			body, _ := io.ReadAll(httpResp.Body)
			return fmt.Errorf("delete by query failed with status %d: %s", httpResp.StatusCode, string(body))
		}
	}

	return nil
}

// GetDocument retrieves a document by its document ID
func (s *SearchEngine) GetDocument(ctx context.Context, documentID string) (*domain.DocumentContent, error) {
	// Use low-level client to avoid typed response issues
	resp, err := s.client.Client.Do(ctx, opensearchapi.DocumentGetReq{
		Index:      s.indexName,
		DocumentID: documentID,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Handle 404 - document not found
	if resp.StatusCode == 404 {
		return nil, domain.ErrNotFound
	}

	// Handle other HTTP errors
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get document failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response body
	var result struct {
		Source map[string]any `json:"_source"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Map OpenSearch _source fields to domain.DocumentContent
	doc := &domain.DocumentContent{
		DocumentID: getString(result.Source, "document_id"),
		SourceID:   getString(result.Source, "source_id"),
		Title:      getString(result.Source, "title"),
		Body:       getString(result.Source, "content"),
		Path:       getString(result.Source, "path"),
		MimeType:   getString(result.Source, "mime_type"),
	}

	return doc, nil
}

// Helper functions to extract values from source map

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(m map[string]any, key string) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case int64:
			return int(val)
		case float64:
			return int(val)
		}
	}
	return 0
}

// isIndexNotFoundError checks if an opensearch error is due to a missing index.
// The v4 client returns errors for non-2xx responses, so 404 (index_not_found)
// arrives as an error rather than a response we can inspect.
func isIndexNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "index_not_found_exception") ||
		strings.Contains(msg, "no such index")
}
