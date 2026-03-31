package config

import "github.com/custodia-labs/sercha-core/internal/core/domain"

// This file contains helpers for computing capabilities from configuration.
// The main capabilities computation is in config.go's GetCapabilities() method.

// FeatureFlags represents computed feature flags based on available capabilities
type FeatureFlags struct {
	SemanticSearch  bool `json:"semantic_search"`
	VectorIndexing  bool `json:"vector_indexing"`
	LLMGeneration   bool `json:"llm_generation"`
}

// ComputeFeatureFlags returns feature flags based on available AI providers
func (c *Config) ComputeFeatureFlags() FeatureFlags {
	flags := FeatureFlags{}

	// Semantic search requires embedding provider
	flags.SemanticSearch = len(c.aiCredentials) > 0 && c.hasEmbeddingProvider()

	// Vector indexing same as semantic search
	flags.VectorIndexing = flags.SemanticSearch

	// LLM generation requires LLM provider
	flags.LLMGeneration = c.hasLLMProvider()

	return flags
}

// hasEmbeddingProvider checks if any configured AI provider supports embedding
func (c *Config) hasEmbeddingProvider() bool {
	embeddingProviders := []string{
		string(domain.AIProviderOpenAI),
		string(domain.AIProviderOllama),
		string(domain.AIProviderCohere),
		string(domain.AIProviderVoyage),
	}

	for _, provider := range embeddingProviders {
		if c.aiCredentials[domain.AIProvider(provider)] != nil {
			return true
		}
	}
	return false
}

// hasLLMProvider checks if any configured AI provider supports LLM
func (c *Config) hasLLMProvider() bool {
	llmProviders := []string{
		string(domain.AIProviderOpenAI),
		string(domain.AIProviderAnthropic),
		string(domain.AIProviderOllama),
	}

	for _, provider := range llmProviders {
		if c.aiCredentials[domain.AIProvider(provider)] != nil {
			return true
		}
	}
	return false
}
