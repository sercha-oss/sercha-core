package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driving"
)

// ErrorResponse represents an API error response
// @Description API error response
type ErrorResponse struct {
	Error string `json:"error" example:"invalid request body"`
}

// StatusResponse represents a simple status response
// @Description Simple status response
type StatusResponse struct {
	Status string `json:"status" example:"ok"`
}

// VersionResponse represents the API version response
// @Description API version response
type VersionResponse struct {
	Version string `json:"version" example:"1.0.0"`
}

// Health endpoints

// HealthResponse represents the health check response with component status
type HealthResponse struct {
	Status     string                    `json:"status"`               // overall status: "healthy" or "degraded"
	Components map[string]ComponentHealth `json:"components,omitempty"` // individual component health
}

// ComponentHealth represents health status of a single component
type ComponentHealth struct {
	Status  string `json:"status"`            // "healthy" or "unhealthy"
	Message string `json:"message,omitempty"` // optional message for unhealthy components
}

// handleHealth godoc
// @Summary      Health check
// @Description  Returns 200 if the service is up, with status of each dependency in the body
// @Tags         Health
// @Produce      json
// @Success      200  {object}  HealthResponse  "Service is up with dependency status"
// @Router       /health [get]
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	components := make(map[string]ComponentHealth)
	allHealthy := true

	// Check PostgreSQL health
	if s.db != nil {
		if err := s.db.Ping(r.Context()); err != nil {
			components["postgres"] = ComponentHealth{
				Status:  "unhealthy",
				Message: err.Error(),
			}
			allHealthy = false
		} else {
			components["postgres"] = ComponentHealth{Status: "healthy"}
		}
	}

	// Check Vespa health
	if s.vespaAdminService != nil {
		if err := s.vespaAdminService.HealthCheck(r.Context()); err != nil {
			components["vespa"] = ComponentHealth{
				Status:  "unhealthy",
				Message: err.Error(),
			}
			allHealthy = false
		} else {
			components["vespa"] = ComponentHealth{Status: "healthy"}
		}
	}

	// Check Redis health (optional)
	if s.redisClient != nil {
		if err := s.redisClient.Ping(r.Context()); err != nil {
			components["redis"] = ComponentHealth{
				Status:  "unhealthy",
				Message: err.Error(),
			}
			allHealthy = false
		} else {
			components["redis"] = ComponentHealth{Status: "healthy"}
		}
	}

	// Server is always healthy if it's responding
	components["server"] = ComponentHealth{Status: "healthy"}

	resp := HealthResponse{
		Status:     "healthy",
		Components: components,
	}

	if !allHealthy {
		resp.Status = "degraded"
	}

	// Always return 200 - service is up and can respond
	writeJSON(w, http.StatusOK, resp)
}

// handleReady godoc
// @Summary      Readiness check
// @Description  Returns the readiness status of the API (checks database and service connections)
// @Tags         Health
// @Produce      json
// @Success      200  {object}  StatusResponse
// @Router       /ready [get]
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	// TODO: Check database and service connections
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// handleVersion godoc
// @Summary      Get API version
// @Description  Returns the current API version
// @Tags         Health
// @Produce      json
// @Success      200  {object}  VersionResponse
// @Router       /version [get]
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": s.version})
}

// handleGetCapabilities godoc
// @Summary      Get capabilities
// @Description  Returns information about what features are available based on environment configuration
// @Tags         Capabilities
// @Produce      json
// @Success      200  {object}  driving.CapabilitiesResponse
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /capabilities [get]
func (s *Server) handleGetCapabilities(w http.ResponseWriter, r *http.Request) {
	if s.capabilitiesService == nil {
		writeError(w, http.StatusServiceUnavailable, "capabilities service not available")
		return
	}

	caps, err := s.capabilitiesService.GetCapabilities(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get capabilities")
		return
	}

	writeJSON(w, http.StatusOK, caps)
}

// Auth endpoints

// handleLogin godoc
// @Summary      User login
// @Description  Authenticate with email and password to receive a JWT token
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      domain.LoginRequest  true  "Login credentials"
// @Success      200      {object}  domain.LoginResponse
// @Failure      400      {object}  ErrorResponse  "Invalid request body"
// @Failure      401      {object}  ErrorResponse  "Invalid credentials or account disabled"
// @Failure      500      {object}  ErrorResponse  "Internal server error"
// @Router       /auth/login [post]
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req domain.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := s.authService.Authenticate(r.Context(), req)
	if err != nil {
		switch err {
		case domain.ErrInvalidCredentials:
			writeError(w, http.StatusUnauthorized, "invalid credentials")
		case domain.ErrUnauthorized:
			writeError(w, http.StatusUnauthorized, "account disabled")
		default:
			writeError(w, http.StatusInternalServerError, "authentication failed")
		}
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleRefresh godoc
// @Summary      Refresh token
// @Description  Exchange a refresh token for a new JWT token
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      domain.RefreshRequest  true  "Refresh token"
// @Success      200      {object}  domain.LoginResponse
// @Failure      400      {object}  ErrorResponse  "Invalid request body"
// @Failure      401      {object}  ErrorResponse  "Invalid refresh token"
// @Router       /auth/refresh [post]
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req domain.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := s.authService.RefreshToken(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleLogout godoc
// @Summary      Logout user
// @Description  Invalidate the current session token
// @Tags         Authentication
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  StatusResponse
// @Router       /auth/logout [post]
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := extractBearerToken(r)
	if token == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	_ = s.authService.Logout(r.Context(), token)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Setup endpoint (no auth required, one-time use)

// handleSetup godoc
// @Summary      Initial setup
// @Description  Create the initial admin user. This endpoint can only be called once when no users exist.
// @Tags         Setup
// @Accept       json
// @Produce      json
// @Param        request  body      driving.SetupRequest  true  "Admin user details"
// @Success      201      {object}  driving.SetupResponse
// @Failure      400      {object}  ErrorResponse  "Invalid input"
// @Failure      403      {object}  ErrorResponse  "Setup already complete"
// @Failure      500      {object}  ErrorResponse  "Setup failed"
// @Router       /setup [post]
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req driving.SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := s.userService.Setup(r.Context(), req)
	if err != nil {
		switch err {
		case domain.ErrInvalidInput:
			writeError(w, http.StatusBadRequest, "email, password, and name are required")
		case domain.ErrForbidden:
			writeError(w, http.StatusForbidden, "setup already complete")
		default:
			writeError(w, http.StatusInternalServerError, "setup failed")
		}
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

// handleSetupStatus godoc
// @Summary      Get setup status
// @Description  Returns the current setup state for FTUE (First-Time User Experience) flow. This endpoint is public and does not require authentication.
// @Tags         Setup
// @Produce      json
// @Success      200  {object}  driving.SetupStatusResponse
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /setup/status [get]
func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	if s.setupService == nil {
		writeError(w, http.StatusServiceUnavailable, "setup service not available")
		return
	}

	status, err := s.setupService.GetStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get setup status")
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// User endpoints

// handleGetMe godoc
// @Summary      Get current user
// @Description  Get the currently authenticated user's profile
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  domain.UserSummary
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      404  {object}  ErrorResponse  "User not found"
// @Router       /me [get]
func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	authCtx := GetAuthContext(r.Context())
	if authCtx == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := s.userService.Get(r.Context(), authCtx.UserID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, user.ToSummary())
}

// handleListUsers godoc
// @Summary      List all users
// @Description  Get a list of all users (admin only)
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   domain.UserSummary
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /users [get]
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.userService.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	summaries := make([]*domain.UserSummary, len(users))
	for i, u := range users {
		summaries[i] = u.ToSummary()
	}

	writeJSON(w, http.StatusOK, summaries)
}

// handleCreateUser godoc
// @Summary      Create user
// @Description  Create a new user (admin only)
// @Tags         Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      driving.CreateUserRequest  true  "User details"
// @Success      201      {object}  domain.UserSummary
// @Failure      400      {object}  ErrorResponse  "Invalid input"
// @Failure      401      {object}  ErrorResponse  "Unauthorized"
// @Failure      403      {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      409      {object}  ErrorResponse  "User already exists"
// @Failure      500      {object}  ErrorResponse  "Internal server error"
// @Router       /users [post]
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req driving.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.userService.Create(r.Context(), req)
	if err != nil {
		switch err {
		case domain.ErrAlreadyExists:
			writeError(w, http.StatusConflict, "user already exists")
		case domain.ErrInvalidInput:
			writeError(w, http.StatusBadRequest, "invalid input")
		default:
			writeError(w, http.StatusInternalServerError, "failed to create user")
		}
		return
	}

	writeJSON(w, http.StatusCreated, user.ToSummary())
}

// handleGetUser godoc
// @Summary      Get user
// @Description  Get a user by ID (admin only)
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "User ID"
// @Success      200  {object}  domain.UserSummary
// @Failure      400  {object}  ErrorResponse  "Missing user ID"
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      404  {object}  ErrorResponse  "User not found"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /users/{id} [get]
func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing user id")
		return
	}

	user, err := s.userService.Get(r.Context(), id)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "user not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to get user")
		}
		return
	}

	writeJSON(w, http.StatusOK, user.ToSummary())
}

// handleUpdateUser godoc
// @Summary      Update user
// @Description  Update a user's details (admin only)
// @Tags         Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      string                      true  "User ID"
// @Param        request  body      driving.UpdateUserRequest   true  "User update request"
// @Success      200      {object}  domain.UserSummary
// @Failure      400      {object}  ErrorResponse  "Invalid request body"
// @Failure      401      {object}  ErrorResponse  "Unauthorized"
// @Failure      403      {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      404      {object}  ErrorResponse  "User not found"
// @Failure      500      {object}  ErrorResponse  "Internal server error"
// @Router       /users/{id} [put]
func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing user id")
		return
	}

	var req driving.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.userService.Update(r.Context(), id, req)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "user not found")
		case domain.ErrInvalidInput:
			writeError(w, http.StatusBadRequest, "invalid input")
		default:
			writeError(w, http.StatusInternalServerError, "failed to update user")
		}
		return
	}

	writeJSON(w, http.StatusOK, user.ToSummary())
}

// ResetPasswordRequest represents a password reset request
// @Description Password reset request
type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// handleResetUserPassword godoc
// @Summary      Reset user password
// @Description  Reset a user's password (admin only). This invalidates all existing sessions for the user.
// @Tags         Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      string                  true  "User ID"
// @Param        request  body      ResetPasswordRequest    true  "New password"
// @Success      200      {object}  StatusResponse
// @Failure      400      {object}  ErrorResponse  "Invalid request or missing password"
// @Failure      401      {object}  ErrorResponse  "Unauthorized"
// @Failure      403      {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      404      {object}  ErrorResponse  "User not found"
// @Failure      500      {object}  ErrorResponse  "Internal server error"
// @Router       /users/{id}/reset-password [post]
func (s *Server) handleResetUserPassword(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing user id")
		return
	}

	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}

	if err := s.userService.SetPassword(r.Context(), id, req.Password); err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "user not found")
		case domain.ErrInvalidInput:
			writeError(w, http.StatusBadRequest, "invalid password")
		default:
			writeError(w, http.StatusInternalServerError, "failed to reset password")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password reset successfully"})
}

// handleDeleteUser godoc
// @Summary      Delete user
// @Description  Delete a user by ID (admin only)
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "User ID"
// @Success      200  {object}  StatusResponse
// @Failure      400  {object}  ErrorResponse  "Missing user ID"
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      404  {object}  ErrorResponse  "User not found"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /users/{id} [delete]
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing user id")
		return
	}

	if err := s.userService.Delete(r.Context(), id); err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "user not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to delete user")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Search endpoints

// SearchRequest represents a search query request
// @Description Search query request
type searchRequest struct {
	Query     string            `json:"query" example:"how to configure authentication"`
	Mode      domain.SearchMode `json:"mode,omitempty" example:"hybrid" enums:"hybrid,text,semantic"`
	Limit     int               `json:"limit,omitempty" example:"20"`
	Offset    int               `json:"offset,omitempty" example:"0"`
	SourceIDs []string          `json:"source_ids,omitempty"`
}

// handleSearch godoc
// @Summary      Search documents
// @Description  Execute a search query across all indexed documents. Supports hybrid (BM25 + semantic), text-only, and semantic-only modes.
// @Tags         Search
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      searchRequest  true  "Search query"
// @Success      200      {object}  domain.SearchResult
// @Failure      400      {object}  ErrorResponse  "Invalid request or missing query"
// @Failure      401      {object}  ErrorResponse  "Unauthorized"
// @Failure      500      {object}  ErrorResponse  "Search failed"
// @Router       /search [post]
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	opts := domain.SearchOptions{
		Mode:      req.Mode,
		Limit:     req.Limit,
		Offset:    req.Offset,
		SourceIDs: req.SourceIDs,
	}

	result, err := s.searchService.Search(r.Context(), req.Query, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

	// Track search query for analytics (best effort, don't fail request if tracking fails)
	if s.searchQueryRepo != nil {
		authCtx := GetAuthContext(r.Context())
		if authCtx != nil {
			searchQuery := domain.NewSearchQuery(
				authCtx.TeamID,
				authCtx.UserID,
				req.Query,
				result.Mode,
				result.TotalCount,
				result.Took,
			)
			if len(req.SourceIDs) > 0 {
				searchQuery.WithSourceFilters(req.SourceIDs)
			}
			// Log asynchronously to not slow down the search response
			go func() {
				_ = s.searchQueryRepo.Save(context.Background(), searchQuery)
			}()
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// Document endpoints

// handleGetDocument godoc
// @Summary      Get document
// @Description  Get a document by ID with all its chunks
// @Tags         Documents
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Document ID"
// @Success      200  {object}  domain.DocumentWithChunks
// @Failure      400  {object}  ErrorResponse  "Missing document ID"
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      404  {object}  ErrorResponse  "Document not found"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /documents/{id} [get]
func (s *Server) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing document id")
		return
	}

	doc, err := s.docService.GetWithChunks(r.Context(), id)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "document not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to get document")
		}
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

// DocumentURLResponse represents the response containing a document's external URL
// @Description Document URL response
type DocumentURLResponse struct {
	URL string `json:"url" example:"https://github.com/owner/repo/blob/main/README.md"`
}

// handleOpenDocument godoc
// @Summary      Open document
// @Description  Get the external URL for a document. This returns the original URL in the source system (e.g., GitHub, Notion, etc.).
// @Tags         Documents
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Document ID"
// @Success      200  {object}  DocumentURLResponse
// @Failure      400  {object}  ErrorResponse  "Missing document ID"
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      404  {object}  ErrorResponse  "Document not found"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /documents/{id}/open [get]
func (s *Server) handleOpenDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing document id")
		return
	}

	doc, err := s.docService.Get(r.Context(), id)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "document not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to get document")
		}
		return
	}

	writeJSON(w, http.StatusOK, DocumentURLResponse{URL: doc.Path})
}

// Source endpoints

// handleListSources godoc
// @Summary      List sources
// @Description  Get a list of all configured data sources with sync status
// @Tags         Sources
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   domain.SourceSummary
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /sources [get]
func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.sourceService.ListWithSummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sources")
		return
	}

	writeJSON(w, http.StatusOK, sources)
}

// handleGetSource godoc
// @Summary      Get source
// @Description  Get a data source by ID
// @Tags         Sources
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Source ID"
// @Success      200  {object}  domain.Source
// @Failure      400  {object}  ErrorResponse  "Missing source ID"
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      404  {object}  ErrorResponse  "Source not found"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /sources/{id} [get]
func (s *Server) handleGetSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing source id")
		return
	}

	source, err := s.sourceService.Get(r.Context(), id)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "source not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to get source")
		}
		return
	}

	writeJSON(w, http.StatusOK, source)
}

// handleCreateSource godoc
// @Summary      Create source
// @Description  Create a new data source (admin only)
// @Tags         Sources
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      driving.CreateSourceRequest  true  "Source configuration"
// @Success      201      {object}  domain.Source
// @Failure      400      {object}  ErrorResponse  "Invalid input"
// @Failure      401      {object}  ErrorResponse  "Unauthorized"
// @Failure      403      {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      409      {object}  ErrorResponse  "Source already exists"
// @Failure      500      {object}  ErrorResponse  "Internal server error"
// @Router       /sources [post]
func (s *Server) handleCreateSource(w http.ResponseWriter, r *http.Request) {
	authCtx := GetAuthContext(r.Context())
	if authCtx == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req driving.CreateSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	source, err := s.sourceService.Create(r.Context(), authCtx.UserID, req)
	if err != nil {
		switch err {
		case domain.ErrAlreadyExists:
			writeError(w, http.StatusConflict, "source already exists")
		case domain.ErrInvalidInput:
			writeError(w, http.StatusBadRequest, "invalid input")
		default:
			writeError(w, http.StatusInternalServerError, "failed to create source")
		}
		return
	}

	writeJSON(w, http.StatusCreated, source)
}

// handleDeleteSource godoc
// @Summary      Delete source
// @Description  Delete a data source by ID (admin only). This also removes all indexed documents from this source.
// @Tags         Sources
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Source ID"
// @Success      200  {object}  StatusResponse
// @Failure      400  {object}  ErrorResponse  "Missing source ID"
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      404  {object}  ErrorResponse  "Source not found"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /sources/{id} [delete]
func (s *Server) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing source id")
		return
	}

	if err := s.sourceService.Delete(r.Context(), id); err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "source not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to delete source")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// SyncAcceptedResponse represents the response when sync is triggered
// @Description Sync accepted response
type SyncAcceptedResponse struct {
	Status   string `json:"status" example:"accepted"`
	SourceID string `json:"source_id" example:"src_abc123"`
}

// handleTriggerSync godoc
// @Summary      Trigger sync
// @Description  Trigger a sync operation for a specific source (admin only)
// @Tags         Sources
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      string  true  "Source ID"
// @Success      202       {object}  SyncAcceptedResponse
// @Failure      400       {object}  ErrorResponse  "Missing source ID"
// @Failure      401       {object}  ErrorResponse  "Unauthorized"
// @Failure      403       {object}  ErrorResponse  "Forbidden - admin only"
// @Router       /sources/{id}/sync [post]
func (s *Server) handleTriggerSync(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("id")
	if sourceID == "" {
		writeError(w, http.StatusBadRequest, "missing source id")
		return
	}

	// Verify source exists
	source, err := s.sourceService.Get(r.Context(), sourceID)
	if err != nil {
		if err == domain.ErrNotFound {
			writeError(w, http.StatusNotFound, "source not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get source")
		return
	}

	// Check if taskQueue is available
	if s.taskQueue == nil {
		writeError(w, http.StatusServiceUnavailable, "task queue not configured")
		return
	}

	// Create and enqueue sync task
	// Note: Using "default" as team_id since we're single-org
	task := domain.NewSyncSourceTask("default", source.ID)
	if err := s.taskQueue.Enqueue(r.Context(), task); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue sync task")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":    "accepted",
		"source_id": sourceID,
		"task_id":   task.ID,
	})
}

// Settings endpoints

// handleGetSettings godoc
// @Summary      Get settings
// @Description  Get system settings (admin only)
// @Tags         Settings
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  domain.Settings
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /settings [get]
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.settingsService.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get settings")
		return
	}

	writeJSON(w, http.StatusOK, settings)
}

// handleUpdateSettings godoc
// @Summary      Update settings
// @Description  Update system settings (admin only)
// @Tags         Settings
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      driving.UpdateSettingsRequest  true  "Settings to update"
// @Success      200      {object}  domain.Settings
// @Failure      400      {object}  ErrorResponse  "Invalid request"
// @Failure      401      {object}  ErrorResponse  "Unauthorized"
// @Failure      403      {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500      {object}  ErrorResponse  "Internal server error"
// @Router       /settings [put]
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	authCtx := GetAuthContext(r.Context())
	if authCtx == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req driving.UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	settings, err := s.settingsService.Update(r.Context(), authCtx.UserID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update settings")
		return
	}

	writeJSON(w, http.StatusOK, settings)
}

// AI Settings endpoints

// handleGetAISettings godoc
// @Summary      Get AI settings
// @Description  Get AI provider configuration (admin only). Shows provider/model choice and credential availability from environment.
// @Tags         AI Settings
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  aiSettingsResponse
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /settings/ai [get]
func (s *Server) handleGetAISettings(w http.ResponseWriter, r *http.Request) {
	aiSettings, err := s.settingsService.GetAISettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get AI settings")
		return
	}

	// Get capabilities to determine credential availability
	caps, err := s.capabilitiesService.GetCapabilities(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get capabilities")
		return
	}

	// Build response with info from settings (user choices) and capabilities (env config)
	resp := aiSettingsResponse{
		Embedding: aiProviderInfo{
			Provider:     aiSettings.Embedding.Provider,
			Model:        aiSettings.Embedding.Model,
			HasAPIKey:    isProviderConfigured(aiSettings.Embedding.Provider, caps.AIProviders.Embedding),
			IsConfigured: aiSettings.Embedding.IsConfigured(),
		},
		LLM: aiProviderInfo{
			Provider:     aiSettings.LLM.Provider,
			Model:        aiSettings.LLM.Model,
			HasAPIKey:    isProviderConfigured(aiSettings.LLM.Provider, caps.AIProviders.LLM),
			IsConfigured: aiSettings.LLM.IsConfigured(),
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

// isProviderConfigured checks if a provider is in the list of configured providers
func isProviderConfigured(provider domain.AIProvider, configuredProviders []domain.AIProvider) bool {
	for _, p := range configuredProviders {
		if p == provider {
			return true
		}
	}
	return false
}

type aiSettingsResponse struct {
	Embedding aiProviderInfo `json:"embedding"`
	LLM       aiProviderInfo `json:"llm"`
}

// aiProviderInfo represents AI provider configuration status
// @Description AI provider configuration status
type aiProviderInfo struct {
	Provider     domain.AIProvider `json:"provider,omitempty" example:"openai"`
	Model        string            `json:"model,omitempty" example:"text-embedding-3-small"`
	HasAPIKey    bool              `json:"has_api_key" example:"true"` // True if credentials are configured via environment
	IsConfigured bool              `json:"is_configured" example:"true"` // True if provider and model are selected
}

// handleUpdateAISettings godoc
// @Summary      Update AI settings
// @Description  Update AI provider configuration (admin only). This triggers hot-reload of AI services.
// @Tags         AI Settings
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      driving.UpdateAISettingsRequest  true  "AI settings to update"
// @Success      200      {object}  driving.AISettingsStatus
// @Failure      400      {object}  ErrorResponse  "Invalid configuration or unsupported provider"
// @Failure      401      {object}  ErrorResponse  "Unauthorized"
// @Failure      403      {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500      {object}  ErrorResponse  "Internal server error"
// @Router       /settings/ai [put]
func (s *Server) handleUpdateAISettings(w http.ResponseWriter, r *http.Request) {
	var req driving.UpdateAISettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	status, err := s.settingsService.UpdateAISettings(r.Context(), req)
	if err != nil {
		switch err {
		case domain.ErrInvalidInput:
			writeError(w, http.StatusBadRequest, "invalid AI configuration")
		case domain.ErrInvalidProvider:
			writeError(w, http.StatusBadRequest, "unsupported AI provider")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// handleGetAIStatus godoc
// @Summary      Get AI status
// @Description  Get the current status of AI services including embedding, LLM, and Vespa connection status
// @Tags         AI Settings
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  driving.AISettingsStatus
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /settings/ai/status [get]
func (s *Server) handleGetAIStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.settingsService.GetAIStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get AI status")
		return
	}

	// Add Vespa status if service is available
	if s.vespaAdminService != nil {
		vespaStatus, err := s.vespaAdminService.Status(r.Context())
		if err == nil && vespaStatus != nil {
			status.Vespa = driving.VespaServiceStatus{
				Connected:         vespaStatus.Connected,
				SchemaMode:        vespaStatus.SchemaMode,
				EmbeddingsEnabled: vespaStatus.EmbeddingsEnabled,
				EmbeddingDim:      vespaStatus.EmbeddingDim,
				CanUpgrade:        vespaStatus.CanUpgrade,
				Healthy:           vespaStatus.Healthy,
			}
			// Include embedding dimension in embedding status if Vespa is configured with embeddings
			if vespaStatus.EmbeddingsEnabled && vespaStatus.EmbeddingDim > 0 {
				status.Embedding.EmbeddingDim = vespaStatus.EmbeddingDim
			}
		}
	}

	writeJSON(w, http.StatusOK, status)
}

// handleTestAIConnection godoc
// @Summary      Test AI connection
// @Description  Test connectivity to configured AI providers (admin only)
// @Tags         AI Settings
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  StatusResponse
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      503  {object}  ErrorResponse  "AI service unavailable"
// @Router       /settings/ai/test [post]
func (s *Server) handleTestAIConnection(w http.ResponseWriter, r *http.Request) {
	if err := s.settingsService.TestConnection(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "connected"})
}

// handleGetAIProviders godoc
// @Summary      Get AI providers
// @Description  Returns metadata about available AI providers and their models. Used for populating provider/model selection dropdowns in the UI.
// @Tags         AI Settings
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  driving.AIProvidersResponse
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /settings/ai/providers [get]
func (s *Server) handleGetAIProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := s.settingsService.GetAIProviders(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get AI providers")
		return
	}

	writeJSON(w, http.StatusOK, providers)
}

// Vespa admin endpoints

// handleVespaConnect godoc
// @Summary      Connect to Vespa
// @Description  Connect to a Vespa cluster and deploy the search schema (admin only). Use dev_mode=true for local development, dev_mode=false for production clusters.
// @Tags         Vespa
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      driving.ConnectVespaRequest  true  "Vespa connection settings"
// @Success      200      {object}  driving.VespaStatus
// @Failure      400      {object}  ErrorResponse  "Invalid request"
// @Failure      401      {object}  ErrorResponse  "Unauthorized"
// @Failure      403      {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500      {object}  ErrorResponse  "Connection failed"
// @Router       /admin/vespa/connect [post]
func (s *Server) handleVespaConnect(w http.ResponseWriter, r *http.Request) {
	var req driving.ConnectVespaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && r.ContentLength > 0 {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	status, err := s.vespaAdminService.Connect(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// handleVespaStatus godoc
// @Summary      Get Vespa status
// @Description  Get the current Vespa connection and schema status (admin only)
// @Tags         Vespa
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  driving.VespaStatus
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /admin/vespa/status [get]
func (s *Server) handleVespaStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.vespaAdminService.Status(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// handleVespaHealth godoc
// @Summary      Vespa health check
// @Description  Check if the Vespa cluster is healthy (admin only)
// @Tags         Vespa
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  StatusResponse
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      503  {object}  ErrorResponse  "Vespa unhealthy"
// @Router       /admin/vespa/health [get]
func (s *Server) handleVespaHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.vespaAdminService.HealthCheck(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// VespaMetricsResponse represents Vespa cluster metrics
type VespaMetricsResponse struct {
	Documents struct {
		Total   int64 `json:"total"`
		Ready   int64 `json:"ready"`
		Active  int64 `json:"active"`
		Removed int64 `json:"removed"`
	} `json:"documents"`
	Storage struct {
		DiskUsedBytes     int64   `json:"disk_used_bytes"`
		DiskUsedPercent   float64 `json:"disk_used_percent"`
		DataSizeBytes     int64   `json:"data_size_bytes"`
		MemoryUsedBytes   int64   `json:"memory_used_bytes"`
		MemoryUsedPercent float64 `json:"memory_used_percent"`
	} `json:"storage"`
	QueryPerformance struct {
		TotalQueries     int64   `json:"total_queries"`
		QueriesPerSecond float64 `json:"queries_per_second"`
		AvgLatencyMs     float64 `json:"avg_latency_ms"`
		FailedQueries    int64   `json:"failed_queries"`
		DegradedQueries  int64   `json:"degraded_queries"`
		EmptyResults     int64   `json:"empty_results"`
	} `json:"query_performance"`
	Feed struct {
		TotalOperations     int64   `json:"total_operations"`
		SucceededOperations int64   `json:"succeeded_operations"`
		FailedOperations    int64   `json:"failed_operations"`
		PendingOperations   int64   `json:"pending_operations"`
		AvgLatencyMs        float64 `json:"avg_latency_ms"`
	} `json:"feed"`
	Nodes     []VespaNodeMetricsResponse `json:"nodes"`
	Timestamp int64                      `json:"timestamp"`
}

// VespaNodeMetricsResponse represents metrics for a single Vespa node
type VespaNodeMetricsResponse struct {
	Hostname         string  `json:"hostname"`
	Role             string  `json:"role"`
	DocumentCount    int64   `json:"document_count"`
	DiskUsedBytes    int64   `json:"disk_used_bytes"`
	DiskUsedPercent  float64 `json:"disk_used_percent"`
	MemoryUsedBytes  int64   `json:"memory_used_bytes"`
	MemoryUsedPercent float64 `json:"memory_used_percent"`
}

// handleVespaMetrics godoc
// @Summary      Get Vespa metrics
// @Description  Get detailed Vespa cluster metrics including document counts, storage, query performance, and feed stats (admin only)
// @Tags         Vespa Admin
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  VespaMetricsResponse
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /admin/vespa/metrics [get]
func (s *Server) handleVespaMetrics(w http.ResponseWriter, r *http.Request) {
	// Get real metrics from Vespa cluster
	metrics, err := s.vespaAdminService.GetMetrics(r.Context())
	if err != nil {
		// Log the error but return partial data if possible
		// Fall back to status-based metrics
		status, statusErr := s.vespaAdminService.Status(r.Context())
		if statusErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to get vespa metrics")
			return
		}

		// Return basic metrics from status
		response := VespaMetricsResponse{
			Timestamp: status.IndexedChunks,
		}
		response.Documents.Total = status.IndexedChunks
		response.Documents.Ready = status.IndexedChunks
		response.Documents.Active = status.IndexedChunks
		writeJSON(w, http.StatusOK, response)
		return
	}

	// Transform domain metrics to API response
	response := VespaMetricsResponse{
		Timestamp: metrics.Timestamp,
	}

	// Document metrics
	response.Documents.Total = metrics.Documents.Total
	response.Documents.Ready = metrics.Documents.Ready
	response.Documents.Active = metrics.Documents.Active
	response.Documents.Removed = metrics.Documents.Removed

	// Storage metrics
	response.Storage.DiskUsedBytes = metrics.Storage.DiskUsedBytes
	response.Storage.DiskUsedPercent = metrics.Storage.DiskUsedPercent
	response.Storage.DataSizeBytes = metrics.Storage.DataSizeBytes
	response.Storage.MemoryUsedBytes = metrics.Storage.MemoryUsedBytes
	response.Storage.MemoryUsedPercent = metrics.Storage.MemoryUsedPercent

	// Query performance metrics
	response.QueryPerformance.TotalQueries = metrics.QueryPerformance.TotalQueries
	response.QueryPerformance.QueriesPerSecond = metrics.QueryPerformance.QueriesPerSecond
	response.QueryPerformance.AvgLatencyMs = metrics.QueryPerformance.AvgLatencyMs

	// Feed metrics
	response.Feed.TotalOperations = metrics.Feed.PutOperations + metrics.Feed.UpdateOperations + metrics.Feed.RemoveOperations
	response.Feed.AvgLatencyMs = metrics.Feed.AvgLatencyMs

	// Build node metrics from services
	response.Nodes = make([]VespaNodeMetricsResponse, 0)
	for _, svc := range metrics.Services {
		if svc.Status == "up" {
			node := VespaNodeMetricsResponse{
				Hostname:  svc.Name,
				Role:      svc.Name,
				MemoryUsedBytes: svc.MemoryMB * 1024 * 1024,
			}
			// Add document count for searchnode
			if svc.Name == "vespa.searchnode" {
				node.DocumentCount = metrics.Documents.Active
				node.DiskUsedBytes = metrics.Storage.DiskUsedBytes
				node.DiskUsedPercent = metrics.Storage.DiskUsedPercent
				node.MemoryUsedPercent = metrics.Storage.MemoryUsedPercent
			}
			response.Nodes = append(response.Nodes, node)
		}
	}

	writeJSON(w, http.StatusOK, response)
}

// Provider configuration endpoints

// handleListProviders godoc
// @Summary      List providers
// @Description  Get a list of all available data source providers with their configuration status (admin only)
// @Tags         Providers
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   driving.ProviderListItem
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /providers [get]
func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) {
	if s.providerService == nil {
		writeError(w, http.StatusServiceUnavailable, "provider service not configured - set MASTER_KEY to enable")
		return
	}

	providers, err := s.providerService.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list providers")
		return
	}

	writeJSON(w, http.StatusOK, providers)
}

// OAuth flow endpoints

// handleOAuthAuthorize godoc
// @Summary      Start OAuth authorization
// @Description  Initiate an OAuth authorization flow for a provider. Returns an authorization URL to redirect the user to. The user will be redirected back to /api/v1/oauth/callback after authorizing.
// @Tags         OAuth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        provider  path      string                     true  "Provider type (e.g., github, slack, notion)"
// @Param        request   body      driving.AuthorizeRequest   true  "Authorization request (provider_type optional, inferred from path)"
// @Success      200       {object}  driving.AuthorizeResponse
// @Failure      400       {object}  ErrorResponse  "Invalid request or provider type"
// @Failure      401       {object}  ErrorResponse  "Unauthorized"
// @Failure      403       {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      404       {object}  ErrorResponse  "Provider not configured"
// @Failure      500       {object}  ErrorResponse  "Internal server error"
// @Router       /oauth/{provider}/authorize [post]
func (s *Server) handleOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	if s.oauthService == nil {
		writeError(w, http.StatusServiceUnavailable, "oauth service not configured")
		return
	}

	providerType := domain.ProviderType(r.PathValue("provider"))
	if providerType == "" {
		writeError(w, http.StatusBadRequest, "missing provider type")
		return
	}

	var req driving.AuthorizeRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	// Use provider from path if not specified in body
	if req.ProviderType == "" {
		req.ProviderType = providerType
	}

	resp, err := s.oauthService.Authorize(r.Context(), req)
	if err != nil {
		switch err {
		case driving.ErrOAuthProviderNotFound:
			writeError(w, http.StatusNotFound, "provider not configured")
		case driving.ErrOAuthProviderDisabled:
			writeError(w, http.StatusBadRequest, "provider is disabled")
		default:
			writeError(w, http.StatusInternalServerError, "failed to start authorization: "+err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleOAuthCallback godoc
// @Summary      OAuth callback
// @Description  Handle the OAuth callback from an external provider. This endpoint is called by the provider after the user authorizes the application. It exchanges the authorization code for tokens and creates a connector installation, then redirects to the UI.
// @Tags         OAuth
// @Param        code               query     string  false  "Authorization code from provider"
// @Param        state              query     string  true   "State parameter for CSRF protection"
// @Param        error              query     string  false  "Error code if authorization failed"
// @Param        error_description  query     string  false  "Error description if authorization failed"
// @Success      302  "Redirect to UI oauth/complete page"
// @Failure      302  "Redirect to UI oauth/complete page with error"
// @Router       /oauth/callback [get]
func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	uiBaseURL := os.Getenv("UI_BASE_URL")
	if uiBaseURL == "" {
		uiBaseURL = "http://localhost:3000" // Default for development
	}

	// Helper to redirect with error
	redirectWithError := func(errCode, errDesc string) {
		params := url.Values{}
		params.Set("error", errCode)
		if errDesc != "" {
			params.Set("error_description", errDesc)
		}
		http.Redirect(w, r, fmt.Sprintf("%s/oauth/complete?%s", uiBaseURL, params.Encode()), http.StatusFound)
	}

	if s.oauthService == nil {
		redirectWithError("service_unavailable", "oauth service not configured")
		return
	}

	req := driving.CallbackRequest{
		Code:             r.URL.Query().Get("code"),
		State:            r.URL.Query().Get("state"),
		Error:            r.URL.Query().Get("error"),
		ErrorDescription: r.URL.Query().Get("error_description"),
	}

	if req.State == "" {
		redirectWithError("invalid_request", "missing state parameter")
		return
	}

	resp, err := s.oauthService.Callback(r.Context(), req)
	if err != nil {
		// Check for OAuth-specific errors
		if oauthErr, ok := err.(*driving.OAuthError); ok {
			redirectWithError(oauthErr.Code, oauthErr.Description)
			return
		}

		switch err {
		case driving.ErrOAuthInvalidState:
			redirectWithError("invalid_state", "invalid or expired state")
		case driving.ErrOAuthProviderNotFound:
			redirectWithError("provider_not_found", "provider not configured")
		default:
			redirectWithError("callback_failed", err.Error())
		}
		return
	}

	// Success - redirect to UI with connection details
	params := url.Values{}
	params.Set("connection_id", resp.Installation.ID)
	params.Set("provider", string(resp.Installation.ProviderType))
	params.Set("name", resp.Installation.Name)
	if resp.ReturnContext != "" {
		params.Set("return_context", resp.ReturnContext)
	}
	redirectURL := fmt.Sprintf("%s/oauth/complete?%s", uiBaseURL, params.Encode())
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// Installation endpoints

// handleCreateConnection godoc
// @Summary      Create connection
// @Description  Create a new installation for non-OAuth connectors (API key, path-based). Used for connectors like localfs.
// @Tags         Connections
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      driving.CreateConnectionRequest  true  "Installation configuration"
// @Success      201  {object}  domain.ConnectionSummary
// @Failure      400  {object}  ErrorResponse  "Invalid request"
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /connections [post]
func (s *Server) handleCreateConnection(w http.ResponseWriter, r *http.Request) {
	if s.connectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "connection service not configured")
		return
	}

	var req driving.CreateConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	installation, err := s.connectionService.Create(r.Context(), req)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, "missing required fields")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create installation")
		return
	}

	writeJSON(w, http.StatusCreated, installation)
}

// handleListConnections godoc
// @Summary      List connections
// @Description  Get all connector connections. Connections represent authenticated connections to external data sources (OAuth tokens, API keys, etc.).
// @Tags         Connections
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   domain.ConnectionSummary
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /connections [get]
func (s *Server) handleListConnections(w http.ResponseWriter, r *http.Request) {
	if s.connectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "connection service not configured")
		return
	}

	installations, err := s.connectionService.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list installations")
		return
	}

	writeJSON(w, http.StatusOK, installations)
}

// handleGetConnection godoc
// @Summary      Get connection
// @Description  Get a connector installation by ID. Returns installation metadata without secrets.
// @Tags         Connections
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Installation ID"
// @Success      200  {object}  domain.ConnectionSummary
// @Failure      400  {object}  ErrorResponse  "Missing installation ID"
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      404  {object}  ErrorResponse  "Connection not found"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /connections/{id} [get]
func (s *Server) handleGetConnection(w http.ResponseWriter, r *http.Request) {
	if s.connectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "connection service not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing connection id")
		return
	}

	installation, err := s.connectionService.Get(r.Context(), id)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "installation not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to get installation")
		}
		return
	}

	writeJSON(w, http.StatusOK, installation)
}

// handleDeleteConnection godoc
// @Summary      Delete connection
// @Description  Delete a connector installation. Cannot delete installations that are in use by sources.
// @Tags         Connections
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Installation ID"
// @Success      200  {object}  StatusResponse
// @Failure      400  {object}  ErrorResponse  "Missing installation ID"
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      404  {object}  ErrorResponse  "Connection not found"
// @Failure      409  {object}  ErrorResponse  "Connection in use by sources"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /connections/{id} [delete]
func (s *Server) handleDeleteConnection(w http.ResponseWriter, r *http.Request) {
	if s.connectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "connection service not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing connection id")
		return
	}

	err := s.connectionService.Delete(r.Context(), id)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "installation not found")
		case domain.ErrInUse:
			writeError(w, http.StatusConflict, "installation is in use by one or more sources")
		default:
			writeError(w, http.StatusInternalServerError, "failed to delete installation")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleListContainers godoc
// @Summary      List containers
// @Description  List available containers (repositories, drives, spaces, etc.) for an installation. Use this to populate a resource picker UI for selecting which containers to index.
// @Tags         Connections
// @Produce      json
// @Security     BearerAuth
// @Param        id      path      string  true   "Installation ID"
// @Param        cursor  query     string  false  "Pagination cursor from previous response"
// @Success      200     {object}  driving.ListContainersResponse
// @Failure      400     {object}  ErrorResponse  "Missing installation ID"
// @Failure      401     {object}  ErrorResponse  "Unauthorized"
// @Failure      403     {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      404     {object}  ErrorResponse  "Connection not found"
// @Failure      500     {object}  ErrorResponse  "Internal server error"
// @Router       /connections/{id}/containers [get]
func (s *Server) handleListContainers(w http.ResponseWriter, r *http.Request) {
	if s.connectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "connection service not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing connection id")
		return
	}

	cursor := r.URL.Query().Get("cursor")

	containers, err := s.connectionService.ListContainers(r.Context(), id, cursor)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "installation not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to list containers: "+err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, containers)
}

// handleGetConnectionSources godoc
// @Summary      Get connection sources
// @Description  List all sources using a specific connection. This shows which sources depend on this connection.
// @Tags         Connections
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Connection ID"
// @Success      200  {array}   domain.Source
// @Failure      400  {object}  ErrorResponse  "Missing connection ID"
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      404  {object}  ErrorResponse  "Connection not found"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /connections/{id}/sources [get]
func (s *Server) handleGetConnectionSources(w http.ResponseWriter, r *http.Request) {
	if s.connectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "connection service not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing connection id")
		return
	}

	// Verify connection exists
	_, err := s.connectionService.Get(r.Context(), id)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "connection not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to get connection")
		}
		return
	}

	// Get sources for this connection
	sources, err := s.sourceService.ListByConnection(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sources")
		return
	}

	writeJSON(w, http.StatusOK, sources)
}

// handleTestConnection godoc
// @Summary      Test connection
// @Description  Test if an installation's credentials are still valid. This attempts to authenticate with the external service.
// @Tags         Connections
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Installation ID"
// @Success      200  {object}  StatusResponse
// @Failure      400  {object}  ErrorResponse  "Missing installation ID"
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      404  {object}  ErrorResponse  "Connection not found"
// @Failure      503  {object}  ErrorResponse  "Credentials invalid or service unavailable"
// @Router       /connections/{id}/test [post]
func (s *Server) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	if s.connectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "connection service not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing connection id")
		return
	}

	err := s.connectionService.TestConnection(r.Context(), id)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "installation not found")
		default:
			writeError(w, http.StatusServiceUnavailable, "connection test failed: "+err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "connected"})
}

// Source containers endpoint

// UpdateContainersRequest represents a request to update source container containers
// @Description Request to update which containers a source should index
type UpdateContainersRequest struct {
	// Containers is the list of containers to index.
	// Empty list means index all available containers.
	Containers []domain.Container `json:"containers"`
}

// handleUpdateSourceContainers godoc
// @Summary      Update source containers
// @Description  Update which containers (repos, drives, spaces) a source should index. Pass an empty array to index all available containers.
// @Tags         Sources
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      string                   true  "Source ID"
// @Param        request  body      UpdateContainersRequest   true  "Container containers"
// @Success      200      {object}  StatusResponse
// @Failure      400      {object}  ErrorResponse  "Invalid request"
// @Failure      401      {object}  ErrorResponse  "Unauthorized"
// @Failure      403      {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      404      {object}  ErrorResponse  "Source not found"
// @Failure      500      {object}  ErrorResponse  "Internal server error"
// @Router       /sources/{id}/containers [put]
func (s *Server) handleUpdateSourceContainers(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing source id")
		return
	}

	var req UpdateContainersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := s.sourceService.UpdateContainers(r.Context(), id, req.Containers)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "source not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to update containers")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// Sync state endpoints

// handleGetSyncState godoc
// @Summary      Get sync state for source
// @Description  Get the current sync state and statistics for a specific source. Returns the last sync time, status, and document/chunk counts from the most recent sync operation.
// @Tags         Sources
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Source ID"
// @Success      200  {object}  domain.SyncState
// @Failure      400  {object}  ErrorResponse  "Missing source ID"
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      404  {object}  ErrorResponse  "Source not found"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /sources/{id}/sync [get]
func (s *Server) handleGetSyncState(w http.ResponseWriter, r *http.Request) {
	if s.syncOrchestrator == nil {
		writeError(w, http.StatusServiceUnavailable, "sync orchestrator not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing source id")
		return
	}

	state, err := s.syncOrchestrator.GetSyncState(r.Context(), id)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "source not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to get sync state: "+err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, state)
}

// handleListSyncStates godoc
// @Summary      List sync states
// @Description  Get sync states for all sources. Returns the sync status, last sync time, and statistics for each source.
// @Tags         Sources
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   domain.SyncState
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /sources/sync-states [get]
func (s *Server) handleListSyncStates(w http.ResponseWriter, r *http.Request) {
	if s.syncOrchestrator == nil {
		writeError(w, http.StatusServiceUnavailable, "sync orchestrator not configured")
		return
	}

	states, err := s.syncOrchestrator.ListSyncStates(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sync states: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, states)
}

// Document chunk endpoints

// handleGetDocumentChunks godoc
// @Summary      Get document chunks
// @Description  Get all chunks for a specific document. Chunks are the indexed segments of a document used for search.
// @Tags         Documents
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Document ID"
// @Success      200  {object}  domain.DocumentWithChunks
// @Failure      400  {object}  ErrorResponse  "Missing document ID"
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      404  {object}  ErrorResponse  "Document not found"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /documents/{id}/chunks [get]
func (s *Server) handleGetDocumentChunks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing document id")
		return
	}

	doc, err := s.docService.GetWithChunks(r.Context(), id)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "document not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to get document chunks: "+err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

// Source document endpoints

// SourceDocumentsResponse represents a paginated list of documents for a source
// @Description Paginated list of documents belonging to a source
type SourceDocumentsResponse struct {
	Documents []*domain.Document `json:"documents"`
	Total     int                `json:"total"`
	Limit     int                `json:"limit"`
	Offset    int                `json:"offset"`
}

// handleListSourceDocuments godoc
// @Summary      List source documents
// @Description  Get all documents indexed from a specific source with pagination support.
// @Tags         Sources
// @Produce      json
// @Security     BearerAuth
// @Param        id      path      string  true   "Source ID"
// @Param        limit   query     int     false  "Maximum number of documents to return (default 20, max 100)"
// @Param        offset  query     int     false  "Number of documents to skip (default 0)"
// @Success      200     {object}  SourceDocumentsResponse
// @Failure      400     {object}  ErrorResponse  "Missing source ID or invalid parameters"
// @Failure      401     {object}  ErrorResponse  "Unauthorized"
// @Failure      404     {object}  ErrorResponse  "Source not found"
// @Failure      500     {object}  ErrorResponse  "Internal server error"
// @Router       /sources/{id}/documents [get]
func (s *Server) handleListSourceDocuments(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing source id")
		return
	}

	// Parse pagination parameters
	limit := 20
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := parseInt(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := parseInt(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Get documents for source
	docs, err := s.docService.GetBySource(r.Context(), id, limit, offset)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "source not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to list documents: "+err.Error())
		}
		return
	}

	// Get total count
	total, err := s.docService.CountBySource(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count documents: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, SourceDocumentsResponse{
		Documents: docs,
		Total:     total,
		Limit:     limit,
		Offset:    offset,
	})
}

// Source update endpoints

// handleUpdateSource godoc
// @Summary      Update source
// @Description  Update a source's configuration, name, or enabled status (admin only).
// @Tags         Sources
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      string                      true  "Source ID"
// @Param        request  body      driving.UpdateSourceRequest true  "Source update request"
// @Success      200      {object}  domain.Source
// @Failure      400      {object}  ErrorResponse  "Invalid request"
// @Failure      401      {object}  ErrorResponse  "Unauthorized"
// @Failure      403      {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      404      {object}  ErrorResponse  "Source not found"
// @Failure      500      {object}  ErrorResponse  "Internal server error"
// @Router       /sources/{id} [put]
func (s *Server) handleUpdateSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing source id")
		return
	}

	var req driving.UpdateSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	source, err := s.sourceService.Update(r.Context(), id, req)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "source not found")
		case domain.ErrInvalidInput:
			writeError(w, http.StatusBadRequest, "invalid source data")
		default:
			writeError(w, http.StatusInternalServerError, "failed to update source: "+err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, source)
}

// handleEnableSource godoc
// @Summary      Enable source
// @Description  Enable a source for syncing. Enabled sources will be included in scheduled syncs.
// @Tags         Sources
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Source ID"
// @Success      200  {object}  StatusResponse
// @Failure      400  {object}  ErrorResponse  "Missing source ID"
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      404  {object}  ErrorResponse  "Source not found"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /sources/{id}/enable [post]
func (s *Server) handleEnableSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing source id")
		return
	}

	err := s.sourceService.Enable(r.Context(), id)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "source not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to enable source: "+err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

// handleDisableSource godoc
// @Summary      Disable source
// @Description  Disable a source from syncing. Disabled sources will not be included in scheduled syncs.
// @Tags         Sources
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Source ID"
// @Success      200  {object}  StatusResponse
// @Failure      400  {object}  ErrorResponse  "Missing source ID"
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      404  {object}  ErrorResponse  "Source not found"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /sources/{id}/disable [post]
func (s *Server) handleDisableSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing source id")
		return
	}

	err := s.sourceService.Disable(r.Context(), id)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "source not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to disable source: "+err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

// Admin stats endpoints

// AdminStatsResponse represents system-wide statistics
// @Description System-wide statistics for the admin dashboard
type AdminStatsResponse struct {
	Documents     DocumentStats     `json:"documents"`
	Chunks        ChunkStats        `json:"chunks"`
	Sources       SourceStats       `json:"sources"`
	Connections ConnectionStats `json:"installations"`
	Users         UserStats         `json:"users"`
}

// DocumentStats represents document statistics
type DocumentStats struct {
	Total int `json:"total"`
}

// ChunkStats represents chunk statistics
type ChunkStats struct {
	Total int `json:"total"`
}

// SourceStats represents source statistics
type SourceStats struct {
	Total   int `json:"total"`
	Enabled int `json:"enabled"`
}

// ConnectionStats represents installation statistics
type ConnectionStats struct {
	Total int `json:"total"`
}

// UserStats represents user statistics
type UserStats struct {
	Total int `json:"total"`
}

// handleGetAdminStats godoc
// @Summary      Get admin statistics
// @Description  Get system-wide statistics including document counts, chunk counts, source counts, installation counts, and user counts. Useful for admin dashboards and monitoring.
// @Tags         Admin
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  AdminStatsResponse
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /admin/stats [get]
func (s *Server) handleGetAdminStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get document count
	docCount, err := s.docService.Count(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get document count: "+err.Error())
		return
	}

	// Get sources and count enabled
	sources, err := s.sourceService.List(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get sources: "+err.Error())
		return
	}
	enabledCount := 0
	for _, src := range sources {
		if src.Enabled {
			enabledCount++
		}
	}

	// Get connection count
	var installCount int
	if s.connectionService != nil {
		installations, err := s.connectionService.List(ctx)
		if err == nil {
			installCount = len(installations)
		}
	}

	// Get user count
	var userCount int
	if s.userService != nil {
		users, err := s.userService.List(ctx)
		if err == nil {
			userCount = len(users)
		}
	}

	// Note: Chunk count would require adding a Count method to the chunk store
	// For now we report 0 and can enhance this later
	chunkCount := 0

	writeJSON(w, http.StatusOK, AdminStatsResponse{
		Documents: DocumentStats{
			Total: docCount,
		},
		Chunks: ChunkStats{
			Total: chunkCount,
		},
		Sources: SourceStats{
			Total:   len(sources),
			Enabled: enabledCount,
		},
		Connections: ConnectionStats{
			Total: installCount,
		},
		Users: UserStats{
			Total: userCount,
		},
	})
}

// Admin dashboard endpoints

// handleListJobs godoc
// @Summary      List job history
// @Description  Retrieve job execution history with filtering and pagination
// @Tags         Admin
// @Produce      json
// @Security     BearerAuth
// @Param        status  query     string  false  "Filter by status (pending, processing, completed, failed)"
// @Param        type    query     string  false  "Filter by task type (sync_source, sync_all)"
// @Param        limit   query     int     false  "Maximum number of jobs to return (default: 50, max: 100)"
// @Param        offset  query     int     false  "Number of jobs to skip for pagination"
// @Success      200     {object}  domain.JobHistory
// @Failure      401     {object}  ErrorResponse  "Unauthorized"
// @Failure      403     {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500     {object}  ErrorResponse  "Internal server error"
// @Router       /admin/jobs [get]
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	if s.adminService == nil {
		writeError(w, http.StatusServiceUnavailable, "admin service not available")
		return
	}

	teamID := getTeamID(r.Context())
	if teamID == "" {
		writeError(w, http.StatusUnauthorized, "missing team context")
		return
	}

	req := driving.ListJobsRequest{
		Limit:  50,
		Offset: 0,
	}

	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		req.Status = domain.TaskStatus(statusStr)
	}
	if typeStr := r.URL.Query().Get("type"); typeStr != "" {
		req.Type = domain.TaskType(typeStr)
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := parseInt(limitStr); err == nil {
			req.Limit = limit
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := parseInt(offsetStr); err == nil {
			req.Offset = offset
		}
	}

	history, err := s.adminService.ListJobs(r.Context(), teamID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list jobs: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, history)
}

// handleGetUpcomingJobs godoc
// @Summary      Get upcoming jobs
// @Description  Retrieve pending tasks and scheduled task configurations
// @Tags         Admin
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  domain.UpcomingJobs
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /admin/jobs/upcoming [get]
func (s *Server) handleGetUpcomingJobs(w http.ResponseWriter, r *http.Request) {
	if s.adminService == nil {
		writeError(w, http.StatusServiceUnavailable, "admin service not available")
		return
	}

	teamID := getTeamID(r.Context())
	if teamID == "" {
		writeError(w, http.StatusUnauthorized, "missing team context")
		return
	}

	upcoming, err := s.adminService.GetUpcomingJobs(r.Context(), teamID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get upcoming jobs: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, upcoming)
}

// handleGetJob godoc
// @Summary      Get job details
// @Description  Retrieve detailed information about a specific job
// @Tags         Admin
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Job ID"
// @Success      200  {object}  domain.JobDetail
// @Failure      400  {object}  ErrorResponse  "Missing job ID"
// @Failure      401  {object}  ErrorResponse  "Unauthorized"
// @Failure      403  {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      404  {object}  ErrorResponse  "Job not found"
// @Failure      500  {object}  ErrorResponse  "Internal server error"
// @Router       /admin/jobs/{id} [get]
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	if s.adminService == nil {
		writeError(w, http.StatusServiceUnavailable, "admin service not available")
		return
	}

	teamID := getTeamID(r.Context())
	if teamID == "" {
		writeError(w, http.StatusUnauthorized, "missing team context")
		return
	}

	jobID := r.PathValue("id")
	if jobID == "" {
		writeError(w, http.StatusBadRequest, "missing job id")
		return
	}

	job, err := s.adminService.GetJob(r.Context(), teamID, jobID)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "job not found")
		case domain.ErrUnauthorized:
			writeError(w, http.StatusForbidden, "job belongs to different team")
		default:
			writeError(w, http.StatusInternalServerError, "failed to get job: "+err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, job)
}

// handleGetJobStats godoc
// @Summary      Get job statistics
// @Description  Compute aggregated job statistics for a time period
// @Tags         Admin
// @Produce      json
// @Security     BearerAuth
// @Param        period  query     string  false  "Time period (24h, 7d, 30d)" default(24h)
// @Success      200     {object}  domain.JobStats
// @Failure      401     {object}  ErrorResponse  "Unauthorized"
// @Failure      403     {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500     {object}  ErrorResponse  "Internal server error"
// @Router       /admin/jobs/stats [get]
func (s *Server) handleGetJobStats(w http.ResponseWriter, r *http.Request) {
	if s.adminService == nil {
		writeError(w, http.StatusServiceUnavailable, "admin service not available")
		return
	}

	teamID := getTeamID(r.Context())
	if teamID == "" {
		writeError(w, http.StatusUnauthorized, "missing team context")
		return
	}

	period := driving.JobStatsPeriod24Hours
	if periodStr := r.URL.Query().Get("period"); periodStr != "" {
		period = driving.JobStatsPeriod(periodStr)
	}

	stats, err := s.adminService.GetJobStats(r.Context(), teamID, period)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get job stats: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// handleGetSearchAnalytics godoc
// @Summary      Get search analytics
// @Description  Compute search usage analytics for a time period
// @Tags         Admin
// @Produce      json
// @Security     BearerAuth
// @Param        period  query     string  false  "Time period (24h, 7d, 30d)" default(24h)
// @Success      200     {object}  domain.SearchAnalytics
// @Failure      401     {object}  ErrorResponse  "Unauthorized"
// @Failure      403     {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500     {object}  ErrorResponse  "Internal server error"
// @Router       /admin/search/analytics [get]
func (s *Server) handleGetSearchAnalytics(w http.ResponseWriter, r *http.Request) {
	if s.adminService == nil {
		writeError(w, http.StatusServiceUnavailable, "admin service not available")
		return
	}

	teamID := getTeamID(r.Context())
	if teamID == "" {
		writeError(w, http.StatusUnauthorized, "missing team context")
		return
	}

	period := driving.SearchAnalyticsPeriod24Hours
	if periodStr := r.URL.Query().Get("period"); periodStr != "" {
		period = driving.SearchAnalyticsPeriod(periodStr)
	}

	analytics, err := s.adminService.GetSearchAnalytics(r.Context(), teamID, period)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get search analytics: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, analytics)
}

// handleGetSearchHistory godoc
// @Summary      Get search history
// @Description  Retrieve recent search queries
// @Tags         Admin
// @Produce      json
// @Security     BearerAuth
// @Param        limit   query     int     false  "Maximum number of searches to return (default: 50, max: 100)"
// @Success      200     {array}   domain.SearchQuery
// @Failure      401     {object}  ErrorResponse  "Unauthorized"
// @Failure      403     {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500     {object}  ErrorResponse  "Internal server error"
// @Router       /admin/search/history [get]
func (s *Server) handleGetSearchHistory(w http.ResponseWriter, r *http.Request) {
	if s.adminService == nil {
		writeError(w, http.StatusServiceUnavailable, "admin service not available")
		return
	}

	teamID := getTeamID(r.Context())
	if teamID == "" {
		writeError(w, http.StatusUnauthorized, "missing team context")
		return
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := parseInt(limitStr); err == nil {
			limit = l
		}
	}

	history, err := s.adminService.GetSearchHistory(r.Context(), teamID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get search history: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, history)
}

// handleGetSearchMetrics godoc
// @Summary      Get search metrics
// @Description  Compute search performance metrics for a time period
// @Tags         Admin
// @Produce      json
// @Security     BearerAuth
// @Param        period  query     string  false  "Time period (24h, 7d, 30d)" default(24h)
// @Success      200     {object}  domain.SearchMetrics
// @Failure      401     {object}  ErrorResponse  "Unauthorized"
// @Failure      403     {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500     {object}  ErrorResponse  "Internal server error"
// @Router       /admin/search/metrics [get]
func (s *Server) handleGetSearchMetrics(w http.ResponseWriter, r *http.Request) {
	if s.adminService == nil {
		writeError(w, http.StatusServiceUnavailable, "admin service not available")
		return
	}

	teamID := getTeamID(r.Context())
	if teamID == "" {
		writeError(w, http.StatusUnauthorized, "missing team context")
		return
	}

	period := driving.SearchAnalyticsPeriod24Hours
	if periodStr := r.URL.Query().Get("period"); periodStr != "" {
		period = driving.SearchAnalyticsPeriod(periodStr)
	}

	metrics, err := s.adminService.GetSearchMetrics(r.Context(), teamID, period)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get search metrics: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, metrics)
}

// TriggerReindexResponse represents the response from triggering a reindex
// @Description Response containing task IDs created for reindex operation
type TriggerReindexResponse struct {
	TaskIDs []string `json:"task_ids"`
	Message string   `json:"message"`
}

// handleTriggerReindex godoc
// @Summary      Trigger reindex
// @Description  Create tasks to reindex sources. If no source IDs are provided, all enabled sources are reindexed.
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      driving.TriggerReindexRequest  true  "Reindex options"
// @Success      202      {object}  TriggerReindexResponse
// @Failure      400      {object}  ErrorResponse  "Invalid request body"
// @Failure      401      {object}  ErrorResponse  "Unauthorized"
// @Failure      403      {object}  ErrorResponse  "Forbidden - admin only"
// @Failure      500      {object}  ErrorResponse  "Internal server error"
// @Router       /admin/reindex [post]
func (s *Server) handleTriggerReindex(w http.ResponseWriter, r *http.Request) {
	if s.adminService == nil {
		writeError(w, http.StatusServiceUnavailable, "admin service not available")
		return
	}

	teamID := getTeamID(r.Context())
	if teamID == "" {
		writeError(w, http.StatusUnauthorized, "missing team context")
		return
	}

	var req driving.TriggerReindexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	taskIDs, err := s.adminService.TriggerReindex(r.Context(), teamID, req)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeError(w, http.StatusNotFound, "one or more sources not found")
		case domain.ErrInvalidInput:
			writeError(w, http.StatusBadRequest, "invalid reindex request")
		default:
			writeError(w, http.StatusInternalServerError, "failed to trigger reindex: "+err.Error())
		}
		return
	}

	message := fmt.Sprintf("Reindex triggered for %d source(s)", len(taskIDs))
	writeJSON(w, http.StatusAccepted, TriggerReindexResponse{
		TaskIDs: taskIDs,
		Message: message,
	})
}

// parseInt is a helper to parse integer query parameters
func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// getTeamID retrieves the team ID from the auth context
func getTeamID(ctx context.Context) string {
	authCtx := GetAuthContext(ctx)
	if authCtx == nil {
		return ""
	}
	return authCtx.TeamID
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
