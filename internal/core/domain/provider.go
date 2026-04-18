package domain

import "time"

// ProviderType identifies a data source provider
type ProviderType string

const (
	// Only implemented connectors
	ProviderTypeGitHub   ProviderType = "github"
	ProviderTypeLocalFS  ProviderType = "localfs"
	ProviderTypeNotion   ProviderType = "notion"
	ProviderTypeOneDrive ProviderType = "onedrive"
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

// PlatformType identifies an authentication boundary (OAuth client)
// Multiple services can share a platform (e.g., google_drive and google_docs both use google)
type PlatformType string

const (
	// Only implemented platforms (1:1 with providers for now)
	PlatformGitHub    PlatformType = "github"
	PlatformLocalFS   PlatformType = "localfs"
	PlatformNotion    PlatformType = "notion"
	PlatformMicrosoft PlatformType = "microsoft"
)

// PlatformFor returns the platform that owns a given service/provider.
// For 1:1 connectors (all current ones), platform == provider.
// Multi-service platforms (Google, Microsoft, Atlassian) will map
// multiple ProviderTypes to one PlatformType when implemented.
func PlatformFor(provider ProviderType) PlatformType {
	switch provider {
	case ProviderTypeGitHub:
		return PlatformGitHub
	case ProviderTypeLocalFS:
		return PlatformLocalFS
	case ProviderTypeNotion:
		return PlatformNotion
	case ProviderTypeOneDrive:
		return PlatformMicrosoft
	default:
		return PlatformType(provider)
	}
}

// ServicesFor returns all services under a platform.
func ServicesFor(platform PlatformType) []ProviderType {
	switch platform {
	case PlatformGitHub:
		return []ProviderType{ProviderTypeGitHub}
	case PlatformLocalFS:
		return []ProviderType{ProviderTypeLocalFS}
	case PlatformNotion:
		return []ProviderType{ProviderTypeNotion}
	case PlatformMicrosoft:
		return []ProviderType{ProviderTypeOneDrive}
	default:
		return []ProviderType{ProviderType(platform)}
	}
}

func PlatformDisplayName(platform PlatformType) string {
	switch platform {
	case PlatformGitHub:
		return "GitHub"
	case PlatformLocalFS:
		return "Local Filesystem"
	case PlatformNotion:
		return "Notion"
	case PlatformMicrosoft:
		return "Microsoft"
	default:
		return string(platform)
	}
}
