package services

import (
	"context"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven/mocks"
)

func TestDocumentService_Get(t *testing.T) {
	documentStore := mocks.NewMockDocumentStore()
	searchEngine := mocks.NewMockSearchEngine()
	svc := NewDocumentService(documentStore, searchEngine)

	// Create a document
	doc := &domain.Document{
		ID:        "doc-123",
		SourceID:  "source-456",
		Title:     "Test Document",
		MimeType:  "text/markdown",
		CreatedAt: time.Now(),
	}
	_ = documentStore.Save(context.Background(), doc)

	// Get the document
	result, err := svc.Get(context.Background(), "doc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != doc.ID {
		t.Errorf("expected document ID %s, got %s", doc.ID, result.ID)
	}
	if result.Title != doc.Title {
		t.Errorf("expected title %s, got %s", doc.Title, result.Title)
	}

	// Get non-existent document
	_, err = svc.Get(context.Background(), "non-existent")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDocumentService_GetContent(t *testing.T) {
	documentStore := mocks.NewMockDocumentStore()
	searchEngine := mocks.NewMockSearchEngine()
	svc := NewDocumentService(documentStore, searchEngine)

	// Index a document into the search engine
	docContent := &domain.DocumentContent{
		DocumentID: "doc-123",
		SourceID:   "source-456",
		Title:      "Test Document",
		Body:       "Full document content from OpenSearch",
		Path:       "/test/path",
		MimeType:   "text/markdown",
		Metadata:   map[string]string{"author": "test-user"},
	}
	_ = searchEngine.IndexDocument(context.Background(), docContent)

	// Get document content (should come from search engine)
	content, err := svc.GetContent(context.Background(), "doc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content.DocumentID != "doc-123" {
		t.Errorf("expected document ID doc-123, got %s", content.DocumentID)
	}
	if content.Title != "Test Document" {
		t.Errorf("expected title 'Test Document', got %s", content.Title)
	}
	if content.Body != "Full document content from OpenSearch" {
		t.Errorf("expected body from search engine, got %s", content.Body)
	}

	// Get content for non-existent document
	_, err = svc.GetContent(context.Background(), "non-existent")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDocumentService_GetBySource(t *testing.T) {
	documentStore := mocks.NewMockDocumentStore()
	searchEngine := mocks.NewMockSearchEngine()
	svc := NewDocumentService(documentStore, searchEngine)

	// Create documents for a source
	for i := 0; i < 5; i++ {
		doc := &domain.Document{
			ID:       generateID(),
			SourceID: "source-123",
			Title:    "Document " + string(rune('0'+i)),
		}
		_ = documentStore.Save(context.Background(), doc)
	}

	// Create documents for another source
	for i := 0; i < 3; i++ {
		doc := &domain.Document{
			ID:       generateID(),
			SourceID: "source-456",
			Title:    "Other Document " + string(rune('0'+i)),
		}
		_ = documentStore.Save(context.Background(), doc)
	}

	// Get documents for source-123
	docs, err := svc.GetBySource(context.Background(), "source-123", 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 5 {
		t.Errorf("expected 5 documents, got %d", len(docs))
	}

	// Test pagination
	docs, err = svc.GetBySource(context.Background(), "source-123", 2, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 2 {
		t.Errorf("expected 2 documents with limit 2, got %d", len(docs))
	}

	docs, err = svc.GetBySource(context.Background(), "source-123", 10, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 2 {
		t.Errorf("expected 2 documents with offset 3, got %d", len(docs))
	}
}

func TestDocumentService_GetBySource_LimitValidation(t *testing.T) {
	documentStore := mocks.NewMockDocumentStore()
	searchEngine := mocks.NewMockSearchEngine()
	svc := NewDocumentService(documentStore, searchEngine)

	// Create documents
	for i := 0; i < 10; i++ {
		doc := &domain.Document{
			ID:       generateID(),
			SourceID: "source-123",
			Title:    "Document " + string(rune('0'+i)),
		}
		_ = documentStore.Save(context.Background(), doc)
	}

	// Test with 0 limit (should default to 50)
	docs, err := svc.GetBySource(context.Background(), "source-123", 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 10 {
		t.Errorf("expected 10 documents, got %d", len(docs))
	}

	// Test with negative limit
	docs, err = svc.GetBySource(context.Background(), "source-123", -1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 10 {
		t.Errorf("expected 10 documents with negative limit, got %d", len(docs))
	}
}

func TestDocumentService_Count(t *testing.T) {
	documentStore := mocks.NewMockDocumentStore()
	searchEngine := mocks.NewMockSearchEngine()
	svc := NewDocumentService(documentStore, searchEngine)

	// Create documents
	for i := 0; i < 10; i++ {
		doc := &domain.Document{
			ID:       generateID(),
			SourceID: "source-" + string(rune('0'+i%3)),
			Title:    "Document " + string(rune('0'+i)),
		}
		_ = documentStore.Save(context.Background(), doc)
	}

	// Count all
	count, err := svc.Count(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 10 {
		t.Errorf("expected 10 documents, got %d", count)
	}
}

func TestDocumentService_CountBySource(t *testing.T) {
	documentStore := mocks.NewMockDocumentStore()
	searchEngine := mocks.NewMockSearchEngine()
	svc := NewDocumentService(documentStore, searchEngine)

	// Create documents for source-123
	for i := 0; i < 5; i++ {
		doc := &domain.Document{
			ID:       generateID(),
			SourceID: "source-123",
			Title:    "Document " + string(rune('0'+i)),
		}
		_ = documentStore.Save(context.Background(), doc)
	}

	// Create documents for source-456
	for i := 0; i < 3; i++ {
		doc := &domain.Document{
			ID:       generateID(),
			SourceID: "source-456",
			Title:    "Other Document " + string(rune('0'+i)),
		}
		_ = documentStore.Save(context.Background(), doc)
	}

	// Count by source
	count, err := svc.CountBySource(context.Background(), "source-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 documents for source-123, got %d", count)
	}

	count, err = svc.CountBySource(context.Background(), "source-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 documents for source-456, got %d", count)
	}

	count, err = svc.CountBySource(context.Background(), "non-existent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 documents for non-existent source, got %d", count)
	}
}
