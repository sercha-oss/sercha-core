package ai

import (
	"testing"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	if factory == nil {
		t.Fatal("expected non-nil factory")
	}
}

func TestFactory_CreateEmbeddingService_NilSettings(t *testing.T) {
	factory := NewFactory()

	svc, err := factory.CreateEmbeddingService(nil, nil)
	if err != nil {
		t.Errorf("expected no error for nil settings, got %v", err)
	}
	if svc != nil {
		t.Error("expected nil service for nil settings")
	}
}

func TestFactory_CreateEmbeddingService_NotConfigured(t *testing.T) {
	factory := NewFactory()

	settings := &domain.EmbeddingSettings{
		Provider: "",
		Model:    "",
	}

	svc, err := factory.CreateEmbeddingService(settings, nil)
	if err != nil {
		t.Errorf("expected no error for unconfigured settings, got %v", err)
	}
	if svc != nil {
		t.Error("expected nil service for unconfigured settings")
	}
}

func TestFactory_CreateEmbeddingService_OpenAI(t *testing.T) {
	factory := NewFactory()

	settings := &domain.EmbeddingSettings{
		Provider: domain.AIProviderOpenAI,
		Model:    "text-embedding-3-small",
	}

	credentials := &driven.AICredentials{
		APIKey: "sk-test",
	}

	// OpenAI embedding is implemented
	svc, err := factory.CreateEmbeddingService(settings, credentials)
	if err != nil {
		t.Errorf("expected no error for OpenAI, got %v", err)
	}
	if svc == nil {
		t.Error("expected non-nil service for OpenAI")
	}
}

func TestFactory_CreateEmbeddingService_Ollama(t *testing.T) {
	factory := NewFactory()

	settings := &domain.EmbeddingSettings{
		Provider: domain.AIProviderOllama,
		Model:    "nomic-embed-text",
	}

	credentials := &driven.AICredentials{
		BaseURL: "http://localhost:11434",
	}

	// Currently returns error since not implemented
	_, err := factory.CreateEmbeddingService(settings, credentials)
	if err == nil {
		t.Error("expected error since Ollama not yet implemented")
	}
}

func TestFactory_CreateEmbeddingService_Voyage(t *testing.T) {
	factory := NewFactory()

	settings := &domain.EmbeddingSettings{
		Provider: domain.AIProviderVoyage,
		Model:    "voyage-2",
	}

	credentials := &driven.AICredentials{
		APIKey: "test-key",
	}

	// Currently returns error since not implemented
	_, err := factory.CreateEmbeddingService(settings, credentials)
	if err == nil {
		t.Error("expected error since Voyage not yet implemented")
	}
}

func TestFactory_CreateEmbeddingService_Cohere(t *testing.T) {
	factory := NewFactory()

	settings := &domain.EmbeddingSettings{
		Provider: domain.AIProviderCohere,
		Model:    "embed-english-v3.0",
	}

	credentials := &driven.AICredentials{
		APIKey: "test-key",
	}

	// Currently returns error since not implemented
	_, err := factory.CreateEmbeddingService(settings, credentials)
	if err == nil {
		t.Error("expected error since Cohere not yet implemented")
	}
}

func TestFactory_CreateEmbeddingService_InvalidProvider(t *testing.T) {
	factory := NewFactory()

	settings := &domain.EmbeddingSettings{
		Provider: "invalid-provider",
		Model:    "some-model",
	}

	credentials := &driven.AICredentials{
		APIKey: "test-key",
	}

	_, err := factory.CreateEmbeddingService(settings, credentials)
	if err == nil {
		t.Error("expected error for invalid provider")
	}
}

func TestFactory_CreateLLMService_NilSettings(t *testing.T) {
	factory := NewFactory()

	svc, err := factory.CreateLLMService(nil, nil)
	if err != nil {
		t.Errorf("expected no error for nil settings, got %v", err)
	}
	if svc != nil {
		t.Error("expected nil service for nil settings")
	}
}

func TestFactory_CreateLLMService_NotConfigured(t *testing.T) {
	factory := NewFactory()

	settings := &domain.LLMSettings{
		Provider: "",
		Model:    "",
	}

	credentials := &driven.AICredentials{
		APIKey: "test-key",
	}
	svc, err := factory.CreateLLMService(settings, credentials)
	if err != nil {
		t.Errorf("expected no error for unconfigured settings, got %v", err)
	}
	if svc != nil {
		t.Error("expected nil service for unconfigured settings")
	}
}

func TestFactory_CreateLLMService_OpenAI(t *testing.T) {
	factory := NewFactory()

	settings := &domain.LLMSettings{
		Provider: domain.AIProviderOpenAI,
		Model:    "gpt-4o-mini",
	}

	// Currently returns error since not implemented
	credentials := &driven.AICredentials{
		APIKey: "test-key",
	}
	_, err := factory.CreateLLMService(settings, credentials)
	if err == nil {
		t.Error("expected error since OpenAI LLM not yet implemented")
	}
}

func TestFactory_CreateLLMService_Anthropic(t *testing.T) {
	factory := NewFactory()

	settings := &domain.LLMSettings{
		Provider: domain.AIProviderAnthropic,
		Model:    "claude-3-5-sonnet-20241022",
	}

	// Currently returns error since not implemented
	credentials := &driven.AICredentials{
		APIKey: "test-key",
	}
	_, err := factory.CreateLLMService(settings, credentials)
	if err == nil {
		t.Error("expected error since Anthropic not yet implemented")
	}
}

func TestFactory_CreateLLMService_Ollama(t *testing.T) {
	factory := NewFactory()

	settings := &domain.LLMSettings{
		Provider: domain.AIProviderOllama,
		Model:    "llama3.2",
	}

	// Currently returns error since not implemented
	credentials := &driven.AICredentials{
		BaseURL: "http://localhost:11434",
	}
	_, err := factory.CreateLLMService(settings, credentials)
	if err == nil {
		t.Error("expected error since Ollama LLM not yet implemented")
	}
}

func TestFactory_CreateLLMService_InvalidProvider(t *testing.T) {
	factory := NewFactory()

	settings := &domain.LLMSettings{
		Provider: "invalid-provider",
		Model:    "some-model",
	}

	credentials := &driven.AICredentials{
		APIKey: "test-key",
	}
	_, err := factory.CreateLLMService(settings, credentials)
	if err == nil {
		t.Error("expected error for invalid provider")
	}
}

// Test that factory properly implements the interface
func TestFactory_ImplementsInterface(t *testing.T) {
	factory := NewFactory()

	// These calls ensure the factory methods have correct signatures
	_, _ = factory.CreateEmbeddingService(nil, nil)
	_, _ = factory.CreateLLMService(nil, nil)
}
