package domain

import "time"

// Connection represents a connector connection with stored credentials.
// A connection is the authenticated link to a provider (GitHub, Google, etc.)
// Sources reference connections to get their authentication context.
type Connection struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	ProviderType ProviderType `json:"provider_type"`
	AuthMethod   AuthMethod   `json:"auth_method"`

	// Secrets contains decrypted secret values (never persisted as-is)
	// This is populated when fetching from store, nil when listing
	Secrets *ConnectionSecrets `json:"-"`

	// OAuth metadata (non-secret, safe to expose)
	OAuthTokenType string     `json:"oauth_token_type,omitempty"`
	OAuthExpiry    *time.Time `json:"oauth_expiry,omitempty"`
	OAuthScopes    []string   `json:"oauth_scopes,omitempty"`

	// AccountID is the provider account identifier (email, username)
	// Used for display and uniqueness constraint
	AccountID string `json:"account_id,omitempty"`

	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// ConnectionSecrets contains decrypted secret values.
// These are encrypted before storage and decrypted on retrieval.
type ConnectionSecrets struct {
	// OAuth2 tokens
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`

	// API Key (for api_key auth method)
	APIKey string `json:"api_key,omitempty"`

	// Service Account JSON (for service_account auth method, e.g., Google)
	ServiceAccountJSON string `json:"service_account_json,omitempty"`
}

// ConnectionSummary is a safe view without secrets for listing.
type ConnectionSummary struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	ProviderType ProviderType `json:"provider_type"`
	AuthMethod   AuthMethod   `json:"auth_method"`
	AccountID    string       `json:"account_id,omitempty"`
	OAuthExpiry  *time.Time   `json:"oauth_expiry,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	LastUsedAt   *time.Time   `json:"last_used_at,omitempty"`
}

// ToSummary converts Connection to ConnectionSummary.
func (c *Connection) ToSummary() *ConnectionSummary {
	return &ConnectionSummary{
		ID:           c.ID,
		Name:         c.Name,
		ProviderType: c.ProviderType,
		AuthMethod:   c.AuthMethod,
		AccountID:    c.AccountID,
		OAuthExpiry:  c.OAuthExpiry,
		CreatedAt:    c.CreatedAt,
		LastUsedAt:   c.LastUsedAt,
	}
}

// NeedsRefresh returns true if OAuth tokens should be refreshed.
// Returns true if within 5 minutes of expiry.
func (c *Connection) NeedsRefresh() bool {
	if c.OAuthExpiry == nil {
		return false
	}
	return time.Until(*c.OAuthExpiry) < 5*time.Minute
}

// IsExpired returns true if OAuth tokens have expired.
func (c *Connection) IsExpired() bool {
	if c.OAuthExpiry == nil {
		return false
	}
	return time.Now().After(*c.OAuthExpiry)
}

// HasSecrets returns true if the connection has secrets loaded.
func (c *Connection) HasSecrets() bool {
	return c.Secrets != nil
}

// GetAccessToken returns the access token if available.
// For OAuth2: returns the access token
// For API Key/PAT: returns the API key
func (c *Connection) GetAccessToken() string {
	if c.Secrets == nil {
		return ""
	}
	if c.AuthMethod == AuthMethodOAuth2 {
		return c.Secrets.AccessToken
	}
	if c.AuthMethod == AuthMethodAPIKey || c.AuthMethod == AuthMethodPAT {
		return c.Secrets.APIKey
	}
	return ""
}
