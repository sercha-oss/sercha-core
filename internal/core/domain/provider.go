package domain

import "time"

// ProviderType identifies a data source provider
type ProviderType string

const (
	// Code repositories
	ProviderTypeGitHub    ProviderType = "github"
	ProviderTypeGitLab    ProviderType = "gitlab"
	ProviderTypeBitbucket ProviderType = "bitbucket"

	// Communication
	ProviderTypeSlack   ProviderType = "slack"
	ProviderTypeDiscord ProviderType = "discord"
	ProviderTypeMSTeams ProviderType = "msteams"

	// Documentation
	ProviderTypeNotion     ProviderType = "notion"
	ProviderTypeConfluence ProviderType = "confluence"
	ProviderTypeGoogleDocs ProviderType = "google_docs"

	// Project management
	ProviderTypeJira   ProviderType = "jira"
	ProviderTypeLinear ProviderType = "linear"

	// File storage
	ProviderTypeGoogleDrive ProviderType = "google_drive"
	ProviderTypeDropbox     ProviderType = "dropbox"
	ProviderTypeOneDrive    ProviderType = "onedrive"
	ProviderTypeS3          ProviderType = "s3"

	// Local/Development
	ProviderTypeLocalFS ProviderType = "localfs"

	// Other
	ProviderTypeZendesk  ProviderType = "zendesk"
	ProviderTypeIntercom ProviderType = "intercom"
)

// AuthProvider holds OAuth configuration for a provider
type AuthProvider struct {
	Type         ProviderType `json:"type"`
	Name         string       `json:"name"`         // Display name
	AuthURL      string       `json:"auth_url"`     // OAuth authorization URL
	TokenURL     string       `json:"token_url"`    // OAuth token URL
	Scopes       []string     `json:"scopes"`       // Required OAuth scopes
	ClientID     string       `json:"client_id"`    // OAuth client ID (public)
	ClientSecret string       `json:"-"`            // OAuth client secret (never serialize)
	RedirectURL  string       `json:"redirect_url"` // OAuth callback URL
}

// ProviderInfo provides metadata about a provider
type ProviderInfo struct {
	Type        ProviderType `json:"type"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	IconURL     string       `json:"icon_url,omitempty"`
	AuthMethods []AuthMethod `json:"auth_methods"`
	DocsURL     string       `json:"docs_url,omitempty"`
	Available   bool         `json:"available"` // Whether connector is implemented
}

// CoreProviders returns the 11 providers for Sercha Core
func CoreProviders() []ProviderType {
	return []ProviderType{
		ProviderTypeGitHub,
		ProviderTypeGitLab,
		ProviderTypeSlack,
		ProviderTypeNotion,
		ProviderTypeConfluence,
		ProviderTypeJira,
		ProviderTypeGoogleDrive,
		ProviderTypeGoogleDocs,
		ProviderTypeLinear,
		ProviderTypeDropbox,
		ProviderTypeS3,
	}
}

// ProviderConfig represents OAuth app configuration for a provider type.
// One config per provider - multiple installations can use the same config.
type ProviderConfig struct {
	ProviderType ProviderType

	// Decrypted secrets (never persisted as-is)
	Secrets *ProviderSecrets

	// Non-secret configuration (can override defaults)
	AuthURL     string   // OAuth authorization URL
	TokenURL    string   // OAuth token URL
	Scopes      []string // OAuth scopes
	RedirectURI string   // OAuth callback URL

	// Metadata
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ProviderSecrets contains decrypted secret values.
// Encrypted before storage using same AES-GCM as installations.
type ProviderSecrets struct {
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	APIKey       string `json:"api_key,omitempty"` // For non-OAuth providers
}

// ProviderConfigSummary is a safe view without secrets (for listing).
type ProviderConfigSummary struct {
	ProviderType ProviderType
	Enabled      bool
	HasSecrets   bool // true if secrets are configured
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// IsConfigured returns true if the provider has credentials set.
func (p *ProviderConfig) IsConfigured() bool {
	if p.Secrets == nil {
		return false
	}
	return p.Secrets.ClientID != "" || p.Secrets.APIKey != ""
}
