package indexing

import (
	"context"
	"log/slog"
	"unicode/utf8"

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/pipeline/stages/textfilter"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

const DocLoaderStageID = "doc-loader"

// DefaultMaxIndexedChars caps the per-document body length sent to every
// downstream indexing stage (BM25, vector chunking/embedding, entity
// extraction). 150,000 characters is roughly 50 pages of prose — enough to
// fully cover focused docs (memos, contracts, reports) while truncating the
// long-tail outliers (1,400-page operational manuals, academic textbooks
// that bloat into multi-MB extractions).
//
// All consumers see the SAME truncated body — there's no "BM25 has more
// than the vector store" inconsistency. This keeps hybrid retrieval
// coherent and ensures the masker has analysed every byte that could be
// returned in a search snippet.
//
// 0 means no limit. Operators override per-deployment via the binary's
// wiring (typically a DOC_LOADER_MAX_INDEXED_CHARS env var).
const DefaultMaxIndexedChars = 150000

// DocLoaderFactory creates doc-loader stages.
type DocLoaderFactory struct {
	descriptor      pipeline.StageDescriptor
	maxIndexedChars int
}

// NewDocLoaderFactory creates a new doc-loader factory.
//
// maxIndexedChars: 0 means no truncation. Production wiring passes
// DefaultMaxIndexedChars (or an env-driven override).
func NewDocLoaderFactory(maxIndexedChars int) *DocLoaderFactory {
	return &DocLoaderFactory{
		maxIndexedChars: maxIndexedChars,
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
		descriptor:      f.descriptor,
		searchEngine:    searchEngine,
		maxIndexedChars: f.maxIndexedChars,
	}, nil
}

// Validate validates the stage configuration.
func (f *DocLoaderFactory) Validate(config pipeline.StageConfig) error {
	return nil
}

// DocLoaderStage indexes full document text to OpenSearch for BM25 search
// and passes the input through for downstream stages (chunker, embedder, vector-loader).
type DocLoaderStage struct {
	descriptor      pipeline.StageDescriptor
	searchEngine    driven.SearchEngine
	maxIndexedChars int
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

	// Truncate oversize bodies up-front so EVERY downstream stage —
	// OpenSearch (this stage), entity-extractor, chunker, embedder, vector
	// loader — sees the same truncated content. The cap protects the
	// pipeline from the long tail of giant docs (1,400-page manuals,
	// multi-MB academic PDFs) that would otherwise produce thousands of
	// chunks per doc and pin worker goroutines for tens of minutes apiece.
	if s.maxIndexedChars > 0 && len(indexInput.Content) > s.maxIndexedChars {
		originalLen := len(indexInput.Content)
		indexInput.Content = safeTruncate(indexInput.Content, s.maxIndexedChars)
		if indexInput.Metadata == nil {
			indexInput.Metadata = make(map[string]any)
		}
		indexInput.Metadata["truncated"] = true
		indexInput.Metadata["original_chars"] = originalLen
		indexInput.Metadata["indexed_chars"] = len(indexInput.Content)
		slog.Warn("doc-loader: truncated oversize document",
			"document_id", indexInput.DocumentID,
			"title", indexInput.Title,
			"original_chars", originalLen,
			"indexed_chars", len(indexInput.Content),
		)
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

	slog.Debug("indexed document for BM25 search",
		"document_id", indexInput.DocumentID,
		"content_length", len(body))

	// Pass through for downstream stages (chunker → embedder → vector-loader)
	return indexInput, nil
}

// safeTruncate slices s at the largest valid UTF-8 boundary <= n bytes. A
// naive s[:n] can split a multi-byte rune and produce invalid UTF-8, which
// then breaks JSON marshalling and regex matching downstream. Walks back
// at most 3 bytes (max rune length is 4) to find the boundary.
func safeTruncate(s string, n int) string {
	if n <= 0 || n >= len(s) {
		return s
	}
	for i := 0; i < 4 && n-i > 0; i++ {
		if utf8.RuneStart(s[n-i]) {
			return s[:n-i]
		}
	}
	return s[:n]
}

// Ensure DocLoaderFactory implements StageFactory.
var _ pipelineport.StageFactory = (*DocLoaderFactory)(nil)

// Ensure DocLoaderStage implements Stage.
var _ pipelineport.Stage = (*DocLoaderStage)(nil)
