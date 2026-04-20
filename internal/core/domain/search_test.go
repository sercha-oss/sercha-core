package domain

import (
	"encoding/json"
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

func TestSearchOptionsWithBoostTerms(t *testing.T) {
	tests := []struct {
		name       string
		opts       SearchOptions
		wantBoosts int
		wantNil    bool
	}{
		{
			name: "with boost terms",
			opts: SearchOptions{
				Mode:   SearchModeHybrid,
				Limit:  20,
				Offset: 0,
				BoostTerms: map[string]float64{
					"kubernetes": 2.0,
					"helm":       1.5,
					"production": 1.2,
				},
			},
			wantBoosts: 3,
			wantNil:    false,
		},
		{
			name: "without boost terms",
			opts: SearchOptions{
				Mode:   SearchModeTextOnly,
				Limit:  10,
				Offset: 0,
			},
			wantBoosts: 0,
			wantNil:    true,
		},
		{
			name: "empty boost terms map",
			opts: SearchOptions{
				Mode:       SearchModeSemanticOnly,
				Limit:      50,
				Offset:     10,
				BoostTerms: map[string]float64{},
			},
			wantBoosts: 0,
			wantNil:    false,
		},
		{
			name: "single boost term",
			opts: SearchOptions{
				Mode:  SearchModeHybrid,
				Limit: 20,
				BoostTerms: map[string]float64{
					"urgent": 3.0,
				},
			},
			wantBoosts: 1,
			wantNil:    false,
		},
		{
			name: "boost with filters and source IDs",
			opts: SearchOptions{
				Mode:      SearchModeHybrid,
				Limit:     20,
				Offset:    0,
				SourceIDs: []string{"source-1", "source-2"},
				Filters: Filters{
					MimeTypes: []string{"text/plain"},
				},
				BoostTerms: map[string]float64{
					"important": 2.5,
				},
			},
			wantBoosts: 1,
			wantNil:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantNil && tt.opts.BoostTerms != nil {
				t.Error("expected BoostTerms to be nil")
			}
			if !tt.wantNil && tt.wantBoosts > 0 && tt.opts.BoostTerms == nil {
				t.Error("expected BoostTerms to be non-nil")
			}
			if len(tt.opts.BoostTerms) != tt.wantBoosts {
				t.Errorf("expected %d boost terms, got %d", tt.wantBoosts, len(tt.opts.BoostTerms))
			}

			// Verify specific boost values if present
			if tt.wantBoosts > 0 {
				for term, boost := range tt.opts.BoostTerms {
					if boost <= 0 {
						t.Errorf("boost for term %q should be positive, got %f", term, boost)
					}
				}
			}
		})
	}
}

func TestSearchOptionsBoostTermsValues(t *testing.T) {
	opts := SearchOptions{
		Mode:  SearchModeHybrid,
		Limit: 20,
		BoostTerms: map[string]float64{
			"kubernetes": 2.0,
			"helm":       1.5,
			"docker":     0.8,
		},
	}

	// Test specific boost values
	if boost, ok := opts.BoostTerms["kubernetes"]; !ok || boost != 2.0 {
		t.Errorf("kubernetes boost = %v, want 2.0", boost)
	}
	if boost, ok := opts.BoostTerms["helm"]; !ok || boost != 1.5 {
		t.Errorf("helm boost = %v, want 1.5", boost)
	}
	if boost, ok := opts.BoostTerms["docker"]; !ok || boost != 0.8 {
		t.Errorf("docker boost = %v, want 0.8", boost)
	}

	// Test non-existent term
	if _, ok := opts.BoostTerms["nonexistent"]; ok {
		t.Error("nonexistent term should not be in BoostTerms")
	}
}

func TestDefaultSearchOptionsHasNoBoostTerms(t *testing.T) {
	opts := DefaultSearchOptions()

	if opts.BoostTerms != nil {
		t.Errorf("default SearchOptions should have nil BoostTerms, got %v", opts.BoostTerms)
	}
	if len(opts.BoostTerms) != 0 {
		t.Errorf("default SearchOptions should have 0 boost terms, got %d", len(opts.BoostTerms))
	}
}

func TestSearchOptionsJSONMarshaling(t *testing.T) {
	tests := []struct {
		name     string
		opts     SearchOptions
		wantJSON string
	}{
		{
			name: "with boost terms",
			opts: SearchOptions{
				Mode:   SearchModeHybrid,
				Limit:  20,
				Offset: 0,
				BoostTerms: map[string]float64{
					"kubernetes": 2.0,
					"helm":       1.5,
				},
			},
			wantJSON: `{"mode":"hybrid","limit":20,"offset":0,"filters":{},"boost_terms":{"kubernetes":2.0,"helm":1.5}}`,
		},
		{
			name: "without boost terms omits field",
			opts: SearchOptions{
				Mode:   SearchModeTextOnly,
				Limit:  10,
				Offset: 0,
			},
			wantJSON: `{"mode":"text","limit":10,"offset":0,"filters":{}}`,
		},
		{
			name: "empty boost terms map omits field",
			opts: SearchOptions{
				Mode:       SearchModeSemanticOnly,
				Limit:      50,
				Offset:     10,
				BoostTerms: map[string]float64{},
			},
			wantJSON: `{"mode":"semantic","limit":50,"offset":10,"filters":{}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.opts)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			// Parse both to compare without key order issues
			var gotMap, wantMap map[string]any
			if err := json.Unmarshal(data, &gotMap); err != nil {
				t.Fatalf("Unmarshal got: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.wantJSON), &wantMap); err != nil {
				t.Fatalf("Unmarshal want: %v", err)
			}

			// Check all expected fields are present
			for key, wantVal := range wantMap {
				gotVal, ok := gotMap[key]
				if !ok {
					t.Errorf("missing key %q in marshaled JSON", key)
					continue
				}

				// Special handling for nested maps (boost_terms)
				if key == "boost_terms" {
					wantBoosts := wantVal.(map[string]any)
					gotBoosts := gotVal.(map[string]any)
					if len(gotBoosts) != len(wantBoosts) {
						t.Errorf("boost_terms length = %d, want %d", len(gotBoosts), len(wantBoosts))
					}
					for term, wantBoost := range wantBoosts {
						if gotBoost, ok := gotBoosts[term]; !ok {
							t.Errorf("missing boost term %q", term)
						} else if gotBoost != wantBoost {
							t.Errorf("boost for %q = %v, want %v", term, gotBoost, wantBoost)
						}
					}
				}
			}

			// Check no unexpected required fields (skip omitempty fields like source_ids, document_ids)
			for key := range gotMap {
				if _, ok := wantMap[key]; !ok {
					// Allow omitempty fields to be present or absent
					if key != "source_ids" && key != "document_ids" {
						t.Errorf("unexpected key %q in marshaled JSON", key)
					}
				}
			}
		})
	}
}

func TestSearchOptionsJSONUnmarshaling(t *testing.T) {
	tests := []struct {
		name         string
		json         string
		wantMode     SearchMode
		wantLimit    int
		wantBoosts   map[string]float64
		wantErr      bool
		validateMore func(*testing.T, SearchOptions)
	}{
		{
			name:       "unmarshal with boost terms",
			json:       `{"mode":"hybrid","limit":20,"offset":0,"boost_terms":{"kubernetes":2.0,"helm":1.5}}`,
			wantMode:   SearchModeHybrid,
			wantLimit:  20,
			wantBoosts: map[string]float64{"kubernetes": 2.0, "helm": 1.5},
			wantErr:    false,
		},
		{
			name:       "unmarshal without boost terms",
			json:       `{"mode":"text","limit":10,"offset":5}`,
			wantMode:   SearchModeTextOnly,
			wantLimit:  10,
			wantBoosts: nil,
			wantErr:    false,
		},
		{
			name:       "unmarshal with empty boost terms",
			json:       `{"mode":"semantic","limit":30,"boost_terms":{}}`,
			wantMode:   SearchModeSemanticOnly,
			wantLimit:  30,
			wantBoosts: map[string]float64{},
			wantErr:    false,
		},
		{
			name:       "unmarshal with single boost term",
			json:       `{"mode":"hybrid","limit":20,"boost_terms":{"production":3.0}}`,
			wantMode:   SearchModeHybrid,
			wantLimit:  20,
			wantBoosts: map[string]float64{"production": 3.0},
			wantErr:    false,
		},
		{
			name:       "unmarshal with fractional boost",
			json:       `{"mode":"hybrid","limit":20,"boost_terms":{"important":1.25}}`,
			wantMode:   SearchModeHybrid,
			wantLimit:  20,
			wantBoosts: map[string]float64{"important": 1.25},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts SearchOptions
			err := json.Unmarshal([]byte(tt.json), &opts)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if opts.Mode != tt.wantMode {
					t.Errorf("Mode = %v, want %v", opts.Mode, tt.wantMode)
				}
				if opts.Limit != tt.wantLimit {
					t.Errorf("Limit = %v, want %v", opts.Limit, tt.wantLimit)
				}

				// Check BoostTerms
				if tt.wantBoosts == nil {
					if len(opts.BoostTerms) > 0 {
						t.Errorf("BoostTerms should be nil or empty, got %v", opts.BoostTerms)
					}
				} else {
					if len(opts.BoostTerms) != len(tt.wantBoosts) {
						t.Errorf("BoostTerms length = %d, want %d", len(opts.BoostTerms), len(tt.wantBoosts))
					}
					for term, wantBoost := range tt.wantBoosts {
						if gotBoost, ok := opts.BoostTerms[term]; !ok {
							t.Errorf("missing boost term %q", term)
						} else if gotBoost != wantBoost {
							t.Errorf("boost for %q = %v, want %v", term, gotBoost, wantBoost)
						}
					}
				}

				if tt.validateMore != nil {
					tt.validateMore(t, opts)
				}
			}
		})
	}
}

func TestSearchOptionsJSONRoundTrip(t *testing.T) {
	original := SearchOptions{
		Mode:      SearchModeHybrid,
		Limit:     25,
		Offset:    10,
		SourceIDs: []string{"source-1", "source-2"},
		BoostTerms: map[string]float64{
			"kubernetes": 2.0,
			"helm":       1.5,
			"production": 1.2,
		},
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Unmarshal
	var decoded SearchOptions
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	// Verify round-trip
	if decoded.Mode != original.Mode {
		t.Errorf("Mode = %v, want %v", decoded.Mode, original.Mode)
	}
	if decoded.Limit != original.Limit {
		t.Errorf("Limit = %v, want %v", decoded.Limit, original.Limit)
	}
	if decoded.Offset != original.Offset {
		t.Errorf("Offset = %v, want %v", decoded.Offset, original.Offset)
	}
	if len(decoded.BoostTerms) != len(original.BoostTerms) {
		t.Errorf("BoostTerms length = %d, want %d", len(decoded.BoostTerms), len(original.BoostTerms))
	}
	for term, originalBoost := range original.BoostTerms {
		if decodedBoost, ok := decoded.BoostTerms[term]; !ok {
			t.Errorf("missing boost term %q after round-trip", term)
		} else if decodedBoost != originalBoost {
			t.Errorf("boost for %q = %v, want %v", term, decodedBoost, originalBoost)
		}
	}
}
