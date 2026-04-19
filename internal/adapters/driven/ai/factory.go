package ai

import (
	"fmt"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure Factory implements AIServiceFactory
var _ driven.AIServiceFactory = (*Factory)(nil)

// Factory creates AI services based on configuration
type Factory struct{}

// NewFactory creates a new AI service factory
func NewFactory() *Factory {
	return &Factory{}
}

// CreateEmbeddingService creates an embedding service from settings and credentials
func (f *Factory) CreateEmbeddingService(settings *domain.EmbeddingSettings, credentials *driven.AICredentials) (driven.EmbeddingService, error) {
	if settings == nil || !settings.IsConfigured() {
		return nil, nil
	}

	if credentials == nil {
		return nil, fmt.Errorf("credentials required for provider %s", settings.Provider)
	}

	switch settings.Provider {
	case domain.AIProviderOpenAI:
		return NewOpenAIEmbedding(credentials.APIKey, settings.Model, credentials.BaseURL)
	case domain.AIProviderOllama:
		return NewOllamaEmbedding(credentials.BaseURL, settings.Model)
	case domain.AIProviderVoyage:
		return NewVoyageEmbedding(credentials.APIKey, settings.Model)
	case domain.AIProviderCohere:
		return NewCohereEmbedding(credentials.APIKey, settings.Model)
	default:
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidProvider, settings.Provider)
	}
}

// CreateLLMService creates an LLM service from settings and credentials
func (f *Factory) CreateLLMService(settings *domain.LLMSettings, credentials *driven.AICredentials) (driven.LLMService, error) {
	if settings == nil || !settings.IsConfigured() {
		return nil, nil
	}

	if credentials == nil {
		return nil, fmt.Errorf("credentials required for provider %s", settings.Provider)
	}

	switch settings.Provider {
	case domain.AIProviderOpenAI:
		return NewOpenAILLM(credentials.APIKey, settings.Model, credentials.BaseURL)
	case domain.AIProviderAnthropic:
		return NewAnthropicLLM(credentials.APIKey, settings.Model)
	case domain.AIProviderOllama:
		return NewOllamaLLM(credentials.BaseURL, settings.Model)
	default:
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidProvider, settings.Provider)
	}
}

// Placeholder constructors - these will be replaced with actual implementations
// Note: NewOpenAIEmbedding is implemented in openai_embedding.go

func NewOllamaEmbedding(baseURL, model string) (driven.EmbeddingService, error) {
	// TODO: Implement Ollama embedding adapter
	return nil, fmt.Errorf("ollama embedding adapter not yet implemented")
}

func NewVoyageEmbedding(apiKey, model string) (driven.EmbeddingService, error) {
	// TODO: Implement Voyage embedding adapter
	return nil, fmt.Errorf("voyage embedding adapter not yet implemented")
}

func NewCohereEmbedding(apiKey, model string) (driven.EmbeddingService, error) {
	// TODO: Implement Cohere embedding adapter
	return nil, fmt.Errorf("cohere embedding adapter not yet implemented")
}

func NewAnthropicLLM(apiKey, model string) (driven.LLMService, error) {
	// TODO: Implement Anthropic LLM adapter
	return nil, fmt.Errorf("anthropic LLM adapter not yet implemented")
}

func NewOllamaLLM(baseURL, model string) (driven.LLMService, error) {
	// TODO: Implement Ollama LLM adapter
	return nil, fmt.Errorf("ollama LLM adapter not yet implemented")
}
