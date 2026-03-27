package main

// @title           Sercha Core API
// @version         1.0
// @description     Privacy-focused enterprise search API. Sercha Core provides full-text and semantic search across your connected data sources.

// @contact.name   Sercha OSS
// @contact.url    https://github.com/custodia-labs/sercha-core/issues

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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/custodia-labs/sercha-core/internal/adapters/driven/ai"
	"github.com/custodia-labs/sercha-core/internal/adapters/driven/auth"
	"github.com/custodia-labs/sercha-core/internal/adapters/driven/connectors"
	"github.com/custodia-labs/sercha-core/internal/adapters/driven/connectors/github"
	"github.com/custodia-labs/sercha-core/internal/adapters/driven/connectors/localfs"
	pipelineexec "github.com/custodia-labs/sercha-core/internal/adapters/driven/pipeline/executor"
	pipelinereg "github.com/custodia-labs/sercha-core/internal/adapters/driven/pipeline/registry"
	indexingstages "github.com/custodia-labs/sercha-core/internal/adapters/driven/pipeline/stages/indexing"
	searchstages "github.com/custodia-labs/sercha-core/internal/adapters/driven/pipeline/stages/search"
	"github.com/custodia-labs/sercha-core/internal/adapters/driven/postgres"
	postgresqueue "github.com/custodia-labs/sercha-core/internal/adapters/driven/queue/postgres"
	redisqueue "github.com/custodia-labs/sercha-core/internal/adapters/driven/queue/redis"
	redisadapter "github.com/custodia-labs/sercha-core/internal/adapters/driven/redis"
	"github.com/custodia-labs/sercha-core/internal/adapters/driven/vespa"
	"github.com/custodia-labs/sercha-core/internal/adapters/driving/http"
	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driving"
	"github.com/custodia-labs/sercha-core/internal/core/services"
	"github.com/custodia-labs/sercha-core/internal/normalisers"
	"github.com/custodia-labs/sercha-core/internal/postprocessors"
	"github.com/custodia-labs/sercha-core/internal/runtime"
	"github.com/custodia-labs/sercha-core/internal/worker"
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

// capabilityProvider wraps a service as a capability provider
type capabilityProvider struct {
	capType         pipeline.CapabilityType
	id              string
	instance        any
	avail           func() bool
	instanceResolver func() any // Optional: resolve instance dynamically
}

func (p *capabilityProvider) Type() pipeline.CapabilityType { return p.capType }
func (p *capabilityProvider) ID() string                    { return p.id }
func (p *capabilityProvider) Instance() any {
	if p.instanceResolver != nil {
		return p.instanceResolver()
	}
	return p.instance
}
func (p *capabilityProvider) Available() bool {
	if p.avail == nil {
		return p.Instance() != nil
	}
	return p.avail()
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

	// Configuration from environment
	port := getEnvInt("PORT", 8080)
	databaseURL := getEnv("DATABASE_URL", "postgres://sercha:sercha_dev@localhost:5432/sercha?sslmode=disable")
	redisURL := getEnv("REDIS_URL", "")
	vespaConfigURL := getEnv("VESPA_CONFIG_URL", "http://localhost:19071")      // Config server (deployment)
	vespaContainerURL := getEnv("VESPA_CONTAINER_URL", "http://localhost:8080") // Container cluster (document/search API)
	baseURL := getEnv("BASE_URL", fmt.Sprintf("http://localhost:%d", port))

	// Single org
	const teamID = "default"

	// JWT secret and encryption key - auto-derived if not set
	jwtSecret := getOrGenerateSecret("JWT_SECRET", databaseURL)
	masterKey := getMasterKey(jwtSecret)

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
		URL:             databaseURL,
		MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime: time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME_SEC", 300)) * time.Second,
		ConnMaxIdleTime: time.Duration(getEnvInt("DB_CONN_MAX_IDLE_SEC", 60)) * time.Second,
	}
	db, err := postgres.Connect(ctx, dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

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
		defer redisClient.Close()
		log.Println("Redis connected")
	}

	// ===== Initialize Vespa =====
	log.Println("Connecting to Vespa...")
	searchEngine := vespa.NewSearchEngine(vespa.DefaultConfig(vespaContainerURL))
	if err := searchEngine.HealthCheck(ctx); err != nil {
		log.Printf("Warning: Vespa health check failed: %v (search may not work)", err)
	} else {
		log.Println("Vespa connected")
	}

	// ===== Driven adapters (infrastructure) =====
	authAdapter := auth.NewAdapter(jwtSecret)
	aiFactory := ai.NewFactory()

	// ===== PostgreSQL Stores =====
	userStore := postgres.NewUserStore(db)
	documentStore := postgres.NewDocumentStore(db)
	chunkStore := postgres.NewChunkStore(db)
	sourceStore := postgres.NewSourceStore(db)
	syncStore := postgres.NewSyncStateStore(db)
	settingsStore := postgres.NewSettingsStore(db)
	schedulerStore := postgres.NewSchedulerStore(db)
	vespaConfigStore := postgres.NewVespaConfigStore(db)

	// ===== Vespa Deployer =====
	vespaDeployer := vespa.NewDeployer()

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
	var installationStore driven.InstallationStore
	var oauthStateStore driven.OAuthStateStore
	var providerConfigStore driven.ProviderConfigStore

	// Create secret encryptor (shared by all stores that encrypt secrets)
	encryptor, err := postgres.NewSecretEncryptor(masterKey)
	if err != nil {
		log.Fatalf("Failed to create secret encryptor: %v", err)
	}

	// Create stores
	installationStore = postgres.NewInstallationStore(db.DB, encryptor)
	providerConfigStore = postgres.NewProviderConfigStore(db.DB, encryptor)
	oauthStateStore = postgres.NewOAuthStateStore(db.DB)

	// Create token provider factory
	tokenProviderFactory := auth.NewTokenProviderFactory(installationStore)

	// Register token refreshers for each provider type
	// These dynamically load OAuth credentials from provider_configs table
	tokenProviderFactory.RegisterRefresher(domain.ProviderTypeGitHub, func(ctx context.Context, refreshToken string) (*driven.OAuthToken, error) {
		cfg, err := providerConfigStore.Get(ctx, domain.ProviderTypeGitHub)
		if err != nil {
			return nil, fmt.Errorf("failed to get github provider config: %w", err)
		}
		if cfg == nil || cfg.Secrets == nil || cfg.Secrets.ClientID == "" {
			return nil, fmt.Errorf("github provider not configured - use POST /api/v1/providers/github/config")
		}
		return github.NewOAuthHandler().RefreshToken(ctx, cfg.Secrets.ClientID, cfg.Secrets.ClientSecret, refreshToken)
	})

	// Create connector factory
	factory := connectors.NewFactory(tokenProviderFactory)

	// Register GitHub connector
	factory.Register(github.NewBuilder())
	factory.RegisterOAuthHandler(domain.ProviderTypeGitHub, github.NewOAuthHandler())

	// Register LocalFS connector (for testing/development)
	localfsAllowedRoots := []string{"/data", "/tmp"}
	if envRoots := getEnv("LOCALFS_ALLOWED_ROOTS", ""); envRoots != "" {
		localfsAllowedRoots = append(localfsAllowedRoots, envRoots)
	}
	factory.Register(localfs.NewBuilder(localfsAllowedRoots))

	connectorFactory = factory
	log.Printf("Connector infrastructure initialized (providers: %v)", factory.SupportedTypes())
	log.Printf("  OAuth callback URL: %s/api/v1/oauth/callback", baseURL)

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
	postProcessorPipeline := postprocessors.DefaultPipeline()

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
	if err := stageRegistry.Register(indexingstages.NewLoaderFactory()); err != nil {
		log.Fatalf("Failed to register loader stage: %v", err)
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
	// Vector store (Vespa) - always available
	if err := capabilityRegistry.Register(&capabilityProvider{
		capType:  pipeline.CapabilityVectorStore,
		id:       "vespa",
		instance: searchEngine,
		avail:    func() bool { return true },
	}); err != nil {
		log.Fatalf("Failed to register vector store capability: %v", err)
	}

	// Embedder - dynamically available via runtimeServices
	// The instance is resolved at runtime when needed
	if err := capabilityRegistry.Register(&capabilityProvider{
		capType: pipeline.CapabilityEmbedder,
		id:      "default",
		instanceResolver: func() any {
			return runtimeServices.EmbeddingService()
		},
		avail: func() bool {
			return runtimeServices.EmbeddingService() != nil
		},
	}); err != nil {
		log.Fatalf("Failed to register embedder capability: %v", err)
	}

	// Register default indexing pipeline
	indexingPipeline := pipeline.PipelineDefinition{
		ID:   "default-indexing",
		Name: "Default Indexing Pipeline",
		Type: pipeline.PipelineTypeIndexing,
		Stages: []pipeline.StageConfig{
			{StageID: "chunker", Enabled: true, Parameters: map[string]any{"chunk_size": 512}},
			{StageID: "embedder", Enabled: true},
			{StageID: "loader", Enabled: true},
		},
	}
	if err := pipelineRegistry.Register(indexingPipeline); err != nil {
		log.Fatalf("Failed to register indexing pipeline: %v", err)
	}
	if err := pipelineRegistry.SetDefault(pipeline.PipelineTypeIndexing, "default-indexing"); err != nil {
		log.Fatalf("Failed to set default indexing pipeline: %v", err)
	}

	// Register default search pipeline (BM25-only, no embedding required)
	searchPipelineBM25 := pipeline.PipelineDefinition{
		ID:   "default-search-bm25",
		Name: "Default Search Pipeline (BM25)",
		Type: pipeline.PipelineTypeSearch,
		Stages: []pipeline.StageConfig{
			{StageID: "query-parser", Enabled: true},
			{StageID: "bm25-retriever", Enabled: true, Parameters: map[string]any{"top_k": 100}},
			{StageID: "ranker", Enabled: true, Parameters: map[string]any{"limit": 20}},
			{StageID: "presenter", Enabled: true, Parameters: map[string]any{"snippet_length": 200}},
		},
	}
	if err := pipelineRegistry.Register(searchPipelineBM25); err != nil {
		log.Fatalf("Failed to register search pipeline: %v", err)
	}
	if err := pipelineRegistry.SetDefault(pipeline.PipelineTypeSearch, "default-search-bm25"); err != nil {
		log.Fatalf("Failed to set default search pipeline: %v", err)
	}

	// Create pipeline builder and executors
	pipelineBuilder := pipelineexec.NewPipelineBuilder(stageRegistry)
	indexingExecutor := pipelineexec.NewIndexingExecutor(pipelineBuilder, pipelineRegistry, capabilityRegistry, nil)
	searchExecutor := pipelineexec.NewSearchExecutor(pipelineBuilder, pipelineRegistry, capabilityRegistry)

	log.Println("Pipeline infrastructure initialized")

	// Services (core business logic)
	authService := services.NewAuthService(userStore, sessionStore, authAdapter)
	userService := services.NewUserService(userStore, sessionStore, authAdapter, teamID)
	sourceService := services.NewSourceService(sourceStore, documentStore, syncStore, searchEngine)
	documentService := services.NewDocumentService(documentStore, chunkStore)
	searchService := services.NewSearchService(searchEngine, documentStore, runtimeServices, searchExecutor, nil)
	settingsService := services.NewSettingsService(settingsStore, aiFactory, runtimeServices, teamID)
	vespaAdminService := services.NewVespaAdminService(vespaDeployer, vespaConfigStore, settingsStore, searchEngine, runtimeServices, teamID, vespaConfigURL)

	// Provider service (only available when MASTER_KEY is set)
	var providerService driving.ProviderService
	if providerConfigStore != nil {
		providerService = services.NewProviderService(providerConfigStore)
	}

	// OAuth service (handles OAuth flows for connector installations)
	oauthService := services.NewOAuthService(services.OAuthServiceConfig{
		ProviderConfigStore: providerConfigStore,
		OAuthStateStore:     oauthStateStore,
		InstallationStore:   installationStore,
		ConnectorFactory:    factory,
		BaseURL:             baseURL,
	})

	// Installation service (manages connector installations)
	installationService := services.NewInstallationService(services.InstallationServiceConfig{
		InstallationStore:      installationStore,
		SourceStore:            sourceStore,
		ContainerListerFactory: containerListerFactory,
		TokenProviderFactory:   tokenProviderFactory,
	})

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
		ConnectorFactory: connectorFactory,
		NormaliserReg:    normaliserRegistry,
		LegacyPipeline:   postProcessorPipeline,
		Services:         runtimeServices,
		Logger:           slog.Default(),
		IndexingExecutor: indexingExecutor,
		CapabilitySet:    nil, // Built per-execution by executor
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
		runAPI(port, authService, userService, searchService, sourceService, documentService, settingsService, vespaAdminService, providerService, oauthService, installationService, syncOrchestrator, taskQueue, db, redisPing)

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
		runAPI(port, authService, userService, searchService, sourceService, documentService, settingsService, vespaAdminService, providerService, oauthService, installationService, syncOrchestrator, taskQueue, db, redisPing)

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
	vespaAdminService driving.VespaAdminService,
	providerService driving.ProviderService,
	oauthService driving.OAuthService,
	installationService driving.InstallationService,
	syncOrchestrator driving.SyncOrchestrator,
	taskQueue driven.TaskQueue,
	db http.Pinger,
	redisClient http.Pinger, // can be nil
) {
	cfg := http.Config{
		Host:    "0.0.0.0",
		Port:    port,
		Version: version,
	}

	server := http.NewServer(
		cfg,
		authService,
		userService,
		searchService,
		sourceService,
		documentService,
		settingsService,
		vespaAdminService,
		providerService,
		oauthService,
		installationService,
		syncOrchestrator,
		taskQueue,
		db,
		redisClient,
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

// getOrGenerateSecret returns the JWT secret from env var or derives one from database URL.
// This allows the app to "just work" without requiring explicit configuration.
// The derived secret is stable across restarts (based on database URL).
func getOrGenerateSecret(envKey, databaseURL string) string {
	if secret := os.Getenv(envKey); secret != "" {
		return secret
	}

	// Derive a stable secret from database URL - unique per installation
	hash := sha256.Sum256([]byte("sercha-jwt-secret:" + databaseURL))
	derived := hex.EncodeToString(hash[:])
	log.Printf("Note: %s not set, using auto-derived secret (stable across restarts)", envKey)
	return derived
}

// getMasterKey returns a 32-byte encryption key for secrets.
// If MASTER_KEY env var is set (64 hex chars), it's decoded and used directly.
// Otherwise, derives a key from JWT_SECRET using SHA-256.
func getMasterKey(jwtSecret string) []byte {
	if masterKeyHex := os.Getenv("MASTER_KEY"); masterKeyHex != "" {
		masterKey, err := hex.DecodeString(masterKeyHex)
		if err != nil || len(masterKey) != 32 {
			log.Fatalf("MASTER_KEY must be 64 hex characters (32 bytes): got %d bytes", len(masterKey))
		}
		return masterKey
	}

	// Derive from JWT_SECRET
	hash := sha256.Sum256([]byte("sercha-master-key:" + jwtSecret))
	return hash[:]
}
