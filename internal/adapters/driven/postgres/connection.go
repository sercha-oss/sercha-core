package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure ConnectionStore implements the interface.
var _ driven.ConnectionStore = (*ConnectionStore)(nil)

// ConnectionStore implements driven.ConnectionStore using PostgreSQL.
type ConnectionStore struct {
	db        *sql.DB
	encryptor *SecretEncryptor
}

// NewConnectionStore creates a new PostgreSQL-backed connection store.
func NewConnectionStore(db *sql.DB, encryptor *SecretEncryptor) *ConnectionStore {
	return &ConnectionStore{
		db:        db,
		encryptor: encryptor,
	}
}

// Save stores a new connection or updates an existing one.
func (s *ConnectionStore) Save(ctx context.Context, conn *domain.Connection) error {
	// Encrypt secrets if present
	var secretBlob []byte
	if conn.Secrets != nil {
		var err error
		secretBlob, err = s.encryptor.Encrypt(conn.Secrets)
		if err != nil {
			return fmt.Errorf("encrypt secrets: %w", err)
		}
	}

	query := `
		INSERT INTO connector_installations (
			id, name, provider_type, platform, auth_method, secret_blob,
			oauth_token_type, oauth_expiry, oauth_scopes, account_id,
			created_at, updated_at, last_used_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			provider_type = EXCLUDED.provider_type,
			platform = EXCLUDED.platform,
			auth_method = EXCLUDED.auth_method,
			secret_blob = EXCLUDED.secret_blob,
			oauth_token_type = EXCLUDED.oauth_token_type,
			oauth_expiry = EXCLUDED.oauth_expiry,
			oauth_scopes = EXCLUDED.oauth_scopes,
			account_id = EXCLUDED.account_id,
			updated_at = EXCLUDED.updated_at,
			last_used_at = EXCLUDED.last_used_at
	`

	now := time.Now()
	if conn.CreatedAt.IsZero() {
		conn.CreatedAt = now
	}
	conn.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, query,
		conn.ID,
		conn.Name,
		conn.ProviderType,
		conn.Platform,
		conn.AuthMethod,
		secretBlob,
		nullString(conn.OAuthTokenType),
		nullTime(conn.OAuthExpiry),
		pq.Array(conn.OAuthScopes),
		nullString(conn.AccountID),
		conn.CreatedAt,
		conn.UpdatedAt,
		nullTime(conn.LastUsedAt),
	)
	if err != nil {
		return fmt.Errorf("save connection: %w", err)
	}

	return nil
}

// Get retrieves a connection by ID with decrypted secrets.
func (s *ConnectionStore) Get(ctx context.Context, id string) (*domain.Connection, error) {
	query := `
		SELECT id, name, provider_type, platform, auth_method, secret_blob,
			   oauth_token_type, oauth_expiry, oauth_scopes, account_id,
			   created_at, updated_at, last_used_at
		FROM connector_installations
		WHERE id = $1
	`

	var conn domain.Connection
	var secretBlob []byte
	var oauthTokenType, accountID sql.NullString
	var oauthExpiry, lastUsedAt sql.NullTime
	var oauthScopes []string

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&conn.ID,
		&conn.Name,
		&conn.ProviderType,
		&conn.Platform,
		&conn.AuthMethod,
		&secretBlob,
		&oauthTokenType,
		&oauthExpiry,
		pq.Array(&oauthScopes),
		&accountID,
		&conn.CreatedAt,
		&conn.UpdatedAt,
		&lastUsedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}

	// Decrypt secrets if present
	if len(secretBlob) > 0 {
		conn.Secrets = &domain.ConnectionSecrets{}
		if err := s.encryptor.Decrypt(secretBlob, conn.Secrets); err != nil {
			return nil, fmt.Errorf("decrypt secrets: %w", err)
		}
	}

	conn.OAuthTokenType = oauthTokenType.String
	if oauthExpiry.Valid {
		conn.OAuthExpiry = &oauthExpiry.Time
	}
	conn.OAuthScopes = oauthScopes
	conn.AccountID = accountID.String
	if lastUsedAt.Valid {
		conn.LastUsedAt = &lastUsedAt.Time
	}

	return &conn, nil
}

// List retrieves all connections as summaries (no secrets).
func (s *ConnectionStore) List(ctx context.Context) ([]*domain.ConnectionSummary, error) {
	query := `
		SELECT id, name, provider_type, platform, auth_method, account_id,
			   oauth_expiry, created_at, last_used_at
		FROM connector_installations
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var summaries []*domain.ConnectionSummary
	for rows.Next() {
		var summary domain.ConnectionSummary
		var accountID sql.NullString
		var oauthExpiry, lastUsedAt sql.NullTime

		if err := rows.Scan(
			&summary.ID,
			&summary.Name,
			&summary.ProviderType,
			&summary.Platform,
			&summary.AuthMethod,
			&accountID,
			&oauthExpiry,
			&summary.CreatedAt,
			&lastUsedAt,
		); err != nil {
			return nil, fmt.Errorf("scan connection: %w", err)
		}

		summary.AccountID = accountID.String
		if oauthExpiry.Valid {
			summary.OAuthExpiry = &oauthExpiry.Time
		}
		if lastUsedAt.Valid {
			summary.LastUsedAt = &lastUsedAt.Time
		}

		summaries = append(summaries, &summary)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate connections: %w", err)
	}

	return summaries, nil
}

// Delete removes a connection by ID.
func (s *ConnectionStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM connector_installations WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// GetByPlatform retrieves connections for a platform type (no secrets).
func (s *ConnectionStore) GetByPlatform(ctx context.Context, platform domain.PlatformType) ([]*domain.ConnectionSummary, error) {
	query := `
		SELECT id, name, provider_type, platform, auth_method, account_id,
			   oauth_expiry, created_at, last_used_at
		FROM connector_installations
		WHERE platform = $1
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, platform)
	if err != nil {
		return nil, fmt.Errorf("list connections by platform: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var summaries []*domain.ConnectionSummary
	for rows.Next() {
		var summary domain.ConnectionSummary
		var accountID sql.NullString
		var oauthExpiry, lastUsedAt sql.NullTime

		if err := rows.Scan(
			&summary.ID,
			&summary.Name,
			&summary.ProviderType,
			&summary.Platform,
			&summary.AuthMethod,
			&accountID,
			&oauthExpiry,
			&summary.CreatedAt,
			&lastUsedAt,
		); err != nil {
			return nil, fmt.Errorf("scan connection: %w", err)
		}

		summary.AccountID = accountID.String
		if oauthExpiry.Valid {
			summary.OAuthExpiry = &oauthExpiry.Time
		}
		if lastUsedAt.Valid {
			summary.LastUsedAt = &lastUsedAt.Time
		}

		summaries = append(summaries, &summary)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate connections: %w", err)
	}

	return summaries, nil
}

// GetByAccountID retrieves a connection by platform type and account ID.
func (s *ConnectionStore) GetByAccountID(ctx context.Context, platform domain.PlatformType, accountID string) (*domain.Connection, error) {
	query := `
		SELECT id, name, provider_type, platform, auth_method, secret_blob,
			   oauth_token_type, oauth_expiry, oauth_scopes, account_id,
			   created_at, updated_at, last_used_at
		FROM connector_installations
		WHERE platform = $1 AND account_id = $2
	`

	var conn domain.Connection
	var secretBlob []byte
	var oauthTokenType, accountIDVal sql.NullString
	var oauthExpiry, lastUsedAt sql.NullTime
	var oauthScopes []string

	err := s.db.QueryRowContext(ctx, query, platform, accountID).Scan(
		&conn.ID,
		&conn.Name,
		&conn.ProviderType,
		&conn.Platform,
		&conn.AuthMethod,
		&secretBlob,
		&oauthTokenType,
		&oauthExpiry,
		pq.Array(&oauthScopes),
		&accountIDVal,
		&conn.CreatedAt,
		&conn.UpdatedAt,
		&lastUsedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil // Not found is not an error for this method
	}
	if err != nil {
		return nil, fmt.Errorf("get connection by account: %w", err)
	}

	// Decrypt secrets if present
	if len(secretBlob) > 0 {
		conn.Secrets = &domain.ConnectionSecrets{}
		if err := s.encryptor.Decrypt(secretBlob, conn.Secrets); err != nil {
			return nil, fmt.Errorf("decrypt secrets: %w", err)
		}
	}

	conn.OAuthTokenType = oauthTokenType.String
	if oauthExpiry.Valid {
		conn.OAuthExpiry = &oauthExpiry.Time
	}
	conn.OAuthScopes = oauthScopes
	conn.AccountID = accountIDVal.String
	if lastUsedAt.Valid {
		conn.LastUsedAt = &lastUsedAt.Time
	}

	return &conn, nil
}

// UpdateSecrets updates the encrypted secrets and OAuth metadata.
func (s *ConnectionStore) UpdateSecrets(ctx context.Context, id string, secrets *domain.ConnectionSecrets, expiry *time.Time) error {
	var secretBlob []byte
	if secrets != nil {
		var err error
		secretBlob, err = s.encryptor.Encrypt(secrets)
		if err != nil {
			return fmt.Errorf("encrypt secrets: %w", err)
		}
	}

	query := `
		UPDATE connector_installations
		SET secret_blob = $1, oauth_expiry = $2, updated_at = $3
		WHERE id = $4
	`

	result, err := s.db.ExecContext(ctx, query, secretBlob, nullTime(expiry), time.Now(), id)
	if err != nil {
		return fmt.Errorf("update secrets: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// UpdateLastUsed updates the last_used_at timestamp.
func (s *ConnectionStore) UpdateLastUsed(ctx context.Context, id string) error {
	query := `
		UPDATE connector_installations
		SET last_used_at = $1
		WHERE id = $2
	`

	result, err := s.db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update last used: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// Helper functions for nullable values

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}
