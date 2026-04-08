package domain

import (
	"testing"
	"time"
)

func TestSearchModeConstants(t *testing.T) {
	if SearchModeHybrid != "hybrid" {
		t.Errorf("expected SearchModeHybrid = 'hybrid', got %s", SearchModeHybrid)
	}
	if SearchModeTextOnly != "text" {
		t.Errorf("expected SearchModeTextOnly = 'text', got %s", SearchModeTextOnly)
	}
	if SearchModeSemanticOnly != "semantic" {
		t.Errorf("expected SearchModeSemanticOnly = 'semantic', got %s", SearchModeSemanticOnly)
	}
}

func TestDefaultSearchOptions(t *testing.T) {
	opts := DefaultSearchOptions()

	if opts.Mode != SearchModeHybrid {
		t.Errorf("expected default mode Hybrid, got %s", opts.Mode)
	}
	if opts.Limit != 20 {
		t.Errorf("expected default limit 20, got %d", opts.Limit)
	}
	if opts.Offset != 0 {
		t.Errorf("expected default offset 0, got %d", opts.Offset)
	}
	if len(opts.SourceIDs) != 0 {
		t.Errorf("expected empty SourceIDs, got %v", opts.SourceIDs)
	}
}

func TestSearchOptions(t *testing.T) {
	opts := SearchOptions{
		Mode:      SearchModeSemanticOnly,
		Limit:     50,
		Offset:    10,
		SourceIDs: []string{"source-1", "source-2"},
		Filters: Filters{
			MimeTypes: []string{"text/plain", "text/markdown"},
		},
	}

	if opts.Mode != SearchModeSemanticOnly {
		t.Errorf("expected mode SemanticOnly, got %s", opts.Mode)
	}
	if opts.Limit != 50 {
		t.Errorf("expected limit 50, got %d", opts.Limit)
	}
	if opts.Offset != 10 {
		t.Errorf("expected offset 10, got %d", opts.Offset)
	}
	if len(opts.SourceIDs) != 2 {
		t.Errorf("expected 2 source IDs, got %d", len(opts.SourceIDs))
	}
	if len(opts.Filters.MimeTypes) != 2 {
		t.Errorf("expected 2 mime types, got %d", len(opts.Filters.MimeTypes))
	}
}

func TestFilters(t *testing.T) {
	now := time.Now()
	before := now.Add(-24 * time.Hour)

	filters := Filters{
		MimeTypes:  []string{"text/plain"},
		DateAfter:  &before,
		DateBefore: &now,
	}

	if len(filters.MimeTypes) != 1 {
		t.Errorf("expected 1 mime type, got %d", len(filters.MimeTypes))
	}
	if filters.DateAfter == nil {
		t.Error("expected DateAfter to be set")
	}
	if filters.DateBefore == nil {
		t.Error("expected DateBefore to be set")
	}
	if !filters.DateAfter.Before(*filters.DateBefore) {
		t.Error("DateAfter should be before DateBefore")
	}
}

func TestSearchResult(t *testing.T) {
	items := []*SearchResultItem{
		{
			DocumentID: "doc-1",
			Title:      "Test Document",
			Snippet:    "test content",
			Score:      0.95,
		},
		{
			DocumentID: "doc-2",
			Title:      "Another Document",
			Snippet:    "more content",
			Score:      0.85,
		},
	}

	result := &SearchResult{
		Query:      "test query",
		Mode:       SearchModeHybrid,
		Results:    items,
		TotalCount: 100,
		Took:       100 * time.Millisecond,
	}

	if result.Query != "test query" {
		t.Errorf("expected query 'test query', got %s", result.Query)
	}
	if result.Mode != SearchModeHybrid {
		t.Errorf("expected mode Hybrid, got %s", result.Mode)
	}
	if len(result.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(result.Results))
	}
	if result.TotalCount != 100 {
		t.Errorf("expected total count 100, got %d", result.TotalCount)
	}
	if result.Took != 100*time.Millisecond {
		t.Errorf("expected took 100ms, got %v", result.Took)
	}
}

func TestRankedChunk(t *testing.T) {
	chunk := &Chunk{
		ID:         "chunk-1",
		DocumentID: "doc-1",
		Content:    "test content",
	}
	doc := &Document{
		ID:    "doc-1",
		Title: "Test Document",
	}

	ranked := &RankedChunk{
		Chunk:      chunk,
		Document:   doc,
		Score:      0.95,
		Highlights: []string{"<em>test</em> content"},
	}

	if ranked.Chunk.ID != "chunk-1" {
		t.Errorf("expected chunk ID chunk-1, got %s", ranked.Chunk.ID)
	}
	if ranked.Document.ID != "doc-1" {
		t.Errorf("expected document ID doc-1, got %s", ranked.Document.ID)
	}
	if ranked.Score != 0.95 {
		t.Errorf("expected score 0.95, got %f", ranked.Score)
	}
	if len(ranked.Highlights) != 1 {
		t.Errorf("expected 1 highlight, got %d", len(ranked.Highlights))
	}
}
