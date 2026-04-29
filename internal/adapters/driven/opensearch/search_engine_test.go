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

						// Verify should clauses: 1 exact-title term + N boost-term multi_match.
						shouldClauses, ok := boolQuery["should"].([]any)
						if !ok {
							t.Error("Expected should clauses (exact-title boost + boost terms)")
						}
						// 1 exact-title term + 2 boost terms = 3
						if len(shouldClauses) != 3 {
							t.Errorf("Expected 3 should clauses (1 exact-title + 2 boost terms), got %d", len(shouldClauses))
						}

						// Filter to the boost-term multi_match clauses and verify their structure.
						boostMatches := 0
						for _, clause := range shouldClauses {
							clauseMap := clause.(map[string]any)
							multiMatch, isMultiMatch := clauseMap["multi_match"].(map[string]any)
							if !isMultiMatch {
								// Skip the exact-title term clause.
								continue
							}
							boostMatches++

							if _, ok := multiMatch["query"]; !ok {
								t.Error("Expected query in multi_match")
							}
							if _, ok := multiMatch["boost"]; !ok {
								t.Error("Expected boost in multi_match")
							}

							fields, ok := multiMatch["fields"].([]any)
							// Boost-term should clause now searches across
							// title, content, path.text, path.basename, metadata.
							if !ok || len(fields) != 5 {
								t.Errorf("Expected 5 fields in boost-term multi_match, got %v", fields)
							}
						}
						if boostMatches != 2 {
							t.Errorf("Expected 2 boost-term multi_match clauses, got %d", boostMatches)
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

						// Even without boost terms there is one should clause:
						// the exact-title `term` boost on title.raw.
						shouldClauses, ok := boolQuery["should"].([]any)
						if !ok || len(shouldClauses) != 1 {
							t.Errorf("Expected 1 should clause (exact-title boost), got %d", len(shouldClauses))
						} else if _, isTerm := shouldClauses[0].(map[string]any)["term"]; !isTerm {
							t.Errorf("Expected exact-title `term` should clause, got %v", shouldClauses[0])
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

						// Should clauses: 1 exact-title term + 1 boost-term multi_match.
						shouldClauses, ok := boolQuery["should"].([]any)
						if !ok || len(shouldClauses) != 2 {
							t.Errorf("Expected 2 should clauses (exact-title + 1 boost), got %d", len(shouldClauses))
						}

						// Locate the boost-term multi_match clause and verify its value.
						var boostMM map[string]any
						for _, c := range shouldClauses {
							if mm, isMM := c.(map[string]any)["multi_match"].(map[string]any); isMM {
								boostMM = mm
								break
							}
						}
						if boostMM == nil {
							t.Fatal("expected a multi_match boost-term clause among shoulds")
						}
						if boostMM["query"] != "production" {
							t.Errorf("Expected query 'production', got %v", boostMM["query"])
						}
						if boostMM["boost"].(float64) != 3.0 {
							t.Errorf("Expected boost 3.0, got %v", boostMM["boost"])
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

				// 1 exact-title term + 3 boost terms = 4
				if len(shouldClauses) != 4 {
					t.Errorf("Expected 4 should clauses (1 exact-title + 3 boost terms), got %d", len(shouldClauses))
				}

				// Filter to boost-term multi_match clauses (skip exact-title term).
				foundTerms := make(map[string]float64)
				for _, clause := range shouldClauses {
					clauseMap := clause.(map[string]any)
					multiMatch, isMM := clauseMap["multi_match"].(map[string]any)
					if !isMM {
						continue // exact-title `term` clause
					}

					term := multiMatch["query"].(string)
					boost := multiMatch["boost"].(float64)
					foundTerms[term] = boost

					// Boost-term should clause searches title, content, path.text,
					// path.basename, metadata — five fields.
					fields := multiMatch["fields"].([]any)
					if len(fields) != 5 {
						t.Errorf("Expected 5 fields, got %d (%v)", len(fields), fields)
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
					multiMatch, isMM := clauseMap["multi_match"].(map[string]any)
					if !isMM {
						continue // exact-title `term` clause
					}
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

// TestSearchEngine_SearchDocuments_QueryStructure validates the OpenSearch query
// shape: a fuzzy multi_match plus one phrase clause per opts.Phrases entry,
// all in bool.must so phrases are required (not just score-boosting).
func TestSearchEngine_SearchDocuments_QueryStructure(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		phrases []string
		check   func(*testing.T, []any)
	}{
		{
			name:    "no phrases — single fuzzy multi_match clause",
			query:   "merge sort",
			phrases: nil,
			check: func(t *testing.T, must []any) {
				if len(must) != 1 {
					t.Fatalf("want 1 must clause, got %d", len(must))
				}
				mm := must[0].(map[string]any)["multi_match"].(map[string]any)
				if mm["query"] != "merge sort" {
					t.Errorf("query = %v, want %q", mm["query"], "merge sort")
				}
				if mm["fuzziness"] != "AUTO" {
					t.Errorf("fuzziness = %v, want AUTO", mm["fuzziness"])
				}
				if mm["type"] != "most_fields" {
					t.Errorf("type = %v, want most_fields", mm["type"])
				}
			},
		},
		{
			name:    "single phrase — fuzzy multi_match plus phrase multi_match",
			query:   "stable",
			phrases: []string{"merge sort"},
			check: func(t *testing.T, must []any) {
				if len(must) != 2 {
					t.Fatalf("want 2 must clauses (loose + phrase), got %d", len(must))
				}
				phrase := must[1].(map[string]any)["multi_match"].(map[string]any)
				if phrase["query"] != "merge sort" {
					t.Errorf("phrase query = %v, want %q", phrase["query"], "merge sort")
				}
				if phrase["type"] != "phrase" {
					t.Errorf("phrase clause type = %v, want phrase", phrase["type"])
				}
				// Phrase clauses share the same field set as the loose
				// fuzzy match: title^3, path.basename^3, path.text^2,
				// content, metadata^1.5.
				fields := phrase["fields"].([]any)
				wantFields := []any{"title^3", "path.basename^3", "path.text^2", "content", "metadata^1.5"}
				if len(fields) != len(wantFields) {
					t.Errorf("phrase fields len = %d, want %d (%v)", len(fields), len(wantFields), fields)
				}
				for i, w := range wantFields {
					if i >= len(fields) || fields[i] != w {
						t.Errorf("phrase fields = %v, want %v", fields, wantFields)
						break
					}
				}
			},
		},
		{
			name:    "multiple phrases — one must clause each",
			query:   "rust",
			phrases: []string{"zero cost", "borrow checker"},
			check: func(t *testing.T, must []any) {
				if len(must) != 3 {
					t.Fatalf("want 3 must clauses (loose + 2 phrases), got %d", len(must))
				}
				seen := make(map[string]bool)
				for _, c := range must[1:] {
					mm := c.(map[string]any)["multi_match"].(map[string]any)
					if mm["type"] != "phrase" {
						t.Errorf("non-phrase clause snuck in: %v", mm)
					}
					seen[mm["query"].(string)] = true
				}
				if !seen["zero cost"] || !seen["borrow checker"] {
					t.Errorf("missing phrase: %v", seen)
				}
			},
		},
		{
			name:    "blank phrases are dropped",
			query:   "test",
			phrases: []string{"", "   ", "real phrase"},
			check: func(t *testing.T, must []any) {
				if len(must) != 2 {
					t.Fatalf("want 2 must clauses (loose + 1 real phrase), got %d", len(must))
				}
				phrase := must[1].(map[string]any)["multi_match"].(map[string]any)
				if phrase["query"] != "real phrase" {
					t.Errorf("phrase query = %v, want %q", phrase["query"], "real phrase")
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
						t.Fatalf("decode request: %v", err)
					}
					must := reqBody["query"].(map[string]any)["bool"].(map[string]any)["must"].([]any)
					tt.check(t, must)

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

			engine, err := NewSearchEngine(Config{
				URL:       ts.URL,
				IndexName: "sercha_chunks",
				Timeout:   5 * time.Second,
			})
			if err != nil {
				t.Fatalf("NewSearchEngine: %v", err)
			}

			_, _, err = engine.SearchDocuments(context.Background(), tt.query, domain.SearchOptions{
				Limit:   10,
				Phrases: tt.phrases,
			})
			if err != nil {
				t.Errorf("SearchDocuments: %v", err)
			}
		})
	}
}

// TestSearchEngine_EnsureIndex_Mapping validates the index mapping uses the
// english analyser (with stemming/stop words) and a title.raw keyword sub-field.
func TestSearchEngine_EnsureIndex_Mapping(t *testing.T) {
	var capturedMapping map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "HEAD":
			// Index doesn't exist — force ensureIndex to create it.
			w.WriteHeader(http.StatusNotFound)
		case r.Method == "PUT":
			if err := json.NewDecoder(r.Body).Decode(&capturedMapping); err != nil {
				t.Fatalf("decode mapping: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"acknowledged": true})
		case r.Method == "POST" && strings.Contains(r.URL.Path, "_doc"):
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "created"})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ts.Close()

	engine, err := NewSearchEngine(Config{
		URL:       ts.URL,
		IndexName: "sercha_chunks",
		Timeout:   5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewSearchEngine: %v", err)
	}

	// ensureIndex runs lazily on first IndexDocument call.
	if err := engine.IndexDocument(context.Background(), &domain.DocumentContent{
		DocumentID: "d1",
		Title:      "test",
		Body:       "body",
	}); err != nil {
		t.Fatalf("IndexDocument: %v", err)
	}

	if capturedMapping == nil {
		t.Fatal("ensureIndex never sent a PUT — mapping not captured")
	}
	props := capturedMapping["mappings"].(map[string]any)["properties"].(map[string]any)

	title := props["title"].(map[string]any)
	if title["analyzer"] != "english" {
		t.Errorf("title analyzer = %v, want english", title["analyzer"])
	}
	titleFields, ok := title["fields"].(map[string]any)
	if !ok {
		t.Fatal("title.fields missing — title.raw sub-field not configured")
	}
	rawField := titleFields["raw"].(map[string]any)
	if rawField["type"] != "keyword" {
		t.Errorf("title.raw type = %v, want keyword", rawField["type"])
	}
	// title.raw is normalized for case-insensitive exact-title boost.
	if rawField["normalizer"] != "lowercase_keyword" {
		t.Errorf("title.raw normalizer = %v, want lowercase_keyword", rawField["normalizer"])
	}

	content := props["content"].(map[string]any)
	if content["analyzer"] != "english" {
		t.Errorf("content analyzer = %v, want english", content["analyzer"])
	}

	// path is multi-field: keyword + analyzed text + basename.
	path := props["path"].(map[string]any)
	if path["type"] != "keyword" {
		t.Errorf("path type = %v, want keyword", path["type"])
	}
	pathFields, ok := path["fields"].(map[string]any)
	if !ok {
		t.Fatal("path.fields missing — path.text/path.basename subfields not configured")
	}
	if pt, ok := pathFields["text"].(map[string]any); !ok || pt["analyzer"] != "path_analyzer" {
		t.Errorf("path.text analyzer = %v, want path_analyzer", pt["analyzer"])
	}
	if pb, ok := pathFields["basename"].(map[string]any); !ok || pb["analyzer"] != "basename_analyzer" {
		t.Errorf("path.basename analyzer = %v, want basename_analyzer", pb["analyzer"])
	}

	// metadata is flattened so connector-supplied attributes are searchable.
	metadata, ok := props["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata field missing from mapping")
	}
	if metadata["type"] != "flattened" {
		t.Errorf("metadata type = %v, want flattened", metadata["type"])
	}

	// Custom analyzers/normalizer registered on the index settings.
	settings, ok := capturedMapping["settings"].(map[string]any)
	if !ok {
		t.Fatal("index settings missing — analyzer/normalizer not registered")
	}
	analysis, ok := settings["analysis"].(map[string]any)
	if !ok {
		t.Fatal("settings.analysis missing")
	}
	if _, ok := analysis["analyzer"].(map[string]any)["path_analyzer"]; !ok {
		t.Error("path_analyzer not registered in settings.analysis.analyzer")
	}
	if _, ok := analysis["analyzer"].(map[string]any)["basename_analyzer"]; !ok {
		t.Error("basename_analyzer not registered in settings.analysis.analyzer")
	}
	if _, ok := analysis["normalizer"].(map[string]any)["lowercase_keyword"]; !ok {
		t.Error("lowercase_keyword normalizer not registered in settings.analysis.normalizer")
	}
}

// TestSearchEngine_IndexDocument_WritesMetadata verifies that connector-
// supplied metadata is included in the OpenSearch document body so it
// can be searched via the flattened mapping.
func TestSearchEngine_IndexDocument_WritesMetadata(t *testing.T) {
	var capturedDocBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "HEAD":
			w.WriteHeader(http.StatusOK) // index already exists
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "_doc"):
			if err := json.NewDecoder(r.Body).Decode(&capturedDocBody); err != nil {
				t.Fatalf("decode doc body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "created"})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ts.Close()

	engine, err := NewSearchEngine(Config{
		URL:       ts.URL,
		IndexName: "sercha_chunks",
		Timeout:   5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewSearchEngine: %v", err)
	}

	if err := engine.IndexDocument(context.Background(), &domain.DocumentContent{
		DocumentID: "doc-md",
		Title:      "Title",
		Body:       "body",
		Metadata: map[string]string{
			"author": "alice",
			"labels": "kubernetes,helm",
		},
	}); err != nil {
		t.Fatalf("IndexDocument: %v", err)
	}

	if capturedDocBody == nil {
		t.Fatal("IndexDocument never sent the doc PUT")
	}
	md, ok := capturedDocBody["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata not present in indexed document body")
	}
	if md["author"] != "alice" {
		t.Errorf("metadata.author = %v, want alice", md["author"])
	}
	if md["labels"] != "kubernetes,helm" {
		t.Errorf("metadata.labels = %v, want kubernetes,helm", md["labels"])
	}
}

// TestSearchEngine_SearchDocuments_ExactTitleBoost verifies that a `term`
// query against title.raw is added as a should clause for exact-title
// dominance, and that the raw query is lowercased to match the
// lowercase_keyword normalizer applied to title.raw.
func TestSearchEngine_SearchDocuments_ExactTitleBoost(t *testing.T) {
	var capturedReq map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
			if err := json.NewDecoder(r.Body).Decode(&capturedReq); err != nil {
				t.Fatalf("decode req: %v", err)
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
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	engine, err := NewSearchEngine(Config{
		URL:       ts.URL,
		IndexName: "sercha_chunks",
		Timeout:   5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewSearchEngine: %v", err)
	}

	if _, _, err := engine.SearchDocuments(context.Background(), "Kubernetes Setup Guide", domain.SearchOptions{Limit: 10}); err != nil {
		t.Fatalf("SearchDocuments: %v", err)
	}

	boolQuery := capturedReq["query"].(map[string]any)["bool"].(map[string]any)
	shoulds, ok := boolQuery["should"].([]any)
	if !ok || len(shoulds) != 1 {
		t.Fatalf("expected 1 should clause (exact-title boost), got %v", shoulds)
	}
	term, ok := shoulds[0].(map[string]any)["term"].(map[string]any)
	if !ok {
		t.Fatalf("expected `term` should clause, got %v", shoulds[0])
	}
	titleRaw, ok := term["title.raw"].(map[string]any)
	if !ok {
		t.Fatalf("expected term on title.raw, got %v", term)
	}
	if titleRaw["value"] != "kubernetes setup guide" {
		t.Errorf("title.raw value = %v, want lowercased query", titleRaw["value"])
	}
	if titleRaw["boost"].(float64) <= 1.0 {
		t.Errorf("title.raw boost = %v, want >1.0 to dominate fuzzy match", titleRaw["boost"])
	}

	// Verify the multi_match must clause has minimum_should_match.
	mustClauses := boolQuery["must"].([]any)
	mm := mustClauses[0].(map[string]any)["multi_match"].(map[string]any)
	if msm := mm["minimum_should_match"]; msm != "75%" {
		t.Errorf("minimum_should_match = %v, want 75%%", msm)
	}
}

// TestSearchEngine_SearchByQueryDSL_WrapsCallerQueryInBoolEnvelope verifies
// that the caller-supplied query body lands as a must clause inside the
// standard bool envelope, with filters and pagination applied identically
// to SearchDocuments.
func TestSearchEngine_SearchByQueryDSL_WrapsCallerQueryInBoolEnvelope(t *testing.T) {
	var capturedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
			if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
				t.Fatalf("decode request: %v", err)
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
	defer ts.Close()

	engine, err := NewSearchEngine(Config{URL: ts.URL, IndexName: "sercha_chunks", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewSearchEngine: %v", err)
	}

	// Caller-supplied DSL: a function_score wrapper around a match query.
	innerQuery := json.RawMessage(`{"function_score":{"query":{"match":{"content":"hello"}},"boost":2.0}}`)

	_, _, err = engine.SearchByQueryDSL(context.Background(), innerQuery, domain.SearchOptions{
		Limit:     20,
		Offset:    5,
		SourceIDs: []string{"src-1"},
	})
	if err != nil {
		t.Fatalf("SearchByQueryDSL: %v", err)
	}

	// Pagination applied.
	if from, _ := capturedBody["from"].(float64); from != 5 {
		t.Errorf("from = %v, want 5", from)
	}
	if size, _ := capturedBody["size"].(float64); size != 20 {
		t.Errorf("size = %v, want 20", size)
	}

	// Caller's body lands as the only must clause.
	boolQuery := capturedBody["query"].(map[string]any)["bool"].(map[string]any)
	must := boolQuery["must"].([]any)
	if len(must) != 1 {
		t.Fatalf("want 1 must clause (caller's body), got %d", len(must))
	}
	mustClause := must[0].(map[string]any)
	if _, ok := mustClause["function_score"]; !ok {
		t.Errorf("caller's function_score not preserved: %v", mustClause)
	}

	// Source filter still applied via the standard envelope.
	filter := boolQuery["filter"].([]any)
	if len(filter) != 1 {
		t.Fatalf("want 1 filter clause (source_id), got %d", len(filter))
	}

	// Highlight config still attached so consumers get fragment data.
	if _, ok := capturedBody["highlight"]; !ok {
		t.Error("highlight config missing from envelope")
	}
}

// SearchByQueryDSL with empty queryBody is a programmer error and must
// fail fast rather than fire an unbounded match-all at OpenSearch.
func TestSearchEngine_SearchByQueryDSL_EmptyBodyErrors(t *testing.T) {
	engine, err := NewSearchEngine(Config{URL: "http://localhost:0", IndexName: "test", Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewSearchEngine: %v", err)
	}
	_, _, err = engine.SearchByQueryDSL(context.Background(), nil, domain.SearchOptions{})
	if err == nil {
		t.Error("want error for empty queryBody, got nil")
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
	_ = engine.IndexDocument(ctx, &domain.DocumentContent{})
	_, _, _ = engine.SearchDocuments(ctx, "test", domain.SearchOptions{})
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
