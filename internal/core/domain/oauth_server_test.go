package domain

import (
	"testing"
	"time"
)

func TestOAuthClient_IsPublic(t *testing.T) {
	tests := []struct {
		name                    string
		tokenEndpointAuthMethod string
		expected                bool
	}{
		{
			name:                    "public client",
			tokenEndpointAuthMethod: "none",
			expected:                true,
		},
		{
			name:                    "confidential client - client_secret_post",
			tokenEndpointAuthMethod: "client_secret_post",
			expected:                false,
		},
		{
			name:                    "confidential client - client_secret_basic",
			tokenEndpointAuthMethod: "client_secret_basic",
			expected:                false,
		},
		{
			name:                    "empty auth method",
			tokenEndpointAuthMethod: "",
			expected:                false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &OAuthClient{
				TokenEndpointAuthMethod: tt.tokenEndpointAuthMethod,
			}
			if got := client.IsPublic(); got != tt.expected {
				t.Errorf("IsPublic() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOAuthClient_HasRedirectURI(t *testing.T) {
	tests := []struct {
		name         string
		redirectURIs []string
		checkURI     string
		expected     bool
	}{
		{
			name:         "URI present",
			redirectURIs: []string{"http://localhost:3000/callback", "http://app.example.com/callback"},
			checkURI:     "http://localhost:3000/callback",
			expected:     true,
		},
		{
			name:         "URI absent",
			redirectURIs: []string{"http://localhost:3000/callback"},
			checkURI:     "http://malicious.com/callback",
			expected:     false,
		},
		{
			name:         "empty redirect URIs",
			redirectURIs: []string{},
			checkURI:     "http://localhost:3000/callback",
			expected:     false,
		},
		{
			name:         "nil redirect URIs",
			redirectURIs: nil,
			checkURI:     "http://localhost:3000/callback",
			expected:     false,
		},
		{
			name:         "empty URI check",
			redirectURIs: []string{"http://localhost:3000/callback", ""},
			checkURI:     "",
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &OAuthClient{
				RedirectURIs: tt.redirectURIs,
			}
			if got := client.HasRedirectURI(tt.checkURI); got != tt.expected {
				t.Errorf("HasRedirectURI(%s) = %v, want %v", tt.checkURI, got, tt.expected)
			}
		})
	}
}

func TestOAuthClient_HasGrantType(t *testing.T) {
	tests := []struct {
		name       string
		grantTypes []OAuthGrantType
		checkType  OAuthGrantType
		expected   bool
	}{
		{
			name:       "grant type present - authorization_code",
			grantTypes: []OAuthGrantType{GrantTypeAuthorizationCode, GrantTypeRefreshToken},
			checkType:  GrantTypeAuthorizationCode,
			expected:   true,
		},
		{
			name:       "grant type present - refresh_token",
			grantTypes: []OAuthGrantType{GrantTypeAuthorizationCode, GrantTypeRefreshToken},
			checkType:  GrantTypeRefreshToken,
			expected:   true,
		},
		{
			name:       "grant type absent",
			grantTypes: []OAuthGrantType{GrantTypeAuthorizationCode},
			checkType:  GrantTypeRefreshToken,
			expected:   false,
		},
		{
			name:       "empty grant types",
			grantTypes: []OAuthGrantType{},
			checkType:  GrantTypeAuthorizationCode,
			expected:   false,
		},
		{
			name:       "nil grant types",
			grantTypes: nil,
			checkType:  GrantTypeAuthorizationCode,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &OAuthClient{
				GrantTypes: tt.grantTypes,
			}
			if got := client.HasGrantType(tt.checkType); got != tt.expected {
				t.Errorf("HasGrantType(%s) = %v, want %v", tt.checkType, got, tt.expected)
			}
		})
	}
}

func TestOAuthClient_HasScope(t *testing.T) {
	tests := []struct {
		name       string
		scopes     []string
		checkScope string
		expected   bool
	}{
		{
			name:       "scope present",
			scopes:     []string{ScopeMCPSearch, ScopeMCPDocRead, ScopeMCPSourcesList},
			checkScope: ScopeMCPSearch,
			expected:   true,
		},
		{
			name:       "scope absent",
			scopes:     []string{ScopeMCPSearch, ScopeMCPDocRead},
			checkScope: ScopeMCPSourcesList,
			expected:   false,
		},
		{
			name:       "empty scopes",
			scopes:     []string{},
			checkScope: ScopeMCPSearch,
			expected:   false,
		},
		{
			name:       "nil scopes",
			scopes:     nil,
			checkScope: ScopeMCPSearch,
			expected:   false,
		},
		{
			name:       "empty scope check",
			scopes:     []string{ScopeMCPSearch, ""},
			checkScope: "",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &OAuthClient{
				Scopes: tt.scopes,
			}
			if got := client.HasScope(tt.checkScope); got != tt.expected {
				t.Errorf("HasScope(%s) = %v, want %v", tt.checkScope, got, tt.expected)
			}
		})
	}
}

func TestOAuthClient_ValidateScopes(t *testing.T) {
	tests := []struct {
		name      string
		scopes    []string
		requested []string
		expected  []string
	}{
		{
			name:      "all valid",
			scopes:    []string{ScopeMCPSearch, ScopeMCPDocRead, ScopeMCPSourcesList},
			requested: []string{ScopeMCPSearch, ScopeMCPDocRead},
			expected:  []string{},
		},
		{
			name:      "some invalid",
			scopes:    []string{ScopeMCPSearch, ScopeMCPDocRead},
			requested: []string{ScopeMCPSearch, ScopeMCPSourcesList, "invalid:scope"},
			expected:  []string{ScopeMCPSourcesList, "invalid:scope"},
		},
		{
			name:      "all invalid",
			scopes:    []string{ScopeMCPSearch},
			requested: []string{ScopeMCPDocRead, ScopeMCPSourcesList},
			expected:  []string{ScopeMCPDocRead, ScopeMCPSourcesList},
		},
		{
			name:      "empty requested",
			scopes:    []string{ScopeMCPSearch},
			requested: []string{},
			expected:  []string{},
		},
		{
			name:      "nil requested",
			scopes:    []string{ScopeMCPSearch},
			requested: nil,
			expected:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &OAuthClient{
				Scopes: tt.scopes,
			}
			got := client.ValidateScopes(tt.requested)
			if len(got) != len(tt.expected) {
				t.Errorf("ValidateScopes() = %v, want %v", got, tt.expected)
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("ValidateScopes()[%d] = %s, want %s", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestAuthorizationCode_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "not expired",
			expiresAt: time.Now().Add(5 * time.Minute),
			expected:  false,
		},
		{
			name:      "expired",
			expiresAt: time.Now().Add(-1 * time.Minute),
			expected:  true,
		},
		{
			name:      "expires in 1 millisecond (edge case)",
			expiresAt: time.Now().Add(1 * time.Millisecond),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := &AuthorizationCode{
				ExpiresAt: tt.expiresAt,
			}
			if got := code.IsExpired(); got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAuthorizationCode_IsUsable(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		used      bool
		expected  bool
	}{
		{
			name:      "valid - not expired, not used",
			expiresAt: time.Now().Add(5 * time.Minute),
			used:      false,
			expected:  true,
		},
		{
			name:      "expired but not used",
			expiresAt: time.Now().Add(-1 * time.Minute),
			used:      false,
			expected:  false,
		},
		{
			name:      "not expired but already used",
			expiresAt: time.Now().Add(5 * time.Minute),
			used:      true,
			expected:  false,
		},
		{
			name:      "expired and used",
			expiresAt: time.Now().Add(-1 * time.Minute),
			used:      true,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := &AuthorizationCode{
				ExpiresAt: tt.expiresAt,
				Used:      tt.used,
			}
			if got := code.IsUsable(); got != tt.expected {
				t.Errorf("IsUsable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOAuthAccessToken_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "not expired",
			expiresAt: time.Now().Add(10 * time.Minute),
			expected:  false,
		},
		{
			name:      "expired",
			expiresAt: time.Now().Add(-1 * time.Minute),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &OAuthAccessToken{
				ExpiresAt: tt.expiresAt,
			}
			if got := token.IsExpired(); got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOAuthAccessToken_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		revoked   bool
		expected  bool
	}{
		{
			name:      "valid - not expired, not revoked",
			expiresAt: time.Now().Add(10 * time.Minute),
			revoked:   false,
			expected:  true,
		},
		{
			name:      "expired but not revoked",
			expiresAt: time.Now().Add(-1 * time.Minute),
			revoked:   false,
			expected:  false,
		},
		{
			name:      "not expired but revoked",
			expiresAt: time.Now().Add(10 * time.Minute),
			revoked:   true,
			expected:  false,
		},
		{
			name:      "expired and revoked",
			expiresAt: time.Now().Add(-1 * time.Minute),
			revoked:   true,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &OAuthAccessToken{
				ExpiresAt: tt.expiresAt,
				Revoked:   tt.revoked,
			}
			if got := token.IsValid(); got != tt.expected {
				t.Errorf("IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOAuthRefreshToken_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "not expired",
			expiresAt: time.Now().Add(15 * 24 * time.Hour),
			expected:  false,
		},
		{
			name:      "expired",
			expiresAt: time.Now().Add(-1 * time.Hour),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &OAuthRefreshToken{
				ExpiresAt: tt.expiresAt,
			}
			if got := token.IsExpired(); got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOAuthRefreshToken_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		revoked   bool
		rotatedTo string
		expected  bool
	}{
		{
			name:      "valid - not expired, not revoked, not rotated",
			expiresAt: time.Now().Add(15 * 24 * time.Hour),
			revoked:   false,
			rotatedTo: "",
			expected:  true,
		},
		{
			name:      "expired",
			expiresAt: time.Now().Add(-1 * time.Hour),
			revoked:   false,
			rotatedTo: "",
			expected:  false,
		},
		{
			name:      "revoked",
			expiresAt: time.Now().Add(15 * 24 * time.Hour),
			revoked:   true,
			rotatedTo: "",
			expected:  false,
		},
		{
			name:      "rotated",
			expiresAt: time.Now().Add(15 * 24 * time.Hour),
			revoked:   false,
			rotatedTo: "new-token-id",
			expected:  false,
		},
		{
			name:      "expired and revoked",
			expiresAt: time.Now().Add(-1 * time.Hour),
			revoked:   true,
			rotatedTo: "",
			expected:  false,
		},
		{
			name:      "expired and rotated",
			expiresAt: time.Now().Add(-1 * time.Hour),
			revoked:   false,
			rotatedTo: "new-token-id",
			expected:  false,
		},
		{
			name:      "revoked and rotated",
			expiresAt: time.Now().Add(15 * 24 * time.Hour),
			revoked:   true,
			rotatedTo: "new-token-id",
			expected:  false,
		},
		{
			name:      "all invalid - expired, revoked, rotated",
			expiresAt: time.Now().Add(-1 * time.Hour),
			revoked:   true,
			rotatedTo: "new-token-id",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &OAuthRefreshToken{
				ExpiresAt: tt.expiresAt,
				Revoked:   tt.revoked,
				RotatedTo: tt.rotatedTo,
			}
			if got := token.IsValid(); got != tt.expected {
				t.Errorf("IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOAuthRefreshToken_IsRotated(t *testing.T) {
	tests := []struct {
		name      string
		rotatedTo string
		expected  bool
	}{
		{
			name:      "rotated",
			rotatedTo: "new-token-id",
			expected:  true,
		},
		{
			name:      "not rotated - empty string",
			rotatedTo: "",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &OAuthRefreshToken{
				RotatedTo: tt.rotatedTo,
			}
			if got := token.IsRotated(); got != tt.expected {
				t.Errorf("IsRotated() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAuthorizeRequest_ParseScopes(t *testing.T) {
	tests := []struct {
		name     string
		scope    string
		expected []string
	}{
		{
			name:     "space-delimited scopes",
			scope:    "mcp:search mcp:documents:read mcp:sources:list",
			expected: []string{"mcp:search", "mcp:documents:read", "mcp:sources:list"},
		},
		{
			name:     "single scope",
			scope:    "mcp:search",
			expected: []string{"mcp:search"},
		},
		{
			name:     "empty scope",
			scope:    "",
			expected: []string{},
		},
		{
			name:     "multiple spaces",
			scope:    "mcp:search  mcp:documents:read   mcp:sources:list",
			expected: []string{"mcp:search", "mcp:documents:read", "mcp:sources:list"},
		},
		{
			name:     "leading and trailing spaces",
			scope:    "  mcp:search mcp:documents:read  ",
			expected: []string{"mcp:search", "mcp:documents:read"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &AuthorizeRequest{
				Scope: tt.scope,
			}
			got := req.ParseScopes()
			if len(got) != len(tt.expected) {
				t.Errorf("ParseScopes() = %v, want %v", got, tt.expected)
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("ParseScopes()[%d] = %s, want %s", i, got[i], tt.expected[i])
				}
			}
		})
	}
}
