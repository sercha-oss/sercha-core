package config

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure Config implements ConfigProvider
var _ driven.ConfigProvider = (*Config)(nil)

// Config holds application configuration from environment variables
type Config struct {
	// Required configuration
	DatabaseURL string
	JWTSecret   string
	MasterKey   []byte
	BaseURL     string

	// OAuth provider credentials
	oauthCredentials map[domain.ProviderType]*driven.OAuthCredentials

	// AI provider credentials
	aiCredentials map[domain.AIProvider]*driven.AICredentials

	// Operational limits
	limits driven.OperationalLimits
}

// Load reads and validates configuration from environment variables.
// Returns error if required variables are missing or invalid.
func Load() (*Config, error) {
	cfg := &Config{
		oauthCredentials: make(map[domain.ProviderType]*driven.OAuthCredentials),
		aiCredentials:    make(map[domain.AIProvider]*driven.AICredentials),
	}

	// Required variables
	requiredVars := map[string]*string{
		"DATABASE_URL": &cfg.DatabaseURL,
		"JWT_SECRET":   &cfg.JWTSecret,
	}

	var missing []string
	for key, dest := range requiredVars {
		value := os.Getenv(key)
		if value == "" {
			missing = append(missing, key)
		}
		*dest = value
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("required environment variables missing: %s", strings.Join(missing, ", "))
	}

	// Validate JWT_SECRET (must be 64 hex chars)
	if len(cfg.JWTSecret) != 64 {
		return nil, fmt.Errorf("JWT_SECRET must be 64 hex characters (32 bytes), got %d characters", len(cfg.JWTSecret))
	}
	if _, err := hex.DecodeString(cfg.JWTSecret); err != nil {
		return nil, fmt.Errorf("JWT_SECRET must be valid hex: %w", err)
	}

	// MASTER_KEY (required, must be 64 hex chars)
	masterKeyHex := os.Getenv("MASTER_KEY")
	if masterKeyHex == "" {
		return nil, fmt.Errorf("MASTER_KEY environment variable is required (64 hex characters = 32 bytes)")
	}
	masterKey, err := hex.DecodeString(masterKeyHex)
	if err != nil || len(masterKey) != 32 {
		return nil, fmt.Errorf("MASTER_KEY must be 64 hex characters (32 bytes), got %d bytes", len(masterKey))
	}
	cfg.MasterKey = masterKey

	// BASE_URL (optional, defaults based on PORT)
	cfg.BaseURL = os.Getenv("BASE_URL")
	if cfg.BaseURL == "" {
		port := getEnvInt("PORT", 8080)
		cfg.BaseURL = fmt.Sprintf("http://localhost:%d", port)
	}

	// Load OAuth credentials (all optional)
	cfg.loadOAuthCredentials()

	// Load AI credentials (all optional)
	cfg.loadAICredentials()

	// Load operational limits
	cfg.limits = driven.OperationalLimits{
		SyncMinInterval:   getEnvInt("SYNC_MIN_INTERVAL", 5),
		SyncMaxInterval:   getEnvInt("SYNC_MAX_INTERVAL", 1440),
		MaxWorkers:        getEnvInt("MAX_WORKERS", 10),
		MaxResultsPerPage: getEnvInt("MAX_RESULTS_PER_PAGE", 100),
	}

	log.Println("Configuration loaded successfully")
	cfg.logConfiguredProviders()

	return cfg, nil
}

// loadOAuthCredentials loads OAuth credentials for all supported providers
func (c *Config) loadOAuthCredentials() {
	providers := []struct {
		providerType domain.ProviderType
		clientIDKey  string
		secretKey    string
	}{
		{domain.ProviderTypeGitHub, "GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET"},
		{domain.ProviderTypeGitLab, "GITLAB_CLIENT_ID", "GITLAB_CLIENT_SECRET"},
		{domain.ProviderTypeSlack, "SLACK_CLIENT_ID", "SLACK_CLIENT_SECRET"},
		{domain.ProviderTypeNotion, "NOTION_CLIENT_ID", "NOTION_CLIENT_SECRET"},
		{domain.ProviderTypeConfluence, "CONFLUENCE_CLIENT_ID", "CONFLUENCE_CLIENT_SECRET"},
		{domain.ProviderTypeJira, "JIRA_CLIENT_ID", "JIRA_CLIENT_SECRET"},
		{domain.ProviderTypeGoogleDrive, "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET"},
		{domain.ProviderTypeGoogleDocs, "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET"}, // Same as Drive
		{domain.ProviderTypeLinear, "LINEAR_CLIENT_ID", "LINEAR_CLIENT_SECRET"},
		{domain.ProviderTypeDropbox, "DROPBOX_CLIENT_ID", "DROPBOX_CLIENT_SECRET"},
	}

	for _, p := range providers {
		clientID := os.Getenv(p.clientIDKey)
		clientSecret := os.Getenv(p.secretKey)

		// Skip if not configured
		if clientID == "" && clientSecret == "" {
			continue
		}

		// Warn if partially configured
		if clientID == "" || clientSecret == "" {
			log.Printf("Warning: %s partially configured (missing %s or %s), skipping",
				p.providerType, p.clientIDKey, p.secretKey)
			continue
		}

		c.oauthCredentials[p.providerType] = &driven.OAuthCredentials{
			ClientID:     clientID,
			ClientSecret: clientSecret,
		}
	}
}

// loadAICredentials loads AI provider credentials
func (c *Config) loadAICredentials() {
	// OpenAI
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		c.aiCredentials[domain.AIProviderOpenAI] = &driven.AICredentials{
			APIKey:  apiKey,
			BaseURL: getEnvDefault("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		}
	}

	// Anthropic
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		c.aiCredentials[domain.AIProviderAnthropic] = &driven.AICredentials{
			APIKey:  apiKey,
			BaseURL: getEnvDefault("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),
		}
	}

	// Ollama (no API key needed, just base URL)
	if baseURL := os.Getenv("OLLAMA_BASE_URL"); baseURL != "" {
		c.aiCredentials[domain.AIProviderOllama] = &driven.AICredentials{
			APIKey:  "", // Not needed for Ollama
			BaseURL: baseURL,
		}
	}

	// Cohere
	if apiKey := os.Getenv("COHERE_API_KEY"); apiKey != "" {
		c.aiCredentials[domain.AIProviderCohere] = &driven.AICredentials{
			APIKey:  apiKey,
			BaseURL: getEnvDefault("COHERE_BASE_URL", "https://api.cohere.ai"),
		}
	}

	// Voyage
	if apiKey := os.Getenv("VOYAGE_API_KEY"); apiKey != "" {
		c.aiCredentials[domain.AIProviderVoyage] = &driven.AICredentials{
			APIKey:  apiKey,
			BaseURL: getEnvDefault("VOYAGE_BASE_URL", "https://api.voyageai.com/v1"),
		}
	}
}

// logConfiguredProviders logs which providers are configured
func (c *Config) logConfiguredProviders() {
	if len(c.oauthCredentials) > 0 {
		var providers []string
		for pt := range c.oauthCredentials {
			providers = append(providers, string(pt))
		}
		log.Printf("OAuth providers configured: %s", strings.Join(providers, ", "))
	} else {
		log.Println("No OAuth providers configured")
	}

	if len(c.aiCredentials) > 0 {
		var providers []string
		for ap := range c.aiCredentials {
			providers = append(providers, string(ap))
		}
		log.Printf("AI providers configured: %s", strings.Join(providers, ", "))
	} else {
		log.Println("No AI providers configured")
	}
}

// GetOAuthCredentials implements driven.ConfigProvider
func (c *Config) GetOAuthCredentials(provider domain.ProviderType) *driven.OAuthCredentials {
	return c.oauthCredentials[provider]
}

// GetAICredentials implements driven.ConfigProvider
func (c *Config) GetAICredentials(provider domain.AIProvider) *driven.AICredentials {
	return c.aiCredentials[provider]
}

// IsOAuthConfigured implements driven.ConfigProvider
func (c *Config) IsOAuthConfigured(provider domain.ProviderType) bool {
	creds := c.oauthCredentials[provider]
	return creds != nil && creds.ClientID != "" && creds.ClientSecret != ""
}

// IsAIConfigured implements driven.ConfigProvider
func (c *Config) IsAIConfigured(provider domain.AIProvider) bool {
	creds := c.aiCredentials[provider]
	if creds == nil {
		return false
	}
	// Ollama doesn't require API key, just base URL
	if provider == domain.AIProviderOllama {
		return creds.BaseURL != ""
	}
	return creds.APIKey != ""
}

// GetCapabilities implements driven.ConfigProvider
func (c *Config) GetCapabilities() *driven.Capabilities {
	caps := &driven.Capabilities{
		OAuthProviders:     make([]domain.ProviderType, 0),
		EmbeddingProviders: make([]domain.AIProvider, 0),
		LLMProviders:       make([]domain.AIProvider, 0),
		Limits:             c.limits,
	}

	// OAuth providers
	for pt := range c.oauthCredentials {
		caps.OAuthProviders = append(caps.OAuthProviders, pt)
	}

	// AI providers - categorize by capability
	embeddingProviders := []domain.AIProvider{
		domain.AIProviderOpenAI,
		domain.AIProviderOllama,
		domain.AIProviderCohere,
		domain.AIProviderVoyage,
	}
	llmProviders := []domain.AIProvider{
		domain.AIProviderOpenAI,
		domain.AIProviderAnthropic,
		domain.AIProviderOllama,
	}

	for _, provider := range embeddingProviders {
		if c.IsAIConfigured(provider) {
			caps.EmbeddingProviders = append(caps.EmbeddingProviders, provider)
		}
	}

	for _, provider := range llmProviders {
		if c.IsAIConfigured(provider) {
			caps.LLMProviders = append(caps.LLMProviders, provider)
		}
	}

	return caps
}

// GetBaseURL implements driven.ConfigProvider
func (c *Config) GetBaseURL() string {
	return c.BaseURL
}

// Helper functions

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intVal, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("Warning: invalid integer for %s: %v, using default %d", key, err, defaultValue)
		return defaultValue
	}
	return intVal
}

func getEnvDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
