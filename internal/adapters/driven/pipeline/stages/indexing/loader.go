package indexing

import (
	"context"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/custodia-labs/sercha-core/internal/core/ports/driven/pipeline"
)

const LoaderStageID = "loader"

// LoaderFactory creates loader stages.
type LoaderFactory struct {
	descriptor pipeline.StageDescriptor
}

// NewLoaderFactory creates a new loader factory.
func NewLoaderFactory() *LoaderFactory {
	return &LoaderFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          LoaderStageID,
			Name:        "Chunk Loader",
			Type:        pipeline.StageTypeLoader,
			InputShape:  pipeline.ShapeEmbeddedChunk,
			OutputShape: pipeline.ShapeIndexedDoc,
			Cardinality: pipeline.CardinalityManyToOne,
			Capabilities: []pipeline.CapabilityRequirement{
				{Type: pipeline.CapabilityVectorStore, Mode: pipeline.CapabilityRequired},
			},
			Version: "1.0.0",
		},
	}
}

// StageID returns the stage identifier.
func (f *LoaderFactory) StageID() string {
	return f.descriptor.ID
}

// Descriptor returns the stage descriptor.
func (f *LoaderFactory) Descriptor() pipeline.StageDescriptor {
	return f.descriptor
}

// Create creates a new loader stage.
func (f *LoaderFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	// Get search engine (vector store) from capabilities
	inst, ok := capabilities.Get(pipeline.CapabilityVectorStore)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "vector_store capability not available"}
	}

	searchEngine, ok := inst.Instance.(driven.SearchEngine)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "invalid vector_store instance type"}
	}

	return &LoaderStage{
		descriptor:   f.descriptor,
		searchEngine: searchEngine,
	}, nil
}

// Validate validates the stage configuration.
func (f *LoaderFactory) Validate(config pipeline.StageConfig) error {
	return nil
}

// LoaderStage persists chunks to the search engine.
type LoaderStage struct {
	descriptor   pipeline.StageDescriptor
	searchEngine driven.SearchEngine
}

// Descriptor returns the stage descriptor.
func (s *LoaderStage) Descriptor() pipeline.StageDescriptor {
	return s.descriptor
}

// Process persists embedded chunks and returns indexing output.
func (s *LoaderStage) Process(ctx context.Context, input any) (any, error) {
	chunks, ok := input.([]*pipeline.Chunk)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected []*pipeline.Chunk"}
	}

	if len(chunks) == 0 {
		return &pipeline.IndexingOutput{}, nil
	}

	// Convert pipeline chunks to domain chunks
	domainChunks := make([]*domain.Chunk, len(chunks))
	for i, chunk := range chunks {
		domainChunks[i] = &domain.Chunk{
			ID:         chunk.ID,
			DocumentID: chunk.DocumentID,
			SourceID:   "", // Will be set by caller context
			Content:    chunk.Content,
			Embedding:  chunk.Embedding,
			Position:   chunk.Position,
			StartChar:  chunk.StartOffset,
			EndChar:    chunk.EndOffset,
			CreatedAt:  time.Now(),
		}
	}

	// Index chunks
	if err := s.searchEngine.Index(ctx, domainChunks); err != nil {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "failed to index chunks", Err: err}
	}

	// Collect chunk IDs for output
	chunkIDs := make([]string, len(chunks))
	for i, chunk := range chunks {
		chunkIDs[i] = chunk.ID
	}

	// Get document ID from first chunk
	documentID := ""
	if len(chunks) > 0 {
		documentID = chunks[0].DocumentID
	}

	return &pipeline.IndexingOutput{
		DocumentID: documentID,
		ChunkIDs:   chunkIDs,
	}, nil
}

// Ensure LoaderFactory implements StageFactory.
var _ pipelineport.StageFactory = (*LoaderFactory)(nil)

// Ensure LoaderStage implements Stage.
var _ pipelineport.Stage = (*LoaderStage)(nil)
