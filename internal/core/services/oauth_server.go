package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
	"golang.org/x/crypto/bcrypt"
)

// Ensure oauthServerService implements OAuthServerService
var _ driving.OAuthServerService = (*oauthServerService)(nil)

// OAuthServerServiceConfig holds configuration for the OAuth Server service
type OAuthServerServiceConfig struct {
	ClientStore  driven.OAuthClientStore
	CodeStore    driven.AuthorizationCodeStore
	TokenStore   driven.OAuthTokenStore
	JWTSecret    string // For signing access tokens
	MCPServerURL string // For audience validation
}

// oauthServerService implements the OAuth 2.0 Authorization Server
type oauthServerService struct {
	clientStore  driven.OAuthClientStore
	codeStore    driven.AuthorizationCodeStore
	tokenStore   driven.OAuthTokenStore
	jwtSecret    string
	mcpServerURL string
}

// NewOAuthServerService creates a new OAuth Server service
func NewOAuthServerService(cfg OAuthServerServiceConfig) driving.OAuthServerService {
	return &oauthServerService{
		clientStore:  cfg.ClientStore,
		codeStore:    cfg.CodeStore,
		tokenStore:   cfg.TokenStore,
		jwtSecret:    cfg.JWTSecret,
		mcpServerURL: cfg.MCPServerURL,
	}
}

// RegisterClient handles dynamic client registration (RFC 7591)
func (s *oauthServerService) RegisterClient(ctx context.Context, req domain.ClientRegistrationRequest) (*domain.ClientRegistrationResponse, error) {
	// Validate required fields
	if req.Name == "" {
		return nil, fmt.Errorf("%w: client_name is required", domain.ErrInvalidInput)
	}
	if len(req.RedirectURIs) == 0 {
		return nil, fmt.Errorf("%w: redirect_uris is required", domain.ErrInvalidInput)
	}

	// Default grant_types to authorization_code if empty
	grantTypes := req.GrantTypes
	if len(grantTypes) == 0 {
		grantTypes = []domain.OAuthGrantType{domain.GrantTypeAuthorizationCode}
	}

	// Default response_types to code if empty
	responseTypes := req.ResponseTypes
	if len(responseTypes) == 0 {
		responseTypes = []string{"code"}
	}

	// Default scopes to DefaultMCPScopes if empty
	scopes := req.Scopes
	if len(scopes) == 0 {
		scopes = domain.DefaultMCPScopes
	}

	// Validate all scopes are known MCP scopes
	validScopes := map[string]bool{
		domain.ScopeMCPSearch:      true,
		domain.ScopeMCPDocRead:     true,
		domain.ScopeMCPSourcesList: true,
	}
	for _, scope := range scopes {
		if !validScopes[scope] {
			return nil, fmt.Errorf("%w: unknown scope %s", domain.ErrInvalidScope, scope)
		}
	}

	// Default application_type to native if empty
	applicationType := req.ApplicationType
	if applicationType == "" {
		applicationType = "native"
	}
	if applicationType != "native" && applicationType != "web" {
		return nil, fmt.Errorf("%w: application_type must be 'native' or 'web'", domain.ErrInvalidInput)
	}

	// Default token_endpoint_auth_method to none (public clients per MCP spec)
	authMethod := req.TokenEndpointAuthMethod
	if authMethod == "" {
		authMethod = "none"
	}
	if authMethod != "none" && authMethod != "client_secret_post" {
		return nil, fmt.Errorf("%w: token_endpoint_auth_method must be 'none' or 'client_secret_post'", domain.ErrInvalidInput)
	}

	// Generate client_id
	clientID := uuid.New().String()

	// Generate client_secret for confidential clients
	var secretHash string
	var plainSecret string
	if authMethod == "client_secret_post" {
		secret, err := generateRandomBytes(32)
		if err != nil {
			return nil, fmt.Errorf("generate client secret: %w", err)
		}
		plainSecret = hex.EncodeToString(secret)

		// Hash the secret with bcrypt
		hash, err := bcrypt.GenerateFromPassword([]byte(plainSecret), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash client secret: %w", err)
		}
		secretHash = string(hash)
	}

	// Create client
	now := time.Now()
	client := &domain.OAuthClient{
		ID:                      clientID,
		SecretHash:              secretHash,
		Name:                    req.Name,
		RedirectURIs:            req.RedirectURIs,
		GrantTypes:              grantTypes,
		ResponseTypes:           responseTypes,
		Scopes:                  scopes,
		ApplicationType:         applicationType,
		TokenEndpointAuthMethod: authMethod,
		Active:                  true,
		CreatedAt:               now,
		UpdatedAt:               now,
	}

	// Save to store
	if err := s.clientStore.Save(ctx, client); err != nil {
		return nil, fmt.Errorf("save client: %w", err)
	}

	// Return response with plaintext secret (only time it's shown)
	return &domain.ClientRegistrationResponse{
		ClientID:                clientID,
		ClientSecret:            plainSecret,
		Name:                    req.Name,
		RedirectURIs:            req.RedirectURIs,
		GrantTypes:              grantTypes,
		ResponseTypes:           responseTypes,
		Scopes:                  scopes,
		ApplicationType:         applicationType,
		TokenEndpointAuthMethod: authMethod,
		ClientIDIssuedAt:        now.Unix(),
	}, nil
}

// Authorize processes an authorization request and returns an authorization code
func (s *oauthServerService) Authorize(ctx context.Context, userID string, req domain.AuthorizeRequest) (string, error) {
	// Validate response_type
	if req.ResponseType != "code" {
		return "", fmt.Errorf("%w: response_type must be 'code'", domain.ErrUnsupportedResponseType)
	}

	// Validate PKCE (required per MCP spec)
	if req.CodeChallengeMethod != "S256" {
		return "", fmt.Errorf("%w: code_challenge_method must be 'S256'", domain.ErrInvalidCodeChallenge)
	}
	if req.CodeChallenge == "" {
		return "", fmt.Errorf("%w: code_challenge is required", domain.ErrInvalidCodeChallenge)
	}

	// Look up client
	client, err := s.clientStore.Get(ctx, req.ClientID)
	if err != nil {
		return "", fmt.Errorf("%w: client not found", domain.ErrInvalidClient)
	}

	// Validate client is active
	if !client.Active {
		return "", fmt.Errorf("%w: client is inactive", domain.ErrInvalidClient)
	}

	// Validate redirect_uri is registered
	if !client.HasRedirectURI(req.RedirectURI) {
		return "", fmt.Errorf("%w: redirect_uri not registered", domain.ErrInvalidRedirectURI)
	}

	// Validate client has authorization_code grant type
	if !client.HasGrantType(domain.GrantTypeAuthorizationCode) {
		return "", fmt.Errorf("%w: client does not support authorization_code", domain.ErrUnsupportedGrantType)
	}

	// Parse and validate scopes
	requestedScopes := req.ParseScopes()
	if len(requestedScopes) == 0 {
		requestedScopes = client.Scopes // Default to all client scopes
	}

	// Validate requested scopes against client's allowed scopes
	invalidScopes := client.ValidateScopes(requestedScopes)
	if len(invalidScopes) > 0 {
		return "", fmt.Errorf("%w: invalid scopes: %s", domain.ErrInvalidScope, strings.Join(invalidScopes, ", "))
	}

	// Generate authorization code
	codeBytes, err := generateRandomBytes(32)
	if err != nil {
		return "", fmt.Errorf("generate authorization code: %w", err)
	}
	code := hex.EncodeToString(codeBytes)

	// Store authorization code
	authCode := &domain.AuthorizationCode{
		Code:          code,
		ClientID:      req.ClientID,
		UserID:        userID,
		RedirectURI:   req.RedirectURI,
		Scopes:        requestedScopes,
		CodeChallenge: req.CodeChallenge,
		Resource:      req.Resource,
		ExpiresAt:     time.Now().Add(domain.AuthorizationCodeTTL),
		Used:          false,
		CreatedAt:     time.Now(),
	}

	if err := s.codeStore.Save(ctx, authCode); err != nil {
		return "", fmt.Errorf("save authorization code: %w", err)
	}

	return code, nil
}

// Token exchanges an authorization code or refresh token for tokens
func (s *oauthServerService) Token(ctx context.Context, req domain.TokenRequest) (*domain.TokenResponse, error) {
	switch req.GrantType {
	case domain.GrantTypeAuthorizationCode:
		return s.handleAuthorizationCodeGrant(ctx, req)
	case domain.GrantTypeRefreshToken:
		return s.handleRefreshTokenGrant(ctx, req)
	default:
		return nil, fmt.Errorf("%w: %s", domain.ErrUnsupportedGrantType, req.GrantType)
	}
}

// handleAuthorizationCodeGrant handles the authorization_code grant type
func (s *oauthServerService) handleAuthorizationCodeGrant(ctx context.Context, req domain.TokenRequest) (*domain.TokenResponse, error) {
	// Retrieve and mark code as used (atomic single-use)
	authCode, err := s.codeStore.GetAndMarkUsed(ctx, req.Code)
	if err != nil {
		return nil, fmt.Errorf("%w: authorization code not found", domain.ErrInvalidGrant)
	}

	// Validate code is not expired
	if authCode.IsExpired() {
		return nil, domain.ErrExpiredCode
	}

	// Note: code single-use is enforced atomically by GetAndMarkUsed
	// (UPDATE ... WHERE used = false). A second call returns "not found".

	// Validate client_id matches
	if authCode.ClientID != req.ClientID {
		return nil, fmt.Errorf("%w: client_id mismatch", domain.ErrInvalidClient)
	}

	// Validate redirect_uri matches
	if authCode.RedirectURI != req.RedirectURI {
		return nil, fmt.Errorf("%w: redirect_uri mismatch", domain.ErrInvalidRedirectURI)
	}

	// Validate PKCE: SHA256(code_verifier) == code_challenge
	if !validatePKCE(req.CodeVerifier, authCode.CodeChallenge) {
		return nil, fmt.Errorf("%w: code_verifier does not match code_challenge", domain.ErrInvalidCodeChallenge)
	}

	// Get client for secret validation (if confidential)
	client, err := s.clientStore.Get(ctx, req.ClientID)
	if err != nil {
		return nil, fmt.Errorf("%w: client not found", domain.ErrInvalidClient)
	}

	// Validate client_secret for confidential clients
	if !client.IsPublic() {
		if req.ClientSecret == "" {
			return nil, fmt.Errorf("%w: client_secret required", domain.ErrInvalidClient)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(client.SecretHash), []byte(req.ClientSecret)); err != nil {
			return nil, fmt.Errorf("%w: invalid client_secret", domain.ErrInvalidClient)
		}
	}

	// Use resource from request, or fall back to code's resource
	audience := req.Resource
	if audience == "" {
		audience = authCode.Resource
	}
	if audience == "" {
		audience = s.mcpServerURL
	}

	// Generate access token (JWT)
	accessTokenID := uuid.New().String()
	expiresAt := time.Now().Add(domain.AccessTokenTTL)
	accessTokenJWT, err := s.generateAccessToken(accessTokenID, authCode.UserID, req.ClientID, authCode.Scopes, audience, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	// Save access token to store
	accessToken := &domain.OAuthAccessToken{
		ID:        accessTokenID,
		ClientID:  req.ClientID,
		UserID:    authCode.UserID,
		Scopes:    authCode.Scopes,
		Audience:  audience,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
		Revoked:   false,
	}
	if err := s.tokenStore.SaveAccessToken(ctx, accessToken); err != nil {
		return nil, fmt.Errorf("save access token: %w", err)
	}

	// Generate refresh token
	refreshTokenID, err := generateRandomToken(32)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	refreshToken := &domain.OAuthRefreshToken{
		ID:            refreshTokenID,
		AccessTokenID: accessTokenID,
		ClientID:      req.ClientID,
		UserID:        authCode.UserID,
		Scopes:        authCode.Scopes,
		Audience:      audience,
		ExpiresAt:     time.Now().Add(domain.RefreshTokenTTL),
		CreatedAt:     time.Now(),
		Revoked:       false,
		RotatedTo:     "",
	}
	if err := s.tokenStore.SaveRefreshToken(ctx, refreshToken); err != nil {
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	return &domain.TokenResponse{
		AccessToken:  accessTokenJWT,
		TokenType:    "Bearer",
		ExpiresIn:    int64(domain.AccessTokenTTL.Seconds()),
		RefreshToken: refreshTokenID,
		Scope:        strings.Join(authCode.Scopes, " "),
	}, nil
}

// handleRefreshTokenGrant handles the refresh_token grant type
func (s *oauthServerService) handleRefreshTokenGrant(ctx context.Context, req domain.TokenRequest) (*domain.TokenResponse, error) {
	// Look up refresh token
	refreshToken, err := s.tokenStore.GetRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("%w: refresh token not found", domain.ErrInvalidGrant)
	}

	// Validate token is valid (not expired, not revoked, not rotated)
	if !refreshToken.IsValid() {
		return nil, fmt.Errorf("%w: refresh token invalid", domain.ErrInvalidGrant)
	}

	// Validate client_id matches
	if refreshToken.ClientID != req.ClientID {
		return nil, fmt.Errorf("%w: client_id mismatch", domain.ErrInvalidClient)
	}

	// Get client for secret validation (if confidential)
	client, err := s.clientStore.Get(ctx, req.ClientID)
	if err != nil {
		return nil, fmt.Errorf("%w: client not found", domain.ErrInvalidClient)
	}

	// Validate client_secret for confidential clients
	if !client.IsPublic() {
		if req.ClientSecret == "" {
			return nil, fmt.Errorf("%w: client_secret required", domain.ErrInvalidClient)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(client.SecretHash), []byte(req.ClientSecret)); err != nil {
			return nil, fmt.Errorf("%w: invalid client_secret", domain.ErrInvalidClient)
		}
	}

	// Generate new access token
	newAccessTokenID := uuid.New().String()
	expiresAt := time.Now().Add(domain.AccessTokenTTL)
	accessTokenJWT, err := s.generateAccessToken(newAccessTokenID, refreshToken.UserID, req.ClientID, refreshToken.Scopes, refreshToken.Audience, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	// Save new access token
	accessToken := &domain.OAuthAccessToken{
		ID:        newAccessTokenID,
		ClientID:  req.ClientID,
		UserID:    refreshToken.UserID,
		Scopes:    refreshToken.Scopes,
		Audience:  refreshToken.Audience,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
		Revoked:   false,
	}
	if err := s.tokenStore.SaveAccessToken(ctx, accessToken); err != nil {
		return nil, fmt.Errorf("save access token: %w", err)
	}

	// Generate new refresh token
	newRefreshTokenID, err := generateRandomToken(32)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	newRefreshToken := &domain.OAuthRefreshToken{
		ID:            newRefreshTokenID,
		AccessTokenID: newAccessTokenID,
		ClientID:      req.ClientID,
		UserID:        refreshToken.UserID,
		Scopes:        refreshToken.Scopes,
		Audience:      refreshToken.Audience,
		ExpiresAt:     time.Now().Add(domain.RefreshTokenTTL),
		CreatedAt:     time.Now(),
		Revoked:       false,
		RotatedTo:     "",
	}
	if err := s.tokenStore.SaveRefreshToken(ctx, newRefreshToken); err != nil {
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	// Rotate old refresh token to new one
	if err := s.tokenStore.RotateRefreshToken(ctx, req.RefreshToken, newRefreshTokenID); err != nil {
		return nil, fmt.Errorf("rotate refresh token: %w", err)
	}

	return &domain.TokenResponse{
		AccessToken:  accessTokenJWT,
		TokenType:    "Bearer",
		ExpiresIn:    int64(domain.AccessTokenTTL.Seconds()),
		RefreshToken: newRefreshTokenID,
		Scope:        strings.Join(refreshToken.Scopes, " "),
	}, nil
}

// Revoke revokes an access or refresh token
func (s *oauthServerService) Revoke(ctx context.Context, req domain.RevokeRequest) error {
	// Try to parse as JWT to get jti (access token)
	token, err := s.parseAccessToken(req.Token)
	if err == nil {
		// It's a JWT access token
		jti, ok := token.Claims.(jwt.MapClaims)["jti"].(string)
		if ok {
			_ = s.tokenStore.RevokeAccessToken(ctx, jti)
		}
		return nil // Always return success per RFC 7009
	}

	// Otherwise try as refresh token ID
	_ = s.tokenStore.RevokeRefreshToken(ctx, req.Token)

	// Always return success per RFC 7009, even if token not found
	return nil
}

// ValidateAccessToken validates a bearer token and returns token info
func (s *oauthServerService) ValidateAccessToken(ctx context.Context, tokenString string) (*driving.OAuthTokenInfo, error) {
	// Parse JWT
	token, err := s.parseAccessToken(tokenString)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrTokenInvalid, err)
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, domain.ErrTokenInvalid
	}

	// Extract jti (token ID)
	jti, ok := claims["jti"].(string)
	if !ok {
		return nil, fmt.Errorf("%w: missing jti claim", domain.ErrTokenInvalid)
	}

	// Look up token in store
	storedToken, err := s.tokenStore.GetAccessToken(ctx, jti)
	if err != nil {
		return nil, fmt.Errorf("%w: token not found in store", domain.ErrTokenInvalid)
	}

	// Validate not revoked
	if storedToken.Revoked {
		return nil, domain.ErrTokenRevoked
	}

	// Validate not expired
	if storedToken.IsExpired() {
		return nil, domain.ErrTokenExpired
	}

	// Extract sub (user ID)
	sub, ok := claims["sub"].(string)
	if !ok {
		return nil, fmt.Errorf("%w: missing sub claim", domain.ErrTokenInvalid)
	}

	// Extract client_id
	clientID, ok := claims["client_id"].(string)
	if !ok {
		return nil, fmt.Errorf("%w: missing client_id claim", domain.ErrTokenInvalid)
	}

	// Extract scope
	scopeStr, _ := claims["scope"].(string)
	scopes := strings.Fields(scopeStr)

	// Extract aud (audience) — jwt/v5 parses "aud" as []string
	var aud string
	switch v := claims["aud"].(type) {
	case string:
		aud = v
	case []interface{}:
		if len(v) > 0 {
			aud, _ = v[0].(string)
		}
	}

	// Validate audience matches MCP server URL
	if aud != s.mcpServerURL {
		return nil, fmt.Errorf("%w: audience mismatch", domain.ErrTokenInvalid)
	}

	return &driving.OAuthTokenInfo{
		UserID:   sub,
		ClientID: clientID,
		Scopes:   scopes,
		Audience: aud,
	}, nil
}

// GetServerMetadata returns OAuth Authorization Server Metadata (RFC 8414)
func (s *oauthServerService) GetServerMetadata(baseURL string) *driving.OAuthServerMetadata {
	return &driving.OAuthServerMetadata{
		Issuer:                baseURL,
		AuthorizationEndpoint: baseURL + "/oauth/authorize",
		TokenEndpoint:         baseURL + "/oauth/token",
		RegistrationEndpoint:  baseURL + "/oauth/register",
		RevocationEndpoint:    baseURL + "/oauth/revoke",
		ResponseTypesSupported: []string{
			"code",
		},
		GrantTypesSupported: []string{
			string(domain.GrantTypeAuthorizationCode),
			string(domain.GrantTypeRefreshToken),
		},
		CodeChallengeMethodsSupported: []string{
			"S256",
		},
		TokenEndpointAuthMethodsSupported: []string{
			"none",
			"client_secret_post",
		},
		ScopesSupported: []string{
			domain.ScopeMCPSearch,
			domain.ScopeMCPDocRead,
			domain.ScopeMCPSourcesList,
		},
		ResourceIndicatorsSupported: true,
	}
}

// GetClientPublicInfo returns non-sensitive public info about an OAuth client
func (s *oauthServerService) GetClientPublicInfo(ctx context.Context, clientID string) (*driving.ClientPublicInfo, error) {
	client, err := s.clientStore.Get(ctx, clientID)
	if err != nil {
		return nil, err
	}

	// Treat inactive clients as not found
	if !client.Active {
		return nil, domain.ErrNotFound
	}

	return &driving.ClientPublicInfo{
		ClientID:        client.ID,
		Name:            client.Name,
		ApplicationType: client.ApplicationType,
	}, nil
}

// Helper functions

// generateAccessToken creates a signed JWT access token
func (s *oauthServerService) generateAccessToken(jti, userID, clientID string, scopes []string, audience string, expiresAt time.Time) (string, error) {
	claims := jwt.MapClaims{
		"jti":       jti,
		"sub":       userID,
		"client_id": clientID,
		"scope":     strings.Join(scopes, " "),
		"aud":       audience,
		"iat":       time.Now().Unix(),
		"exp":       expiresAt.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

// parseAccessToken parses and validates a JWT access token
func (s *oauthServerService) parseAccessToken(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})
}

// validatePKCE validates that SHA256(verifier) == challenge
func validatePKCE(verifier, challenge string) bool {
	if verifier == "" || challenge == "" {
		return false
	}

	// Compute SHA256 hash of verifier
	hash := sha256.Sum256([]byte(verifier))

	// Base64url encode the hash
	computed := base64.RawURLEncoding.EncodeToString(hash[:])

	return computed == challenge
}

// generateRandomBytes generates cryptographically secure random bytes
func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// generateRandomToken generates a cryptographically secure random token
func generateRandomToken(n int) (string, error) {
	b, err := generateRandomBytes(n)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
