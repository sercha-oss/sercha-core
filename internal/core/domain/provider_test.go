package domain

import (
	"testing"
)

func TestProviderTypeConstants(t *testing.T) {
	tests := []struct {
		provider ProviderType
		expected string
	}{
		{ProviderTypeGitHub, "github"},
		{ProviderTypeLocalFS, "localfs"},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			if string(tt.provider) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(tt.provider))
			}
		})
	}
}

func TestAuthProvider(t *testing.T) {
	provider := AuthProvider{
		Type:         ProviderTypeGitHub,
		Name:         "GitHub",
		AuthURL:      "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		Scopes:       []string{"repo", "user"},
		ClientID:     "client-id-123",
		ClientSecret: "client-secret-456",
		RedirectURL:  "https://app.example.com/callback",
	}

	if provider.Type != ProviderTypeGitHub {
		t.Errorf("expected Type github, got %s", provider.Type)
	}
	if provider.Name != "GitHub" {
		t.Errorf("expected Name GitHub, got %s", provider.Name)
	}
	if provider.AuthURL != "https://github.com/login/oauth/authorize" {
		t.Errorf("unexpected AuthURL: %s", provider.AuthURL)
	}
	if provider.TokenURL != "https://github.com/login/oauth/access_token" {
		t.Errorf("unexpected TokenURL: %s", provider.TokenURL)
	}
	if len(provider.Scopes) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(provider.Scopes))
	}
	if provider.ClientID != "client-id-123" {
		t.Errorf("expected ClientID client-id-123, got %s", provider.ClientID)
	}
	if provider.ClientSecret != "client-secret-456" {
		t.Errorf("expected ClientSecret client-secret-456, got %s", provider.ClientSecret)
	}
}

func TestProviderConfig_IsConfigured(t *testing.T) {
	tests := []struct {
		name     string
		config   *ProviderConfig
		expected bool
	}{
		{
			name: "nil secrets",
			config: &ProviderConfig{
				ProviderType: ProviderTypeGitHub,
				Secrets:      nil,
			},
			expected: false,
		},
		{
			name: "empty secrets",
			config: &ProviderConfig{
				ProviderType: ProviderTypeGitHub,
				Secrets:      &ProviderSecrets{},
			},
			expected: false,
		},
		{
			name: "with client_id",
			config: &ProviderConfig{
				ProviderType: ProviderTypeGitHub,
				Secrets: &ProviderSecrets{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
				},
			},
			expected: true,
		},
		{
			name: "with api_key only",
			config: &ProviderConfig{
				ProviderType: ProviderTypeLocalFS,
				Secrets: &ProviderSecrets{
					APIKey: "test-api-key",
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.IsConfigured(); got != tt.expected {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProviderSecrets(t *testing.T) {
	secrets := ProviderSecrets{
		ClientID:     "client-id-123",
		ClientSecret: "client-secret-456",
		APIKey:       "api-key-789",
	}

	if secrets.ClientID != "client-id-123" {
		t.Errorf("expected ClientID client-id-123, got %s", secrets.ClientID)
	}
	if secrets.ClientSecret != "client-secret-456" {
		t.Errorf("expected ClientSecret client-secret-456, got %s", secrets.ClientSecret)
	}
	if secrets.APIKey != "api-key-789" {
		t.Errorf("expected APIKey api-key-789, got %s", secrets.APIKey)
	}
}

func TestProviderConfigSummary(t *testing.T) {
	summary := ProviderConfigSummary{
		ProviderType: ProviderTypeGitHub,
		Enabled:      true,
		HasSecrets:   true,
	}

	if summary.ProviderType != ProviderTypeGitHub {
		t.Errorf("expected ProviderType github, got %s", summary.ProviderType)
	}
	if !summary.Enabled {
		t.Error("expected Enabled to be true")
	}
	if !summary.HasSecrets {
		t.Error("expected HasSecrets to be true")
	}
}

func TestPlatformFor(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderType
		expected PlatformType
	}{
		{"GitHub", ProviderTypeGitHub, PlatformGitHub},
		{"LocalFS", ProviderTypeLocalFS, PlatformLocalFS},
		{"Unknown provider", ProviderType("unknown"), PlatformType("unknown")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PlatformFor(tt.provider); got != tt.expected {
				t.Errorf("PlatformFor(%s) = %s, want %s", tt.provider, got, tt.expected)
			}
		})
	}
}

func TestServicesFor(t *testing.T) {
	tests := []struct {
		name     string
		platform PlatformType
		expected []ProviderType
	}{
		{"GitHub", PlatformGitHub, []ProviderType{ProviderTypeGitHub}},
		{"LocalFS", PlatformLocalFS, []ProviderType{ProviderTypeLocalFS}},
		{"Unknown platform", PlatformType("unknown"), []ProviderType{ProviderType("unknown")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ServicesFor(tt.platform)
			if len(got) != len(tt.expected) {
				t.Errorf("ServicesFor(%s) returned %d services, want %d", tt.platform, len(got), len(tt.expected))
				return
			}
			for i, service := range got {
				if service != tt.expected[i] {
					t.Errorf("ServicesFor(%s)[%d] = %s, want %s", tt.platform, i, service, tt.expected[i])
				}
			}
		})
	}
}

func TestPlatformDisplayName(t *testing.T) {
	tests := []struct {
		platform PlatformType
		expected string
	}{
		{PlatformGitHub, "GitHub"},
		{PlatformLocalFS, "Local Filesystem"},
		{PlatformType("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.platform), func(t *testing.T) {
			if got := PlatformDisplayName(tt.platform); got != tt.expected {
				t.Errorf("PlatformDisplayName(%s) = %s, want %s", tt.platform, got, tt.expected)
			}
		})
	}
}
