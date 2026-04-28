package domain

import (
	"testing"
	"time"
)

func TestDocument(t *testing.T) {
	now := time.Now()
	doc := &Document{
		ID:         "doc-123",
		SourceID:   "source-456",
		ExternalID: "ext-789",
		Path:       "/path/to/file.md",
		Title:      "Test Document",
		MimeType:   "text/markdown",
		Metadata: map[string]string{
			"author": "test-user",
		},
		CreatedAt: now,
		UpdatedAt: now,
		IndexedAt: now,
	}

	if doc.ID != "doc-123" {
		t.Errorf("expected ID doc-123, got %s", doc.ID)
	}
	if doc.SourceID != "source-456" {
		t.Errorf("expected SourceID source-456, got %s", doc.SourceID)
	}
	if doc.ExternalID != "ext-789" {
		t.Errorf("expected ExternalID ext-789, got %s", doc.ExternalID)
	}
	if doc.Path != "/path/to/file.md" {
		t.Errorf("expected Path /path/to/file.md, got %s", doc.Path)
	}
	if doc.Title != "Test Document" {
		t.Errorf("expected Title 'Test Document', got %s", doc.Title)
	}
	if doc.MimeType != "text/markdown" {
		t.Errorf("expected MimeType text/markdown, got %s", doc.MimeType)
	}
	if doc.Metadata["author"] != "test-user" {
		t.Errorf("expected author test-user, got %s", doc.Metadata["author"])
	}
}

func TestDocumentContent(t *testing.T) {
	content := &DocumentContent{
		DocumentID: "doc-123",
		Title:      "Test Document",
		Body:       "This is the document body with more content.",
		Metadata: map[string]string{
			"source": "test",
		},
	}

	if content.DocumentID != "doc-123" {
		t.Errorf("expected DocumentID doc-123, got %s", content.DocumentID)
	}
	if content.Title != "Test Document" {
		t.Errorf("expected Title 'Test Document', got %s", content.Title)
	}
	if content.Body != "This is the document body with more content." {
		t.Errorf("unexpected body content")
	}
	if content.Metadata["source"] != "test" {
		t.Errorf("expected source test, got %s", content.Metadata["source"])
	}
}

