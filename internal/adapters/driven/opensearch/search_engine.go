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
	// metadata is connector-supplied (author, labels, parent path, etc.).
	// Indexing it under the flattened mapping makes every value searchable
	// without per-key declarations. omitempty: a nil/empty map renders as
	// {} which OpenSearch accepts but inflates the inverted index.
	if len(doc.Metadata) > 0 {
		body["metadata"] = doc.Metadata
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
//
// Field weighting:
//
//	title^3            primary signal — the document's name
//	path.basename^3    filename-only signal (e.g. `auth.go`); same weight as
//	                   title since for code/file content the basename is
//	                   effectively the title
//	path.text^2        full-path tokens (`docs/k8s/deploy` matches a query
//	                   for "deploy" anywhere in the tree)
//	content            body text, default weight
//	metadata^1.5       connector-supplied attributes (author, labels, etc.)
//
// minimum_should_match: 75% requires the bulk of a multi-term query to
// match. For 1-2 word queries the percent rounds down to "all required";
// for 4+ word queries one term may be missing. Tightens precision on
// natural-language queries without hurting recall on short ones.
//
// Exact-title dominance is layered on top via a `should` clause in
// buildBoolEnvelope — a `term` query on title.raw with high boost so
// "Kubernetes Setup" exact-title-matches dominate fuzzy "kubernetes"
// matches across content.
func (s *SearchEngine) SearchDocuments(ctx context.Context, query string, opts domain.SearchOptions) ([]driven.DocumentResult, int, error) {
	// Bool.must is a logical AND across must clauses, so the loose match plus
	// each match_phrase clause all need to be satisfied. That's the right
	// semantics for quoted phrases — `merge sort "stable on equal keys"`
	// should require the phrase to appear, not just contribute score.
	matchFields := []string{
		"title^3",
		"path.basename^3",
		"path.text^2",
		"content",
		"metadata^1.5",
	}
	mustClauses := []any{
		map[string]any{
			"multi_match": map[string]any{
				"query":                query,
				"fields":               matchFields,
				"type":                 "most_fields",
				"fuzziness":            "AUTO", // 0 edits ≤2 chars, 1 edit 3-5, 2 edits 6+
				"minimum_should_match": "75%",
			},
		},
	}
	for _, phrase := range opts.Phrases {
		if strings.TrimSpace(phrase) == "" {
			continue
		}
		mustClauses = append(mustClauses, map[string]any{
			"multi_match": map[string]any{
				"query":  phrase,
				"fields": matchFields,
				"type":   "phrase",
			},
		})
	}

	searchQuery := s.buildBoolEnvelope(mustClauses, opts, query)
	return s.executeSearch(ctx, searchQuery)
}

// SearchByQueryDSL runs an arbitrary OpenSearch query body inside the
// adapter's standard bool envelope (filters, highlights, pagination).
// Caller owns the inner query shape — useful when the standard match-based
// retrieval doesn't fit (e.g. function_score, custom rescoring, or any
// other query DSL the existing methods don't expose).
//
// queryBody is wrapped as a single must clause; opts.SourceIDs and
// opts.DocumentIDFilter still apply as filter clauses, so the standard
// access boundaries are preserved. opts.BoostTerms still produce
// should clauses for score boosting. opts.Offset and opts.Limit drive
// from/size pagination.
//
// Not on the driven.SearchEngine port — callers reach this method via
// runtime type-assertion on the concrete *opensearch.SearchEngine, so
// alternative backends aren't forced to implement arbitrary OpenSearch
// DSL.
func (s *SearchEngine) SearchByQueryDSL(ctx context.Context, queryBody json.RawMessage, opts domain.SearchOptions) ([]driven.DocumentResult, int, error) {
	if len(queryBody) == 0 {
		return nil, 0, fmt.Errorf("queryBody is empty")
	}
	mustClauses := []any{json.RawMessage(queryBody)}
	// SearchByQueryDSL passes the caller's query verbatim — there is no
	// natural-language string to derive an exact-title boost from, so
	// pass an empty rawQuery to skip that should clause.
	searchQuery := s.buildBoolEnvelope(mustClauses, opts, "")
	return s.executeSearch(ctx, searchQuery)
}

// buildBoolEnvelope wraps a slice of must clauses in the standard bool
// query envelope: must + optional should (boost terms, exact-title boost)
// + optional filter (source/document filters), plus pagination and
// highlights. Shared by SearchDocuments and SearchByQueryDSL so they
// apply filters and highlights identically.
//
// rawQuery is the user's original query string used to build the
// exact-title-match should clause. SearchByQueryDSL passes "" because
// the caller's query is opaque DSL.
func (s *SearchEngine) buildBoolEnvelope(mustClauses []any, opts domain.SearchOptions, rawQuery string) map[string]any {
	boolQuery := map[string]any{
		"must": mustClauses,
	}

	var shouldClauses []any

	// Exact-title boost. A `term` query against the lowercase-normalised
	// title.raw subfield dominates fuzzy matches when the entire query
	// IS the title. Skipped for empty / whitespace-only queries (no
	// signal) and for SearchByQueryDSL (opaque caller query).
	if trimmed := strings.TrimSpace(rawQuery); trimmed != "" {
		shouldClauses = append(shouldClauses, map[string]any{
			"term": map[string]any{
				"title.raw": map[string]any{
					"value": strings.ToLower(trimmed),
					"boost": 10.0,
				},
			},
		})
	}

	if len(opts.BoostTerms) > 0 {
		for term, boost := range opts.BoostTerms {
			shouldClauses = append(shouldClauses, map[string]any{
				"multi_match": map[string]any{
					"query":  term,
					"fields": []string{"title", "content", "path.text", "path.basename", "metadata"},
					"boost":  boost,
				},
			})
		}
	}

	if len(shouldClauses) > 0 {
		boolQuery["should"] = shouldClauses
	}

	var filterClauses []any
	if len(opts.SourceIDs) > 0 {
		filterClauses = append(filterClauses, map[string]any{
			"terms": map[string]any{"source_id": opts.SourceIDs},
		})
	}
	// Three-case contract on opts.DocumentIDFilter:
	//   - nil or !Apply: no filter clause.
	//   - Apply && len(IDs) == 0: authoritative deny-all; emit a match-nothing clause.
	//   - Apply && len(IDs) > 0: allow-list on document_id.
	if f := opts.DocumentIDFilter; f != nil && f.Apply {
		if len(f.IDs) == 0 {
			filterClauses = append(filterClauses, map[string]any{
				"ids": map[string]any{"values": []string{}},
			})
		} else {
			filterClauses = append(filterClauses, map[string]any{
				"terms": map[string]any{"document_id": f.IDs},
			})
		}
	}
	if len(filterClauses) > 0 {
		boolQuery["filter"] = filterClauses
	}

	return map[string]any{
		"query": map[string]any{"bool": boolQuery},
		"from":  opts.Offset,
		"size":  opts.Limit,
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
}

// executeSearch marshals the prepared query, hits OpenSearch, and parses
// the typed response into []DocumentResult. Index-not-found errors are
// translated to an empty result set so a fresh deployment doesn't 500
// before the first document is indexed.
func (s *SearchEngine) executeSearch(ctx context.Context, searchQuery map[string]any) ([]driven.DocumentResult, int, error) {
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

	// Create index with document-level mapping.
	//
	// The english analyser lowercases, strips english stop words, and applies
	// porter stemming — so "authentication" indexes as "authent" and a query
	// for "auth" or "authenticate" still matches. The previous standard
	// analyser only lowercased + tokenised, leaving morphological variants
	// unreachable without LLM query expansion.
	//
	// title.raw is a keyword sub-field for exact-match deduplication and
	// future sort-by-title support. _id-based dedup is already covered by
	// document_id; .raw is for cases where two distinct documents share a
	// title (e.g. a Notion duplicate page).
	// Custom analyzers / normalizers:
	//
	//   path_analyzer       Tokenises `docs/k8s/deploy.md` on `/`, `-`, `_`, `.`
	//                       and lowercases. Lets a query "deploy" hit a path
	//                       containing "/deploy.md" without forcing the user
	//                       to know the full directory.
	//   basename_analyzer   Strips the directory prefix via a pattern_replace
	//                       char filter, then tokenises the remainder. Used
	//                       on the path.basename subfield so a query "auth"
	//                       lands on `internal/auth/handlers.go` regardless
	//                       of where in the tree it sits.
	//   lowercase_keyword   Normaliser on title.raw so `term` queries against
	//                       it are case-insensitive (the exact-title boost
	//                       below relies on this).
	settings := map[string]any{
		"analysis": map[string]any{
			"char_filter": map[string]any{
				"strip_dir": map[string]any{
					"type":        "pattern_replace",
					"pattern":     ".*/",
					"replacement": "",
				},
			},
			"tokenizer": map[string]any{
				"path_punct": map[string]any{
					"type":    "pattern",
					"pattern": "[/_\\-.]+",
				},
			},
			"analyzer": map[string]any{
				"path_analyzer": map[string]any{
					"type":      "custom",
					"tokenizer": "path_punct",
					"filter":    []string{"lowercase"},
				},
				"basename_analyzer": map[string]any{
					"type":        "custom",
					"char_filter": []string{"strip_dir"},
					"tokenizer":   "path_punct",
					"filter":      []string{"lowercase"},
				},
			},
			"normalizer": map[string]any{
				"lowercase_keyword": map[string]any{
					"type":   "custom",
					"filter": []string{"lowercase"},
				},
			},
		},
	}

	mapping := map[string]any{
		"settings": settings,
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
					"analyzer": "english",
					"fields": map[string]any{
						// keyword subfield, lowercased so case-insensitive
						// `term` queries dominate exact-title hits without
						// requiring callers to lowercase upstream.
						"raw": map[string]any{
							"type":       "keyword",
							"normalizer": "lowercase_keyword",
						},
					},
				},
				"content": map[string]any{
					"type":     "text",
					"analyzer": "english",
				},
				// Multi-field on path:
				//   path           keyword (filtering, sort, exact match)
				//   path.text      analyzed (full-text against path tokens)
				//   path.basename  filename-only (highest-signal subfield;
				//                  matches `auth.go` regardless of directory)
				"path": map[string]any{
					"type": "keyword",
					"fields": map[string]any{
						"text": map[string]any{
							"type":     "text",
							"analyzer": "path_analyzer",
						},
						"basename": map[string]any{
							"type":     "text",
							"analyzer": "basename_analyzer",
						},
					},
				},
				// Connector-supplied metadata is heterogeneous (author,
				// labels, repo name, parent path, etc.). flattened indexes
				// every value under a single inverted index so all metadata
				// is searchable without per-key mapping. Scoring is coarser
				// than per-field analysis, but the alternative — leaving
				// metadata out of search entirely as it was before — is
				// strictly worse.
				"metadata": map[string]any{
					"type": "flattened",
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
