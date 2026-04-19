package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// LLMService provides a generic completion interface for Large Language Models.
// This is a pure provider interface - task-specific logic (query expansion,
// summarization, etc.) is implemented in pipeline stages.
type LLMService interface {
	// Complete sends a completion request to the LLM and returns the response.
	// This is the primary method for all LLM interactions.
	Complete(ctx context.Context, req domain.CompletionRequest) (domain.CompletionResponse, error)

	// Model returns the model name being used (e.g., "gpt-4o", "claude-3-5-sonnet")
	Model() string

	// Ping verifies the LLM service is reachable and properly configured
	Ping(ctx context.Context) error

	// Close releases resources held by the LLM service
	Close() error
}
