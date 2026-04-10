package mocks

import (
	"context"
	"testing"
)

// TestMockVectorIndex_SearchWithContent_NoFilter tests SearchWithContent without source filter
func TestMockVectorIndex_SearchWithContent_NoFilter(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	// Index chunks with different sources
	_ = mock.IndexBatch(ctx,
		[]string{"c1", "c2", "c3"},
		[]string{"d1", "d1", "d2"},
		[]string{"src-A", "src-B", "src-A"},
		[]string{"content1", "content2", "content3"},
		[][]float32{{0.1}, {0.2}, {0.3}},
	)

	// Search without source filter — should return all
	results, err := mock.SearchWithContent(ctx, []float32{0.1}, 10, nil)
	if err != nil {
		t.Fatalf("SearchWithContent() error = %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results without filter, got %d", len(results))
	}
}

// TestMockVectorIndex_SearchWithContent_EmptyFilter tests with empty source slice
func TestMockVectorIndex_SearchWithContent_EmptyFilter(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	_ = mock.IndexBatch(ctx,
		[]string{"c1", "c2"},
		[]string{"d1", "d2"},
		[]string{"src-A", "src-B"},
		[]string{"content1", "content2"},
		[][]float32{{0.1}, {0.2}},
	)

	// Empty slice = no filter
	results, err := mock.SearchWithContent(ctx, []float32{0.1}, 10, []string{})
	if err != nil {
		t.Fatalf("SearchWithContent() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results with empty filter, got %d", len(results))
	}
}

// TestMockVectorIndex_SearchWithContent_SourceFilter tests filtering by source_id
func TestMockVectorIndex_SearchWithContent_SourceFilter(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	_ = mock.IndexBatch(ctx,
		[]string{"c1", "c2", "c3", "c4"},
		[]string{"d1", "d1", "d2", "d3"},
		[]string{"src-A", "src-B", "src-A", "src-C"},
		[]string{"alpha", "bravo", "charlie", "delta"},
		[][]float32{{0.1}, {0.2}, {0.3}, {0.4}},
	)

	// Filter to src-A only
	results, err := mock.SearchWithContent(ctx, []float32{0.1}, 10, []string{"src-A"})
	if err != nil {
		t.Fatalf("SearchWithContent() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results for src-A, got %d", len(results))
	}

	for _, r := range results {
		// Verify all returned chunks belong to src-A
		srcID := mock.sourceIDs[r.ChunkID]
		if srcID != "src-A" {
			t.Errorf("chunk %s has source %q, expected src-A", r.ChunkID, srcID)
		}
	}
}

// TestMockVectorIndex_SearchWithContent_MultiSourceFilter tests filtering by multiple sources
func TestMockVectorIndex_SearchWithContent_MultiSourceFilter(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	_ = mock.IndexBatch(ctx,
		[]string{"c1", "c2", "c3"},
		[]string{"d1", "d2", "d3"},
		[]string{"src-A", "src-B", "src-C"},
		[]string{"alpha", "bravo", "charlie"},
		[][]float32{{0.1}, {0.2}, {0.3}},
	)

	// Filter to src-A and src-C
	results, err := mock.SearchWithContent(ctx, []float32{0.1}, 10, []string{"src-A", "src-C"})
	if err != nil {
		t.Fatalf("SearchWithContent() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results for src-A+src-C, got %d", len(results))
	}
}

// TestMockVectorIndex_SearchWithContent_NoMatch tests filter that matches nothing
func TestMockVectorIndex_SearchWithContent_NoMatch(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	_ = mock.IndexBatch(ctx,
		[]string{"c1"},
		[]string{"d1"},
		[]string{"src-A"},
		[]string{"content"},
		[][]float32{{0.1}},
	)

	results, err := mock.SearchWithContent(ctx, []float32{0.1}, 10, []string{"src-NONEXISTENT"})
	if err != nil {
		t.Fatalf("SearchWithContent() error = %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for non-existent source, got %d", len(results))
	}
}

// TestMockVectorIndex_SearchWithContent_KLimit tests k limits results
func TestMockVectorIndex_SearchWithContent_KLimit(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	_ = mock.IndexBatch(ctx,
		[]string{"c1", "c2", "c3"},
		[]string{"d1", "d2", "d3"},
		[]string{"src-A", "src-A", "src-A"},
		[]string{"one", "two", "three"},
		[][]float32{{0.1}, {0.2}, {0.3}},
	)

	results, err := mock.SearchWithContent(ctx, []float32{0.1}, 2, nil)
	if err != nil {
		t.Fatalf("SearchWithContent() error = %v", err)
	}

	if len(results) > 2 {
		t.Errorf("expected at most 2 results with k=2, got %d", len(results))
	}
}

// TestMockVectorIndex_IndexBatch_StoresSourceIDs tests that IndexBatch stores source_ids
func TestMockVectorIndex_IndexBatch_StoresSourceIDs(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	err := mock.IndexBatch(ctx,
		[]string{"c1", "c2"},
		[]string{"d1", "d2"},
		[]string{"src-X", "src-Y"},
		[]string{"content1", "content2"},
		[][]float32{{0.1}, {0.2}},
	)
	if err != nil {
		t.Fatalf("IndexBatch() error = %v", err)
	}

	if mock.sourceIDs["c1"] != "src-X" {
		t.Errorf("sourceIDs[c1] = %q, want src-X", mock.sourceIDs["c1"])
	}
	if mock.sourceIDs["c2"] != "src-Y" {
		t.Errorf("sourceIDs[c2] = %q, want src-Y", mock.sourceIDs["c2"])
	}
}

// TestMockVectorIndex_Reset_ClearsSourceIDs tests Reset clears source_ids map
func TestMockVectorIndex_Reset_ClearsSourceIDs(t *testing.T) {
	mock := NewMockVectorIndex()
	ctx := context.Background()

	_ = mock.IndexBatch(ctx,
		[]string{"c1"},
		[]string{"d1"},
		[]string{"src-A"},
		[]string{"content"},
		[][]float32{{0.1}},
	)

	if len(mock.sourceIDs) != 1 {
		t.Fatalf("sourceIDs should have 1 entry before reset")
	}

	mock.Reset()

	if len(mock.sourceIDs) != 0 {
		t.Errorf("sourceIDs should be empty after reset, got %d", len(mock.sourceIDs))
	}
}
