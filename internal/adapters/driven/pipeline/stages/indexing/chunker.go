package indexing

import (
	"context"
	"log/slog"
	"strings"

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/pipeline/stages/chunking"
	"github.com/sercha-oss/sercha-core/internal/adapters/driven/pipeline/stages/textfilter"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

const (
	ChunkerStageID      = "chunker"
	DefaultChunkSize    = 1024
	DefaultChunkOverlap = 100
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

// ChunkerStage splits content into chunks. When the input contains ATX-style
// markdown headings (emitted by the HTML, PDF, and Notion normalisers) the
// chunker splits on those boundaries and prepends each chunk's section
// heading so the embedding has topical context. Headingless input falls
// back to plain size-based windowing.
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

	chunks := s.chunkText(indexInput.DocumentID, indexInput.SourceID, indexInput.MimeType, indexInput.Content)

	return chunks, nil
}

// chunkText splits text into chunks. If the input has any ATX headings
// outside of fenced code blocks, it splits on those and emits one or more
// chunks per section (size-windowed within long sections, with the heading
// prepended). Otherwise it falls back to plain size windowing.
func (s *ChunkerStage) chunkText(documentID, sourceID, mimeType, text string) []*pipeline.Chunk {
	if len(text) == 0 {
		return nil
	}
	text = strings.TrimSpace(text)

	sections := chunking.SplitSections(text)
	if len(sections) <= 1 && sections[0].Heading == "" {
		// No headings detected — preserve the existing size-based behaviour
		// verbatim. The forward-progress guards here are load-bearing for
		// the non-text-skip case (regression test
		// TestChunkerStage_NoInfiniteLoopOnMixedContent).
		return s.windowText(documentID, sourceID, mimeType, text, "")
	}

	sections = chunking.MergeTinySections(sections, chunking.MinSectionLength)

	var chunks []*pipeline.Chunk
	for _, sec := range sections {
		secChunks := s.windowText(documentID, sourceID, mimeType, sec.Body, sec.Heading)
		chunks = append(chunks, secChunks...)
	}
	// Re-number positions across the document so consumers see a stable
	// 0..N-1 sequence regardless of section count.
	for i, c := range chunks {
		c.Position = i
	}
	return chunks
}

// windowText emits size-bounded chunks of body text. When heading is
// non-empty it's prepended to each chunk's content (`heading\n\nbody`),
// giving the embedder topical context for sub-chunks of long sections.
// The returned chunks have document-relative offsets — StartOffset/EndOffset
// refer to body, not to the prepended heading.
func (s *ChunkerStage) windowText(documentID, sourceID, mimeType, body, heading string) []*pipeline.Chunk {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}

	var chunks []*pipeline.Chunk
	position := 0
	offset := 0

	for offset < len(body) {
		end := offset + s.chunkSize
		if end > len(body) {
			end = len(body)
		}

		// Prefer to break at a word boundary when we're not at end-of-body.
		if end < len(body) {
			lastSpace := strings.LastIndex(body[offset:end], " ")
			if lastSpace > 0 {
				end = offset + lastSpace
			}
		}

		chunkBody := strings.TrimSpace(body[offset:end])
		if len(chunkBody) > 0 && textfilter.IsLikelyNonTextWithMime(chunkBody, mimeType) {
			slog.Debug("skipping non-text chunk",
				"document_id", documentID,
				"position", position,
				"content_length", len(chunkBody))
		} else if len(chunkBody) > 0 {
			content := chunkBody
			if heading != "" {
				content = heading + "\n\n" + chunkBody
			}
			chunks = append(chunks, &pipeline.Chunk{
				ID:          domain.GenerateID(),
				DocumentID:  documentID,
				SourceID:    sourceID,
				Content:     content,
				Position:    position,
				StartOffset: offset,
				EndOffset:   end,
				Metadata:    make(map[string]any),
			})
			position++
		}

		// If we just consumed the body's tail there's nothing left to
		// overlap into — terminate cleanly. Without this, a body that
		// fits in one chunk-window would produce a duplicate "overlap
		// window" chunk and inflate the count.
		if end >= len(body) {
			break
		}

		// Move forward with overlap, but guarantee forward progress.
		// When a chunk is skipped (e.g. non-text filter), the last appended
		// chunk may be far behind, so we must also guard against regressing
		// past the current offset. Three independent safety nets — all
		// load-bearing per TestChunkerStage_NoInfiniteLoopOnMixedContent.
		prevOffset := offset
		offset = end - s.chunkOverlap
		if offset <= prevOffset {
			offset = end
		} else if len(chunks) > 0 && offset <= chunks[len(chunks)-1].StartOffset {
			offset = end
		} else if len(chunks) == 0 {
			offset = end
		}
	}

	return chunks
}

// Ensure ChunkerFactory implements StageFactory.
var _ pipelineport.StageFactory = (*ChunkerFactory)(nil)

// Ensure ChunkerStage implements Stage.
var _ pipelineport.Stage = (*ChunkerStage)(nil)
