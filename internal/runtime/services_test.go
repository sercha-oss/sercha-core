package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// mockEmbeddingService is a mock implementation for testing
type mockEmbeddingService struct {
	healthCheckErr error
	closed         bool
}

func (m *mockEmbeddingService) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, nil
}

func (m *mockEmbeddingService) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return nil, nil
}

func (m *mockEmbeddingService) Dimensions() int {
	return 384
}

func (m *mockEmbeddingService) Model() string {
	return "test-model"
}

func (m *mockEmbeddingService) HealthCheck(ctx context.Context) error {
	return m.healthCheckErr
}

func (m *mockEmbeddingService) Close() error {
	m.closed = true
	return nil
}

// mockLLMService is a mock implementation for testing
type mockLLMService struct {
	pingErr error
	closed  bool
}

func (m *mockLLMService) Complete(ctx context.Context, req domain.CompletionRequest) (domain.CompletionResponse, error) {
	return domain.CompletionResponse{
		Content: "mock response",
		Usage: domain.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}, nil
}

func (m *mockLLMService) Model() string {
	return "test-llm"
}

func (m *mockLLMService) Ping(ctx context.Context) error {
	return m.pingErr
}

func (m *mockLLMService) Close() error {
	m.closed = true
	return nil
}

func TestNewServices(t *testing.T) {
	config := domain.NewRuntimeConfig("postgres")
	services := NewServices(config)

	if services == nil {
		t.Fatal("expected non-nil services")
	}
	if services.Config() != config {
		t.Error("expected config to match")
	}
}

func TestServices_EmbeddingService(t *testing.T) {
	config := domain.NewRuntimeConfig("postgres")
	services := NewServices(config)

	// Initially nil
	if services.EmbeddingService() != nil {
		t.Error("expected nil embedding service initially")
	}

	// Set embedding service
	mock := &mockEmbeddingService{}
	services.SetEmbeddingService(mock)

	if services.EmbeddingService() == nil {
		t.Error("expected non-nil embedding service after set")
	}
	if !config.EmbeddingAvailable() {
		t.Error("expected embedding to be available")
	}

	// Set to nil
	services.SetEmbeddingService(nil)
	if services.EmbeddingService() != nil {
		t.Error("expected nil embedding service after clearing")
	}
	if config.EmbeddingAvailable() {
		t.Error("expected embedding to be unavailable")
	}
	if !mock.closed {
		t.Error("expected old service to be closed")
	}
}

func TestServices_LLMService(t *testing.T) {
	config := domain.NewRuntimeConfig("postgres")
	services := NewServices(config)

	// Initially nil
	if services.LLMService() != nil {
		t.Error("expected nil LLM service initially")
	}

	// Set LLM service
	mock := &mockLLMService{}
	services.SetLLMService(mock)

	if services.LLMService() == nil {
		t.Error("expected non-nil LLM service after set")
	}
	if !config.LLMAvailable() {
		t.Error("expected LLM to be available")
	}

	// Set to nil
	services.SetLLMService(nil)
	if services.LLMService() != nil {
		t.Error("expected nil LLM service after clearing")
	}
	if config.LLMAvailable() {
		t.Error("expected LLM to be unavailable")
	}
	if !mock.closed {
		t.Error("expected old service to be closed")
	}
}

func TestServices_ValidateAndSetEmbedding(t *testing.T) {
	config := domain.NewRuntimeConfig("postgres")
	services := NewServices(config)
	ctx := context.Background()

	t.Run("successful validation", func(t *testing.T) {
		mock := &mockEmbeddingService{}
		err := services.ValidateAndSetEmbedding(ctx, mock)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if services.EmbeddingService() == nil {
			t.Error("expected embedding service to be set")
		}
	})

	t.Run("failed validation", func(t *testing.T) {
		mock := &mockEmbeddingService{healthCheckErr: errors.New("connection failed")}
		err := services.ValidateAndSetEmbedding(ctx, mock)
		if err == nil {
			t.Error("expected error")
		}
		if !mock.closed {
			t.Error("expected failed service to be closed")
		}
	})

	t.Run("nil service", func(t *testing.T) {
		err := services.ValidateAndSetEmbedding(ctx, nil)
		if err != nil {
			t.Errorf("unexpected error for nil service: %v", err)
		}
	})
}

func TestServices_ValidateAndSetLLM(t *testing.T) {
	config := domain.NewRuntimeConfig("postgres")
	services := NewServices(config)
	ctx := context.Background()

	t.Run("successful validation", func(t *testing.T) {
		mock := &mockLLMService{}
		err := services.ValidateAndSetLLM(ctx, mock)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if services.LLMService() == nil {
			t.Error("expected LLM service to be set")
		}
	})

	t.Run("failed validation", func(t *testing.T) {
		mock := &mockLLMService{pingErr: errors.New("connection failed")}
		err := services.ValidateAndSetLLM(ctx, mock)
		if err == nil {
			t.Error("expected error")
		}
		if !mock.closed {
			t.Error("expected failed service to be closed")
		}
	})

	t.Run("nil service", func(t *testing.T) {
		err := services.ValidateAndSetLLM(ctx, nil)
		if err != nil {
			t.Errorf("unexpected error for nil service: %v", err)
		}
	})
}

func TestServices_Close(t *testing.T) {
	config := domain.NewRuntimeConfig("postgres")
	services := NewServices(config)

	embMock := &mockEmbeddingService{}
	llmMock := &mockLLMService{}

	services.SetEmbeddingService(embMock)
	services.SetLLMService(llmMock)

	err := services.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !embMock.closed {
		t.Error("expected embedding service to be closed")
	}
	if !llmMock.closed {
		t.Error("expected LLM service to be closed")
	}
}

func TestServices_ReplaceService_ClosesOld(t *testing.T) {
	config := domain.NewRuntimeConfig("postgres")
	services := NewServices(config)

	old := &mockEmbeddingService{}
	new := &mockEmbeddingService{}

	services.SetEmbeddingService(old)
	services.SetEmbeddingService(new)

	if !old.closed {
		t.Error("expected old service to be closed when replaced")
	}
	if new.closed {
		t.Error("expected new service to remain open")
	}
}
