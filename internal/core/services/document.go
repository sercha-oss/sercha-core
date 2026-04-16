package services

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
)

// Ensure documentService implements DocumentService
var _ driving.DocumentService = (*documentService)(nil)

// documentService implements the DocumentService interface
type documentService struct {
	documentStore driven.DocumentStore
	searchEngine  driven.SearchEngine
}

// NewDocumentService creates a new DocumentService
func NewDocumentService(
	documentStore driven.DocumentStore,
	searchEngine driven.SearchEngine,
) driving.DocumentService {
	return &documentService{
		documentStore: documentStore,
		searchEngine:  searchEngine,
	}
}

// Get retrieves a document by ID
func (s *documentService) Get(ctx context.Context, id string) (*domain.Document, error) {
	return s.documentStore.Get(ctx, id)
}

// GetWithChunks retrieves a document with its chunks.
// Note: This method is deprecated. It returns the document with empty chunks.
// Use GetContent instead to retrieve document content from the search engine.
func (s *documentService) GetWithChunks(ctx context.Context, id string) (*domain.DocumentWithChunks, error) {
	doc, err := s.documentStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return &domain.DocumentWithChunks{
		Document: doc,
		Chunks:   []*domain.Chunk{},
	}, nil
}

// GetContent retrieves the full content of a document from the search index.
// This uses OpenSearch to fetch the already-indexed full-text content instead of
// reconstructing from chunks. Returns domain.ErrNotFound if not in search index.
func (s *documentService) GetContent(ctx context.Context, id string) (*domain.DocumentContent, error) {
	return s.searchEngine.GetDocument(ctx, id)
}

// GetBySource retrieves all documents for a source
func (s *documentService) GetBySource(ctx context.Context, sourceID string, limit, offset int) ([]*domain.Document, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	return s.documentStore.GetBySource(ctx, sourceID, limit, offset)
}

// Count returns the total number of documents
func (s *documentService) Count(ctx context.Context) (int, error) {
	return s.documentStore.Count(ctx)
}

// CountBySource returns the document count for a source
func (s *documentService) CountBySource(ctx context.Context, sourceID string) (int, error) {
	return s.documentStore.CountBySource(ctx, sourceID)
}
