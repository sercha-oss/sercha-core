package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driving"
)

// Ensure oauthService implements OAuthService
var _ driving.OAuthService = (*oauthService)(nil)

// OAuthServiceConfig holds configuration for the OAuth service.
type OAuthServiceConfig struct {
	// ConfigProvider retrieves OAuth app credentials from environment variables.
	ConfigProvider driven.ConfigProvider

	// OAuthStateStore manages OAuth flow state.
	OAuthStateStore driven.OAuthStateStore

	// InstallationStore persists connector installations.
	InstallationStore driven.InstallationStore

	// OAuthHandlerFactory provides OAuth handlers per provider.
	// Port interface - abstracts connector factory.
	OAuthHandlerFactory driven.OAuthHandlerFactory
}

// oauthService implements the OAuthService interface.
type oauthService struct {
	configProvider      driven.ConfigProvider
	oauthStateStore     driven.OAuthStateStore
	installationStore   driven.InstallationStore
	oauthHandlerFactory driven.OAuthHandlerFactory
}

// NewOAuthService creates a new OAuth service.
func NewOAuthService(cfg OAuthServiceConfig) driving.OAuthService {
	return &oauthService{
		configProvider:      cfg.ConfigProvider,
		oauthStateStore:     cfg.OAuthStateStore,
		installationStore:   cfg.InstallationStore,
		oauthHandlerFactory: cfg.OAuthHandlerFactory,
	}
}

// Authorize starts an OAuth authorization flow.
// It generates PKCE credentials, stores state, and returns the authorization URL.
func (s *oauthService) Authorize(ctx context.Context, req driving.AuthorizeRequest) (*driving.AuthorizeResponse, error) {
	// Check if provider is configured via environment variables
	if !s.configProvider.IsOAuthConfigured(req.ProviderType) {
		return nil, driving.ErrOAuthProviderNotFound
	}

	// Get OAuth credentials from config
	credentials := s.configProvider.GetOAuthCredentials(req.ProviderType)
	if credentials == nil {
		return nil, driving.ErrOAuthProviderNotFound
	}

	// Get OAuth handler for the provider
	oauthHandler := s.oauthHandlerFactory.GetOAuthHandler(req.ProviderType)
	if oauthHandler == nil {
		return nil, driving.ErrOAuthProviderNotFound
	}

	// Generate state (CSRF protection)
	state, err := generateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	// Generate PKCE code verifier and challenge
	codeVerifier, err := generateRandomString(64)
	if err != nil {
		return nil, fmt.Errorf("generate code verifier: %w", err)
	}
	codeChallenge := generateCodeChallenge(codeVerifier)

	// Build redirect URI
	redirectURI := s.configProvider.GetBaseURL() + "/api/v1/oauth/callback"

	// Get default scopes from handler
	defaults := oauthHandler.DefaultConfig()
	scopes := defaults.Scopes

	// Store state for validation during callback
	expiresAt := time.Now().Add(10 * time.Minute)
	oauthState := &driven.OAuthState{
		State:        state,
		ProviderType: string(req.ProviderType),
		CodeVerifier: codeVerifier,
		RedirectURI:  redirectURI,
		CreatedAt:    time.Now(),
		ExpiresAt:    expiresAt,
	}

	if err := s.oauthStateStore.Save(ctx, oauthState); err != nil {
		return nil, fmt.Errorf("save oauth state: %w", err)
	}

	// Build authorization URL
	authURL := oauthHandler.BuildAuthURL(
		credentials.ClientID,
		redirectURI,
		state,
		codeChallenge,
		scopes,
	)

	return &driving.AuthorizeResponse{
		AuthorizationURL: authURL,
		State:            state,
		ExpiresAt:        expiresAt.Format(time.RFC3339),
	}, nil
}

// Callback handles the OAuth callback from the provider.
// It validates state, exchanges the code for tokens, and creates an installation.
func (s *oauthService) Callback(ctx context.Context, req driving.CallbackRequest) (*driving.CallbackResponse, error) {
	// Check for error from provider
	if req.Error != "" {
		return nil, &driving.OAuthError{
			Code:        req.Error,
			Description: req.ErrorDescription,
		}
	}

	// Validate and consume state (single-use)
	oauthState, err := s.oauthStateStore.GetAndDelete(ctx, req.State)
	if err != nil {
		return nil, fmt.Errorf("get oauth state: %w", err)
	}
	if oauthState == nil {
		return nil, driving.ErrOAuthInvalidState
	}

	providerType := domain.ProviderType(oauthState.ProviderType)

	// Check if provider is still configured
	if !s.configProvider.IsOAuthConfigured(providerType) {
		return nil, driving.ErrOAuthProviderNotFound
	}

	// Get OAuth credentials from config
	credentials := s.configProvider.GetOAuthCredentials(providerType)
	if credentials == nil {
		return nil, driving.ErrOAuthProviderNotFound
	}

	// Get OAuth handler
	oauthHandler := s.oauthHandlerFactory.GetOAuthHandler(providerType)
	if oauthHandler == nil {
		return nil, driving.ErrOAuthProviderNotFound
	}

	// Exchange code for tokens
	token, err := oauthHandler.ExchangeCode(
		ctx,
		credentials.ClientID,
		credentials.ClientSecret,
		req.Code,
		oauthState.RedirectURI,
		oauthState.CodeVerifier,
	)
	if err != nil {
		return nil, &driving.OAuthError{
			Code:        "exchange_failed",
			Description: err.Error(),
		}
	}

	// Get user info to identify the account
	userInfo, err := oauthHandler.GetUserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, &driving.OAuthError{
			Code:        "user_info_failed",
			Description: err.Error(),
		}
	}

	// Check if installation already exists for this account
	existing, err := s.installationStore.GetByAccountID(ctx, providerType, userInfo.ID)
	if err != nil {
		return nil, fmt.Errorf("check existing installation: %w", err)
	}

	var installation *domain.Installation
	if existing != nil {
		// Update existing installation with new tokens
		var expiry *time.Time
		if token.ExpiresIn > 0 {
			t := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
			expiry = &t
		}

		err = s.installationStore.UpdateSecrets(ctx, existing.ID, &domain.InstallationSecrets{
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
		}, expiry)
		if err != nil {
			return nil, fmt.Errorf("update installation secrets: %w", err)
		}

		installation = existing
		installation.OAuthExpiry = expiry
	} else {
		// Create new installation
		installationID, err := generateInstallationID()
		if err != nil {
			return nil, fmt.Errorf("generate installation id: %w", err)
		}

		// Build installation name
		name := fmt.Sprintf("%s (%s)", providerDisplayName(providerType), userInfo.ID)
		if userInfo.Name != "" {
			name = fmt.Sprintf("%s (%s)", providerDisplayName(providerType), userInfo.Name)
		}

		var expiry *time.Time
		if token.ExpiresIn > 0 {
			t := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
			expiry = &t
		}

		installation = &domain.Installation{
			ID:             installationID,
			Name:           name,
			ProviderType:   providerType,
			AuthMethod:     domain.AuthMethodOAuth2,
			AccountID:      userInfo.ID,
			OAuthTokenType: token.TokenType,
			OAuthExpiry:    expiry,
			OAuthScopes:    splitScopes(token.Scope),
			Secrets: &domain.InstallationSecrets{
				AccessToken:  token.AccessToken,
				RefreshToken: token.RefreshToken,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := s.installationStore.Save(ctx, installation); err != nil {
			return nil, fmt.Errorf("save installation: %w", err)
		}
	}

	// Build response message
	accountDisplay := userInfo.ID
	if userInfo.Name != "" {
		accountDisplay = userInfo.Name
	}
	if userInfo.Email != "" && userInfo.Email != userInfo.ID {
		accountDisplay = userInfo.Email
	}

	return &driving.CallbackResponse{
		Installation: installation.ToSummary(),
		Message:      fmt.Sprintf("Successfully connected to %s as %s", providerDisplayName(providerType), accountDisplay),
	}, nil
}

// generateRandomString generates a cryptographically secure random string.
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:length], nil
}

// generateCodeChallenge creates a PKCE code challenge from a verifier (S256 method).
func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// generateInstallationID generates a unique installation ID.
func generateInstallationID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "inst_" + hex.EncodeToString(bytes), nil
}

// providerDisplayName returns a human-readable name for a provider.
func providerDisplayName(pt domain.ProviderType) string {
	switch pt {
	case domain.ProviderTypeGitHub:
		return "GitHub"
	case domain.ProviderTypeGitLab:
		return "GitLab"
	case domain.ProviderTypeSlack:
		return "Slack"
	case domain.ProviderTypeNotion:
		return "Notion"
	case domain.ProviderTypeConfluence:
		return "Confluence"
	case domain.ProviderTypeJira:
		return "Jira"
	case domain.ProviderTypeGoogleDrive:
		return "Google Drive"
	case domain.ProviderTypeGoogleDocs:
		return "Google Docs"
	case domain.ProviderTypeLinear:
		return "Linear"
	case domain.ProviderTypeDropbox:
		return "Dropbox"
	case domain.ProviderTypeS3:
		return "Amazon S3"
	default:
		return string(pt)
	}
}

// splitScopes splits a space-separated scope string into a slice.
func splitScopes(scope string) []string {
	if scope == "" {
		return nil
	}
	// Handle both space and comma separated scopes
	var scopes []string
	for _, s := range []byte(scope) {
		if s == ' ' || s == ',' {
			continue
		}
	}
	// Simple split by space
	var current string
	for _, c := range scope {
		if c == ' ' || c == ',' {
			if current != "" {
				scopes = append(scopes, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		scopes = append(scopes, current)
	}
	return scopes
}
