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

// Ensure OAuthClientStore implements the interface.
var _ driven.OAuthClientStore = (*OAuthClientStore)(nil)

// OAuthClientStore implements driven.OAuthClientStore using PostgreSQL.
type OAuthClientStore struct {
	db *sql.DB
}

// NewOAuthClientStore creates a new PostgreSQL-backed OAuth client store.
func NewOAuthClientStore(db *sql.DB) *OAuthClientStore {
	return &OAuthClientStore{db: db}
}

// Save stores a new client or updates an existing one.
func (s *OAuthClientStore) Save(ctx context.Context, client *domain.OAuthClient) error {
	now := time.Now()
	if client.CreatedAt.IsZero() {
		client.CreatedAt = now
	}
	client.UpdatedAt = now

	query := `
		INSERT INTO oauth_clients (
			id, secret_hash, name, redirect_uris, grant_types, response_types,
			scopes, application_type, token_endpoint_auth_method, active,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (id) DO UPDATE SET
			secret_hash = EXCLUDED.secret_hash,
			name = EXCLUDED.name,
			redirect_uris = EXCLUDED.redirect_uris,
			grant_types = EXCLUDED.grant_types,
			response_types = EXCLUDED.response_types,
			scopes = EXCLUDED.scopes,
			application_type = EXCLUDED.application_type,
			token_endpoint_auth_method = EXCLUDED.token_endpoint_auth_method,
			active = EXCLUDED.active,
			updated_at = EXCLUDED.updated_at
	`

	// Convert grant types to strings for storage
	grantTypes := make([]string, len(client.GrantTypes))
	for i, gt := range client.GrantTypes {
		grantTypes[i] = string(gt)
	}

	_, err := s.db.ExecContext(ctx, query,
		client.ID,
		nullString(client.SecretHash),
		client.Name,
		pq.Array(client.RedirectURIs),
		pq.Array(grantTypes),
		pq.Array(client.ResponseTypes),
		pq.Array(client.Scopes),
		client.ApplicationType,
		client.TokenEndpointAuthMethod,
		client.Active,
		client.CreatedAt,
		client.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save oauth client: %w", err)
	}

	return nil
}

// Get retrieves a client by client_id.
func (s *OAuthClientStore) Get(ctx context.Context, clientID string) (*domain.OAuthClient, error) {
	query := `
		SELECT id, secret_hash, name, redirect_uris, grant_types, response_types,
			   scopes, application_type, token_endpoint_auth_method, active,
			   created_at, updated_at
		FROM oauth_clients
		WHERE id = $1
	`

	var client domain.OAuthClient
	var secretHash sql.NullString
	var redirectURIs, grantTypesStr, responseTypes, scopes []string

	err := s.db.QueryRowContext(ctx, query, clientID).Scan(
		&client.ID,
		&secretHash,
		&client.Name,
		pq.Array(&redirectURIs),
		pq.Array(&grantTypesStr),
		pq.Array(&responseTypes),
		pq.Array(&scopes),
		&client.ApplicationType,
		&client.TokenEndpointAuthMethod,
		&client.Active,
		&client.CreatedAt,
		&client.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get oauth client: %w", err)
	}

	client.SecretHash = secretHash.String
	client.RedirectURIs = redirectURIs
	client.ResponseTypes = responseTypes
	client.Scopes = scopes

	// Convert grant type strings back to domain type
	client.GrantTypes = make([]domain.OAuthGrantType, len(grantTypesStr))
	for i, gt := range grantTypesStr {
		client.GrantTypes[i] = domain.OAuthGrantType(gt)
	}

	return &client, nil
}

// Delete removes a client by client_id.
func (s *OAuthClientStore) Delete(ctx context.Context, clientID string) error {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM oauth_clients WHERE id = $1", clientID)
	if err != nil {
		return fmt.Errorf("delete oauth client: %w", err)
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

// List retrieves all registered clients.
func (s *OAuthClientStore) List(ctx context.Context) ([]*domain.OAuthClient, error) {
	query := `
		SELECT id, secret_hash, name, redirect_uris, grant_types, response_types,
			   scopes, application_type, token_endpoint_auth_method, active,
			   created_at, updated_at
		FROM oauth_clients
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list oauth clients: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var clients []*domain.OAuthClient
	for rows.Next() {
		var client domain.OAuthClient
		var secretHash sql.NullString
		var redirectURIs, grantTypesStr, responseTypes, scopes []string

		if err := rows.Scan(
			&client.ID,
			&secretHash,
			&client.Name,
			pq.Array(&redirectURIs),
			pq.Array(&grantTypesStr),
			pq.Array(&responseTypes),
			pq.Array(&scopes),
			&client.ApplicationType,
			&client.TokenEndpointAuthMethod,
			&client.Active,
			&client.CreatedAt,
			&client.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan oauth client: %w", err)
		}

		client.SecretHash = secretHash.String
		client.RedirectURIs = redirectURIs
		client.ResponseTypes = responseTypes
		client.Scopes = scopes

		// Convert grant type strings back to domain type
		client.GrantTypes = make([]domain.OAuthGrantType, len(grantTypesStr))
		for i, gt := range grantTypesStr {
			client.GrantTypes[i] = domain.OAuthGrantType(gt)
		}

		clients = append(clients, &client)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate oauth clients: %w", err)
	}

	return clients, nil
}

// Ensure AuthorizationCodeStore implements the interface.
var _ driven.AuthorizationCodeStore = (*AuthorizationCodeStore)(nil)

// AuthorizationCodeStore implements driven.AuthorizationCodeStore using PostgreSQL.
type AuthorizationCodeStore struct {
	db *sql.DB
}

// NewAuthorizationCodeStore creates a new PostgreSQL-backed authorization code store.
func NewAuthorizationCodeStore(db *sql.DB) *AuthorizationCodeStore {
	return &AuthorizationCodeStore{db: db}
}

// Save stores a new authorization code.
func (s *AuthorizationCodeStore) Save(ctx context.Context, code *domain.AuthorizationCode) error {
	now := time.Now()
	if code.CreatedAt.IsZero() {
		code.CreatedAt = now
	}

	query := `
		INSERT INTO oauth_authorization_codes (
			code, client_id, user_id, redirect_uri, scopes, code_challenge,
			resource, used, expires_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := s.db.ExecContext(ctx, query,
		code.Code,
		code.ClientID,
		code.UserID,
		code.RedirectURI,
		pq.Array(code.Scopes),
		code.CodeChallenge,
		nullString(code.Resource),
		code.Used,
		code.ExpiresAt,
		code.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save authorization code: %w", err)
	}

	return nil
}

// GetAndMarkUsed atomically retrieves the code and marks it as used.
func (s *AuthorizationCodeStore) GetAndMarkUsed(ctx context.Context, code string) (*domain.AuthorizationCode, error) {
	query := `
		UPDATE oauth_authorization_codes
		SET used = true
		WHERE code = $1 AND used = false
		RETURNING code, client_id, user_id, redirect_uri, scopes, code_challenge,
				  resource, used, expires_at, created_at
	`

	var authCode domain.AuthorizationCode
	var resource sql.NullString
	var scopes []string

	err := s.db.QueryRowContext(ctx, query, code).Scan(
		&authCode.Code,
		&authCode.ClientID,
		&authCode.UserID,
		&authCode.RedirectURI,
		pq.Array(&scopes),
		&authCode.CodeChallenge,
		&resource,
		&authCode.Used,
		&authCode.ExpiresAt,
		&authCode.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get and mark used authorization code: %w", err)
	}

	authCode.Resource = resource.String
	authCode.Scopes = scopes

	return &authCode, nil
}

// Cleanup removes expired authorization codes.
func (s *AuthorizationCodeStore) Cleanup(ctx context.Context) error {
	query := `DELETE FROM oauth_authorization_codes WHERE expires_at < NOW()`

	_, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("cleanup authorization codes: %w", err)
	}

	return nil
}

// Ensure OAuthTokenStore implements the interface.
var _ driven.OAuthTokenStore = (*OAuthTokenStore)(nil)

// OAuthTokenStore implements driven.OAuthTokenStore using PostgreSQL.
type OAuthTokenStore struct {
	db *sql.DB
}

// NewOAuthTokenStore creates a new PostgreSQL-backed OAuth token store.
func NewOAuthTokenStore(db *sql.DB) *OAuthTokenStore {
	return &OAuthTokenStore{db: db}
}

// SaveAccessToken stores a new access token.
func (s *OAuthTokenStore) SaveAccessToken(ctx context.Context, token *domain.OAuthAccessToken) error {
	now := time.Now()
	if token.CreatedAt.IsZero() {
		token.CreatedAt = now
	}

	query := `
		INSERT INTO oauth_access_tokens (
			id, client_id, user_id, scopes, audience, revoked, expires_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := s.db.ExecContext(ctx, query,
		token.ID,
		token.ClientID,
		token.UserID,
		pq.Array(token.Scopes),
		nullString(token.Audience),
		token.Revoked,
		token.ExpiresAt,
		token.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save access token: %w", err)
	}

	return nil
}

// GetAccessToken retrieves an access token by its ID (jti claim).
func (s *OAuthTokenStore) GetAccessToken(ctx context.Context, tokenID string) (*domain.OAuthAccessToken, error) {
	query := `
		SELECT id, client_id, user_id, scopes, audience, revoked, expires_at, created_at
		FROM oauth_access_tokens
		WHERE id = $1
	`

	var token domain.OAuthAccessToken
	var audience sql.NullString
	var scopes []string

	err := s.db.QueryRowContext(ctx, query, tokenID).Scan(
		&token.ID,
		&token.ClientID,
		&token.UserID,
		pq.Array(&scopes),
		&audience,
		&token.Revoked,
		&token.ExpiresAt,
		&token.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	token.Audience = audience.String
	token.Scopes = scopes

	return &token, nil
}

// RevokeAccessToken marks an access token as revoked.
func (s *OAuthTokenStore) RevokeAccessToken(ctx context.Context, tokenID string) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE oauth_access_tokens SET revoked = true WHERE id = $1", tokenID)
	if err != nil {
		return fmt.Errorf("revoke access token: %w", err)
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

// SaveRefreshToken stores a new refresh token.
func (s *OAuthTokenStore) SaveRefreshToken(ctx context.Context, token *domain.OAuthRefreshToken) error {
	now := time.Now()
	if token.CreatedAt.IsZero() {
		token.CreatedAt = now
	}

	query := `
		INSERT INTO oauth_refresh_tokens (
			id, access_token_id, client_id, user_id, scopes, audience,
			revoked, rotated_to, expires_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := s.db.ExecContext(ctx, query,
		token.ID,
		token.AccessTokenID,
		token.ClientID,
		token.UserID,
		pq.Array(token.Scopes),
		nullString(token.Audience),
		token.Revoked,
		nullString(token.RotatedTo),
		token.ExpiresAt,
		token.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save refresh token: %w", err)
	}

	return nil
}

// GetRefreshToken retrieves a refresh token by its ID.
func (s *OAuthTokenStore) GetRefreshToken(ctx context.Context, tokenID string) (*domain.OAuthRefreshToken, error) {
	query := `
		SELECT id, access_token_id, client_id, user_id, scopes, audience,
			   revoked, rotated_to, expires_at, created_at
		FROM oauth_refresh_tokens
		WHERE id = $1
	`

	var token domain.OAuthRefreshToken
	var audience, rotatedTo sql.NullString
	var scopes []string

	err := s.db.QueryRowContext(ctx, query, tokenID).Scan(
		&token.ID,
		&token.AccessTokenID,
		&token.ClientID,
		&token.UserID,
		pq.Array(&scopes),
		&audience,
		&token.Revoked,
		&rotatedTo,
		&token.ExpiresAt,
		&token.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get refresh token: %w", err)
	}

	token.Audience = audience.String
	token.RotatedTo = rotatedTo.String
	token.Scopes = scopes

	return &token, nil
}

// RevokeRefreshToken marks a refresh token as revoked.
func (s *OAuthTokenStore) RevokeRefreshToken(ctx context.Context, tokenID string) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE oauth_refresh_tokens SET revoked = true WHERE id = $1", tokenID)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
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

// RotateRefreshToken marks the old token as rotated and returns the new token ID.
func (s *OAuthTokenStore) RotateRefreshToken(ctx context.Context, oldTokenID string, newTokenID string) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE oauth_refresh_tokens SET rotated_to = $2, revoked = true WHERE id = $1",
		oldTokenID, newTokenID)
	if err != nil {
		return fmt.Errorf("rotate refresh token: %w", err)
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

// RevokeAllForClient revokes all tokens (access and refresh) for a given client.
func (s *OAuthTokenStore) RevokeAllForClient(ctx context.Context, clientID string) error {
	// Revoke all access tokens
	_, err := s.db.ExecContext(ctx,
		"UPDATE oauth_access_tokens SET revoked = true WHERE client_id = $1", clientID)
	if err != nil {
		return fmt.Errorf("revoke access tokens for client: %w", err)
	}

	// Revoke all refresh tokens
	_, err = s.db.ExecContext(ctx,
		"UPDATE oauth_refresh_tokens SET revoked = true WHERE client_id = $1", clientID)
	if err != nil {
		return fmt.Errorf("revoke refresh tokens for client: %w", err)
	}

	return nil
}

// Cleanup removes expired tokens.
func (s *OAuthTokenStore) Cleanup(ctx context.Context) error {
	// Delete expired access tokens
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM oauth_access_tokens WHERE expires_at < NOW()")
	if err != nil {
		return fmt.Errorf("cleanup access tokens: %w", err)
	}

	// Delete expired refresh tokens
	_, err = s.db.ExecContext(ctx,
		"DELETE FROM oauth_refresh_tokens WHERE expires_at < NOW()")
	if err != nil {
		return fmt.Errorf("cleanup refresh tokens: %w", err)
	}

	return nil
}
