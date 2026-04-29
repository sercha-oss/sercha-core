package indexing

import (
	"context"

	"golang.org/x/sync/errgroup"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

const (
	EmbedderStageID = "embedder"

	// defaultEmbedderBatchSize is the chunk count per embedder.Embed call
	// when stage config does not override batch_size. Larger batches mean
	// fewer round-trips per document; 96 is in-line with provider per-call
	// limits and roughly cuts the request count by 3x vs the previous 32.
	defaultEmbedderBatchSize = 96

	// defaultEmbedderConcurrency is the number of in-flight embedder
	// batches per document when stage config does not override
	// embedder_concurrency. Combined with the per-container doc-level
	// worker pool (see SyncOrchestratorConfig.Concurrency), this caps
	// total in-flight embedder calls at roughly Concurrency *
	// embedder_concurrency.
	defaultEmbedderConcurrency = 2
)

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
				{Type: pipeline.CapabilityEmbedder, Mode: pipeline.CapabilityOptional},
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
		// Embedder capability is optional - return no-op stage
		return &NoOpEmbedderStage{descriptor: f.descriptor}, nil
	}

	embedder, ok := inst.Instance.(driven.EmbeddingService)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "invalid embedder instance type"}
	}

	batchSize := defaultEmbedderBatchSize
	if bs, ok := config.Parameters["batch_size"].(float64); ok {
		batchSize = int(bs)
	}

	concurrency := defaultEmbedderConcurrency
	if c, ok := config.Parameters["embedder_concurrency"].(float64); ok {
		concurrency = int(c)
	}
	if concurrency < 1 {
		concurrency = 1
	}

	return &EmbedderStage{
		descriptor:  f.descriptor,
		embedder:    embedder,
		batchSize:   batchSize,
		concurrency: concurrency,
	}, nil
}

// Validate validates the stage configuration.
func (f *EmbedderFactory) Validate(config pipeline.StageConfig) error {
	return nil
}

// EmbedderStage generates embeddings for chunks.
type EmbedderStage struct {
	descriptor  pipeline.StageDescriptor
	embedder    driven.EmbeddingService
	batchSize   int
	concurrency int
}

// Descriptor returns the stage descriptor.
func (s *EmbedderStage) Descriptor() pipeline.StageDescriptor {
	return s.descriptor
}

// Process generates embeddings for input chunks.
//
// Batches run concurrently up to s.concurrency; results are written
// back to the source slice using captured indices so the original
// chunk ordering is preserved without any post-merge step.
//
// Failure semantics differ from the orchestrator's per-doc fan-out: a
// partial set of embeddings is useless to the downstream vector loader,
// so any batch error fast-fails the whole stage via errgroup.Wait
// short-circuit. The orchestrator already treats a stage failure as a
// per-doc error and increments stats.Errors accordingly.
func (s *EmbedderStage) Process(ctx context.Context, input any) (any, error) {
	chunks, ok := input.([]*pipeline.Chunk)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected []*pipeline.Chunk"}
	}

	if len(chunks) == 0 {
		return chunks, nil
	}

	g, gctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, s.concurrency)

	for start := 0; start < len(chunks); start += s.batchSize {
		end := start + s.batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		start, end := start, end
		sem <- struct{}{}
		g.Go(func() error {
			defer func() { <-sem }()

			batch := chunks[start:end]
			texts := make([]string, len(batch))
			for j, chunk := range batch {
				texts[j] = chunk.Content
			}

			embeddings, err := s.embedder.Embed(gctx, texts)
			if err != nil {
				return &StageError{Stage: s.descriptor.ID, Message: "embedding failed", Err: err}
			}

			for j, embedding := range embeddings {
				batch[j].Embedding = embedding
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return chunks, nil
}

// NoOpEmbedderStage is a pass-through stage used when embedder capability is not available.
// It passes chunks through unchanged without generating embeddings.
type NoOpEmbedderStage struct {
	descriptor pipeline.StageDescriptor
}

// Descriptor returns the stage descriptor.
func (s *NoOpEmbedderStage) Descriptor() pipeline.StageDescriptor {
	return s.descriptor
}

// Process passes chunks through unchanged.
func (s *NoOpEmbedderStage) Process(ctx context.Context, input any) (any, error) {
	chunks, ok := input.([]*pipeline.Chunk)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected []*pipeline.Chunk"}
	}

	// Pass through unchanged - no embeddings added
	return chunks, nil
}

// Ensure EmbedderFactory implements StageFactory.
var _ pipelineport.StageFactory = (*EmbedderFactory)(nil)

// Ensure EmbedderStage implements Stage.
var _ pipelineport.Stage = (*EmbedderStage)(nil)

// Ensure NoOpEmbedderStage implements Stage.
var _ pipelineport.Stage = (*NoOpEmbedderStage)(nil)
