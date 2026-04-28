package domain

import "time"

// Document represents an indexed document from a source
type Document struct {
	ID         string            `json:"id"`
	SourceID   string            `json:"source_id"`
	ExternalID string            `json:"external_id"` // ID from the source system
	Path       string            `json:"path"`        // Path or URL in source
	Title      string            `json:"title"`
	MimeType   string            `json:"mime_type"`
	Metadata   map[string]string `json:"metadata"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	IndexedAt  time.Time         `json:"indexed_at"`
}

// DocumentContent holds the full content of a document for indexing.
type DocumentContent struct {
	DocumentID string            `json:"document_id"`
	SourceID   string            `json:"source_id"`
	Title      string            `json:"title"`
	Body       string            `json:"body"`
	Path       string            `json:"path"`
	MimeType   string            `json:"mime_type"`
	Metadata   map[string]string `json:"metadata"`
}
