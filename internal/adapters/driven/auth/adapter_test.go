package auth

import (
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

func TestNewAdapter(t *testing.T) {
	adapter := NewAdapter("test-secret")
	if string(adapter.jwtSecret) != "test-secret" {
		t.Error("expected jwt secret to be set")
	}
}

func TestNewAdapterWithCost(t *testing.T) {
	adapter := NewAdapterWithCost("test-secret", 4)
	if adapter.bcryptCost != 4 {
		t.Errorf("expected bcrypt cost 4, got %d", adapter.bcryptCost)
	}
}

func TestHashPassword(t *testing.T) {
	adapter := NewAdapterWithCost("secret", 4) // Low cost for faster tests

	hash, err := adapter.HashPassword("mypassword")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if hash == "" {
		t.Error("expected non-empty hash")
	}

	if hash == "mypassword" {
		t.Error("hash should not equal plaintext password")
	}

	// Hash should start with bcrypt prefix
	if len(hash) < 60 {
		t.Error("expected bcrypt hash to be at least 60 characters")
	}
}

func TestHashPassword_DifferentHashesForSamePassword(t *testing.T) {
	adapter := NewAdapterWithCost("secret", 4)

	hash1, _ := adapter.HashPassword("password123")
	hash2, _ := adapter.HashPassword("password123")

	if hash1 == hash2 {
		t.Error("expected different hashes for same password (due to salt)")
	}
}

func TestVerifyPassword_CorrectPassword(t *testing.T) {
	adapter := NewAdapterWithCost("secret", 4)

	password := "correctpassword"
	hash, _ := adapter.HashPassword(password)

	if !adapter.VerifyPassword(password, hash) {
		t.Error("expected password verification to succeed")
	}
}

func TestVerifyPassword_IncorrectPassword(t *testing.T) {
	adapter := NewAdapterWithCost("secret", 4)

	hash, _ := adapter.HashPassword("correctpassword")

	if adapter.VerifyPassword("wrongpassword", hash) {
		t.Error("expected password verification to fail for wrong password")
	}
}

func TestVerifyPassword_InvalidHash(t *testing.T) {
	adapter := NewAdapter("secret")

	if adapter.VerifyPassword("password", "not-a-valid-hash") {
		t.Error("expected verification to fail for invalid hash")
	}
}

func TestGenerateToken(t *testing.T) {
	adapter := NewAdapter("test-jwt-secret")

	now := time.Now()
	claims := &domain.TokenClaims{
		UserID:    "user-123",
		Email:     "test@example.com",
		Role:      domain.RoleMember,
		TeamID:    "team-456",
		SessionID: "session-789",
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(24 * time.Hour).Unix(),
	}

	token, err := adapter.GenerateToken(claims)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	if token == "" {
		t.Error("expected non-empty token")
	}

	// JWT tokens have 3 parts separated by dots
	parts := 0
	for _, c := range token {
		if c == '.' {
			parts++
		}
	}
	if parts != 2 {
		t.Errorf("expected JWT with 2 dots (3 parts), got %d dots", parts)
	}
}

func TestParseToken_ValidToken(t *testing.T) {
	adapter := NewAdapter("test-jwt-secret")

	now := time.Now()
	originalClaims := &domain.TokenClaims{
		UserID:    "user-123",
		Email:     "test@example.com",
		Role:      domain.RoleAdmin,
		TeamID:    "team-456",
		SessionID: "session-789",
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(24 * time.Hour).Unix(),
	}

	token, _ := adapter.GenerateToken(originalClaims)

	parsedClaims, err := adapter.ParseToken(token)
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	if parsedClaims.UserID != originalClaims.UserID {
		t.Errorf("expected UserID %s, got %s", originalClaims.UserID, parsedClaims.UserID)
	}
	if parsedClaims.Email != originalClaims.Email {
		t.Errorf("expected Email %s, got %s", originalClaims.Email, parsedClaims.Email)
	}
	if parsedClaims.Role != originalClaims.Role {
		t.Errorf("expected Role %s, got %s", originalClaims.Role, parsedClaims.Role)
	}
	if parsedClaims.TeamID != originalClaims.TeamID {
		t.Errorf("expected TeamID %s, got %s", originalClaims.TeamID, parsedClaims.TeamID)
	}
	if parsedClaims.SessionID != originalClaims.SessionID {
		t.Errorf("expected SessionID %s, got %s", originalClaims.SessionID, parsedClaims.SessionID)
	}
}

func TestParseToken_ExpiredToken(t *testing.T) {
	adapter := NewAdapter("test-jwt-secret")

	// Create a token that expired in the past
	pastTime := time.Now().Add(-2 * time.Hour)
	claims := &domain.TokenClaims{
		UserID:    "user-123",
		Email:     "test@example.com",
		Role:      domain.RoleMember,
		TeamID:    "team-456",
		SessionID: "session-789",
		IssuedAt:  pastTime.Add(-24 * time.Hour).Unix(),
		ExpiresAt: pastTime.Unix(), // Expired 2 hours ago
	}

	token, _ := adapter.GenerateToken(claims)

	_, err := adapter.ParseToken(token)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestParseToken_InvalidToken(t *testing.T) {
	adapter := NewAdapter("test-jwt-secret")

	_, err := adapter.ParseToken("invalid.token.here")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestParseToken_WrongSecret(t *testing.T) {
	adapter1 := NewAdapter("secret-1")
	adapter2 := NewAdapter("secret-2")

	now := time.Now()
	claims := &domain.TokenClaims{
		UserID:    "user-123",
		Email:     "test@example.com",
		Role:      domain.RoleMember,
		TeamID:    "team-456",
		SessionID: "session-789",
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(24 * time.Hour).Unix(),
	}

	// Generate token with adapter1's secret
	token, _ := adapter1.GenerateToken(claims)

	// Try to parse with adapter2's secret
	_, err := adapter2.ParseToken(token)
	if err == nil {
		t.Error("expected error when parsing token with wrong secret")
	}
}

func TestParseToken_MalformedToken(t *testing.T) {
	adapter := NewAdapter("test-secret")

	testCases := []string{
		"",
		"not-a-jwt",
		"only.two.parts.missing",
		"header.payload", // missing signature
	}

	for _, tc := range testCases {
		_, err := adapter.ParseToken(tc)
		if err == nil {
			t.Errorf("expected error for malformed token: %q", tc)
		}
	}
}

func TestRoundTrip_AllRoles(t *testing.T) {
	adapter := NewAdapter("test-secret")

	roles := []domain.Role{
		domain.RoleMember,
		domain.RoleAdmin,
		domain.RoleViewer,
	}

	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
			now := time.Now()
			claims := &domain.TokenClaims{
				UserID:    "user-123",
				Email:     "test@example.com",
				Role:      role,
				TeamID:    "team-456",
				SessionID: "session-789",
				IssuedAt:  now.Unix(),
				ExpiresAt: now.Add(24 * time.Hour).Unix(),
			}

			token, err := adapter.GenerateToken(claims)
			if err != nil {
				t.Fatalf("failed to generate token: %v", err)
			}

			parsed, err := adapter.ParseToken(token)
			if err != nil {
				t.Fatalf("failed to parse token: %v", err)
			}

			if parsed.Role != role {
				t.Errorf("expected role %s, got %s", role, parsed.Role)
			}
		})
	}
}

// Benchmark tests
func BenchmarkHashPassword(b *testing.B) {
	adapter := NewAdapterWithCost("secret", 4) // Low cost for benchmarks

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapter.HashPassword("testpassword")
	}
}

func BenchmarkVerifyPassword(b *testing.B) {
	adapter := NewAdapterWithCost("secret", 4)
	hash, _ := adapter.HashPassword("testpassword")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = adapter.VerifyPassword("testpassword", hash)
	}
}

func BenchmarkGenerateToken(b *testing.B) {
	adapter := NewAdapter("test-secret")
	now := time.Now()
	claims := &domain.TokenClaims{
		UserID:    "user-123",
		Email:     "test@example.com",
		Role:      domain.RoleMember,
		TeamID:    "team-456",
		SessionID: "session-789",
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(24 * time.Hour).Unix(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapter.GenerateToken(claims)
	}
}

func BenchmarkParseToken(b *testing.B) {
	adapter := NewAdapter("test-secret")
	now := time.Now()
	claims := &domain.TokenClaims{
		UserID:    "user-123",
		Email:     "test@example.com",
		Role:      domain.RoleMember,
		TeamID:    "team-456",
		SessionID: "session-789",
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(24 * time.Hour).Unix(),
	}
	token, _ := adapter.GenerateToken(claims)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapter.ParseToken(token)
	}
}
