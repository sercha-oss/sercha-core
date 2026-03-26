package search

import (
	"context"
	"sort"

	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/custodia-labs/sercha-core/internal/core/ports/driven/pipeline"
)

const RankerStageID = "ranker"

// RankerFactory creates ranker stages.
type RankerFactory struct {
	descriptor pipeline.StageDescriptor
}

// NewRankerFactory creates a new ranker factory.
func NewRankerFactory() *RankerFactory {
	return &RankerFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          RankerStageID,
			Name:        "Result Ranker",
			Type:        pipeline.StageTypeRanker,
			InputShape:  pipeline.ShapeCandidate,
			OutputShape: pipeline.ShapeRankedResult,
			Cardinality: pipeline.CardinalityManyToMany,
			Version:     "1.0.0",
		},
	}
}

// StageID returns the stage identifier.
func (f *RankerFactory) StageID() string {
	return f.descriptor.ID
}

// Descriptor returns the stage descriptor.
func (f *RankerFactory) Descriptor() pipeline.StageDescriptor {
	return f.descriptor
}

// Create creates a new ranker stage.
func (f *RankerFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	limit := 20
	if l, ok := config.Parameters["limit"].(float64); ok {
		limit = int(l)
	}

	return &RankerStage{
		descriptor: f.descriptor,
		limit:      limit,
	}, nil
}

// Validate validates the stage configuration.
func (f *RankerFactory) Validate(config pipeline.StageConfig) error {
	return nil
}

// RankerStage ranks and deduplicates candidates.
type RankerStage struct {
	descriptor pipeline.StageDescriptor
	limit      int
}

// Descriptor returns the stage descriptor.
func (s *RankerStage) Descriptor() pipeline.StageDescriptor {
	return s.descriptor
}

// Process ranks input candidates.
func (s *RankerStage) Process(ctx context.Context, input any) (any, error) {
	candidates, ok := input.([]*pipeline.Candidate)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected []*pipeline.Candidate"}
	}

	if len(candidates) == 0 {
		return candidates, nil
	}

	// Deduplicate by chunk ID
	seen := make(map[string]bool)
	var deduped []*pipeline.Candidate
	for _, c := range candidates {
		if !seen[c.ChunkID] {
			seen[c.ChunkID] = true
			deduped = append(deduped, c)
		}
	}

	// Sort by score descending
	sort.Slice(deduped, func(i, j int) bool {
		return deduped[i].Score > deduped[j].Score
	})

	// Apply limit
	if len(deduped) > s.limit {
		deduped = deduped[:s.limit]
	}

	return deduped, nil
}

// Ensure RankerFactory implements StageFactory.
var _ pipelineport.StageFactory = (*RankerFactory)(nil)

// Ensure RankerStage implements Stage.
var _ pipelineport.Stage = (*RankerStage)(nil)
