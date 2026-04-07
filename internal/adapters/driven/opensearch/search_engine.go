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
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
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
		return nil, 0, fmt.Errorf("search failed: %w", err)
	}

	// Check for HTTP errors
	if httpResp := resp.Inspect().Response; httpResp != nil {
		if httpResp.StatusCode == 404 {
			// Index doesn't exist, return empty results
			return []*domain.RankedChunk{}, 0, nil
		}
		if httpResp.StatusCode != 200 {
			body, _ := io.ReadAll(httpResp.Body)
			return nil, 0, fmt.Errorf("search failed with status %d: %s", httpResp.StatusCode, string(body))
		}
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

// ensureIndex creates the index with the correct mapping if it doesn't exist
func (s *SearchEngine) ensureIndex(ctx context.Context) error {
	// Check if index exists
	req := opensearchapi.IndicesExistsReq{
		Indices: []string{s.indexName},
	}

	resp, err := s.client.Indices.Exists(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to check index existence: %w", err)
	}
	defer resp.Body.Close()

	// Index exists (200)
	if resp.StatusCode == 200 {
		return nil
	}

	// Create index with mapping
	mapping := map[string]any{
		"mappings": map[string]any{
			"properties": map[string]any{
				"id": map[string]any{
					"type": "keyword",
				},
				"document_id": map[string]any{
					"type": "keyword",
				},
				"source_id": map[string]any{
					"type": "keyword",
				},
				"content": map[string]any{
					"type":     "text",
					"analyzer": "standard",
				},
				"chunk_position": map[string]any{
					"type": "integer",
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

// deleteByQuery is a helper for deleting documents matching a query
func (s *SearchEngine) deleteByQuery(ctx context.Context, query map[string]any) error {
	queryBody, err := json.Marshal(query)
	if err != nil {
		return fmt.Errorf("failed to marshal query: %w", err)
	}

	req := opensearchapi.DocumentDeleteByQueryReq{
		Indices: []string{s.indexName},
		Body:    bytes.NewReader(queryBody),
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
		if httpResp.StatusCode != 200 {
			body, _ := io.ReadAll(httpResp.Body)
			return fmt.Errorf("delete by query failed with status %d: %s", httpResp.StatusCode, string(body))
		}
	}

	return nil
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
