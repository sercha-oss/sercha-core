package search

import (
	"context"
	"math"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
)

func TestRankerFactory_StageID(t *testing.T) {
	f := NewRankerFactory()
	if f.StageID() != "ranker" {
		t.Errorf("StageID() = %q, want %q", f.StageID(), "ranker")
	}
}

func TestRankerFactory_Descriptor(t *testing.T) {
	f := NewRankerFactory()
	d := f.Descriptor()

	if d.InputShape != pipeline.ShapeCandidate {
		t.Errorf("InputShape = %q, want %q", d.InputShape, pipeline.ShapeCandidate)
	}
	if d.OutputShape != pipeline.ShapeRankedResult {
		t.Errorf("OutputShape = %q, want %q", d.OutputShape, pipeline.ShapeRankedResult)
	}
}

func TestRankerStage_EmptyInput(t *testing.T) {
	f := NewRankerFactory()
	stage, err := f.Create(pipeline.StageConfig{Parameters: map[string]any{}}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	result, err := stage.Process(context.Background(), []*pipeline.Candidate{})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	candidates := result.([]*pipeline.Candidate)
	if len(candidates) != 0 {
		t.Errorf("Process() returned %d candidates, want 0", len(candidates))
	}
}

func TestRankerStage_InvalidInput(t *testing.T) {
	f := NewRankerFactory()
	stage, err := f.Create(pipeline.StageConfig{Parameters: map[string]any{}}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err = stage.Process(context.Background(), "not candidates")
	if err == nil {
		t.Fatal("Process() should return error for invalid input")
	}
}

func TestRankerStage_SingleSource(t *testing.T) {
	f := NewRankerFactory()
	stage, err := f.Create(pipeline.StageConfig{
		Parameters: map[string]any{"limit": float64(10)},
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Each candidate has a unique DocumentID so they remain separate after doc-level dedup
	candidates := []*pipeline.Candidate{
		{DocumentID: "d1", ChunkID: "c1", Score: 0.9, Source: "bm25"},
		{DocumentID: "d2", ChunkID: "c2", Score: 0.7, Source: "bm25"},
		{DocumentID: "d3", ChunkID: "c3", Score: 0.5, Source: "bm25"},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	ranked := result.([]*pipeline.Candidate)
	if len(ranked) != 3 {
		t.Fatalf("Process() returned %d candidates, want 3", len(ranked))
	}

	// With single source, RRF should maintain rank order (d1 > d2 > d3)
	if ranked[0].DocumentID != "d1" {
		t.Errorf("ranked[0].DocumentID = %q, want d1", ranked[0].DocumentID)
	}
	if ranked[1].DocumentID != "d2" {
		t.Errorf("ranked[1].DocumentID = %q, want d2", ranked[1].DocumentID)
	}
	if ranked[2].DocumentID != "d3" {
		t.Errorf("ranked[2].DocumentID = %q, want d3", ranked[2].DocumentID)
	}

	// Scores should be descending
	if ranked[0].Score <= ranked[1].Score {
		t.Errorf("scores not descending: %v >= %v", ranked[0].Score, ranked[1].Score)
	}

	// With min-max normalization: scores 0.9, 0.7, 0.5 → 100%, 50%, 0%
	if math.Abs(ranked[0].Score-100.0) > 0.01 {
		t.Errorf("ranked[0].Score = %v, want ~100 (max score)", ranked[0].Score)
	}
	if math.Abs(ranked[1].Score-50.0) > 0.01 {
		t.Errorf("ranked[1].Score = %v, want ~50 (mid score)", ranked[1].Score)
	}
	if math.Abs(ranked[2].Score-0.0) > 0.01 {
		t.Errorf("ranked[2].Score = %v, want ~0 (min score)", ranked[2].Score)
	}
}

func TestRankerStage_MultiSource_RRF(t *testing.T) {
	f := NewRankerFactory()
	stage, err := f.Create(pipeline.StageConfig{
		Parameters: map[string]any{
			"limit": float64(10),
			"rrf_k": float64(60),
		},
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Document "d1" appears in both sources (rank 1 in bm25, rank 2 in vector)
	// Document "d2" appears only in bm25 (rank 2)
	// Document "d3" appears only in vector (rank 1)
	// Document "d4" appears in both (rank 3 in bm25, rank 3 in vector)
	candidates := []*pipeline.Candidate{
		{DocumentID: "d1", ChunkID: "", Score: 0.9, Source: "bm25", Content: "doc 1"},
		{DocumentID: "d2", ChunkID: "", Score: 0.7, Source: "bm25", Content: "doc 2"},
		{DocumentID: "d4", ChunkID: "", Score: 0.3, Source: "bm25", Content: "doc 4"},
		{DocumentID: "d3", ChunkID: "c3", Score: 0.95, Source: "vector", Content: "chunk 3"},
		{DocumentID: "d1", ChunkID: "c1", Score: 0.8, Source: "vector", Content: "chunk 1"},
		{DocumentID: "d4", ChunkID: "c4", Score: 0.7, Source: "vector", Content: "chunk 4"},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	ranked := result.([]*pipeline.Candidate)
	if len(ranked) != 4 {
		t.Fatalf("Process() returned %d candidates, want 4 (deduped by DocumentID)", len(ranked))
	}

	// d1 appears in both sources: RRF(d1) = 1/(60+1) + 1/(60+2) ≈ 0.01639 + 0.01613 = 0.03252
	// d3 appears in vector rank 1: RRF(d3) = 1/(60+1) ≈ 0.01639
	// d2 appears in bm25 rank 2:   RRF(d2) = 1/(60+2) ≈ 0.01613
	// d4 appears in both rank 3:   RRF(d4) = 1/(60+3) + 1/(60+3) ≈ 0.01587 + 0.01587 = 0.03175
	// Expected order: d1 > d4 > d3 > d2
	if ranked[0].DocumentID != "d1" {
		t.Errorf("ranked[0].DocumentID = %q, want d1 (highest RRF)", ranked[0].DocumentID)
	}
	if ranked[1].DocumentID != "d4" {
		t.Errorf("ranked[1].DocumentID = %q, want d4", ranked[1].DocumentID)
	}

	// Verify normalized percentage scores are roughly correct
	// d1: rank 1 in bm25, rank 2 in vector
	// rawScore = 1/61 + 1/62
	// theoreticalMax = 2 * (1/61) = 2/61
	// normalized = (rawScore / theoreticalMax) * 100
	rawD1 := 1.0/61.0 + 1.0/62.0
	theoreticalMax := 2.0 * (1.0 / 61.0)
	expectedD1 := (rawD1 / theoreticalMax) * 100.0
	if math.Abs(ranked[0].Score-expectedD1) > 0.01 {
		t.Errorf("d1 normalized score = %v, want ≈ %v", ranked[0].Score, expectedD1)
	}

	// Verify raw RRF score is preserved in metadata
	rawScore, ok := ranked[0].Metadata["rrf_raw_score"].(float64)
	if !ok {
		t.Fatal("expected rrf_raw_score metadata on ranked candidates")
	}
	if math.Abs(rawScore-rawD1) > 0.0001 {
		t.Errorf("raw RRF score = %v, want ≈ %v", rawScore, rawD1)
	}

	// Verify rrf_sources metadata
	sources, ok := ranked[0].Metadata["rrf_sources"].([]string)
	if !ok {
		t.Fatal("expected rrf_sources metadata on ranked candidates")
	}
	if len(sources) != 2 {
		t.Errorf("d1 should have 2 sources, got %d", len(sources))
	}
}

func TestRankerStage_Limit(t *testing.T) {
	f := NewRankerFactory()
	stage, err := f.Create(pipeline.StageConfig{
		Parameters: map[string]any{"limit": float64(2)},
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Each candidate has a unique DocumentID
	candidates := []*pipeline.Candidate{
		{DocumentID: "d1", ChunkID: "c1", Score: 0.9, Source: "bm25"},
		{DocumentID: "d2", ChunkID: "c2", Score: 0.7, Source: "bm25"},
		{DocumentID: "d3", ChunkID: "c3", Score: 0.5, Source: "bm25"},
		{DocumentID: "d4", ChunkID: "c4", Score: 0.3, Source: "bm25"},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	ranked := result.([]*pipeline.Candidate)
	if len(ranked) != 2 {
		t.Errorf("Process() returned %d candidates, want 2 (limited)", len(ranked))
	}
}

func TestRankerStage_DuplicateDocIDs_SameSource(t *testing.T) {
	f := NewRankerFactory()
	stage, err := f.Create(pipeline.StageConfig{
		Parameters: map[string]any{"limit": float64(10)},
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Two chunks from the same document in vector source — should aggregate to one doc
	// Plus a different document
	candidates := []*pipeline.Candidate{
		{DocumentID: "d1", ChunkID: "c1", Score: 0.9, Source: "vector"},
		{DocumentID: "d1", ChunkID: "c2", Score: 0.5, Source: "vector"},
		{DocumentID: "d2", ChunkID: "c3", Score: 0.7, Source: "vector"},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	ranked := result.([]*pipeline.Candidate)
	// d1 and d2 — two unique documents
	if len(ranked) != 2 {
		t.Errorf("Process() returned %d candidates, want 2 (deduped by DocumentID)", len(ranked))
	}
}

func TestRankerStage_CustomRRFK(t *testing.T) {
	f := NewRankerFactory()
	stage, err := f.Create(pipeline.StageConfig{
		Parameters: map[string]any{
			"limit": float64(10),
			"rrf_k": float64(1),
		},
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Same document in both sources
	candidates := []*pipeline.Candidate{
		{DocumentID: "d1", ChunkID: "", Score: 0.9, Source: "bm25"},
		{DocumentID: "d1", ChunkID: "c1", Score: 0.8, Source: "vector"},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	ranked := result.([]*pipeline.Candidate)
	if len(ranked) != 1 {
		t.Fatalf("Process() returned %d candidates, want 1", len(ranked))
	}

	// With k=1, 2 sources: raw = 1/(1+1) + 1/(1+1) = 0.5 + 0.5 = 1.0
	// theoreticalMax = 2 * (1/(1+1)) = 2 * 0.5 = 1.0
	// normalized = (1.0 / 1.0) * 100 = 100
	expectedScore := 100.0
	if math.Abs(ranked[0].Score-expectedScore) > 0.0001 {
		t.Errorf("RRF normalized score with k=1 = %v, want %v", ranked[0].Score, expectedScore)
	}
}

func TestRankerStage_EmptySource(t *testing.T) {
	f := NewRankerFactory()
	stage, err := f.Create(pipeline.StageConfig{
		Parameters: map[string]any{"limit": float64(10)},
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Candidates with empty source should be handled — each has unique DocumentID
	candidates := []*pipeline.Candidate{
		{DocumentID: "d1", ChunkID: "c1", Score: 0.9, Source: ""},
		{DocumentID: "d2", ChunkID: "c2", Score: 0.7, Source: ""},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	ranked := result.([]*pipeline.Candidate)
	if len(ranked) != 2 {
		t.Errorf("Process() returned %d candidates, want 2", len(ranked))
	}
}

func TestRankerStage_ChunkAggregation_BestChunkWins(t *testing.T) {
	f := NewRankerFactory()
	stage, err := f.Create(pipeline.StageConfig{
		Parameters: map[string]any{
			"limit": float64(10),
			"rrf_k": float64(60),
		},
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Multiple vector chunks from same document — best chunk should represent the doc
	candidates := []*pipeline.Candidate{
		{DocumentID: "d1", ChunkID: "c1", Score: 0.3, Source: "vector", Content: "low chunk"},
		{DocumentID: "d1", ChunkID: "c2", Score: 0.9, Source: "vector", Content: "best chunk"},
		{DocumentID: "d1", ChunkID: "c3", Score: 0.5, Source: "vector", Content: "mid chunk"},
		{DocumentID: "d2", ChunkID: "c4", Score: 0.8, Source: "vector", Content: "doc2 chunk"},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	ranked := result.([]*pipeline.Candidate)
	if len(ranked) != 2 {
		t.Fatalf("Process() returned %d candidates, want 2 (doc-level)", len(ranked))
	}

	// d1 should rank first (its best chunk score 0.9 > d2's 0.8, so rank 1 vs rank 2)
	if ranked[0].DocumentID != "d1" {
		t.Errorf("ranked[0].DocumentID = %q, want d1 (best chunk 0.9)", ranked[0].DocumentID)
	}
}

func TestRankerStage_ScoreNormalization(t *testing.T) {
	tests := []struct {
		name       string
		rrfK       float64
		candidates []*pipeline.Candidate
		wantScore  float64 // expected normalized score for top result
		wantNumSrc int     // expected number of sources in rrf_sources metadata
	}{
		{
			name: "hybrid rank1 both sources = 100%",
			rrfK: 60,
			candidates: []*pipeline.Candidate{
				{DocumentID: "d1", Score: 0.9, Source: "bm25", Metadata: make(map[string]any)},
				{DocumentID: "d1", ChunkID: "c1", Score: 0.8, Source: "vector", Metadata: make(map[string]any)},
			},
			wantScore:  100.0,
			wantNumSrc: 2,
		},
		{
			name: "hybrid rank1 one source only = 50%",
			rrfK: 60,
			candidates: []*pipeline.Candidate{
				{DocumentID: "d1", Score: 0.9, Source: "bm25", Metadata: make(map[string]any)},
				{DocumentID: "d2", ChunkID: "c2", Score: 0.8, Source: "vector", Metadata: make(map[string]any)},
			},
			wantScore:  50.0,
			wantNumSrc: 1,
		},
		{
			name: "single source rank1 = 100%",
			rrfK: 60,
			candidates: []*pipeline.Candidate{
				{DocumentID: "d1", Score: 0.9, Source: "bm25", Metadata: make(map[string]any)},
				{DocumentID: "d2", Score: 0.7, Source: "bm25", Metadata: make(map[string]any)},
			},
			wantScore:  100.0,
			wantNumSrc: 1,
		},
		{
			name: "single source preserves score gaps",
			rrfK: 60,
			candidates: []*pipeline.Candidate{
				{DocumentID: "d1", Score: 10.0, Source: "bm25", Metadata: make(map[string]any)},
				{DocumentID: "d2", Score: 5.0, Source: "bm25", Metadata: make(map[string]any)},
				{DocumentID: "d3", Score: 1.0, Source: "bm25", Metadata: make(map[string]any)},
			},
			// min-max: d1=(10-1)/(10-1)*100=100, d2=(5-1)/(10-1)*100≈44.4, d3=0
			wantScore:  100.0,
			wantNumSrc: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewRankerFactory()
			stage, err := f.Create(pipeline.StageConfig{
				Parameters: map[string]any{
					"limit": float64(10),
					"rrf_k": tt.rrfK,
				},
			}, nil)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}

			result, err := stage.Process(context.Background(), tt.candidates)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}

			ranked := result.([]*pipeline.Candidate)
			if len(ranked) == 0 {
				t.Fatal("expected at least one ranked result")
			}

			if math.Abs(ranked[0].Score-tt.wantScore) > 0.1 {
				t.Errorf("score = %v, want ~%v", ranked[0].Score, tt.wantScore)
			}

			sources, ok := ranked[0].Metadata["rrf_sources"].([]string)
			if !ok {
				t.Fatal("expected rrf_sources metadata")
			}
			if len(sources) != tt.wantNumSrc {
				t.Errorf("sources count = %d, want %d", len(sources), tt.wantNumSrc)
			}

			// Verify raw score is preserved in metadata
			rawScore, ok := ranked[0].Metadata["rrf_raw_score"].(float64)
			if !ok {
				t.Fatal("expected rrf_raw_score metadata")
			}
			if rawScore <= 0 {
				t.Errorf("raw score should be positive, got %v", rawScore)
			}
		})
	}
}
