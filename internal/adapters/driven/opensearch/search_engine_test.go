package opensearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// TestDefaultConfig validates the default configuration
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.URL != "http://localhost:9200" {
		t.Errorf("URL = %v, want http://localhost:9200", cfg.URL)
	}
	if cfg.IndexName != "sercha_chunks" {
		t.Errorf("IndexName = %v, want sercha_chunks", cfg.IndexName)
	}
	if cfg.Timeout.Seconds() != 30 {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
	if cfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false by default")
	}
}

// TestNewSearchEngine validates search engine creation
func TestNewSearchEngine(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid default config",
			cfg: Config{
				URL:       "http://localhost:9200",
				IndexName: "test_index",
				Timeout:   30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "custom index name",
			cfg: Config{
				URL:       "http://localhost:9200",
				IndexName: "custom_chunks",
				Timeout:   10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "with TLS skip verify",
			cfg: Config{
				URL:                "https://localhost:9200",
				IndexName:          "secure_index",
				Timeout:            30 * time.Second,
				InsecureSkipVerify: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := NewSearchEngine(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewSearchEngine() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if engine.indexName != tt.cfg.IndexName {
					t.Errorf("indexName = %v, want %v", engine.indexName, tt.cfg.IndexName)
				}
				if engine.timeout != tt.cfg.Timeout {
					t.Errorf("timeout = %v, want %v", engine.timeout, tt.cfg.Timeout)
				}
			}
		})
	}
}

// TestSearchEngine_Index validates indexing operations
func TestSearchEngine_Index(t *testing.T) {
	tests := []struct {
		name        string
		chunks      []*domain.Chunk
		setupServer func() *httptest.Server
		wantErr     bool
		errContains string
	}{
		{
			name:   "empty chunks should succeed",
			chunks: []*domain.Chunk{},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					t.Error("should not make any request for empty chunks")
				}))
			},
			wantErr: false,
		},
		{
			name: "single chunk indexed successfully",
			chunks: []*domain.Chunk{
				{
					ID:         "chunk-1",
					DocumentID: "doc-1",
					SourceID:   "source-1",
					Content:    "Test content",
					Position:   0,
				},
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Handle index exists check
					if r.Method == "HEAD" && strings.Contains(r.URL.Path, "sercha_chunks") {
						w.WriteHeader(http.StatusOK)
						return
					}
					// Handle bulk request
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_bulk") {
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"errors": false,
							"items":  []any{},
						})
						return
					}
					w.WriteHeader(http.StatusOK)
				}))
			},
			wantErr: false,
		},
		{
			name: "multiple chunks indexed successfully",
			chunks: []*domain.Chunk{
				{
					ID:         "chunk-1",
					DocumentID: "doc-1",
					SourceID:   "source-1",
					Content:    "First chunk",
					Position:   0,
				},
				{
					ID:         "chunk-2",
					DocumentID: "doc-1",
					SourceID:   "source-1",
					Content:    "Second chunk",
					Position:   1,
				},
				{
					ID:         "chunk-3",
					DocumentID: "doc-2",
					SourceID:   "source-1",
					Content:    "Third chunk",
					Position:   0,
				},
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "HEAD" && strings.Contains(r.URL.Path, "sercha_chunks") {
						w.WriteHeader(http.StatusOK)
						return
					}
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_bulk") {
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"errors": false,
							"items":  []any{},
						})
						return
					}
					w.WriteHeader(http.StatusOK)
				}))
			},
			wantErr: false,
		},
		{
			name: "chunk with special characters",
			chunks: []*domain.Chunk{
				{
					ID:         "chunk-special",
					DocumentID: "doc-1",
					SourceID:   "source-1",
					Content:    "Content with \"quotes\" and \n newlines",
					Position:   0,
				},
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "HEAD" && strings.Contains(r.URL.Path, "sercha_chunks") {
						w.WriteHeader(http.StatusOK)
						return
					}
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_bulk") {
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"errors": false,
							"items":  []any{},
						})
						return
					}
					w.WriteHeader(http.StatusOK)
				}))
			},
			wantErr: false,
		},
		{
			name: "bulk index with errors",
			chunks: []*domain.Chunk{
				{ID: "chunk-1", Content: "Test"},
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "HEAD" {
						w.WriteHeader(http.StatusOK)
						return
					}
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_bulk") {
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"errors": true,
							"items": []map[string]any{
								{
									"index": map[string]any{
										"error": map[string]any{
											"type":   "mapper_parsing_exception",
											"reason": "failed to parse field",
										},
									},
								},
							},
						})
						return
					}
				}))
			},
			wantErr:     true,
			errContains: "bulk index had errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := tt.setupServer()
			defer ts.Close()

			cfg := Config{
				URL:       ts.URL,
				IndexName: "sercha_chunks",
				Timeout:   5 * time.Second,
			}

			engine, err := NewSearchEngine(cfg)
			if err != nil {
				t.Fatalf("NewSearchEngine() error = %v", err)
			}

			err = engine.Index(context.Background(), tt.chunks)
			if (err != nil) != tt.wantErr {
				t.Errorf("Index() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Index() error = %v, want error containing %v", err, tt.errContains)
				}
			}
		})
	}
}

// TestSearchEngine_Search validates BM25 search operations
func TestSearchEngine_Search(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		opts          domain.SearchOptions
		setupServer   func() *httptest.Server
		wantCount     int
		wantTotal     int
		wantErr       bool
		errContains   string
		validateFirst func(*testing.T, *domain.RankedChunk)
	}{
		{
			name:  "successful BM25 search",
			query: "test query",
			opts: domain.SearchOptions{
				Limit:  10,
				Offset: 0,
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{
									"value": 2,
								},
								"hits": []map[string]any{
									{
										"_id":    "chunk-1",
										"_score": 1.5,
										"_source": map[string]any{
											"id":             "chunk-1",
											"document_id":    "doc-1",
											"source_id":      "source-1",
											"content":        "This is a test query result",
											"chunk_position": 0,
										},
										"highlight": map[string][]string{
											"content": {"This is a <em>test</em> <em>query</em> result"},
										},
									},
									{
										"_id":    "chunk-2",
										"_score": 1.2,
										"_source": map[string]any{
											"id":             "chunk-2",
											"document_id":    "doc-1",
											"source_id":      "source-1",
											"content":        "Another test document",
											"chunk_position": 1,
										},
									},
								},
							},
						})
						return
					}
				}))
			},
			wantCount: 2,
			wantTotal: 2,
			wantErr:   false,
			validateFirst: func(t *testing.T, rc *domain.RankedChunk) {
				if rc.Chunk.ID != "chunk-1" {
					t.Errorf("First result ID = %v, want chunk-1", rc.Chunk.ID)
				}
				if rc.Score != 1.5 {
					t.Errorf("First result score = %v, want 1.5", rc.Score)
				}
				if len(rc.Highlights) == 0 {
					t.Error("Expected highlights but got none")
				}
			},
		},
		{
			name:  "search with source filter",
			query: "filtered query",
			opts: domain.SearchOptions{
				Limit:     10,
				Offset:    0,
				SourceIDs: []string{"source-1", "source-2"},
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						// Verify filter is present in request
						var reqBody map[string]any
						_ = json.NewDecoder(r.Body).Decode(&reqBody)
						query := reqBody["query"].(map[string]any)
						boolQuery := query["bool"].(map[string]any)
						if _, ok := boolQuery["filter"]; !ok {
							t.Error("Expected filter in query")
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{"value": 1},
								"hits": []map[string]any{
									{
										"_id":    "chunk-1",
										"_score": 1.0,
										"_source": map[string]any{
											"id":             "chunk-1",
											"document_id":    "doc-1",
											"source_id":      "source-1",
											"content":        "Filtered content",
											"chunk_position": 0,
										},
									},
								},
							},
						})
						return
					}
				}))
			},
			wantCount: 1,
			wantTotal: 1,
			wantErr:   false,
		},
		{
			name:  "search with pagination",
			query: "paginated",
			opts: domain.SearchOptions{
				Limit:  5,
				Offset: 10,
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						// Verify pagination in request
						var reqBody map[string]any
						_ = json.NewDecoder(r.Body).Decode(&reqBody)
						if reqBody["from"].(float64) != 10 {
							t.Errorf("from = %v, want 10", reqBody["from"])
						}
						if reqBody["size"].(float64) != 5 {
							t.Errorf("size = %v, want 5", reqBody["size"])
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{"value": 50},
								"hits":  []map[string]any{},
							},
						})
						return
					}
				}))
			},
			wantCount: 0,
			wantTotal: 50,
			wantErr:   false,
		},
		{
			name:  "search returns empty results",
			query: "test",
			opts: domain.SearchOptions{
				Limit:  10,
				Offset: 0,
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{"value": 0},
								"hits":  []map[string]any{},
							},
						})
						return
					}
				}))
			},
			wantCount: 0,
			wantTotal: 0,
			wantErr:   false,
		},
		{
			name:  "search error",
			query: "error query",
			opts: domain.SearchOptions{
				Limit:  10,
				Offset: 0,
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						w.WriteHeader(http.StatusInternalServerError)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"error": "search failed",
						})
						return
					}
				}))
			},
			wantErr:     true,
			errContains: "search failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := tt.setupServer()
			defer ts.Close()

			cfg := Config{
				URL:       ts.URL,
				IndexName: "sercha_chunks",
				Timeout:   5 * time.Second,
			}

			engine, err := NewSearchEngine(cfg)
			if err != nil {
				t.Fatalf("NewSearchEngine() error = %v", err)
			}

			results, total, err := engine.Search(context.Background(), tt.query, nil, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Search() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Search() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}
			if !tt.wantErr {
				if len(results) != tt.wantCount {
					t.Errorf("Search() returned %d results, want %d", len(results), tt.wantCount)
				}
				if total != tt.wantTotal {
					t.Errorf("Search() total = %d, want %d", total, tt.wantTotal)
				}
				if tt.validateFirst != nil && len(results) > 0 {
					tt.validateFirst(t, results[0])
				}
			}
		})
	}
}

// TestSearchEngine_Delete validates chunk deletion
func TestSearchEngine_Delete(t *testing.T) {
	tests := []struct {
		name        string
		chunkIDs    []string
		setupServer func() *httptest.Server
		wantErr     bool
		errContains string
	}{
		{
			name:     "empty IDs should succeed",
			chunkIDs: []string{},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					t.Error("should not make any request for empty IDs")
				}))
			},
			wantErr: false,
		},
		{
			name:     "delete single chunk",
			chunkIDs: []string{"chunk-1"},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_bulk") {
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"errors": false,
							"items":  []any{},
						})
						return
					}
				}))
			},
			wantErr: false,
		},
		{
			name:     "delete multiple chunks",
			chunkIDs: []string{"chunk-1", "chunk-2", "chunk-3"},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_bulk") {
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"errors": false,
							"items":  []any{},
						})
						return
					}
				}))
			},
			wantErr: false,
		},
		{
			name:     "delete failure",
			chunkIDs: []string{"chunk-1"},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_bulk") {
						w.WriteHeader(http.StatusInternalServerError)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"error": "delete failed",
						})
						return
					}
				}))
			},
			wantErr:     true,
			errContains: "bulk delete failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := tt.setupServer()
			defer ts.Close()

			cfg := Config{
				URL:       ts.URL,
				IndexName: "sercha_chunks",
				Timeout:   5 * time.Second,
			}

			engine, err := NewSearchEngine(cfg)
			if err != nil {
				t.Fatalf("NewSearchEngine() error = %v", err)
			}

			err = engine.Delete(context.Background(), tt.chunkIDs)
			if (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Delete() error = %v, want error containing %v", err, tt.errContains)
				}
			}
		})
	}
}

// TestSearchEngine_DeleteByDocument validates deletion by document ID
func TestSearchEngine_DeleteByDocument(t *testing.T) {
	tests := []struct {
		name        string
		documentID  string
		setupServer func() *httptest.Server
		wantErr     bool
		errContains string
	}{
		{
			name:       "successful deletion",
			documentID: "doc-1",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_delete_by_query") {
						// Verify query contains document_id term
						var reqBody map[string]any
						_ = json.NewDecoder(r.Body).Decode(&reqBody)
						query := reqBody["query"].(map[string]any)
						term := query["term"].(map[string]any)
						if _, ok := term["document_id"]; !ok {
							t.Error("Expected document_id in term query")
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"deleted": 5,
						})
						return
					}
				}))
			},
			wantErr: false,
		},
		{
			name:       "empty document ID",
			documentID: "",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_delete_by_query") {
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"deleted": 0,
						})
						return
					}
				}))
			},
			wantErr: false,
		},
		{
			name:       "deletion error",
			documentID: "doc-1",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_delete_by_query") {
						w.WriteHeader(http.StatusInternalServerError)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"error": "deletion failed",
						})
						return
					}
				}))
			},
			wantErr:     true,
			errContains: "delete by query failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := tt.setupServer()
			defer ts.Close()

			cfg := Config{
				URL:       ts.URL,
				IndexName: "sercha_chunks",
				Timeout:   5 * time.Second,
			}

			engine, err := NewSearchEngine(cfg)
			if err != nil {
				t.Fatalf("NewSearchEngine() error = %v", err)
			}

			err = engine.DeleteByDocument(context.Background(), tt.documentID)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteByDocument() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("DeleteByDocument() error = %v, want error containing %v", err, tt.errContains)
				}
			}
		})
	}
}

// TestSearchEngine_DeleteBySource validates deletion by source ID
func TestSearchEngine_DeleteBySource(t *testing.T) {
	tests := []struct {
		name        string
		sourceID    string
		setupServer func() *httptest.Server
		wantErr     bool
		errContains string
	}{
		{
			name:     "successful deletion",
			sourceID: "source-1",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_delete_by_query") {
						// Verify query contains source_id term
						var reqBody map[string]any
						_ = json.NewDecoder(r.Body).Decode(&reqBody)
						query := reqBody["query"].(map[string]any)
						term := query["term"].(map[string]any)
						if _, ok := term["source_id"]; !ok {
							t.Error("Expected source_id in term query")
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"deleted": 10,
						})
						return
					}
				}))
			},
			wantErr: false,
		},
		{
			name:     "empty source ID",
			sourceID: "",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_delete_by_query") {
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"deleted": 0,
						})
						return
					}
				}))
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := tt.setupServer()
			defer ts.Close()

			cfg := Config{
				URL:       ts.URL,
				IndexName: "sercha_chunks",
				Timeout:   5 * time.Second,
			}

			engine, err := NewSearchEngine(cfg)
			if err != nil {
				t.Fatalf("NewSearchEngine() error = %v", err)
			}

			err = engine.DeleteBySource(context.Background(), tt.sourceID)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteBySource() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("DeleteBySource() error = %v, want error containing %v", err, tt.errContains)
				}
			}
		})
	}
}

// TestSearchEngine_HealthCheck validates health check functionality
func TestSearchEngine_HealthCheck(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func() *httptest.Server
		wantErr     bool
		errContains string
	}{
		{
			name: "healthy cluster",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "GET" && strings.Contains(r.URL.Path, "_cluster/health") {
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"cluster_name": "test-cluster",
							"status":       "green",
						})
						return
					}
				}))
			},
			wantErr: false,
		},
		{
			name: "cluster warning status still healthy",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "GET" && strings.Contains(r.URL.Path, "_cluster/health") {
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"cluster_name": "test-cluster",
							"status":       "yellow",
						})
						return
					}
				}))
			},
			wantErr: false,
		},
		{
			name: "health check error",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "GET" && strings.Contains(r.URL.Path, "_cluster/health") {
						w.WriteHeader(http.StatusServiceUnavailable)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"error": "cluster unavailable",
						})
						return
					}
				}))
			},
			wantErr:     true,
			errContains: "health check failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := tt.setupServer()
			defer ts.Close()

			cfg := Config{
				URL:       ts.URL,
				IndexName: "sercha_chunks",
				Timeout:   5 * time.Second,
			}

			engine, err := NewSearchEngine(cfg)
			if err != nil {
				t.Fatalf("NewSearchEngine() error = %v", err)
			}

			err = engine.HealthCheck(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("HealthCheck() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("HealthCheck() error = %v, want error containing %v", err, tt.errContains)
				}
			}
		})
	}
}

// TestSearchEngine_Count validates count functionality
func TestSearchEngine_Count(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func() *httptest.Server
		wantCount   int64
		wantErr     bool
		errContains string
	}{
		{
			name: "successful count",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_count") {
						w.WriteHeader(http.StatusOK)
						w.Header().Set("Content-Type", "application/json")
						_ = json.NewEncoder(w).Encode(map[string]any{
							"count": 42,
							"_shards": map[string]any{
								"total":      1,
								"successful": 1,
								"skipped":    0,
								"failed":     0,
							},
						})
						return
					}
				}))
			},
			wantCount: 42,
			wantErr:   false,
		},
		{
			name: "zero count",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_count") {
						w.WriteHeader(http.StatusOK)
						w.Header().Set("Content-Type", "application/json")
						_ = json.NewEncoder(w).Encode(map[string]any{
							"count": 0,
							"_shards": map[string]any{
								"total":      1,
								"successful": 1,
								"skipped":    0,
								"failed":     0,
							},
						})
						return
					}
				}))
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "count error",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "GET" && strings.Contains(r.URL.Path, "_count") {
						w.WriteHeader(http.StatusInternalServerError)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"error": "count failed",
						})
						return
					}
				}))
			},
			wantErr:     true,
			errContains: "count failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := tt.setupServer()
			defer ts.Close()

			cfg := Config{
				URL:       ts.URL,
				IndexName: "sercha_chunks",
				Timeout:   5 * time.Second,
			}

			engine, err := NewSearchEngine(cfg)
			if err != nil {
				t.Fatalf("NewSearchEngine() error = %v", err)
			}

			count, err := engine.Count(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Count() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && count != tt.wantCount {
				t.Errorf("Count() = %v, want %v", count, tt.wantCount)
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Count() error = %v, want error containing %v", err, tt.errContains)
				}
			}
		})
	}
}

// TestGetString validates string extraction helper
func TestGetString(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want string
	}{
		{
			name: "existing string",
			m:    map[string]any{"key": "value"},
			key:  "key",
			want: "value",
		},
		{
			name: "missing key",
			m:    map[string]any{},
			key:  "key",
			want: "",
		},
		{
			name: "non-string value",
			m:    map[string]any{"key": 123},
			key:  "key",
			want: "",
		},
		{
			name: "nil map",
			m:    nil,
			key:  "key",
			want: "",
		},
		{
			name: "empty string value",
			m:    map[string]any{"key": ""},
			key:  "key",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getString(tt.m, tt.key)
			if got != tt.want {
				t.Errorf("getString() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetInt validates integer extraction helper
func TestGetInt(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want int
	}{
		{
			name: "int value",
			m:    map[string]any{"key": 42},
			key:  "key",
			want: 42,
		},
		{
			name: "int64 value",
			m:    map[string]any{"key": int64(42)},
			key:  "key",
			want: 42,
		},
		{
			name: "float64 value",
			m:    map[string]any{"key": float64(42.5)},
			key:  "key",
			want: 42,
		},
		{
			name: "missing key",
			m:    map[string]any{},
			key:  "key",
			want: 0,
		},
		{
			name: "string value",
			m:    map[string]any{"key": "42"},
			key:  "key",
			want: 0,
		},
		{
			name: "nil map",
			m:    nil,
			key:  "key",
			want: 0,
		},
		{
			name: "zero value",
			m:    map[string]any{"key": 0},
			key:  "key",
			want: 0,
		},
		{
			name: "negative value",
			m:    map[string]any{"key": -10},
			key:  "key",
			want: -10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getInt(tt.m, tt.key)
			if got != tt.want {
				t.Errorf("getInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSearchEngine_GetDocument validates document retrieval by ID
func TestSearchEngine_GetDocument(t *testing.T) {
	tests := []struct {
		name        string
		documentID  string
		setupServer func() *httptest.Server
		wantErr     bool
		errContains string
		validate    func(*testing.T, *domain.DocumentContent)
	}{
		{
			name:       "successful retrieval",
			documentID: "doc-1",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "GET" && strings.Contains(r.URL.Path, "/sercha_chunks/_doc/doc-1") {
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"_index": "sercha_chunks",
							"_id":    "doc-1",
							"found":  true,
							"_source": map[string]any{
								"document_id": "doc-1",
								"source_id":   "source-1",
								"title":       "Test Document",
								"content":     "Full document body text",
								"path":        "/test/path.md",
								"mime_type":   "text/markdown",
							},
						})
						return
					}
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			wantErr: false,
			validate: func(t *testing.T, doc *domain.DocumentContent) {
				if doc.DocumentID != "doc-1" {
					t.Errorf("DocumentID = %q, want doc-1", doc.DocumentID)
				}
				if doc.SourceID != "source-1" {
					t.Errorf("SourceID = %q, want source-1", doc.SourceID)
				}
				if doc.Title != "Test Document" {
					t.Errorf("Title = %q, want Test Document", doc.Title)
				}
				if doc.Body != "Full document body text" {
					t.Errorf("Body = %q, want Full document body text", doc.Body)
				}
				if doc.Path != "/test/path.md" {
					t.Errorf("Path = %q, want /test/path.md", doc.Path)
				}
				if doc.MimeType != "text/markdown" {
					t.Errorf("MimeType = %q, want text/markdown", doc.MimeType)
				}
			},
		},
		{
			name:       "document not found",
			documentID: "non-existent",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "GET" && strings.Contains(r.URL.Path, "_doc") {
						w.WriteHeader(http.StatusNotFound)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"found": false,
						})
						return
					}
					w.WriteHeader(http.StatusOK)
				}))
			},
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:       "server error",
			documentID: "doc-1",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "GET" && strings.Contains(r.URL.Path, "_doc") {
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = w.Write([]byte(`{"error": "internal error"}`))
						return
					}
					w.WriteHeader(http.StatusOK)
				}))
			},
			wantErr:     true,
			errContains: "get document failed",
		},
		{
			name:       "document with empty fields",
			documentID: "doc-empty",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "GET" && strings.Contains(r.URL.Path, "_doc") {
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"_id":   "doc-empty",
							"found": true,
							"_source": map[string]any{
								"document_id": "doc-empty",
								"source_id":   "",
								"title":       "",
								"content":     "",
							},
						})
						return
					}
					w.WriteHeader(http.StatusOK)
				}))
			},
			wantErr: false,
			validate: func(t *testing.T, doc *domain.DocumentContent) {
				if doc.DocumentID != "doc-empty" {
					t.Errorf("DocumentID = %q, want doc-empty", doc.DocumentID)
				}
				if doc.Body != "" {
					t.Errorf("Body should be empty, got %q", doc.Body)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := tt.setupServer()
			defer ts.Close()

			cfg := Config{
				URL:       ts.URL,
				IndexName: "sercha_chunks",
				Timeout:   5 * time.Second,
			}

			engine, err := NewSearchEngine(cfg)
			if err != nil {
				t.Fatalf("NewSearchEngine() error = %v", err)
			}

			doc, err := engine.GetDocument(context.Background(), tt.documentID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDocument() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("GetDocument() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}
			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, doc)
			}
		})
	}
}

// TestSearchEngine_SearchDocuments_WithDocumentIDFilter validates document ID filtering
func TestSearchEngine_SearchDocuments_WithDocumentIDFilter(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		opts        domain.SearchOptions
		setupServer func(*testing.T) *httptest.Server
		wantCount   int
		wantTotal   int
		wantErr     bool
	}{
		{
			name:  "search with document ID filter",
			query: "test query",
			opts: domain.SearchOptions{
				Limit:            10,
				Offset:           0,
				DocumentIDFilter: domain.AllowDocumentIDs([]string{"doc-1", "doc-2"}),
			},
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						// Verify document_id filter is present in request
						var reqBody map[string]any
						_ = json.NewDecoder(r.Body).Decode(&reqBody)
						query := reqBody["query"].(map[string]any)
						boolQuery := query["bool"].(map[string]any)

						filterClauses, ok := boolQuery["filter"].([]any)
						if !ok {
							t.Error("Expected filter in query")
						}

						// Verify document_id terms filter exists
						foundDocIDFilter := false
						for _, clause := range filterClauses {
							if clauseMap, ok := clause.(map[string]any); ok {
								if terms, ok := clauseMap["terms"].(map[string]any); ok {
									if _, ok := terms["document_id"]; ok {
										foundDocIDFilter = true
										break
									}
								}
							}
						}
						if !foundDocIDFilter {
							t.Error("Expected document_id terms filter in query")
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{"value": 2},
								"hits": []map[string]any{
									{
										"_id":    "doc-1",
										"_score": 1.5,
										"_source": map[string]any{
											"document_id": "doc-1",
											"source_id":   "source-1",
											"title":       "First Document",
											"content":     "First test content",
										},
									},
									{
										"_id":    "doc-2",
										"_score": 1.2,
										"_source": map[string]any{
											"document_id": "doc-2",
											"source_id":   "source-1",
											"title":       "Second Document",
											"content":     "Second test content",
										},
									},
								},
							},
						})
						return
					}
				}))
			},
			wantCount: 2,
			wantTotal: 2,
			wantErr:   false,
		},
		{
			name:  "search with source ID and document ID filters combined",
			query: "test query",
			opts: domain.SearchOptions{
				Limit:            10,
				Offset:           0,
				SourceIDs:        []string{"source-1"},
				DocumentIDFilter: domain.AllowDocumentIDs([]string{"doc-1", "doc-2", "doc-3"}),
			},
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						// Verify both source_id and document_id filters are present
						var reqBody map[string]any
						_ = json.NewDecoder(r.Body).Decode(&reqBody)
						query := reqBody["query"].(map[string]any)
						boolQuery := query["bool"].(map[string]any)

						filterClauses, ok := boolQuery["filter"].([]any)
						if !ok || len(filterClauses) != 2 {
							t.Errorf("Expected 2 filter clauses, got %d", len(filterClauses))
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{"value": 1},
								"hits": []map[string]any{
									{
										"_id":    "doc-1",
										"_score": 1.5,
										"_source": map[string]any{
											"document_id": "doc-1",
											"source_id":   "source-1",
											"title":       "Filtered Document",
											"content":     "Content",
										},
									},
								},
							},
						})
						return
					}
				}))
			},
			wantCount: 1,
			wantTotal: 1,
			wantErr:   false,
		},
		{
			name:  "search without document ID filter",
			query: "test query",
			opts: domain.SearchOptions{
				Limit:  10,
				Offset: 0,
			},
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						// Nil DocumentIDFilter: no document_id clause and no ids clause.
						var reqBody map[string]any
						_ = json.NewDecoder(r.Body).Decode(&reqBody)
						query := reqBody["query"].(map[string]any)
						boolQuery := query["bool"].(map[string]any)

						if filterClauses, ok := boolQuery["filter"].([]any); ok {
							for _, clause := range filterClauses {
								if clauseMap, ok := clause.(map[string]any); ok {
									if terms, ok := clauseMap["terms"].(map[string]any); ok {
										if _, ok := terms["document_id"]; ok {
											t.Error("nil DocumentIDFilter should not emit a document_id terms clause")
										}
									}
									if _, ok := clauseMap["ids"]; ok {
										t.Error("nil DocumentIDFilter should not emit an ids match-nothing clause")
									}
								}
							}
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{"value": 5},
								"hits": []map[string]any{
									{
										"_id":    "doc-x",
										"_score": 1.0,
										"_source": map[string]any{
											"document_id": "doc-x",
											"source_id":   "source-1",
											"title":       "Unfiltered",
											"content":     "Content",
										},
									},
								},
							},
						})
						return
					}
				}))
			},
			wantCount: 1,
			wantTotal: 5,
			wantErr:   false,
		},
		{
			// Deny-all: Apply=true with empty IDs. The adapter must emit a
			// match-nothing clause ("ids":{"values":[]}) so the overall bool
			// query returns zero hits regardless of other clauses — NOT silently
			// skip the filter (that was the fail-open hole this test codifies).
			name:  "deny-all document ID filter emits match-nothing clause",
			query: "test query",
			opts: domain.SearchOptions{
				Limit:            10,
				Offset:           0,
				DocumentIDFilter: domain.DenyAllDocumentIDFilter(),
			},
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						var reqBody map[string]any
						_ = json.NewDecoder(r.Body).Decode(&reqBody)
						query := reqBody["query"].(map[string]any)
						boolQuery := query["bool"].(map[string]any)

						filterClauses, ok := boolQuery["filter"].([]any)
						if !ok {
							t.Fatal("Expected filter clauses for deny-all; got none")
						}

						// Find the ids.values match-nothing clause.
						foundMatchNothing := false
						for _, clause := range filterClauses {
							clauseMap, ok := clause.(map[string]any)
							if !ok {
								continue
							}
							// Deny-all must NOT emit terms.document_id — that would be fail-open.
							if terms, ok := clauseMap["terms"].(map[string]any); ok {
								if _, ok := terms["document_id"]; ok {
									t.Error("deny-all must NOT emit a terms.document_id clause (fail-open regression)")
								}
							}
							ids, ok := clauseMap["ids"].(map[string]any)
							if !ok {
								continue
							}
							values, ok := ids["values"].([]any)
							if !ok {
								t.Errorf("ids.values should be an array, got %T", ids["values"])
								continue
							}
							if len(values) != 0 {
								t.Errorf("ids.values should be empty for deny-all, got %v", values)
							}
							foundMatchNothing = true
						}
						if !foundMatchNothing {
							t.Errorf("deny-all filter must emit ids.values match-nothing clause; filter clauses = %+v", filterClauses)
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{"value": 0},
								"hits":  []map[string]any{},
							},
						})
						return
					}
				}))
			},
			wantCount: 0,
			wantTotal: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := tt.setupServer(t)
			defer ts.Close()

			cfg := Config{
				URL:       ts.URL,
				IndexName: "sercha_chunks",
				Timeout:   5 * time.Second,
			}

			engine, err := NewSearchEngine(cfg)
			if err != nil {
				t.Fatalf("NewSearchEngine() error = %v", err)
			}

			results, total, err := engine.SearchDocuments(context.Background(), tt.query, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("SearchDocuments() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(results) != tt.wantCount {
					t.Errorf("SearchDocuments() returned %d results, want %d", len(results), tt.wantCount)
				}
				if total != tt.wantTotal {
					t.Errorf("SearchDocuments() total = %d, want %d", total, tt.wantTotal)
				}
			}
		})
	}
}

// TestSearchEngine_Search_WithDocumentIDFilter validates document ID filtering in chunk search
func TestSearchEngine_Search_WithDocumentIDFilter(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		opts        domain.SearchOptions
		setupServer func(*testing.T) *httptest.Server
		wantCount   int
		wantErr     bool
	}{
		{
			name:  "chunk search with document ID filter",
			query: "test query",
			opts: domain.SearchOptions{
				Limit:            10,
				Offset:           0,
				DocumentIDFilter: domain.AllowDocumentIDs([]string{"doc-1"}),
			},
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						// Verify document_id filter is present
						var reqBody map[string]any
						_ = json.NewDecoder(r.Body).Decode(&reqBody)
						query := reqBody["query"].(map[string]any)
						boolQuery := query["bool"].(map[string]any)

						if _, ok := boolQuery["filter"]; !ok {
							t.Error("Expected filter in query")
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{"value": 2},
								"hits": []map[string]any{
									{
										"_id":    "chunk-1",
										"_score": 1.5,
										"_source": map[string]any{
											"id":             "chunk-1",
											"document_id":    "doc-1",
											"source_id":      "source-1",
											"content":        "Chunk 1 content",
											"chunk_position": 0,
										},
									},
									{
										"_id":    "chunk-2",
										"_score": 1.2,
										"_source": map[string]any{
											"id":             "chunk-2",
											"document_id":    "doc-1",
											"source_id":      "source-1",
											"content":        "Chunk 2 content",
											"chunk_position": 1,
										},
									},
								},
							},
						})
						return
					}
				}))
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			// Mirrors the SearchDocuments deny-all test: chunk search must also
			// emit the ids.values match-nothing clause — never silently skip.
			name:  "chunk search with deny-all document ID filter emits match-nothing clause",
			query: "test query",
			opts: domain.SearchOptions{
				Limit:            10,
				Offset:           0,
				DocumentIDFilter: domain.DenyAllDocumentIDFilter(),
			},
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						var reqBody map[string]any
						_ = json.NewDecoder(r.Body).Decode(&reqBody)
						query := reqBody["query"].(map[string]any)
						boolQuery := query["bool"].(map[string]any)

						filterClauses, ok := boolQuery["filter"].([]any)
						if !ok {
							t.Fatal("Expected filter clauses for deny-all; got none")
						}
						foundMatchNothing := false
						for _, clause := range filterClauses {
							clauseMap, ok := clause.(map[string]any)
							if !ok {
								continue
							}
							if terms, ok := clauseMap["terms"].(map[string]any); ok {
								if _, ok := terms["document_id"]; ok {
									t.Error("deny-all must NOT emit a terms.document_id clause")
								}
							}
							if ids, ok := clauseMap["ids"].(map[string]any); ok {
								values, _ := ids["values"].([]any)
								if len(values) != 0 {
									t.Errorf("ids.values should be empty, got %v", values)
								}
								foundMatchNothing = true
							}
						}
						if !foundMatchNothing {
							t.Error("deny-all chunk search must emit ids.values match-nothing clause")
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{"value": 0},
								"hits":  []map[string]any{},
							},
						})
						return
					}
				}))
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:  "chunk search with combined filters",
			query: "test query",
			opts: domain.SearchOptions{
				Limit:            10,
				Offset:           0,
				SourceIDs:        []string{"source-1"},
				DocumentIDFilter: domain.AllowDocumentIDs([]string{"doc-1", "doc-2"}),
			},
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						var reqBody map[string]any
						_ = json.NewDecoder(r.Body).Decode(&reqBody)
						query := reqBody["query"].(map[string]any)
						boolQuery := query["bool"].(map[string]any)

						filterClauses, ok := boolQuery["filter"].([]any)
						if !ok || len(filterClauses) != 2 {
							t.Errorf("Expected 2 filter clauses, got %d", len(filterClauses))
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{"value": 1},
								"hits": []map[string]any{
									{
										"_id":    "chunk-1",
										"_score": 1.5,
										"_source": map[string]any{
											"id":             "chunk-1",
											"document_id":    "doc-1",
											"source_id":      "source-1",
											"content":        "Filtered chunk content",
											"chunk_position": 0,
										},
									},
								},
							},
						})
						return
					}
				}))
			},
			wantCount: 1,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := tt.setupServer(t)
			defer ts.Close()

			cfg := Config{
				URL:       ts.URL,
				IndexName: "sercha_chunks",
				Timeout:   5 * time.Second,
			}

			engine, err := NewSearchEngine(cfg)
			if err != nil {
				t.Fatalf("NewSearchEngine() error = %v", err)
			}

			results, _, err := engine.Search(context.Background(), tt.query, nil, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Search() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(results) != tt.wantCount {
				t.Errorf("Search() returned %d results, want %d", len(results), tt.wantCount)
			}
		})
	}
}

// TestSearchEngine_SearchDocuments_WithBoostTerms validates keyword boosting in document search
func TestSearchEngine_SearchDocuments_WithBoostTerms(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		opts           domain.SearchOptions
		setupServer    func(*testing.T) *httptest.Server
		wantCount      int
		wantTotal      int
		wantErr        bool
		validateQuery  func(*testing.T, map[string]any)
	}{
		{
			name:  "search with boost terms",
			query: "kubernetes deployment",
			opts: domain.SearchOptions{
				Limit:  10,
				Offset: 0,
				BoostTerms: map[string]float64{
					"helm":       2.0,
					"production": 1.5,
				},
			},
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						// Parse request body
						var reqBody map[string]any
						if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
							t.Fatalf("Failed to decode request body: %v", err)
						}

						// Verify query structure
						query := reqBody["query"].(map[string]any)
						boolQuery := query["bool"].(map[string]any)

						// Verify must clause exists
						mustClauses, ok := boolQuery["must"].([]any)
						if !ok || len(mustClauses) == 0 {
							t.Error("Expected must clause in bool query")
						}

						// Verify should clauses for boost terms
						shouldClauses, ok := boolQuery["should"].([]any)
						if !ok {
							t.Error("Expected should clauses for boost terms")
						}
						if len(shouldClauses) != 2 {
							t.Errorf("Expected 2 should clauses, got %d", len(shouldClauses))
						}

						// Verify boost term structure
						for _, clause := range shouldClauses {
							clauseMap := clause.(map[string]any)
							multiMatch, ok := clauseMap["multi_match"].(map[string]any)
							if !ok {
								t.Error("Expected multi_match in should clause")
								continue
							}

							// Check required fields
							if _, ok := multiMatch["query"]; !ok {
								t.Error("Expected query in multi_match")
							}
							if _, ok := multiMatch["boost"]; !ok {
								t.Error("Expected boost in multi_match")
							}

							fields, ok := multiMatch["fields"].([]any)
							if !ok || len(fields) != 2 {
								t.Error("Expected fields [title, content] in multi_match")
							}
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{"value": 1},
								"hits": []map[string]any{
									{
										"_id":    "doc-1",
										"_score": 2.5,
										"_source": map[string]any{
											"document_id": "doc-1",
											"source_id":   "source-1",
											"title":       "Kubernetes Helm Guide",
											"content":     "Production deployment with helm",
										},
									},
								},
							},
						})
						return
					}
				}))
			},
			wantCount: 1,
			wantTotal: 1,
			wantErr:   false,
		},
		{
			name:  "search without boost terms",
			query: "kubernetes deployment",
			opts: domain.SearchOptions{
				Limit:  10,
				Offset: 0,
			},
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						var reqBody map[string]any
						if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
							t.Fatalf("Failed to decode request body: %v", err)
						}

						query := reqBody["query"].(map[string]any)
						boolQuery := query["bool"].(map[string]any)

						// Verify NO should clauses when boost terms not specified
						if _, ok := boolQuery["should"]; ok {
							t.Error("Should not have should clauses when BoostTerms is empty")
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{"value": 1},
								"hits": []map[string]any{
									{
										"_id":    "doc-1",
										"_score": 1.0,
										"_source": map[string]any{
											"document_id": "doc-1",
											"source_id":   "source-1",
											"title":       "Kubernetes Guide",
											"content":     "Deployment guide",
										},
									},
								},
							},
						})
						return
					}
				}))
			},
			wantCount: 1,
			wantTotal: 1,
			wantErr:   false,
		},
		{
			name:  "search with single boost term",
			query: "kubernetes",
			opts: domain.SearchOptions{
				Limit:  10,
				Offset: 0,
				BoostTerms: map[string]float64{
					"production": 3.0,
				},
			},
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						var reqBody map[string]any
						if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
							t.Fatalf("Failed to decode request body: %v", err)
						}

						query := reqBody["query"].(map[string]any)
						boolQuery := query["bool"].(map[string]any)

						shouldClauses, ok := boolQuery["should"].([]any)
						if !ok || len(shouldClauses) != 1 {
							t.Errorf("Expected 1 should clause, got %d", len(shouldClauses))
						}

						// Verify boost value
						clause := shouldClauses[0].(map[string]any)
						multiMatch := clause["multi_match"].(map[string]any)
						if multiMatch["query"] != "production" {
							t.Errorf("Expected query 'production', got %v", multiMatch["query"])
						}
						if multiMatch["boost"].(float64) != 3.0 {
							t.Errorf("Expected boost 3.0, got %v", multiMatch["boost"])
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{"value": 1},
								"hits":  []map[string]any{},
							},
						})
						return
					}
				}))
			},
			wantCount: 0,
			wantTotal: 1,
			wantErr:   false,
		},
		{
			name:  "search with boost terms and filters",
			query: "kubernetes",
			opts: domain.SearchOptions{
				Limit:     10,
				Offset:    0,
				SourceIDs: []string{"source-1"},
				BoostTerms: map[string]float64{
					"helm": 2.0,
				},
			},
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						var reqBody map[string]any
						if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
							t.Fatalf("Failed to decode request body: %v", err)
						}

						query := reqBody["query"].(map[string]any)
						boolQuery := query["bool"].(map[string]any)

						// Verify both should and filter clauses exist
						if _, ok := boolQuery["should"]; !ok {
							t.Error("Expected should clause for boost terms")
						}
						if _, ok := boolQuery["filter"]; !ok {
							t.Error("Expected filter clause for source IDs")
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{"value": 1},
								"hits":  []map[string]any{},
							},
						})
						return
					}
				}))
			},
			wantCount: 0,
			wantTotal: 1,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := tt.setupServer(t)
			defer ts.Close()

			cfg := Config{
				URL:       ts.URL,
				IndexName: "sercha_chunks",
				Timeout:   5 * time.Second,
			}

			engine, err := NewSearchEngine(cfg)
			if err != nil {
				t.Fatalf("NewSearchEngine() error = %v", err)
			}

			results, total, err := engine.SearchDocuments(context.Background(), tt.query, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("SearchDocuments() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(results) != tt.wantCount {
					t.Errorf("SearchDocuments() returned %d results, want %d", len(results), tt.wantCount)
				}
				if total != tt.wantTotal {
					t.Errorf("SearchDocuments() total = %d, want %d", total, tt.wantTotal)
				}
			}
		})
	}
}

// TestSearchEngine_Search_WithBoostTerms validates keyword boosting in chunk search
func TestSearchEngine_Search_WithBoostTerms(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		opts        domain.SearchOptions
		setupServer func(*testing.T) *httptest.Server
		wantCount   int
		wantTotal   int
		wantErr     bool
	}{
		{
			name:  "chunk search with boost terms",
			query: "kubernetes deployment",
			opts: domain.SearchOptions{
				Limit:  10,
				Offset: 0,
				BoostTerms: map[string]float64{
					"helm":       2.0,
					"production": 1.5,
				},
			},
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						var reqBody map[string]any
						if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
							t.Fatalf("Failed to decode request body: %v", err)
						}

						query := reqBody["query"].(map[string]any)
						boolQuery := query["bool"].(map[string]any)

						// Verify should clauses exist
						shouldClauses, ok := boolQuery["should"].([]any)
						if !ok || len(shouldClauses) != 2 {
							t.Errorf("Expected 2 should clauses, got %d", len(shouldClauses))
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{"value": 2},
								"hits": []map[string]any{
									{
										"_id":    "chunk-1",
										"_score": 2.5,
										"_source": map[string]any{
											"id":             "chunk-1",
											"document_id":    "doc-1",
											"source_id":      "source-1",
											"content":        "Kubernetes deployment with helm",
											"chunk_position": 0,
										},
									},
									{
										"_id":    "chunk-2",
										"_score": 2.0,
										"_source": map[string]any{
											"id":             "chunk-2",
											"document_id":    "doc-1",
											"source_id":      "source-1",
											"content":        "Production kubernetes setup",
											"chunk_position": 1,
										},
									},
								},
							},
						})
						return
					}
				}))
			},
			wantCount: 2,
			wantTotal: 2,
			wantErr:   false,
		},
		{
			name:  "chunk search without boost terms",
			query: "kubernetes",
			opts: domain.SearchOptions{
				Limit:  10,
				Offset: 0,
			},
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						var reqBody map[string]any
						if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
							t.Fatalf("Failed to decode request body: %v", err)
						}

						query := reqBody["query"].(map[string]any)
						boolQuery := query["bool"].(map[string]any)

						// Verify NO should clauses
						if _, ok := boolQuery["should"]; ok {
							t.Error("Should not have should clauses when BoostTerms is empty")
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{"value": 1},
								"hits": []map[string]any{
									{
										"_id":    "chunk-1",
										"_score": 1.0,
										"_source": map[string]any{
											"id":             "chunk-1",
											"document_id":    "doc-1",
											"source_id":      "source-1",
											"content":        "Kubernetes content",
											"chunk_position": 0,
										},
									},
								},
							},
						})
						return
					}
				}))
			},
			wantCount: 1,
			wantTotal: 1,
			wantErr:   false,
		},
		{
			name:  "chunk search with empty boost terms map",
			query: "test",
			opts: domain.SearchOptions{
				Limit:      10,
				Offset:     0,
				BoostTerms: map[string]float64{},
			},
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
						var reqBody map[string]any
						if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
							t.Fatalf("Failed to decode request body: %v", err)
						}

						query := reqBody["query"].(map[string]any)
						boolQuery := query["bool"].(map[string]any)

						// Empty map should not add should clauses
						if _, ok := boolQuery["should"]; ok {
							t.Error("Should not have should clauses when BoostTerms map is empty")
						}

						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"hits": map[string]any{
								"total": map[string]any{"value": 0},
								"hits":  []map[string]any{},
							},
						})
						return
					}
				}))
			},
			wantCount: 0,
			wantTotal: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := tt.setupServer(t)
			defer ts.Close()

			cfg := Config{
				URL:       ts.URL,
				IndexName: "sercha_chunks",
				Timeout:   5 * time.Second,
			}

			engine, err := NewSearchEngine(cfg)
			if err != nil {
				t.Fatalf("NewSearchEngine() error = %v", err)
			}

			results, total, err := engine.Search(context.Background(), tt.query, nil, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Search() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(results) != tt.wantCount {
					t.Errorf("Search() returned %d results, want %d", len(results), tt.wantCount)
				}
				if total != tt.wantTotal {
					t.Errorf("Search() total = %d, want %d", total, tt.wantTotal)
				}
			}
		})
	}
}

// TestSearchEngine_BoostTermsQueryStructure validates exact OpenSearch query structure
func TestSearchEngine_BoostTermsQueryStructure(t *testing.T) {
	tests := []struct {
		name            string
		boostTerms      map[string]float64
		validateRequest func(*testing.T, map[string]any)
	}{
		{
			name: "multiple boost terms create multiple should clauses",
			boostTerms: map[string]float64{
				"kubernetes": 2.0,
				"helm":       1.5,
				"docker":     1.2,
			},
			validateRequest: func(t *testing.T, reqBody map[string]any) {
				query := reqBody["query"].(map[string]any)
				boolQuery := query["bool"].(map[string]any)
				shouldClauses := boolQuery["should"].([]any)

				if len(shouldClauses) != 3 {
					t.Errorf("Expected 3 should clauses, got %d", len(shouldClauses))
				}

				// Verify each should clause has correct structure
				foundTerms := make(map[string]float64)
				for _, clause := range shouldClauses {
					clauseMap := clause.(map[string]any)
					multiMatch := clauseMap["multi_match"].(map[string]any)

					term := multiMatch["query"].(string)
					boost := multiMatch["boost"].(float64)
					foundTerms[term] = boost

					// Verify fields
					fields := multiMatch["fields"].([]any)
					if len(fields) != 2 {
						t.Errorf("Expected 2 fields, got %d", len(fields))
					}
					if fields[0] != "title" || fields[1] != "content" {
						t.Errorf("Expected fields [title, content], got %v", fields)
					}
				}

				// Verify all boost terms are present with correct values
				for term, expectedBoost := range map[string]float64{
					"kubernetes": 2.0,
					"helm":       1.5,
					"docker":     1.2,
				} {
					if actualBoost, ok := foundTerms[term]; !ok {
						t.Errorf("Missing boost term %q in query", term)
					} else if actualBoost != expectedBoost {
						t.Errorf("Boost for %q = %v, want %v", term, actualBoost, expectedBoost)
					}
				}
			},
		},
		{
			name: "fractional boost values are preserved",
			boostTerms: map[string]float64{
				"important": 1.25,
				"critical":  2.75,
			},
			validateRequest: func(t *testing.T, reqBody map[string]any) {
				query := reqBody["query"].(map[string]any)
				boolQuery := query["bool"].(map[string]any)
				shouldClauses := boolQuery["should"].([]any)

				for _, clause := range shouldClauses {
					clauseMap := clause.(map[string]any)
					multiMatch := clauseMap["multi_match"].(map[string]any)
					term := multiMatch["query"].(string)
					boost := multiMatch["boost"].(float64)

					if term == "important" && boost != 1.25 {
						t.Errorf("Boost for important = %v, want 1.25", boost)
					}
					if term == "critical" && boost != 2.75 {
						t.Errorf("Boost for critical = %v, want 2.75", boost)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
					var reqBody map[string]any
					if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
						t.Fatalf("Failed to decode request body: %v", err)
					}

					tt.validateRequest(t, reqBody)

					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]any{
						"hits": map[string]any{
							"total": map[string]any{"value": 0},
							"hits":  []map[string]any{},
						},
					})
					return
				}
			}))
			defer ts.Close()

			cfg := Config{
				URL:       ts.URL,
				IndexName: "sercha_chunks",
				Timeout:   5 * time.Second,
			}

			engine, err := NewSearchEngine(cfg)
			if err != nil {
				t.Fatalf("NewSearchEngine() error = %v", err)
			}

			opts := domain.SearchOptions{
				Limit:      10,
				Offset:     0,
				BoostTerms: tt.boostTerms,
			}

			_, _, err = engine.SearchDocuments(context.Background(), "test query", opts)
			if err != nil {
				t.Errorf("SearchDocuments() error = %v", err)
			}
		})
	}
}

// TestSearchEngine_InterfaceCompliance validates interface implementation
func TestSearchEngine_InterfaceCompliance(t *testing.T) {
	// This test verifies that SearchEngine implements the driven.SearchEngine interface
	// The compilation will fail if the interface is not properly implemented
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := Config{
		URL:       ts.URL,
		IndexName: "test",
		Timeout:   5 * time.Second,
	}

	engine, err := NewSearchEngine(cfg)
	if err != nil {
		t.Fatalf("NewSearchEngine() error = %v", err)
	}

	// Verify all interface methods are available
	ctx := context.Background()
	_ = engine.Index(ctx, []*domain.Chunk{})
	_, _, _ = engine.Search(ctx, "test", nil, domain.SearchOptions{})
	_ = engine.Delete(ctx, []string{})
	_ = engine.DeleteByDocument(ctx, "doc-1")
	_ = engine.DeleteBySource(ctx, "source-1")
	_ = engine.HealthCheck(ctx)
	_, _ = engine.Count(ctx)
	_, _ = engine.GetDocument(ctx, "doc-1")
}

// TestSearchEngine_ContextCancellation validates context cancellation handling
func TestSearchEngine_ContextCancellation(t *testing.T) {
	// Create a server that delays responses
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := Config{
		URL:       ts.URL,
		IndexName: "test",
		Timeout:   5 * time.Second,
	}

	engine, err := NewSearchEngine(cfg)
	if err != nil {
		t.Fatalf("NewSearchEngine() error = %v", err)
	}

	// Create a context that's immediately cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Operations should respect context cancellation
	err = engine.HealthCheck(ctx)
	if err == nil {
		t.Error("HealthCheck() should return error when context is cancelled")
	}
}
