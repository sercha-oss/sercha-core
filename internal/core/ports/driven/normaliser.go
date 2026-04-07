package driven

import (
	"context"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// Normaliser normalizes raw document content for indexing.
// It transforms provider-specific document formats into normalized text.
type Normaliser interface {
	// Normalise transforms raw content into normalized text.
	// The mimeType helps determine the appropriate processing.
	Normalise(content string, mimeType string) string

	// SupportedTypes returns MIME types this normaliser handles.
	// Can include wildcards like "text/*" or specific types like "text/markdown".
	SupportedTypes() []string

	// Priority returns the normaliser priority (higher = more specific).
	// Priority ranges:
	//   90-100: Connector-specific (e.g., Gmail email normaliser)
	//   50-89:  Format-specific (PDF, Markdown, HTML)
	//   10-49:  Generic (basic text processing)
	//   1-9:    Fallback (raw text extraction)
	Priority() int
}

// NormaliserRegistry manages content normalisers.
// When multiple normalisers match a MIME type, the highest priority one is used.
type NormaliserRegistry interface {
	// Get retrieves the best-matching normaliser for a MIME type.
	// Returns nil if no normaliser is registered for the type.
	// When multiple match, the highest priority normaliser is returned.
	Get(mimeType string) Normaliser

	// GetAll retrieves all normalisers that match a MIME type, sorted by priority (highest first).
	GetAll(mimeType string) []Normaliser

	// Register registers a normaliser.
	Register(normaliser Normaliser)

	// List returns all registered MIME types.
	List() []string
}


// CredentialsStore handles credential persistence (PostgreSQL, encrypted)
type CredentialsStore interface {
	// Save stores credentials (encrypts sensitive fields)
	Save(ctx context.Context, creds *domain.Credentials) error

	// Get retrieves credentials by ID
	Get(ctx context.Context, id string) (*domain.Credentials, error)

	// List retrieves all credentials
	List(ctx context.Context) ([]*domain.Credentials, error)

	// Delete deletes credentials
	Delete(ctx context.Context, id string) error

	// GetByProvider retrieves credentials for a provider type
	GetByProvider(ctx context.Context, providerType domain.ProviderType) ([]*domain.Credentials, error)
}
