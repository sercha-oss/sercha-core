package indexing

import (
	"context"
	"strings"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/custodia-labs/sercha-core/internal/core/ports/driven/pipeline"
)

const (
	ChunkerStageID      = "chunker"
	DefaultChunkSize    = 512
	DefaultChunkOverlap = 50
)

// ChunkerFactory creates chunker stages.
type ChunkerFactory struct {
	descriptor pipeline.StageDescriptor
}

// NewChunkerFactory creates a new chunker factory.
func NewChunkerFactory() *ChunkerFactory {
	return &ChunkerFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          ChunkerStageID,
			Name:        "Text Chunker",
			Type:        pipeline.StageTypeTransformer,
			InputShape:  pipeline.ShapeContent,
			OutputShape: pipeline.ShapeChunk,
			Cardinality: pipeline.CardinalityOneToMany,
			Version:     "1.0.0",
		},
	}
}

// StageID returns the stage identifier.
func (f *ChunkerFactory) StageID() string {
	return f.descriptor.ID
}

// Descriptor returns the stage descriptor.
func (f *ChunkerFactory) Descriptor() pipeline.StageDescriptor {
	return f.descriptor
}

// Create creates a new chunker stage.
func (f *ChunkerFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	chunkSize := DefaultChunkSize
	chunkOverlap := DefaultChunkOverlap

	if size, ok := config.Parameters["chunk_size"].(float64); ok {
		chunkSize = int(size)
	}
	if overlap, ok := config.Parameters["chunk_overlap"].(float64); ok {
		chunkOverlap = int(overlap)
	}

	return &ChunkerStage{
		descriptor:   f.descriptor,
		chunkSize:    chunkSize,
		chunkOverlap: chunkOverlap,
	}, nil
}

// Validate validates the stage configuration.
func (f *ChunkerFactory) Validate(config pipeline.StageConfig) error {
	return nil
}

// ChunkerStage splits content into chunks.
type ChunkerStage struct {
	descriptor   pipeline.StageDescriptor
	chunkSize    int
	chunkOverlap int
}

// Descriptor returns the stage descriptor.
func (s *ChunkerStage) Descriptor() pipeline.StageDescriptor {
	return s.descriptor
}

// Process splits input content into chunks.
func (s *ChunkerStage) Process(ctx context.Context, input any) (any, error) {
	indexInput, ok := input.(*pipeline.IndexingInput)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected *pipeline.IndexingInput"}
	}

	chunks := s.chunkText(indexInput.DocumentID, indexInput.Content)

	return chunks, nil
}

// chunkText splits text into overlapping chunks.
func (s *ChunkerStage) chunkText(documentID, text string) []*pipeline.Chunk {
	if len(text) == 0 {
		return nil
	}

	// Split by sentences/paragraphs for better chunks
	text = strings.TrimSpace(text)

	var chunks []*pipeline.Chunk
	position := 0
	offset := 0

	for offset < len(text) {
		end := offset + s.chunkSize
		if end > len(text) {
			end = len(text)
		}

		// Try to break at word boundary
		if end < len(text) {
			lastSpace := strings.LastIndex(text[offset:end], " ")
			if lastSpace > 0 {
				end = offset + lastSpace
			}
		}

		chunkContent := strings.TrimSpace(text[offset:end])
		if len(chunkContent) > 0 {
			chunks = append(chunks, &pipeline.Chunk{
				ID:          domain.GenerateID(),
				DocumentID:  documentID,
				Content:     chunkContent,
				Position:    position,
				StartOffset: offset,
				EndOffset:   end,
				Metadata:    make(map[string]any),
			})
			position++
		}

		// Move forward with overlap
		offset = end - s.chunkOverlap
		if offset <= chunks[len(chunks)-1].StartOffset {
			offset = end
		}
	}

	return chunks
}

// Ensure ChunkerFactory implements StageFactory.
var _ pipelineport.StageFactory = (*ChunkerFactory)(nil)

// Ensure ChunkerStage implements Stage.
var _ pipelineport.Stage = (*ChunkerStage)(nil)
