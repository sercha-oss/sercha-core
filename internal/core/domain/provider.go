package domain

import (
	"sync"
	"time"
)

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

// Custom registrations layered on top of the built-in tables below. Populated
// at process startup by callers that ship additional connectors and want them
// to map cleanly onto an existing PlatformType (so they share an OAuth client,
// token endpoint, refresh logic, etc) without forking the lookup tables here.
//
// These maps are intended to be written exactly once at startup, before any
// concurrent reads. They are protected by platformRegistryMu only because Go
// requires a memory barrier between a write on one goroutine and a read on
// another; in practice the contention is nil.
var (
	platformRegistryMu       sync.RWMutex
	customProviderToPlatform = map[ProviderType]PlatformType{}
	customPlatformToServices = map[PlatformType][]ProviderType{}
	customPlatformDisplay    = map[PlatformType]string{}
)

// RegisterPlatformMapping declares that a custom ProviderType belongs to an
// existing PlatformType. Wiring code (typically the binary's main) calls this
// at startup before any sync work begins. Subsequent calls to PlatformFor and
// ServicesFor honour the registration.
//
// The corresponding PlatformDisplayName falls back to the platform's built-in
// display name. Use RegisterPlatformDisplayName to override it for a brand-new
// PlatformType not represented in the built-in switch.
//
// Safe to call concurrently with itself; not designed to race with reads.
func RegisterPlatformMapping(provider ProviderType, platform PlatformType) {
	platformRegistryMu.Lock()
	defer platformRegistryMu.Unlock()
	customProviderToPlatform[provider] = platform
	customPlatformToServices[platform] = append(customPlatformToServices[platform], provider)
}

// RegisterPlatformDisplayName declares a human-readable display name for a
// PlatformType not represented in the built-in switch. Optional companion to
// RegisterPlatformMapping for entirely new platforms.
func RegisterPlatformDisplayName(platform PlatformType, displayName string) {
	platformRegistryMu.Lock()
	defer platformRegistryMu.Unlock()
	customPlatformDisplay[platform] = displayName
}

// PlatformFor returns the platform that owns a given service/provider.
// For 1:1 connectors (all current ones), platform == provider.
// Multi-service platforms (Google, Microsoft, Atlassian) map multiple
// ProviderTypes to one PlatformType.
//
// Custom mappings registered via RegisterPlatformMapping take precedence over
// the built-in switch. Unknown providers fall through to PlatformType(provider).
func PlatformFor(provider ProviderType) PlatformType {
	platformRegistryMu.RLock()
	if p, ok := customProviderToPlatform[provider]; ok {
		platformRegistryMu.RUnlock()
		return p
	}
	platformRegistryMu.RUnlock()

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

// ServicesFor returns all services under a platform. The result combines the
// built-in mapping with anything registered via RegisterPlatformMapping.
func ServicesFor(platform PlatformType) []ProviderType {
	var builtin []ProviderType
	switch platform {
	case PlatformGitHub:
		builtin = []ProviderType{ProviderTypeGitHub}
	case PlatformLocalFS:
		builtin = []ProviderType{ProviderTypeLocalFS}
	case PlatformNotion:
		builtin = []ProviderType{ProviderTypeNotion}
	case PlatformMicrosoft:
		builtin = []ProviderType{ProviderTypeOneDrive}
	default:
		builtin = nil
	}

	platformRegistryMu.RLock()
	custom, hasCustom := customPlatformToServices[platform]
	platformRegistryMu.RUnlock()

	if !hasCustom {
		if builtin == nil {
			return []ProviderType{ProviderType(platform)}
		}
		return builtin
	}
	if builtin == nil {
		out := make([]ProviderType, len(custom))
		copy(out, custom)
		return out
	}
	out := make([]ProviderType, 0, len(builtin)+len(custom))
	out = append(out, builtin...)
	out = append(out, custom...)
	return out
}

// PlatformDisplayName returns a human-readable name for a platform. Custom
// names registered via RegisterPlatformDisplayName take precedence; otherwise
// the built-in switch is consulted; otherwise the raw PlatformType string is
// returned.
func PlatformDisplayName(platform PlatformType) string {
	platformRegistryMu.RLock()
	if name, ok := customPlatformDisplay[platform]; ok {
		platformRegistryMu.RUnlock()
		return name
	}
	platformRegistryMu.RUnlock()

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
