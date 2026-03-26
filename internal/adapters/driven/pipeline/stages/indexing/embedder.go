package indexing

import (
	"context"

	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/custodia-labs/sercha-core/internal/core/ports/driven/pipeline"
)

const EmbedderStageID = "embedder"

// EmbedderFactory creates embedder stages.
type EmbedderFactory struct {
	descriptor pipeline.StageDescriptor
}

// NewEmbedderFactory creates a new embedder factory.
func NewEmbedderFactory() *EmbedderFactory {
	return &EmbedderFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          EmbedderStageID,
			Name:        "Chunk Embedder",
			Type:        pipeline.StageTypeEnricher,
			InputShape:  pipeline.ShapeChunk,
			OutputShape: pipeline.ShapeEmbeddedChunk,
			Cardinality: pipeline.CardinalityManyToMany,
			Capabilities: []pipeline.CapabilityRequirement{
				{Type: pipeline.CapabilityEmbedder, Mode: pipeline.CapabilityRequired},
			},
			Version: "1.0.0",
		},
	}
}

// StageID returns the stage identifier.
func (f *EmbedderFactory) StageID() string {
	return f.descriptor.ID
}

// Descriptor returns the stage descriptor.
func (f *EmbedderFactory) Descriptor() pipeline.StageDescriptor {
	return f.descriptor
}

// Create creates a new embedder stage.
func (f *EmbedderFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	// Get embedding service from capabilities
	inst, ok := capabilities.Get(pipeline.CapabilityEmbedder)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "embedder capability not available"}
	}

	embedder, ok := inst.Instance.(driven.EmbeddingService)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "invalid embedder instance type"}
	}

	batchSize := 32
	if bs, ok := config.Parameters["batch_size"].(float64); ok {
		batchSize = int(bs)
	}

	return &EmbedderStage{
		descriptor: f.descriptor,
		embedder:   embedder,
		batchSize:  batchSize,
	}, nil
}

// Validate validates the stage configuration.
func (f *EmbedderFactory) Validate(config pipeline.StageConfig) error {
	return nil
}

// EmbedderStage generates embeddings for chunks.
type EmbedderStage struct {
	descriptor pipeline.StageDescriptor
	embedder   driven.EmbeddingService
	batchSize  int
}

// Descriptor returns the stage descriptor.
func (s *EmbedderStage) Descriptor() pipeline.StageDescriptor {
	return s.descriptor
}

// Process generates embeddings for input chunks.
func (s *EmbedderStage) Process(ctx context.Context, input any) (any, error) {
	chunks, ok := input.([]*pipeline.Chunk)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected []*pipeline.Chunk"}
	}

	if len(chunks) == 0 {
		return chunks, nil
	}

	// Process in batches
	for i := 0; i < len(chunks); i += s.batchSize {
		end := i + s.batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[i:end]
		texts := make([]string, len(batch))
		for j, chunk := range batch {
			texts[j] = chunk.Content
		}

		embeddings, err := s.embedder.Embed(ctx, texts)
		if err != nil {
			return nil, &StageError{Stage: s.descriptor.ID, Message: "embedding failed", Err: err}
		}

		for j, embedding := range embeddings {
			batch[j].Embedding = embedding
		}
	}

	return chunks, nil
}

// Ensure EmbedderFactory implements StageFactory.
var _ pipelineport.StageFactory = (*EmbedderFactory)(nil)

// Ensure EmbedderStage implements Stage.
var _ pipelineport.Stage = (*EmbedderStage)(nil)
