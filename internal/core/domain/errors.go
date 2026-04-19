package domain

import "errors"

// Domain errors - used across all layers
var (
	// ErrNotFound indicates the requested resource was not found
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists indicates the resource already exists
	ErrAlreadyExists = errors.New("already exists")

	// ErrInvalidInput indicates the input is invalid
	ErrInvalidInput = errors.New("invalid input")

	// ErrUnauthorized indicates authentication failed or missing
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden indicates the user lacks permission for this action
	ErrForbidden = errors.New("forbidden")

	// ErrSyncInProgress indicates a sync is already running
	ErrSyncInProgress = errors.New("sync already in progress")

	// ErrConnectorNotFound indicates the connector type is not registered
	ErrConnectorNotFound = errors.New("connector not found")

	// ErrTokenExpired indicates the auth token has expired
	ErrTokenExpired = errors.New("token expired")

	// ErrTokenInvalid indicates the auth token is malformed or invalid
	ErrTokenInvalid = errors.New("token invalid")

	// ErrSessionNotFound indicates the session does not exist
	ErrSessionNotFound = errors.New("session not found")

	// ErrInvalidCredentials indicates wrong email/password combination
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrInvalidProvider indicates an unknown AI provider was specified
	ErrInvalidProvider = errors.New("invalid provider")

	// ErrServiceUnavailable indicates the AI service could not be reached
	ErrServiceUnavailable = errors.New("service unavailable")

	// ErrUnsupportedProvider indicates the connector provider type is not supported
	ErrUnsupportedProvider = errors.New("unsupported provider")

	// ErrUnsupportedAuthMethod indicates the authentication method is not supported
	ErrUnsupportedAuthMethod = errors.New("unsupported auth method")

	// ErrConnectionNotFound indicates the connector connection was not found
	ErrConnectionNotFound = errors.New("connection not found")

	// ErrInUse indicates the resource is in use and cannot be deleted
	ErrInUse = errors.New("resource in use")

	// OAuth 2.0 Authorization Server errors

	// ErrInvalidClient indicates the client authentication failed
	ErrInvalidClient = errors.New("invalid client")

	// ErrInvalidGrant indicates the authorization grant is invalid, expired, or revoked
	ErrInvalidGrant = errors.New("invalid grant")

	// ErrInvalidScope indicates the requested scope is invalid or unauthorized
	ErrInvalidScope = errors.New("invalid scope")

	// ErrExpiredCode indicates the authorization code has expired
	ErrExpiredCode = errors.New("authorization code expired")

	// ErrCodeAlreadyUsed indicates the authorization code has already been used
	ErrCodeAlreadyUsed = errors.New("authorization code already used")

	// ErrInvalidRedirectURI indicates the redirect URI does not match registered URIs
	ErrInvalidRedirectURI = errors.New("invalid redirect uri")

	// ErrUnsupportedGrantType indicates the grant type is not supported
	ErrUnsupportedGrantType = errors.New("unsupported grant type")

	// ErrUnsupportedResponseType indicates the response type is not supported
	ErrUnsupportedResponseType = errors.New("unsupported response type")

	// ErrInvalidCodeChallenge indicates the PKCE code challenge is invalid
	ErrInvalidCodeChallenge = errors.New("invalid code challenge")

	// ErrTokenRevoked indicates the token has been revoked
	ErrTokenRevoked = errors.New("token revoked")

	// LLM service errors

	// ErrRateLimitExceeded indicates the LLM service rate limit was exceeded
	ErrRateLimitExceeded = errors.New("rate limit exceeded")

	// ErrContextLengthExceeded indicates the input exceeds the LLM's context window
	ErrContextLengthExceeded = errors.New("context length exceeded")

	// ErrInvalidModel indicates the specified model is not available or supported
	ErrInvalidModel = errors.New("invalid model")
)
