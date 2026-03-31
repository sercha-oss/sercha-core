package driven

import (
	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// AIServiceFactory creates AI services based on configuration.
// Credentials are provided separately from domain settings.
type AIServiceFactory interface {
	// CreateEmbeddingService creates an embedding service from settings and credentials.
	// Credentials come from ConfigProvider (environment variables).
	// Returns error if settings are invalid or service creation fails.
	CreateEmbeddingService(settings *domain.EmbeddingSettings, credentials *AICredentials) (EmbeddingService, error)

	// CreateLLMService creates an LLM service from settings and credentials.
	// Credentials come from ConfigProvider (environment variables).
	// Returns error if settings are invalid or service creation fails.
	CreateLLMService(settings *domain.LLMSettings, credentials *AICredentials) (LLMService, error)
}
