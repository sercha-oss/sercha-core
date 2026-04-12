package main

// @title           Sercha Core API
// @version         1.0
// @description     Privacy-focused enterprise search API. Sercha Core provides full-text and semantic search across your connected data sources.

// @contact.name   Sercha OSS
// @contact.url    https://github.com/sercha-oss/sercha-core/issues

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8081
// @BasePath  /api/v1
// @schemes   http https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Bearer token. Format: "Bearer {token}"

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	stdhttp "net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/ai"
	"github.com/sercha-oss/sercha-core/internal/adapters/driven/auth"
	"github.com/sercha-oss/sercha-core/internal/adapters/driven/connectors"
	"github.com/sercha-oss/sercha-core/internal/adapters/driven/connectors/github"
	"github.com/sercha-oss/sercha-core/internal/adapters/driven/connectors/localfs"
	"github.com/sercha-oss/sercha-core/internal/adapters/driven/opensearch"
	"github.com/sercha-oss/sercha-core/internal/adapters/driven/pgvector"
	pipelineexec "github.com/sercha-oss/sercha-core/internal/adapters/driven/pipeline/executor"
	"github.com/sercha-oss/sercha-core/internal/adapters/driven/pipeline/providers"
	pipelinereg "github.com/sercha-oss/sercha-core/internal/adapters/driven/pipeline/registry"
	indexingstages "github.com/sercha-oss/sercha-core/internal/adapters/driven/pipeline/stages/indexing"
	searchstages "github.com/sercha-oss/sercha-core/internal/adapters/driven/pipeline/stages/search"
	"github.com/sercha-oss/sercha-core/internal/adapters/driven/postgres"
	postgresqueue "github.com/sercha-oss/sercha-core/internal/adapters/driven/queue/postgres"
	redisqueue "github.com/sercha-oss/sercha-core/internal/adapters/driven/queue/redis"
	redisadapter "github.com/sercha-oss/sercha-core/internal/adapters/driven/redis"
	"github.com/sercha-oss/sercha-core/internal/adapters/driving/http"
	mcpadapter "github.com/sercha-oss/sercha-core/internal/adapters/driving/mcp"
	"github.com/sercha-oss/sercha-core/internal/config"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
	"github.com/sercha-oss/sercha-core/internal/core/services"
	"github.com/sercha-oss/sercha-core/internal/normalisers"
	"github.com/sercha-oss/sercha-core/internal/runtime"
	"github.com/sercha-oss/sercha-core/internal/worker"
	"github.com/redis/go-redis/v9"
)

var version = "dev"

// redisPinger wraps a redis.Client to implement the http.Pinger interface
type redisPinger struct {
	client *redis.Client
}

func (r *redisPinger) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}


func main() {
	// Get run mode: environment variable takes precedence, command arg as fallback
	mode := "all"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	if envMode := os.Getenv("RUN_MODE"); envMode != "" {
		mode = envMode
	}

	log.Printf("sercha-core %s starting in %s mode", version, mode)

	// Load configuration from environment
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Runtime configuration
	port := getEnvInt("PORT", 8080)
	redisURL := getEnv("REDIS_URL", "")

	// Single org
	const teamID = "default"

	// Setup context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutdown signal received, stopping...")
		cancel()
	}()

	// ===== Initialize PostgreSQL =====
	log.Println("Connecting to PostgreSQL...")
	dbConfig := postgres.Config{
		URL:             cfg.DatabaseURL,
		MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime: time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME_SEC", 300)) * time.Second,
		ConnMaxIdleTime: time.Duration(getEnvInt("DB_CONN_MAX_IDLE_SEC", 60)) * time.Second,
	}
	db, err := postgres.Connect(ctx, dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Initialize schema (idempotent)
	if err := db.InitSchema(ctx); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}
	log.Println("PostgreSQL connected and schema initialized")

	// ===== Initialize Redis (optional) =====
	var redisClient *redis.Client
	if redisURL != "" {
		log.Println("Connecting to Redis...")
		opts, err := redis.ParseURL(redisURL)
		if err != nil {
			log.Fatalf("Failed to parse Redis URL: %v", err)
		}
		redisClient = redis.NewClient(opts)
		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Fatalf("Failed to connect to Redis: %v", err)
		}
		defer func() { _ = redisClient.Close() }()
		log.Println("Redis connected")
	}

	// ===== Search Engine (OpenSearch if configured) =====
	var searchEngine driven.SearchEngine = nil
	if cfg.OpenSearchURL != "" {
		log.Println("Initializing OpenSearch search engine...")
		osConfig := opensearch.DefaultConfig()
		osConfig.URL = cfg.OpenSearchURL
		osEngine, err := opensearch.NewSearchEngine(osConfig)
		if err != nil {
			log.Fatalf("Failed to initialize OpenSearch: %v", err)
		}
		// Verify connectivity
		if err := osEngine.HealthCheck(ctx); err != nil {
			log.Printf("Warning: OpenSearch health check failed: %v", err)
		} else {
			searchEngine = osEngine
			cfg.SearchEngineAvailable = true
			log.Printf("OpenSearch connected: %s", cfg.OpenSearchURL)
		}
	} else {
		log.Println("Search engine: disabled (OPENSEARCH_URL not configured)")
	}

	// ===== Vector Index (pgvector if configured) =====
	var vectorIndex driven.VectorIndex = nil
	var pgvectorAdapter *pgvector.VectorIndex = nil
	if cfg.PgvectorURL != "" {
		log.Println("Initializing pgvector vector index...")
		pgvConfig := pgvector.DefaultConfig()
		pgvConfig.URL = cfg.PgvectorURL
		pgvConfig.Dimensions = cfg.PgvectorDimensions
		var err error
		pgvectorAdapter, err = pgvector.New(ctx, pgvConfig)
		if err != nil {
			log.Fatalf("Failed to initialize pgvector: %v", err)
		}
		// Verify connectivity and vector extension
		if err := pgvectorAdapter.HealthCheck(ctx); err != nil {
			log.Printf("Warning: pgvector health check failed: %v", err)
			pgvectorAdapter.Close()
			pgvectorAdapter = nil
		} else {
			// Ensure the embeddings table exists
			if err := pgvectorAdapter.EnsureTable(ctx); err != nil {
				log.Fatalf("Failed to ensure pgvector embeddings table: %v", err)
			}
			vectorIndex = pgvectorAdapter
			cfg.VectorStoreAvailable = true
			log.Printf("pgvector connected: %s (dimensions: %d)", cfg.PgvectorURL, cfg.PgvectorDimensions)
		}
	} else {
		log.Println("Vector index: disabled (PGVECTOR_URL not configured)")
	}
	// Ensure pgvector is closed on shutdown
	if pgvectorAdapter != nil {
		defer pgvectorAdapter.Close()
	}

	// ===== Driven adapters (infrastructure) =====
	authAdapter := auth.NewAdapter(cfg.JWTSecret)
	aiFactory := ai.NewFactory()

	// ===== PostgreSQL Stores =====
	userStore := postgres.NewUserStore(db)
	documentStore := postgres.NewDocumentStore(db)
	chunkStore := postgres.NewChunkStore(db)
	sourceStore := postgres.NewSourceStore(db)
	syncStore := postgres.NewSyncStateStore(db)
	settingsStore := postgres.NewSettingsStore(db)
	schedulerStore := postgres.NewSchedulerStore(db)
	capabilityStore := postgres.NewCapabilityStore(db)
	searchQueryRepo := postgres.NewSearchQueryRepository(db)

	// ===== Session Store (Redis if available, otherwise PostgreSQL) =====
	var sessionStore driven.SessionStore
	if redisClient != nil {
		sessionStore = redisadapter.NewSessionStore(redisClient)
		log.Println("Using Redis session store")
	} else {
		sessionStore = postgres.NewSessionStore(db)
		log.Println("Using PostgreSQL session store")
	}

	// ===== Task Queue (Redis if available, otherwise PostgreSQL) =====
	var taskQueue driven.TaskQueue
	if redisClient != nil {
		var err error
		taskQueue, err = redisqueue.NewQueue(redisClient, fmt.Sprintf("worker-%d", os.Getpid()))
		if err != nil {
			log.Fatalf("Failed to create task queue: %v", err)
		}
		log.Println("Using Redis task queue")
	} else {
		taskQueue = postgresqueue.NewQueue(db.DB)
		log.Println("Using PostgreSQL task queue")
	}

	// ===== Distributed Lock (Redis if available, otherwise PostgreSQL advisory locks) =====
	var distributedLock driven.DistributedLock
	if redisClient != nil {
		distributedLock = redisadapter.NewLock(redisClient)
		log.Println("Using Redis distributed lock")
	} else {
		distributedLock = postgres.NewAdvisoryLock(db)
		log.Println("Using PostgreSQL advisory lock")
	}

	// ===== Connector Infrastructure =====
	var connectorFactory driven.ConnectorFactory
	var installationStore driven.ConnectionStore
	var oauthStateStore driven.OAuthStateStore

	// Create secret encryptor (shared by all stores that encrypt secrets)
	encryptor, err := postgres.NewSecretEncryptor(cfg.MasterKey)
	if err != nil {
		log.Fatalf("Failed to create secret encryptor: %v", err)
	}

	// Create stores
	installationStore = postgres.NewConnectionStore(db.DB, encryptor)
	oauthStateStore = postgres.NewOAuthStateStore(db.DB)

	// ===== OAuth 2.0 Authorization Server Stores =====
	oauthClientStore := postgres.NewOAuthClientStore(db.DB)
	authCodeStore := postgres.NewAuthorizationCodeStore(db.DB)
	oauthTokenStore := postgres.NewOAuthTokenStore(db.DB)

	// Create token provider factory
	tokenProviderFactory := auth.NewTokenProviderFactory(installationStore)

	// Register token refreshers for each platform type
	// These use OAuth credentials from environment variables via ConfigProvider
	tokenProviderFactory.RegisterRefresher(domain.PlatformGitHub, func(ctx context.Context, refreshToken string) (*driven.OAuthToken, error) {
		creds := cfg.GetOAuthCredentials(domain.PlatformGitHub)
		if creds == nil {
			return nil, fmt.Errorf("github provider not configured - set GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET")
		}
		return github.NewOAuthHandler().RefreshToken(ctx, creds.ClientID, creds.ClientSecret, refreshToken)
	})

	// Create connector factory
	factory := connectors.NewFactory(tokenProviderFactory)

	// Register GitHub connector
	factory.Register(github.NewBuilder())
	factory.RegisterOAuthHandler(domain.PlatformGitHub, github.NewOAuthHandler())

	// Register LocalFS connector (for testing/development)
	localfsAllowedRoots := []string{"/data", "/tmp"}
	if envRoots := getEnv("LOCALFS_ALLOWED_ROOTS", ""); envRoots != "" {
		localfsAllowedRoots = append(localfsAllowedRoots, envRoots)
	}
	factory.Register(localfs.NewBuilder(localfsAllowedRoots))

	connectorFactory = factory
	log.Printf("Connector infrastructure initialized (providers: %v)", factory.SupportedTypes())
	log.Printf("  OAuth callback URL: %s/api/v1/oauth/callback", cfg.BaseURL)

	// Create OAuth handler factory adapter
	oauthHandlerFactory := connectors.NewOAuthHandlerFactory(factory)

	// Create container lister factory for installation management
	containerListerFactory := connectors.NewContainerListerFactory()
	// Register GitHub container lister factory
	containerListerFactory.Register(domain.ProviderTypeGitHub,
		github.NewContainerListerFactory(installationStore, tokenProviderFactory, ""))

	// Register LocalFS container lister factory
	containerListerFactory.Register(domain.ProviderTypeLocalFS,
		localfs.NewContainerListerFactory(installationStore))

	// Runtime configuration
	sessionBackend := "postgres"
	if getEnv("REDIS_URL", "") != "" {
		sessionBackend = "redis"
	}
	runtimeConfig := domain.NewRuntimeConfig(sessionBackend)
	runtimeServices := runtime.NewServices(runtimeConfig)

	// Initialize registries (shared across all modes)
	normaliserRegistry := normalisers.DefaultRegistry()

	// ===== Pipeline Infrastructure =====
	log.Println("Initializing pipeline infrastructure...")

	// Create registries
	stageRegistry := pipelinereg.NewStageRegistry()
	pipelineRegistry := pipelinereg.NewPipelineRegistry()
	capabilityRegistry := pipelinereg.NewCapabilityRegistry()

	// Register indexing stage factories
	if err := stageRegistry.Register(indexingstages.NewChunkerFactory()); err != nil {
		log.Fatalf("Failed to register chunker stage: %v", err)
	}
	if err := stageRegistry.Register(indexingstages.NewEmbedderFactory()); err != nil {
		log.Fatalf("Failed to register embedder stage: %v", err)
	}
	if err := stageRegistry.Register(indexingstages.NewDocLoaderFactory()); err != nil {
		log.Fatalf("Failed to register doc-loader stage: %v", err)
	}
	if err := stageRegistry.Register(indexingstages.NewVectorLoaderFactory()); err != nil {
		log.Fatalf("Failed to register vector-loader stage: %v", err)
	}

	// Register search stage factories
	if err := stageRegistry.Register(searchstages.NewQueryParserFactory()); err != nil {
		log.Fatalf("Failed to register query-parser stage: %v", err)
	}
	if err := stageRegistry.Register(searchstages.NewBM25RetrieverFactory()); err != nil {
		log.Fatalf("Failed to register bm25-retriever stage: %v", err)
	}
	if err := stageRegistry.Register(searchstages.NewVectorRetrieverFactory()); err != nil {
		log.Fatalf("Failed to register vector-retriever stage: %v", err)
	}
	if err := stageRegistry.Register(searchstages.NewHybridRetrieverFactory()); err != nil {
		log.Fatalf("Failed to register hybrid-retriever stage: %v", err)
	}
	if err := stageRegistry.Register(searchstages.NewRankerFactory()); err != nil {
		log.Fatalf("Failed to register ranker stage: %v", err)
	}
	if err := stageRegistry.Register(searchstages.NewPresenterFactory()); err != nil {
		log.Fatalf("Failed to register presenter stage: %v", err)
	}

	// Register capability providers
	// Embedder - dynamically available via runtimeServices
	// The instance is resolved at runtime when needed
	if err := capabilityRegistry.Register(&providers.CapabilityProvider{
		CapType:    pipeline.CapabilityEmbedder,
		ProviderID: "default",
		InstanceResolver: func() any {
			return runtimeServices.EmbeddingService()
		},
		AvailFn: func() bool {
			return runtimeServices.EmbeddingService() != nil
		},
	}); err != nil {
		log.Fatalf("Failed to register embedder capability: %v", err)
	}

	// SearchEngine - OpenSearch if configured
	if searchEngine != nil {
		if err := capabilityRegistry.Register(&providers.CapabilityProvider{
			CapType:    pipeline.CapabilitySearchEngine,
			ProviderID: "opensearch",
			Inst:       searchEngine,
			AvailFn:    func() bool { return true },
		}); err != nil {
			log.Fatalf("Failed to register opensearch capability: %v", err)
		}
		log.Println("Registered opensearch as search_engine capability")
	}

	// VectorStore - pgvector if configured
	if vectorIndex != nil {
		if err := capabilityRegistry.Register(&providers.CapabilityProvider{
			CapType:    pipeline.CapabilityVectorStore,
			ProviderID: "pgvector",
			Inst:       vectorIndex,
			AvailFn: func() bool {
				return vectorIndex != nil
			},
		}); err != nil {
			log.Fatalf("Failed to register pgvector capability: %v", err)
		}
		log.Println("Registered pgvector as vector_store capability")
	}

	// Register default indexing pipeline
	indexingPipeline := pipeline.PipelineDefinition{
		ID:   "default-indexing",
		Name: "Default Indexing Pipeline",
		Type: pipeline.PipelineTypeIndexing,
		Stages: []pipeline.StageConfig{
			{StageID: "doc-loader", Enabled: true},
			{StageID: "chunker", Enabled: true, Parameters: map[string]any{"chunk_size": 1024, "chunk_overlap": 100}},
			{StageID: "embedder", Enabled: true},
			{StageID: "vector-loader", Enabled: true},
		},
	}
	if err := pipelineRegistry.Register(indexingPipeline); err != nil {
		log.Fatalf("Failed to register indexing pipeline: %v", err)
	}
	if err := pipelineRegistry.SetDefault(pipeline.PipelineTypeIndexing, "default-indexing"); err != nil {
		log.Fatalf("Failed to set default indexing pipeline: %v", err)
	}

	// Register default search pipeline with all retriever variants.
	// applyPreferences enables exactly one retriever based on the requested search mode.
	searchPipeline := pipeline.PipelineDefinition{
		ID:   "default-search",
		Name: "Default Search Pipeline",
		Type: pipeline.PipelineTypeSearch,
		Stages: []pipeline.StageConfig{
			{StageID: "query-parser", Enabled: true},
			{StageID: "bm25-retriever", Enabled: true, Parameters: map[string]any{"top_k": 100}},
			{StageID: "vector-retriever", Enabled: true, Parameters: map[string]any{"top_k": 100}},
			{StageID: "hybrid-retriever", Enabled: true, Parameters: map[string]any{"top_k": 100}},
			{StageID: "ranker", Enabled: true, Parameters: map[string]any{"limit": 100}},
			{StageID: "presenter", Enabled: true, Parameters: map[string]any{"snippet_length": 200}},
		},
	}
	if err := pipelineRegistry.Register(searchPipeline); err != nil {
		log.Fatalf("Failed to register search pipeline: %v", err)
	}
	if err := pipelineRegistry.SetDefault(pipeline.PipelineTypeSearch, "default-search"); err != nil {
		log.Fatalf("Failed to set default search pipeline: %v", err)
	}

	// Create pipeline builder and executors
	pipelineBuilder := pipelineexec.NewPipelineBuilder(stageRegistry)
	indexingExecutor := pipelineexec.NewIndexingExecutor(pipelineBuilder, pipelineRegistry, capabilityRegistry, nil, stageRegistry)
	searchExecutor := pipelineexec.NewSearchExecutor(pipelineBuilder, pipelineRegistry, capabilityRegistry, stageRegistry)

	log.Println("Pipeline infrastructure initialized")

	// Services (core business logic)
	authService := services.NewAuthService(userStore, sessionStore, authAdapter)
	userService := services.NewUserService(userStore, sessionStore, authAdapter, teamID)
	sourceService := services.NewSourceService(sourceStore, documentStore, syncStore, searchEngine)
	documentService := services.NewDocumentService(documentStore, searchEngine)
	searchService := services.NewSearchService(searchEngine, documentStore, runtimeServices, searchExecutor, capabilityStore, settingsStore, teamID)
	settingsService := services.NewSettingsService(settingsStore, aiFactory, cfg, runtimeServices, teamID)

	// Restore AI services from saved settings (embedding, LLM)
	// This ensures services are available after a restart without requiring
	// the user to re-configure via the API.
	if err := settingsService.RestoreAIServices(ctx); err != nil {
		log.Printf("Warning: failed to restore AI services: %v", err)
	} else {
		if runtimeServices.EmbeddingService() != nil {
			log.Println("Restored embedding service from saved settings")
		}
		if runtimeServices.LLMService() != nil {
			log.Println("Restored LLM service from saved settings")
		}
	}

	setupService := services.NewSetupService(userStore, sourceStore, teamID)

	// Provider service (shows configuration status based on env vars)
	providerService := services.NewProviderService(cfg)

	// Capabilities service
	capabilitiesService := services.NewCapabilitiesService(cfg, capabilityStore)

	// OAuth service (handles OAuth flows for connector installations)
	oauthService := services.NewOAuthService(services.OAuthServiceConfig{
		ConfigProvider:      cfg,
		OAuthHandlerFactory: oauthHandlerFactory,
		OAuthStateStore:     oauthStateStore,
		ConnectionStore:     installationStore,
	})

	// Connection service (manages connector connections)
	connectionService := services.NewConnectionService(services.ConnectionServiceConfig{
		ConnectionStore:        installationStore,
		SourceStore:            sourceStore,
		ContainerListerFactory: containerListerFactory,
		TokenProviderFactory:   tokenProviderFactory,
	})

	// Admin service (admin dashboard operations)
	adminService := services.NewAdminService(
		taskQueue,
		schedulerStore,
		searchQueryRepo,
		sourceStore,
	)

	// ===== OAuth 2.0 Authorization Server + MCP =====
	mcpServerURL := getEnv("MCP_SERVER_URL", cfg.BaseURL+"/mcp")
	mcpEnabled := getEnvBool("MCP_ENABLED", true)

	var oauthServerService driving.OAuthServerService
	var mcpHandler stdhttp.Handler
	if mcpEnabled {
		oauthServerService = services.NewOAuthServerService(services.OAuthServerServiceConfig{
			ClientStore:  oauthClientStore,
			CodeStore:    authCodeStore,
			TokenStore:   oauthTokenStore,
			JWTSecret:    cfg.JWTSecret,
			MCPServerURL: mcpServerURL,
		})

		mcpServer := mcpadapter.NewMCPServer(mcpadapter.MCPServerConfig{
			SearchService:   searchService,
			DocumentService: documentService,
			SourceService:   sourceService,
			OAuthService:    oauthServerService,
			MCPServerURL:    mcpServerURL,
			Version:         version,
		})

		mcpHandler = mcpadapter.NewHTTPHandler(mcpServer, oauthServerService, mcpServerURL)
		log.Printf("MCP server enabled at %s", mcpServerURL)
	} else {
		log.Println("MCP server disabled via MCP_ENABLED=false")
	}

	// Log startup configuration
	log.Printf("Runtime config: session_backend=%s, embedding=%t, llm=%t, search_mode=%s",
		runtimeConfig.SessionBackend,
		runtimeConfig.EmbeddingAvailable(),
		runtimeConfig.LLMAvailable(),
		runtimeConfig.EffectiveSearchMode())

	// Create sync orchestrator for worker mode
	syncOrchestrator := services.NewSyncOrchestrator(services.SyncOrchestratorConfig{
		SourceStore:      sourceStore,
		DocumentStore:    documentStore,
		ChunkStore:       chunkStore,
		SyncStore:        syncStore,
		SearchEngine:     searchEngine,
		VectorIndex:      vectorIndex,
		ConnectorFactory: connectorFactory,
		NormaliserReg:    normaliserRegistry,
		Services:         runtimeServices,
		Logger:           slog.Default(),
		IndexingExecutor: indexingExecutor,
		CapabilitySet:    nil, // Built per-execution by executor
		CapabilityStore:  capabilityStore,
		SettingsStore:    settingsStore,
		TeamID:           teamID,
	})

	// Create scheduler for worker mode (if enabled)
	schedulerEnabled := getEnvBool("SCHEDULER_ENABLED", true)
	schedulerLockRequired := getEnvBool("SCHEDULER_LOCK_REQUIRED", true)

	var scheduler *services.Scheduler
	if schedulerEnabled {
		scheduler = services.NewScheduler(services.SchedulerConfig{
			Store:        schedulerStore,
			TaskQueue:    taskQueue,
			Lock:         distributedLock,
			Logger:       slog.Default(),
			LockRequired: schedulerLockRequired,
		})
		log.Printf("Scheduler enabled (lock_required=%t)", schedulerLockRequired)
	} else {
		log.Println("Scheduler disabled via SCHEDULER_ENABLED=false")
	}

	switch mode {
	case "api":
		// API-only mode: HTTP server, no worker
		var redisPing http.Pinger
		if redisClient != nil {
			redisPing = &redisPinger{client: redisClient}
		}
		runAPI(port, authService, userService, searchService, sourceService, documentService, settingsService, providerService, oauthService, connectionService, syncOrchestrator, capabilitiesService, setupService, adminService, taskQueue, searchQueryRepo, db, redisPing, oauthServerService, mcpHandler)

	case "worker":
		// Worker-only mode: Task processing, scheduler, no HTTP server
		runWorkerMode(ctx, taskQueue, syncOrchestrator, scheduler)

	case "all":
		// Combined mode: Run both API and Worker
		// Start worker in background
		go runWorkerMode(ctx, taskQueue, syncOrchestrator, scheduler)
		// Run API in foreground (blocks)
		var redisPing http.Pinger
		if redisClient != nil {
			redisPing = &redisPinger{client: redisClient}
		}
		runAPI(port, authService, userService, searchService, sourceService, documentService, settingsService, providerService, oauthService, connectionService, syncOrchestrator, capabilitiesService, setupService, adminService, taskQueue, searchQueryRepo, db, redisPing, oauthServerService, mcpHandler)

	default:
		log.Fatalf("Unknown mode: %s (use: api, worker, or all)", mode)
	}
}

func runAPI(
	port int,
	authService driving.AuthService,
	userService driving.UserService,
	searchService driving.SearchService,
	sourceService driving.SourceService,
	documentService driving.DocumentService,
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
	db http.Pinger,
	redisClient http.Pinger, // can be nil
	oauthServerService driving.OAuthServerService, // can be nil
	mcpHandler stdhttp.Handler, // can be nil
) {
	// Parse CORS origins from environment variable
	corsOrigins := parseCORSOrigins(getEnv("CORS_ORIGINS", "*"))

	cfg := http.Config{
		Host:        "0.0.0.0",
		Port:        port,
		Version:     version,
		CORSOrigins: corsOrigins,
		UIBaseURL:   getEnv("UI_BASE_URL", "http://localhost:3000"),
	}

	server := http.NewServer(
		cfg,
		authService,
		userService,
		searchService,
		sourceService,
		documentService,
		settingsService,
		providerService,
		oauthService,
		connectionService,
		syncOrchestrator,
		capabilitiesService,
		setupService,
		adminService,
		taskQueue,
		searchQueryRepo,
		db,
		redisClient,
		oauthServerService,
		mcpHandler,
	)

	log.Printf("API server starting on :%d", port)
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// runWorkerMode starts the worker and scheduler.
// It processes tasks from the queue and runs scheduled syncs.
func runWorkerMode(
	ctx context.Context,
	taskQueue driven.TaskQueue,
	orchestrator *services.SyncOrchestrator,
	scheduler *services.Scheduler,
) {
	log.Println("Starting worker mode...")

	// Create worker
	w := worker.NewWorker(worker.WorkerConfig{
		TaskQueue:      taskQueue,
		Orchestrator:   orchestrator,
		Scheduler:      scheduler,
		Logger:         slog.Default(),
		Concurrency:    getEnvInt("WORKER_CONCURRENCY", 2),
		DequeueTimeout: getEnvInt("WORKER_DEQUEUE_TIMEOUT", 5),
	})

	// Start worker
	if err := w.Start(ctx); err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}

	log.Println("Worker started, processing tasks...")
	log.Println("Worker handles:")
	log.Println("  - sync_source: Sync a specific source")
	log.Println("  - sync_all: Sync all enabled sources")

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	log.Println("Stopping worker...")
	w.Stop()
	log.Println("Worker stopped")
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}

func parseCORSOrigins(value string) []string {
	if value == "" {
		return nil
	}
	// Split by comma and trim whitespace
	parts := strings.Split(value, ",")
	origins := []string{}
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}
