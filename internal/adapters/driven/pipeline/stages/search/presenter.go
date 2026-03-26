package search

import (
	"context"
	"strings"

	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/custodia-labs/sercha-core/internal/core/ports/driven/pipeline"
)

const PresenterStageID = "presenter"

// PresenterFactory creates presenter stages.
type PresenterFactory struct {
	descriptor pipeline.StageDescriptor
}

// NewPresenterFactory creates a new presenter factory.
func NewPresenterFactory() *PresenterFactory {
	return &PresenterFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          PresenterStageID,
			Name:        "Result Presenter",
			Type:        pipeline.StageTypePresenter,
			InputShape:  pipeline.ShapeRankedResult,
			OutputShape: pipeline.ShapePresentedResult,
			Cardinality: pipeline.CardinalityManyToOne,
			Version:     "1.0.0",
		},
	}
}

// StageID returns the stage identifier.
func (f *PresenterFactory) StageID() string {
	return f.descriptor.ID
}

// Descriptor returns the stage descriptor.
func (f *PresenterFactory) Descriptor() pipeline.StageDescriptor {
	return f.descriptor
}

// Create creates a new presenter stage.
func (f *PresenterFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	snippetLength := 200
	if l, ok := config.Parameters["snippet_length"].(float64); ok {
		snippetLength = int(l)
	}

	return &PresenterStage{
		descriptor:    f.descriptor,
		snippetLength: snippetLength,
	}, nil
}

// Validate validates the stage configuration.
func (f *PresenterFactory) Validate(config pipeline.StageConfig) error {
	return nil
}

// PresenterStage formats candidates into the final output.
type PresenterStage struct {
	descriptor    pipeline.StageDescriptor
	snippetLength int
}

// Descriptor returns the stage descriptor.
func (s *PresenterStage) Descriptor() pipeline.StageDescriptor {
	return s.descriptor
}

// Process formats ranked candidates into search output.
func (s *PresenterStage) Process(ctx context.Context, input any) (any, error) {
	candidates, ok := input.([]*pipeline.Candidate)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected []*pipeline.Candidate"}
	}

	results := make([]pipeline.PresentedResult, len(candidates))
	for i, c := range candidates {
		results[i] = pipeline.PresentedResult{
			DocumentID: c.DocumentID,
			ChunkID:    c.ChunkID,
			SourceID:   c.SourceID,
			Title:      s.extractTitle(c),
			Snippet:    s.createSnippet(c.Content),
			Score:      c.Score,
			Metadata:   c.Metadata,
		}
	}

	return &pipeline.SearchOutput{
		Results:    results,
		TotalCount: int64(len(results)),
	}, nil
}

// extractTitle extracts a title from candidate metadata or content.
func (s *PresenterStage) extractTitle(c *pipeline.Candidate) string {
	if title, ok := c.Metadata["title"].(string); ok && title != "" {
		return title
	}

	// Use first line of content as title fallback
	lines := strings.SplitN(c.Content, "\n", 2)
	if len(lines) > 0 {
		title := strings.TrimSpace(lines[0])
		if len(title) > 100 {
			title = title[:100] + "..."
		}
		return title
	}

	return ""
}

// createSnippet creates a snippet from content.
func (s *PresenterStage) createSnippet(content string) string {
	content = strings.TrimSpace(content)
	if len(content) <= s.snippetLength {
		return content
	}

	// Try to break at word boundary
	snippet := content[:s.snippetLength]
	lastSpace := strings.LastIndex(snippet, " ")
	if lastSpace > s.snippetLength/2 {
		snippet = snippet[:lastSpace]
	}

	return snippet + "..."
}

// Ensure PresenterFactory implements StageFactory.
var _ pipelineport.StageFactory = (*PresenterFactory)(nil)

// Ensure PresenterStage implements Stage.
var _ pipelineport.Stage = (*PresenterStage)(nil)
