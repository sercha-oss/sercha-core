package http

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
)

// stripTrailingSlash removes trailing slashes from request paths (except root)
func stripTrailingSlash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && strings.HasSuffix(r.URL.Path, "/") {
			r.URL.Path = strings.TrimSuffix(r.URL.Path, "/")
		}
		next.ServeHTTP(w, r)
	})
}

// Pinger is a simple health check interface
type Pinger interface {
	Ping(ctx context.Context) error
}

// Server represents the HTTP server
type Server struct {
	httpServer *http.Server
	router     *http.ServeMux
	version    string

	// Services
	authService         driving.AuthService
	userService         driving.UserService
	searchService       driving.SearchService
	sourceService       driving.SourceService
	docService          driving.DocumentService
	settingsService     driving.SettingsService
	providerService     driving.ProviderService
	oauthService        driving.OAuthService
	oauthServerService  driving.OAuthServerService // OAuth 2.0 Authorization Server
	connectionService   driving.ConnectionService
	syncOrchestrator    driving.SyncOrchestrator
	capabilitiesService driving.CapabilitiesService
	setupService        driving.SetupService
	adminService        driving.AdminService

	// MCP Handler
	mcpHandler http.Handler // MCP StreamableHTTPHandler with auth

	// Config
	uiBaseURL string // Frontend URL for OAuth redirects

	// Infrastructure
	taskQueue       driven.TaskQueue
	searchQueryRepo driven.SearchQueryRepository // for search tracking
	db              Pinger                       // PostgreSQL health check
	redisClient     Pinger                       // Redis health check (optional)

	// Optional observers (wired post-construction to keep NewServer stable)
	retrievalObserver driven.RetrievalObserver
}

// SetRetrievalObserver installs an optional RetrievalObserver. Passing nil
// disables observer invocation. Safe to call before Start; not safe to call
// concurrently with in-flight requests.
func (s *Server) SetRetrievalObserver(obs driven.RetrievalObserver) {
	s.retrievalObserver = obs
}

// Config holds server configuration
type Config struct {
	Host        string
	Port        int
	Version     string
	CORSOrigins []string
	UIBaseURL   string // Frontend URL for OAuth redirects (default: http://localhost:3000)
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		Host:    "0.0.0.0",
		Port:    8080,
		Version: "dev",
	}
}

// NewServer creates a new HTTP server
func NewServer(
	cfg Config,
	authService driving.AuthService,
	userService driving.UserService,
	searchService driving.SearchService,
	sourceService driving.SourceService,
	docService driving.DocumentService,
	settingsService driving.SettingsService,
	providerService driving.ProviderService,
	oauthService driving.OAuthService,
	connectionService driving.ConnectionService,
	syncOrchestrator driving.SyncOrchestrator,
	capabilitiesService driving.CapabilitiesService,
	setupService driving.SetupService,
	adminService driving.AdminService,
	taskQueue driven.TaskQueue,
	searchQueryRepo driven.SearchQueryRepository,
	db Pinger,
	redisClient Pinger, // can be nil
	oauthServerService driving.OAuthServerService, // can be nil
	mcpHandler http.Handler, // can be nil
) *Server {
	uiBaseURL := cfg.UIBaseURL
	if uiBaseURL == "" {
		uiBaseURL = "http://localhost:3000"
	}

	s := &Server{
		router:              http.NewServeMux(),
		version:             cfg.Version,
		uiBaseURL:           uiBaseURL,
		authService:         authService,
		userService:         userService,
		searchService:       searchService,
		sourceService:       sourceService,
		docService:          docService,
		settingsService:     settingsService,
		providerService:     providerService,
		oauthService:        oauthService,
		oauthServerService:  oauthServerService,
		connectionService:   connectionService,
		syncOrchestrator:    syncOrchestrator,
		capabilitiesService: capabilitiesService,
		setupService:        setupService,
		adminService:        adminService,
		mcpHandler:          mcpHandler,
		taskQueue:           taskQueue,
		searchQueryRepo:     searchQueryRepo,
		db:                  db,
		redisClient:         redisClient,
	}

	// Build handler chain: CORS -> strip trailing slash -> router
	handler := stripTrailingSlash(s.router)
	if len(cfg.CORSOrigins) > 0 {
		corsMiddleware := NewCORSMiddleware(cfg.CORSOrigins)
		handler = corsMiddleware.Handler(handler)
	}

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Create middleware
	authMiddleware := NewAuthMiddleware(s.authService)

	// Health endpoints (no auth)
	s.router.HandleFunc("GET /health", s.handleHealth)
	s.router.HandleFunc("GET /ready", s.handleReady)
	s.router.HandleFunc("GET /version", s.handleVersion)
	s.router.HandleFunc("GET /api/v1/capabilities", s.handleGetCapabilities)

	// Auth endpoints (public)
	s.router.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	s.router.HandleFunc("POST /api/v1/auth/refresh", s.handleRefresh)

	// Setup endpoints (public, no auth required)
	s.router.HandleFunc("POST /api/v1/setup", s.handleSetup)
	s.router.HandleFunc("GET /api/v1/setup/status", s.handleSetupStatus)

	// Auth endpoints (authenticated)
	s.router.Handle("POST /api/v1/auth/logout",
		authMiddleware.Authenticate(http.HandlerFunc(s.handleLogout)))

	// User endpoints
	s.router.Handle("GET /api/v1/me",
		authMiddleware.Authenticate(http.HandlerFunc(s.handleGetMe)))

	// Admin-only user management
	s.router.Handle("GET /api/v1/users",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleListUsers))))
	s.router.Handle("POST /api/v1/users",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleCreateUser))))
	s.router.Handle("GET /api/v1/users/{id}",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleGetUser))))
	s.router.Handle("PUT /api/v1/users/{id}",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleUpdateUser))))
	s.router.Handle("POST /api/v1/users/{id}/reset-password",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleResetUserPassword))))
	s.router.Handle("DELETE /api/v1/users/{id}",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleDeleteUser))))

	// Search endpoints (authenticated)
	s.router.Handle("POST /api/v1/search",
		authMiddleware.Authenticate(http.HandlerFunc(s.handleSearch)))

	// Document endpoints (authenticated)
	s.router.Handle("GET /api/v1/documents/{id}",
		authMiddleware.Authenticate(http.HandlerFunc(s.handleGetDocument)))
	s.router.Handle("GET /api/v1/documents/{id}/chunks",
		authMiddleware.Authenticate(http.HandlerFunc(s.handleGetDocumentChunks)))
	s.router.Handle("GET /api/v1/documents/{id}/open",
		authMiddleware.Authenticate(http.HandlerFunc(s.handleOpenDocument)))

	// Source endpoints (admin-only for mutations)
	s.router.Handle("GET /api/v1/sources",
		authMiddleware.Authenticate(http.HandlerFunc(s.handleListSources)))
	s.router.Handle("POST /api/v1/sources",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleCreateSource))))
	s.router.Handle("GET /api/v1/sources/{id}",
		authMiddleware.Authenticate(http.HandlerFunc(s.handleGetSource)))
	s.router.Handle("PUT /api/v1/sources/{id}",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleUpdateSource))))
	s.router.Handle("DELETE /api/v1/sources/{id}",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleDeleteSource))))
	s.router.Handle("GET /api/v1/sources/{id}/documents",
		authMiddleware.Authenticate(http.HandlerFunc(s.handleListSourceDocuments)))
	s.router.Handle("POST /api/v1/sources/{id}/enable",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleEnableSource))))
	s.router.Handle("POST /api/v1/sources/{id}/disable",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleDisableSource))))

	// Sync endpoints (admin-only) - all under /sources for consistency
	s.router.Handle("POST /api/v1/sources/{id}/sync",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleTriggerSync))))
	s.router.Handle("GET /api/v1/sources/{id}/sync",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleGetSyncState))))
	s.router.Handle("GET /api/v1/sources/sync-states",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleListSyncStates))))

	// Settings endpoints (admin-only)
	s.router.Handle("GET /api/v1/settings",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleGetSettings))))
	s.router.Handle("PUT /api/v1/settings",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleUpdateSettings))))

	// AI settings endpoints (admin-only)
	s.router.Handle("GET /api/v1/settings/ai",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleGetAISettings))))
	s.router.Handle("PUT /api/v1/settings/ai",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleUpdateAISettings))))
	s.router.Handle("GET /api/v1/settings/ai/status",
		authMiddleware.Authenticate(http.HandlerFunc(s.handleGetAIStatus)))
	s.router.Handle("POST /api/v1/settings/ai/test",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleTestAIConnection))))
	s.router.Handle("GET /api/v1/settings/ai/providers",
		authMiddleware.Authenticate(http.HandlerFunc(s.handleGetAIProviders)))

	// Capability preferences endpoints (admin-only)
	s.router.Handle("GET /api/v1/capability-preferences",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleGetCapabilityPreferences))))
	s.router.Handle("PUT /api/v1/capability-preferences",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleUpdateCapabilityPreferences))))

	// Admin endpoints (admin-only)
	s.router.Handle("GET /api/v1/admin/stats",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleGetAdminStats))))

	// Admin dashboard endpoints (admin-only)
	s.router.Handle("GET /api/v1/admin/jobs",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleListJobs))))
	s.router.Handle("GET /api/v1/admin/jobs/upcoming",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleGetUpcomingJobs))))
	s.router.Handle("GET /api/v1/admin/jobs/{id}",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleGetJob))))
	s.router.Handle("GET /api/v1/admin/jobs/stats",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleGetJobStats))))
	s.router.Handle("GET /api/v1/admin/search/analytics",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleGetSearchAnalytics))))
	s.router.Handle("GET /api/v1/admin/search/history",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleGetSearchHistory))))
	s.router.Handle("GET /api/v1/admin/search/metrics",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleGetSearchMetrics))))
	s.router.Handle("POST /api/v1/admin/reindex",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleTriggerReindex))))

	// Provider configuration endpoints (admin-only)
	// Note: Provider credentials are now managed via environment variables, not API
	s.router.Handle("GET /api/v1/providers",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleListProviders))))

	// OAuth flow endpoints (admin-only for authorization initiation)
	s.router.Handle("POST /api/v1/oauth/{provider}/authorize",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleOAuthAuthorize))))
	// Callback is public - receives redirects from OAuth providers
	s.router.HandleFunc("GET /api/v1/oauth/callback", s.handleOAuthCallback)

	// Connection endpoints (admin-only)
	s.router.Handle("POST /api/v1/connections",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleCreateConnection))))
	s.router.Handle("GET /api/v1/connections",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleListConnections))))
	s.router.Handle("GET /api/v1/connections/{id}",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleGetConnection))))
	s.router.Handle("DELETE /api/v1/connections/{id}",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleDeleteConnection))))
	s.router.Handle("GET /api/v1/connections/{id}/containers",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleListContainers))))
	s.router.Handle("GET /api/v1/connections/{id}/sources",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleGetConnectionSources))))
	s.router.Handle("POST /api/v1/connections/{id}/test",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleTestConnection))))

	// Source container management endpoint (admin-only)
	s.router.Handle("PUT /api/v1/sources/{id}/containers",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleUpdateSourceContainers))))

	// OAuth 2.0 Authorization Server endpoints (if configured)
	if s.oauthServerService != nil {
		// OAuth Server Metadata (public)
		s.router.HandleFunc("GET /.well-known/oauth-authorization-server", s.handleOAuthServerMetadata)

		// Dynamic Client Registration (public)
		s.router.HandleFunc("POST /oauth/register", s.handleDynamicClientRegistration)

		// Authorization endpoint (public — redirects to frontend if not authenticated)
		s.router.HandleFunc("GET /oauth/authorize", s.handleOAuthServerAuthorize)

		// Authorization completion (frontend calls with JWT after user login + consent)
		s.router.Handle("POST /oauth/authorize/complete",
			authMiddleware.Authenticate(http.HandlerFunc(s.handleOAuthServerAuthorizeComplete)))

		// Token endpoint (public - client auth via credentials)
		s.router.HandleFunc("POST /oauth/token", s.handleOAuthServerToken)

		// Revocation endpoint (public - client auth via credentials)
		s.router.HandleFunc("POST /oauth/revoke", s.handleOAuthServerRevoke)

		// Client public info (public — used by consent screen to display app name)
		s.router.HandleFunc("GET /oauth/clients/{client_id}", s.handleOAuthClientInfo)

		// Protected Resource Metadata (public)
		s.router.HandleFunc("GET /.well-known/oauth-protected-resource", s.handleProtectedResourceMetadata)
	}

	// MCP endpoint (if configured)
	if s.mcpHandler != nil {
		// MCP server handles its own auth via bearer token middleware
		s.router.Handle("POST /mcp", s.mcpHandler)
		s.router.Handle("GET /mcp", s.mcpHandler)
		s.router.Handle("DELETE /mcp", s.mcpHandler)

		// Protected Resource Metadata under /mcp path (SDK advertises this in WWW-Authenticate)
		if s.oauthServerService != nil {
			s.router.HandleFunc("GET /mcp/.well-known/oauth-protected-resource", s.handleProtectedResourceMetadata)
		}
	}
}

// Start starts the HTTP server with graceful shutdown
func (s *Server) Start() error {
	// Channel to listen for OS signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		log.Printf("Starting server on %s", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-stop
	log.Println("Shutting down server...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	log.Println("Server stopped")
	return nil
}

// Stop stops the server
func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
