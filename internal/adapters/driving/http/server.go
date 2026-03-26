package http

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driving"
)

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
	vespaAdminService   driving.VespaAdminService
	providerService     driving.ProviderService
	oauthService        driving.OAuthService
	installationService driving.InstallationService
	syncOrchestrator    driving.SyncOrchestrator

	// Infrastructure
	taskQueue   driven.TaskQueue
	db          Pinger // PostgreSQL health check
	redisClient Pinger // Redis health check (optional)
}

// Config holds server configuration
type Config struct {
	Host    string
	Port    int
	Version string
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
	vespaAdminService driving.VespaAdminService,
	providerService driving.ProviderService,
	oauthService driving.OAuthService,
	installationService driving.InstallationService,
	syncOrchestrator driving.SyncOrchestrator,
	taskQueue driven.TaskQueue,
	db Pinger,
	redisClient Pinger, // can be nil
) *Server {
	s := &Server{
		router:              http.NewServeMux(),
		version:             cfg.Version,
		authService:         authService,
		userService:         userService,
		searchService:       searchService,
		sourceService:       sourceService,
		docService:          docService,
		settingsService:     settingsService,
		vespaAdminService:   vespaAdminService,
		providerService:     providerService,
		oauthService:        oauthService,
		installationService: installationService,
		syncOrchestrator:    syncOrchestrator,
		taskQueue:           taskQueue,
		db:                  db,
		redisClient:         redisClient,
	}

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      s.router,
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

	// Auth endpoints (public)
	s.router.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	s.router.HandleFunc("POST /api/v1/auth/refresh", s.handleRefresh)

	// Setup endpoint (public, one-time use)
	s.router.HandleFunc("POST /api/v1/setup", s.handleSetup)

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

	// Admin endpoints (admin-only)
	s.router.Handle("GET /api/v1/admin/stats",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleGetAdminStats))))

	// Vespa admin endpoints (admin-only)
	s.router.Handle("POST /api/v1/admin/vespa/connect",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleVespaConnect))))
	s.router.Handle("GET /api/v1/admin/vespa/status",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleVespaStatus))))
	s.router.Handle("GET /api/v1/admin/vespa/health",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleVespaHealth))))

	// Provider configuration endpoints (admin-only)
	s.router.Handle("GET /api/v1/providers",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleListProviders))))
	s.router.Handle("GET /api/v1/providers/{type}/config",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleGetProviderConfig))))
	s.router.Handle("POST /api/v1/providers/{type}/config",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleSaveProviderConfig))))
	s.router.Handle("DELETE /api/v1/providers/{type}/config",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleDeleteProviderConfig))))

	// OAuth flow endpoints (admin-only for authorization initiation)
	s.router.Handle("POST /api/v1/oauth/{provider}/authorize",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleOAuthAuthorize))))
	// Callback is public - receives redirects from OAuth providers
	s.router.HandleFunc("GET /api/v1/oauth/callback", s.handleOAuthCallback)

	// Installation endpoints (admin-only)
	s.router.Handle("POST /api/v1/installations",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleCreateInstallation))))
	s.router.Handle("GET /api/v1/installations",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleListInstallations))))
	s.router.Handle("GET /api/v1/installations/{id}",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleGetInstallation))))
	s.router.Handle("DELETE /api/v1/installations/{id}",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleDeleteInstallation))))
	s.router.Handle("GET /api/v1/installations/{id}/containers",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleListContainers))))
	s.router.Handle("POST /api/v1/installations/{id}/test",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleTestInstallation))))

	// Source selection endpoint (admin-only)
	s.router.Handle("PUT /api/v1/sources/{id}/selection",
		authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(s.handleUpdateSourceSelection))))
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
