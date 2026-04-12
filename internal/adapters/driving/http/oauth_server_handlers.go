package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// OAuth 2.0 Authorization Server endpoints

// handleOAuthServerMetadata returns RFC 8414 OAuth Authorization Server Metadata
// GET /.well-known/oauth-authorization-server
func (s *Server) handleOAuthServerMetadata(w http.ResponseWriter, r *http.Request) {
	if s.oauthServerService == nil {
		writeError(w, http.StatusServiceUnavailable, "oauth server not configured")
		return
	}

	// Get base URL from request
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)

	metadata := s.oauthServerService.GetServerMetadata(baseURL)
	writeJSON(w, http.StatusOK, metadata)
}

// handleDynamicClientRegistration handles RFC 7591 Dynamic Client Registration
// POST /oauth/register
func (s *Server) handleDynamicClientRegistration(w http.ResponseWriter, r *http.Request) {
	if s.oauthServerService == nil {
		writeError(w, http.StatusServiceUnavailable, "oauth server not configured")
		return
	}

	var req domain.ClientRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := s.oauthServerService.RegisterClient(r.Context(), req)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "invalid"):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "failed to register client")
		}
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

// handleOAuthServerAuthorize handles the authorization endpoint
// GET /oauth/authorize - redirects to frontend for login + consent
func (s *Server) handleOAuthServerAuthorize(w http.ResponseWriter, r *http.Request) {
	if s.oauthServerService == nil {
		writeError(w, http.StatusServiceUnavailable, "oauth server not configured")
		return
	}

	// Parse authorization request from query params
	authReq := domain.AuthorizeRequest{
		ResponseType:        r.URL.Query().Get("response_type"),
		ClientID:            r.URL.Query().Get("client_id"),
		RedirectURI:         r.URL.Query().Get("redirect_uri"),
		Scope:               r.URL.Query().Get("scope"),
		State:               r.URL.Query().Get("state"),
		CodeChallenge:       r.URL.Query().Get("code_challenge"),
		CodeChallengeMethod: r.URL.Query().Get("code_challenge_method"),
		Resource:            r.URL.Query().Get("resource"),
	}

	// Validate required params before redirecting
	if authReq.ClientID == "" {
		writeError(w, http.StatusBadRequest, "client_id is required")
		return
	}
	if authReq.RedirectURI == "" {
		writeError(w, http.StatusBadRequest, "redirect_uri is required")
		return
	}

	// Redirect to frontend with all OAuth params preserved
	params := url.Values{}
	params.Set("response_type", authReq.ResponseType)
	params.Set("client_id", authReq.ClientID)
	params.Set("redirect_uri", authReq.RedirectURI)
	if authReq.Scope != "" {
		params.Set("scope", authReq.Scope)
	}
	if authReq.State != "" {
		params.Set("state", authReq.State)
	}
	if authReq.CodeChallenge != "" {
		params.Set("code_challenge", authReq.CodeChallenge)
	}
	if authReq.CodeChallengeMethod != "" {
		params.Set("code_challenge_method", authReq.CodeChallengeMethod)
	}
	if authReq.Resource != "" {
		params.Set("resource", authReq.Resource)
	}

	frontendURL := fmt.Sprintf("%s/oauth/authorize?%s", s.uiBaseURL, params.Encode())
	http.Redirect(w, r, frontendURL, http.StatusFound)
}

// handleOAuthServerAuthorizeComplete handles authorization completion from the frontend
// POST /oauth/authorize/complete - called by frontend with JWT after user login + consent
// Returns JSON with redirect_url for the frontend to navigate to
func (s *Server) handleOAuthServerAuthorizeComplete(w http.ResponseWriter, r *http.Request) {
	if s.oauthServerService == nil {
		writeError(w, http.StatusServiceUnavailable, "oauth server not configured")
		return
	}

	// Parse authorization parameters from JSON body
	var authReq domain.AuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&authReq); err != nil {
		writeOAuthError(w, "invalid_request", "invalid request body")
		return
	}

	// Validate redirect_uri
	if authReq.RedirectURI == "" {
		writeOAuthError(w, "invalid_request", "redirect_uri is required")
		return
	}

	// Get authenticated user from context (middleware ensures this exists)
	authCtx := GetAuthContext(r.Context())
	if authCtx == nil {
		writeError(w, http.StatusUnauthorized, "missing authorization token")
		return
	}

	// Process authorization request
	code, err := s.oauthServerService.Authorize(r.Context(), authCtx.UserID, authReq)
	if err != nil {
		errorCode := "server_error"
		errorDesc := err.Error()

		switch {
		case strings.Contains(err.Error(), "response_type"):
			errorCode = "unsupported_response_type"
		case strings.Contains(err.Error(), "client"):
			errorCode = "unauthorized_client"
		case strings.Contains(err.Error(), "redirect_uri"):
			errorCode = "invalid_request"
		case strings.Contains(err.Error(), "scope"):
			errorCode = "invalid_scope"
		case strings.Contains(err.Error(), "code_challenge"):
			errorCode = "invalid_request"
		}

		writeOAuthError(w, errorCode, errorDesc)
		return
	}

	// Build the redirect URL with code and state
	redirectURL, err := url.Parse(authReq.RedirectURI)
	if err != nil {
		writeOAuthError(w, "server_error", "invalid redirect_uri")
		return
	}

	query := redirectURL.Query()
	query.Set("code", code)
	if authReq.State != "" {
		query.Set("state", authReq.State)
	}
	redirectURL.RawQuery = query.Encode()

	// Return JSON for the frontend to redirect
	writeJSON(w, http.StatusOK, map[string]string{
		"redirect_url": redirectURL.String(),
	})
}

// handleOAuthServerToken handles the token endpoint
// POST /oauth/token - exchanges code/refresh_token for tokens
func (s *Server) handleOAuthServerToken(w http.ResponseWriter, r *http.Request) {
	if s.oauthServerService == nil {
		writeError(w, http.StatusServiceUnavailable, "oauth server not configured")
		return
	}

	// Parse token request from form data (standard OAuth) or JSON
	var tokenReq domain.TokenRequest
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&tokenReq); err != nil {
			writeOAuthError(w, "invalid_request", "invalid request body")
			return
		}
	} else {
		// Parse as application/x-www-form-urlencoded
		if err := r.ParseForm(); err != nil {
			writeOAuthError(w, "invalid_request", "invalid form data")
			return
		}

		tokenReq = domain.TokenRequest{
			GrantType:    domain.OAuthGrantType(r.FormValue("grant_type")),
			Code:         r.FormValue("code"),
			RedirectURI:  r.FormValue("redirect_uri"),
			ClientID:     r.FormValue("client_id"),
			ClientSecret: r.FormValue("client_secret"),
			CodeVerifier: r.FormValue("code_verifier"),
			RefreshToken: r.FormValue("refresh_token"),
			Resource:     r.FormValue("resource"),
		}
	}

	// Process token request
	resp, err := s.oauthServerService.Token(r.Context(), tokenReq)
	if err != nil {
		// Determine OAuth error code based on error type
		errorCode := "server_error"

		switch {
		case strings.Contains(err.Error(), "unsupported_grant_type"):
			errorCode = "unsupported_grant_type"
		case strings.Contains(err.Error(), "invalid_grant"), strings.Contains(err.Error(), "code"):
			errorCode = "invalid_grant"
		case strings.Contains(err.Error(), "invalid_client"), strings.Contains(err.Error(), "client"):
			errorCode = "invalid_client"
		case strings.Contains(err.Error(), "invalid_scope"):
			errorCode = "invalid_scope"
		case strings.Contains(err.Error(), "redirect_uri"):
			errorCode = "invalid_request"
		}

		writeOAuthError(w, errorCode, err.Error())
		return
	}

	// Success - return tokens with cache headers
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, resp)
}

// handleOAuthServerRevoke handles token revocation
// POST /oauth/revoke - revokes access or refresh token
func (s *Server) handleOAuthServerRevoke(w http.ResponseWriter, r *http.Request) {
	if s.oauthServerService == nil {
		writeError(w, http.StatusServiceUnavailable, "oauth server not configured")
		return
	}

	// Parse revoke request from form data or JSON
	var revokeReq domain.RevokeRequest
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&revokeReq); err != nil {
			// Per RFC 7009, even on error we return 200 OK
			w.WriteHeader(http.StatusOK)
			return
		}
	} else {
		// Parse as application/x-www-form-urlencoded
		if err := r.ParseForm(); err != nil {
			// Per RFC 7009, even on error we return 200 OK
			w.WriteHeader(http.StatusOK)
			return
		}

		revokeReq = domain.RevokeRequest{
			Token:         r.FormValue("token"),
			TokenTypeHint: r.FormValue("token_type_hint"),
		}
	}

	// Process revoke request (always returns success per RFC 7009)
	_ = s.oauthServerService.Revoke(r.Context(), revokeReq)

	// Always return 200 OK per RFC 7009, even if token doesn't exist
	w.WriteHeader(http.StatusOK)
}

// handleProtectedResourceMetadata returns RFC 9728 Protected Resource Metadata
// GET /.well-known/oauth-protected-resource (and /mcp/.well-known/oauth-protected-resource)
func (s *Server) handleProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)
	mcpServerURL := baseURL + "/mcp" // MCP endpoint path

	metadata := map[string]interface{}{
		"resource":                 mcpServerURL,
		"authorization_servers":    []string{baseURL},
		"scopes_supported":         []string{domain.ScopeMCPSearch, domain.ScopeMCPDocRead, domain.ScopeMCPSourcesList},
		"bearer_methods_supported": []string{"header"},
	}

	writeJSON(w, http.StatusOK, metadata)
}

// Helper functions

// writeOAuthError writes an OAuth 2.0 error response
func writeOAuthError(w http.ResponseWriter, errorCode, description string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	resp := map[string]string{
		"error": errorCode,
	}
	if description != "" {
		resp["error_description"] = description
	}

	writeJSON(w, http.StatusBadRequest, resp)
}
