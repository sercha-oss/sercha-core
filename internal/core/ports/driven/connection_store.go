package driven

import (
	"context"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// ConnectionStore persists connector connections with encrypted secrets.
type ConnectionStore interface {
	// Save stores a new connection or updates an existing one.
	// Secrets are encrypted before storage.
	Save(ctx context.Context, conn *domain.Connection) error

	// Get retrieves a connection by ID with decrypted secrets.
	// Returns domain.ErrNotFound if the connection doesn't exist.
	Get(ctx context.Context, id string) (*domain.Connection, error)

	// List retrieves all connections as summaries (no secrets).
	List(ctx context.Context) ([]*domain.ConnectionSummary, error)

	// Delete removes a connection by ID.
	// Returns domain.ErrNotFound if the connection doesn't exist.
	Delete(ctx context.Context, id string) error

	// GetByProvider retrieves connections for a provider type (no secrets).
	GetByProvider(ctx context.Context, providerType domain.ProviderType) ([]*domain.ConnectionSummary, error)

	// GetByAccountID retrieves a connection by provider type and account ID.
	// Returns nil if not found.
	GetByAccountID(ctx context.Context, providerType domain.ProviderType, accountID string) (*domain.Connection, error)

	// UpdateSecrets updates the encrypted secrets and OAuth metadata.
	// Used after token refresh.
	UpdateSecrets(ctx context.Context, id string, secrets *domain.ConnectionSecrets, expiry *time.Time) error

	// UpdateLastUsed updates the last_used_at timestamp.
	UpdateLastUsed(ctx context.Context, id string) error
}
