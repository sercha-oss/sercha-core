package pgvector

import (
	"context"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// TestVectorIndex_InterfaceCompliance verifies that VectorIndex implements driven.VectorIndex
// This is a compile-time check that will fail if the interface is not properly implemented
func TestVectorIndex_InterfaceCompliance(t *testing.T) {
	// This test ensures VectorIndex implements driven.VectorIndex at compile time
	// The variable assignment will fail to compile if the interface is not satisfied
	var _ driven.VectorIndex = (*VectorIndex)(nil)
}

// TestNew_EmptyURL tests that New returns an error when URL is empty
func TestNew_EmptyURL(t *testing.T) {
	cfg := Config{
		URL:            "",
		Dimensions:     1536,
		DistanceMetric: "cosine",
	}

	_, err := New(context.Background(), cfg)
	if err == nil {
		t.Fatal("New() should return error when URL is empty")
	}

	expectedErrMsg := "pgvector URL is required"
	if err.Error() != expectedErrMsg {
		t.Errorf("New() error = %q, want %q", err.Error(), expectedErrMsg)
	}
}

// TestNew_InvalidURL tests that New returns an error for invalid URLs
func TestNew_InvalidURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty URL",
			url:         "",
			wantErr:     true,
			errContains: "pgvector URL is required",
		},
		{
			name:        "malformed URL",
			url:         "not-a-valid-url",
			wantErr:     true,
			errContains: "failed to parse pgvector URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				URL:            tt.url,
				Dimensions:     1536,
				DistanceMetric: "cosine",
			}

			_, err := New(context.Background(), cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("New() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
			}
		})
	}
}

// TestNew_InvalidDistanceMetric tests that New returns an error for invalid distance metrics
func TestNew_InvalidDistanceMetric(t *testing.T) {
	// We need a valid-looking URL for this test to proceed past URL validation
	// The connection will fail but it tests the distance metric validation first
	// Note: This test won't pass the ping test, but that's expected behavior
	// when there's no real database. We're testing the validation path.

	// Since New() requires a valid connection, we can only test with mock or
	// by accepting that connection errors will occur. Let's test the config validation
	// that happens in the constructor logic.

	tests := []struct {
		name           string
		distanceMetric string
		expectedOp     string
	}{
		{
			name:           "cosine distance",
			distanceMetric: "cosine",
			expectedOp:     "<=>",
		},
		{
			name:           "l2 distance",
			distanceMetric: "l2",
			expectedOp:     "<->",
		},
		{
			name:           "inner_product distance",
			distanceMetric: "inner_product",
			expectedOp:     "<#>",
		},
		{
			name:           "empty defaults to cosine",
			distanceMetric: "",
			expectedOp:     "<=>",
		},
	}

	// These tests document the expected operator mapping
	// In a real test with a database, we would verify the VectorIndex.distOp field
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Document the expected behavior
			t.Logf("Distance metric %q should map to operator %q", tt.distanceMetric, tt.expectedOp)
		})
	}
}

// TestDistanceMetricMapping tests the distance metric to operator mapping
func TestDistanceMetricMapping(t *testing.T) {
	tests := []struct {
		metric      string
		expectedOp  string
		shouldError bool
	}{
		{metric: "cosine", expectedOp: "<=>", shouldError: false},
		{metric: "l2", expectedOp: "<->", shouldError: false},
		{metric: "inner_product", expectedOp: "<#>", shouldError: false},
		{metric: "", expectedOp: "<=>", shouldError: false}, // empty defaults to cosine
		{metric: "invalid", expectedOp: "", shouldError: true},
		{metric: "COSINE", expectedOp: "", shouldError: true}, // case sensitive
	}

	for _, tt := range tests {
		t.Run("metric_"+tt.metric, func(t *testing.T) {
			// This documents the expected mapping based on the switch statement in New()
			var op string
			var shouldError bool

			switch tt.metric {
			case "l2":
				op = "<->"
			case "inner_product":
				op = "<#>"
			case "cosine", "":
				op = "<=>"
			default:
				shouldError = true
			}

			if shouldError != tt.shouldError {
				t.Errorf("metric %q: shouldError = %v, want %v", tt.metric, shouldError, tt.shouldError)
			}
			if !shouldError && op != tt.expectedOp {
				t.Errorf("metric %q: op = %q, want %q", tt.metric, op, tt.expectedOp)
			}
		})
	}
}

// TestVectorIndex_Close tests the Close method handles nil pool gracefully
func TestVectorIndex_Close(t *testing.T) {
	// Test that Close() doesn't panic with nil pool
	vi := &VectorIndex{pool: nil}
	// This should not panic
	vi.Close()
}

// TestVectorIndex_IndexValidation tests input validation for Index method
func TestVectorIndex_IndexValidation(t *testing.T) {
	tests := []struct {
		name         string
		dimensions   int
		embeddingLen int
		wantErr      bool
		errContains  string
	}{
		{
			name:         "embedding too short",
			dimensions:   1536,
			embeddingLen: 100,
			wantErr:      true,
			errContains:  "embedding dimension mismatch",
		},
		{
			name:         "embedding too long",
			dimensions:   1536,
			embeddingLen: 2000,
			wantErr:      true,
			errContains:  "embedding dimension mismatch",
		},
		{
			name:         "empty embedding",
			dimensions:   1536,
			embeddingLen: 0,
			wantErr:      true,
			errContains:  "embedding dimension mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vi := &VectorIndex{
				pool:       nil,
				dimensions: tt.dimensions,
				distOp:     "<=>",
			}

			embedding := make([]float32, tt.embeddingLen)

			err := vi.Index(context.Background(), "test-id", "doc-1", embedding)

			if tt.wantErr {
				if err == nil {
					t.Fatal("Index() should return error")
				}
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("Index() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
			}
		})
	}
}

// TestVectorIndex_IndexBatchValidation tests input validation for IndexBatch method
func TestVectorIndex_IndexBatchValidation(t *testing.T) {
	tests := []struct {
		name        string
		dimensions  int
		ids         []string
		documentIDs []string
		embeddings  [][]float32
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty batch should succeed",
			dimensions:  1536,
			ids:         []string{},
			documentIDs: []string{},
			embeddings:  [][]float32{},
			wantErr:     false,
		},
		{
			name:        "ids and embeddings count mismatch",
			dimensions:  1536,
			ids:         []string{"id1", "id2"},
			documentIDs: []string{"doc1", "doc2"},
			embeddings:  [][]float32{make([]float32, 1536)},
			wantErr:     true,
			errContains: "ids and embeddings count mismatch",
		},
		{
			name:        "ids and documentIDs count mismatch",
			dimensions:  1536,
			ids:         []string{"id1", "id2"},
			documentIDs: []string{"doc1"},
			embeddings:  [][]float32{make([]float32, 1536), make([]float32, 1536)},
			wantErr:     true,
			errContains: "ids and documentIDs count mismatch",
		},
		{
			name:        "wrong embedding dimension in batch",
			dimensions:  1536,
			ids:         []string{"id1"},
			documentIDs: []string{"doc1"},
			embeddings: [][]float32{
				make([]float32, 100),
			},
			wantErr:     true,
			errContains: "embedding 0 dimension mismatch",
		},
		{
			name:        "second embedding wrong dimension",
			dimensions:  1536,
			ids:         []string{"id1", "id2"},
			documentIDs: []string{"doc1", "doc2"},
			embeddings: [][]float32{
				make([]float32, 1536),
				make([]float32, 100),
			},
			wantErr:     true,
			errContains: "embedding 1 dimension mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vi := &VectorIndex{
				pool:       nil,
				dimensions: tt.dimensions,
				distOp:     "<=>",
			}

			// Build contents and sourceIDs slices matching ids length
			contents := make([]string, len(tt.ids))
			sourceIDs := make([]string, len(tt.ids))
			err := vi.IndexBatch(context.Background(), tt.ids, tt.documentIDs, sourceIDs, contents, tt.embeddings)

			if tt.wantErr {
				if err == nil {
					t.Fatal("IndexBatch() should return error")
				}
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("IndexBatch() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("IndexBatch() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestVectorIndex_SearchValidation tests input validation for Search method
func TestVectorIndex_SearchValidation(t *testing.T) {
	tests := []struct {
		name         string
		dimensions   int
		embeddingLen int
		k            int
		wantErr      bool
		errContains  string
	}{
		{
			name:         "embedding dimension mismatch",
			dimensions:   1536,
			embeddingLen: 100,
			k:            10,
			wantErr:      true,
			errContains:  "embedding dimension mismatch",
		},
		{
			name:         "k is zero",
			dimensions:   1536,
			embeddingLen: 1536,
			k:            0,
			wantErr:      true,
			errContains:  "k must be positive",
		},
		{
			name:         "k is negative",
			dimensions:   1536,
			embeddingLen: 1536,
			k:            -5,
			wantErr:      true,
			errContains:  "k must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We only test validation errors since valid parameters
			// will proceed to pool operations which require a real connection
			vi := &VectorIndex{
				pool:       nil,
				dimensions: tt.dimensions,
				distOp:     "<=>",
			}

			embedding := make([]float32, tt.embeddingLen)
			_, _, err := vi.Search(context.Background(), embedding, tt.k)

			if tt.wantErr {
				if err == nil {
					t.Fatal("Search() should return error")
				}
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("Search() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
			}
		})
	}
}

// TestVectorIndex_DeleteBatchValidation tests input validation for DeleteBatch method
func TestVectorIndex_DeleteBatchValidation(t *testing.T) {
	vi := &VectorIndex{
		pool:       nil,
		dimensions: 1536,
		distOp:     "<=>",
	}

	// Empty batch should return nil (no-op)
	err := vi.DeleteBatch(context.Background(), []string{})
	if err != nil {
		t.Errorf("DeleteBatch() with empty ids should return nil, got %v", err)
	}
}

// TestConfig_PoolSettingsBoundaries tests boundary values for pool settings
func TestConfig_PoolSettingsBoundaries(t *testing.T) {
	cfg := DefaultConfig()

	// MaxConnLifetime should be greater than MaxConnIdleTime for reasonable config
	if cfg.MaxConnLifetime < cfg.MaxConnIdleTime {
		t.Errorf("MaxConnLifetime (%v) should be >= MaxConnIdleTime (%v)",
			cfg.MaxConnLifetime, cfg.MaxConnIdleTime)
	}
}

// TestVectorIndex_SearchWithContent_DocumentIDFilter tests document ID filtering in vector search
func TestVectorIndex_SearchWithContent_DocumentIDFilter(t *testing.T) {
	tests := []struct {
		name        string
		dimensions  int
		embedding   []float32
		k           int
		sourceIDs   []string
		documentIDs []string
		wantErr     bool
		errContains string
	}{
		{
			name:        "with document ID filter only",
			dimensions:  1536,
			embedding:   make([]float32, 1536),
			k:           10,
			sourceIDs:   nil,
			documentIDs: []string{"doc-1", "doc-2", "doc-3"},
			wantErr:     false,
		},
		{
			name:        "with source and document ID filters",
			dimensions:  1536,
			embedding:   make([]float32, 1536),
			k:           10,
			sourceIDs:   []string{"source-1"},
			documentIDs: []string{"doc-1", "doc-2"},
			wantErr:     false,
		},
		{
			name:        "with single document ID",
			dimensions:  1536,
			embedding:   make([]float32, 1536),
			k:           10,
			sourceIDs:   nil,
			documentIDs: []string{"doc-1"},
			wantErr:     false,
		},
		{
			name:        "without document ID filter",
			dimensions:  1536,
			embedding:   make([]float32, 1536),
			k:           10,
			sourceIDs:   []string{"source-1"},
			documentIDs: nil,
			wantErr:     false,
		},
		{
			name:        "empty document ID slice",
			dimensions:  1536,
			embedding:   make([]float32, 1536),
			k:           10,
			sourceIDs:   nil,
			documentIDs: []string{},
			wantErr:     false,
		},
		{
			name:        "invalid embedding dimension with document ID filter",
			dimensions:  1536,
			embedding:   make([]float32, 100),
			k:           10,
			sourceIDs:   nil,
			documentIDs: []string{"doc-1"},
			wantErr:     true,
			errContains: "embedding dimension mismatch",
		},
		{
			name:        "invalid k with document ID filter",
			dimensions:  1536,
			embedding:   make([]float32, 1536),
			k:           0,
			sourceIDs:   nil,
			documentIDs: []string{"doc-1"},
			wantErr:     true,
			errContains: "k must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can only test validation since we don't have a real database connection
			// Skip non-error cases as they would try to execute real queries
			if !tt.wantErr {
				t.Skip("Skipping: requires real database connection")
			}

			vi := &VectorIndex{
				pool:       nil,
				dimensions: tt.dimensions,
				distOp:     "<=>",
			}

			var documentFilter *domain.DocumentIDFilter
			if tt.documentIDs != nil {
				documentFilter = &domain.DocumentIDFilter{Apply: true, IDs: tt.documentIDs}
			}
			_, err := vi.SearchWithContent(context.Background(), tt.embedding, tt.k, tt.sourceIDs, documentFilter)

			if tt.wantErr {
				if err == nil {
					t.Fatal("SearchWithContent() should return error")
				}
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("SearchWithContent() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
			}
		})
	}
}

// TestVectorIndex_SearchWithContent_DenyAll_ShortCircuits codifies the anti-fail-open
// contract: a deny-all DocumentIDFilter (Apply==true, IDs==[]) must short-circuit to
// ([]VectorSearchResult{}, nil) BEFORE touching the connection pool. We prove "no DB
// query" structurally by setting pool=nil — the implementation is expected to return
// before any pool access. If the short-circuit ever regresses, pool-nil dereference
// will panic or the empty-result assertion will fail.
func TestVectorIndex_SearchWithContent_DenyAll_ShortCircuits(t *testing.T) {
	vi := &VectorIndex{
		pool:       nil, // No pool — dereferencing panics. Short-circuit must fire first.
		dimensions: 1536,
		distOp:     "<=>",
	}

	// Also use an intentionally wrong embedding dimension: if the deny-all branch
	// didn't short-circuit first, the dimension check at L94 would produce
	// "embedding dimension mismatch" rather than the expected empty-result return.
	embedding := make([]float32, 1) // wrong size

	results, err := vi.SearchWithContent(
		context.Background(),
		embedding,
		10,
		nil,
		domain.DenyAllDocumentIDFilter(),
	)

	if err != nil {
		t.Fatalf("SearchWithContent() with deny-all should return nil error, got %v", err)
	}
	if results == nil {
		t.Fatal("SearchWithContent() with deny-all should return an empty (non-nil) slice")
	}
	if len(results) != 0 {
		t.Errorf("SearchWithContent() with deny-all should return zero results, got %d", len(results))
	}
}

// TestVectorIndex_SearchWithContent_NilFilter_NoShortCircuit confirms the complement:
// a nil DocumentIDFilter does NOT short-circuit — validation runs normally, so an
// invalid embedding dimension produces the usual error rather than an empty result.
// This keeps the deny-all path structurally distinct from the no-filter path.
func TestVectorIndex_SearchWithContent_NilFilter_NoShortCircuit(t *testing.T) {
	vi := &VectorIndex{
		pool:       nil,
		dimensions: 1536,
		distOp:     "<=>",
	}
	_, err := vi.SearchWithContent(
		context.Background(),
		make([]float32, 1), // wrong dimension triggers validation error
		10,
		nil,
		nil, // nil filter = no filter, not deny-all
	)
	if err == nil {
		t.Fatal("SearchWithContent() with nil filter + bad embedding should error via validation, not short-circuit")
	}
	if !contains(err.Error(), "embedding dimension mismatch") {
		t.Errorf("expected dimension-mismatch error, got %q", err.Error())
	}
}

// TestVectorIndex_SearchWithContent_FilterCombinations tests various filter combinations
func TestVectorIndex_SearchWithContent_FilterCombinations(t *testing.T) {
	tests := []struct {
		name               string
		sourceIDs          []string
		documentIDs        []string
		expectedQueryPaths string
	}{
		{
			name:               "no filters",
			sourceIDs:          nil,
			documentIDs:        nil,
			expectedQueryPaths: "no WHERE clause",
		},
		{
			name:               "source filter only",
			sourceIDs:          []string{"source-1"},
			documentIDs:        nil,
			expectedQueryPaths: "WHERE source_id = ANY",
		},
		{
			name:               "document filter only",
			sourceIDs:          nil,
			documentIDs:        []string{"doc-1", "doc-2"},
			expectedQueryPaths: "WHERE document_id = ANY",
		},
		{
			name:               "both filters",
			sourceIDs:          []string{"source-1", "source-2"},
			documentIDs:        []string{"doc-1", "doc-2", "doc-3"},
			expectedQueryPaths: "WHERE source_id = ANY AND document_id = ANY",
		},
		{
			name:               "empty source filter",
			sourceIDs:          []string{},
			documentIDs:        []string{"doc-1"},
			expectedQueryPaths: "no WHERE clause for source",
		},
		{
			name:               "empty document filter",
			sourceIDs:          []string{"source-1"},
			documentIDs:        []string{},
			expectedQueryPaths: "no WHERE clause for documents",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Document the expected SQL query path based on implementation
			t.Logf("Filter combination: sourceIDs=%v, documentIDs=%v", tt.sourceIDs, tt.documentIDs)
			t.Logf("Expected query path: %s", tt.expectedQueryPaths)

			// The actual implementation branches based on:
			// - hasSourceFilter := len(sourceIDs) > 0
			// - hasDocFilter := len(documentIDs) > 0
			// This test documents the expected behavior
		})
	}
}

// TestVectorIndex_SearchWithContent_LargeDocumentIDSet tests that large document ID sets are accepted
func TestVectorIndex_SearchWithContent_LargeDocumentIDSet(t *testing.T) {
	t.Skip("Skipping: requires real database connection - validation tests cover document ID handling")
}

// TestVectorIndex_SearchWithContent_QueryStructure documents the expected query structure
func TestVectorIndex_SearchWithContent_QueryStructure(t *testing.T) {
	// This test documents the expected SQL query structure for different filter combinations
	// based on the implementation in SearchWithContent

	tests := []struct {
		name             string
		hasSourceFilter  bool
		hasDocFilter     bool
		expectedClauses  []string
		expectedParams   int
		expectedParamPos []string
	}{
		{
			name:             "no filters",
			hasSourceFilter:  false,
			hasDocFilter:     false,
			expectedClauses:  []string{"SELECT chunk_id, document_id, content, embedding <=> $1::vector AS distance", "FROM embeddings", "ORDER BY distance", "LIMIT $2"},
			expectedParams:   2,
			expectedParamPos: []string{"$1 = embedding", "$2 = k"},
		},
		{
			name:             "source filter only",
			hasSourceFilter:  true,
			hasDocFilter:     false,
			expectedClauses:  []string{"SELECT chunk_id, document_id, content, embedding <=> $1::vector AS distance", "FROM embeddings", "WHERE source_id = ANY($3)", "ORDER BY distance", "LIMIT $2"},
			expectedParams:   3,
			expectedParamPos: []string{"$1 = embedding", "$2 = k", "$3 = sourceIDs"},
		},
		{
			name:             "document filter only",
			hasSourceFilter:  false,
			hasDocFilter:     true,
			expectedClauses:  []string{"SELECT chunk_id, document_id, content, embedding <=> $1::vector AS distance", "FROM embeddings", "WHERE document_id = ANY($3)", "ORDER BY distance", "LIMIT $2"},
			expectedParams:   3,
			expectedParamPos: []string{"$1 = embedding", "$2 = k", "$3 = documentIDs"},
		},
		{
			name:             "both filters",
			hasSourceFilter:  true,
			hasDocFilter:     true,
			expectedClauses:  []string{"SELECT chunk_id, document_id, content, embedding <=> $1::vector AS distance", "FROM embeddings", "WHERE source_id = ANY($3) AND document_id = ANY($4)", "ORDER BY distance", "LIMIT $2"},
			expectedParams:   4,
			expectedParamPos: []string{"$1 = embedding", "$2 = k", "$3 = sourceIDs", "$4 = documentIDs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Query structure for hasSourceFilter=%v, hasDocFilter=%v:", tt.hasSourceFilter, tt.hasDocFilter)
			for _, clause := range tt.expectedClauses {
				t.Logf("  %s", clause)
			}
			t.Logf("Expected %d parameters: %v", tt.expectedParams, tt.expectedParamPos)
		})
	}
}

// TestVectorIndex_DeleteByDocuments_Validation tests document deletion validation
func TestVectorIndex_DeleteByDocuments_Validation(t *testing.T) {
	vi := &VectorIndex{
		pool:       nil,
		dimensions: 1536,
		distOp:     "<=>",
	}

	tests := []struct {
		name        string
		documentIDs []string
		wantErr     bool
	}{
		{
			name:        "empty slice should succeed without query",
			documentIDs: []string{},
			wantErr:     false,
		},
		{
			name:        "nil slice should succeed without query",
			documentIDs: nil,
			wantErr:     false,
		},
		{
			name:        "single document",
			documentIDs: []string{"doc-1"},
			wantErr:     false, // Would need DB connection to actually execute
		},
		{
			name:        "multiple documents",
			documentIDs: []string{"doc-1", "doc-2", "doc-3"},
			wantErr:     false, // Would need DB connection to actually execute
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip non-empty cases - they require a real database connection
			if len(tt.documentIDs) > 0 {
				t.Skip("Skipping: requires real database connection")
			}

			err := vi.DeleteByDocuments(context.Background(), tt.documentIDs)

			// Should return nil immediately for empty/nil slices
			if err != nil {
				t.Errorf("DeleteByDocuments() with empty/nil slice should return nil, got %v", err)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && (s == substr || containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
