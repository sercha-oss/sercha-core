package indexing

import (
	"context"
	"log/slog"

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/pipeline/stages/textfilter"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

const DocLoaderStageID = "doc-loader"

// DocLoaderFactory creates doc-loader stages.
type DocLoaderFactory struct {
	descriptor pipeline.StageDescriptor
}

// NewDocLoaderFactory creates a new doc-loader factory.
func NewDocLoaderFactory() *DocLoaderFactory {
	return &DocLoaderFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          DocLoaderStageID,
			Name:        "Document Loader",
			Type:        pipeline.StageTypeLoader,
			InputShape:  pipeline.ShapeContent,
			OutputShape: pipeline.ShapeContent,
			Cardinality: pipeline.CardinalityOneToOne,
			Capabilities: []pipeline.CapabilityRequirement{
				{Type: pipeline.CapabilitySearchEngine, Mode: pipeline.CapabilityRequired},
			},
			Version: "1.0.0",
		},
	}
}

// StageID returns the stage identifier.
func (f *DocLoaderFactory) StageID() string {
	return f.descriptor.ID
}

// Descriptor returns the stage descriptor.
func (f *DocLoaderFactory) Descriptor() pipeline.StageDescriptor {
	return f.descriptor
}

// Create creates a new doc-loader stage.
func (f *DocLoaderFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	inst, ok := capabilities.Get(pipeline.CapabilitySearchEngine)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "search_engine capability not available"}
	}

	searchEngine, ok := inst.Instance.(driven.SearchEngine)
	if !ok {
		return nil, &StageError{Stage: f.descriptor.ID, Message: "invalid search_engine instance type"}
	}

	return &DocLoaderStage{
		descriptor:   f.descriptor,
		searchEngine: searchEngine,
	}, nil
}

// Validate validates the stage configuration.
func (f *DocLoaderFactory) Validate(config pipeline.StageConfig) error {
	return nil
}

// DocLoaderStage indexes full document text to OpenSearch for BM25 search
// and passes the input through for downstream stages (chunker, embedder, vector-loader).
type DocLoaderStage struct {
	descriptor   pipeline.StageDescriptor
	searchEngine driven.SearchEngine
}

// Descriptor returns the stage descriptor.
func (s *DocLoaderStage) Descriptor() pipeline.StageDescriptor {
	return s.descriptor
}

// Process indexes the full document content to OpenSearch and passes the input through.
func (s *DocLoaderStage) Process(ctx context.Context, input any) (any, error) {
	indexInput, ok := input.(*pipeline.IndexingInput)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected *pipeline.IndexingInput"}
	}

	body := indexInput.Content

	// Detect binary/encoded content (e.g. base64-zlib from auto-generated .api.mdx files).
	// Index metadata only; clear content so downstream stages skip chunking/embedding.
	// Known structured-text MIMEs (minified JSON/JS/CSS) bypass the whitespace heuristic
	// to avoid false-positives.
	if textfilter.IsLikelyNonTextWithMime(body, indexInput.MimeType) {
		slog.Warn("skipping binary/encoded content",
			"document_id", indexInput.DocumentID,
			"content_length", len(body),
			"title", indexInput.Title)
		body = ""
		indexInput.Content = ""
	}

	// Index full document to OpenSearch for BM25 text search
	doc := &domain.DocumentContent{
		DocumentID: indexInput.DocumentID,
		SourceID:   indexInput.SourceID,
		Title:      indexInput.Title,
		Body:       body,
		Path:       indexInput.Path,
		MimeType:   indexInput.MimeType,
	}

	if err := s.searchEngine.IndexDocument(ctx, doc); err != nil {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "failed to index document", Err: err}
	}

	slog.Info("indexed document for BM25 search",
		"document_id", indexInput.DocumentID,
		"content_length", len(body))

	// Pass through for downstream stages (chunker → embedder → vector-loader)
	return indexInput, nil
}

// Ensure DocLoaderFactory implements StageFactory.
var _ pipelineport.StageFactory = (*DocLoaderFactory)(nil)

// Ensure DocLoaderStage implements Stage.
var _ pipelineport.Stage = (*DocLoaderStage)(nil)
