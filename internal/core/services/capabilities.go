package services

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
)

// Ensure capabilitiesService implements CapabilitiesService
var _ driving.CapabilitiesService = (*capabilitiesService)(nil)

// capabilitiesService implements the CapabilitiesService interface.
// It exposes information about what features are available based on environment configuration.
type capabilitiesService struct {
	configProvider driven.ConfigProvider
}

// NewCapabilitiesService creates a new CapabilitiesService.
func NewCapabilitiesService(configProvider driven.ConfigProvider) driving.CapabilitiesService {
	return &capabilitiesService{
		configProvider: configProvider,
	}
}

// GetCapabilities returns information about available features.
func (s *capabilitiesService) GetCapabilities(ctx context.Context) (*driving.CapabilitiesResponse, error) {
	capabilities := s.configProvider.GetCapabilities()

	return &driving.CapabilitiesResponse{
		OAuthProviders: capabilities.OAuthProviders,
		AIProviders: driving.AIProvidersCapability{
			Embedding: capabilities.EmbeddingProviders,
			LLM:       capabilities.LLMProviders,
		},
		Features: driving.FeaturesCapability{
			SemanticSearch: len(capabilities.EmbeddingProviders) > 0,
			VectorIndexing: len(capabilities.EmbeddingProviders) > 0, // Backend check handled by capability system
		},
		Limits: driving.LimitsCapability{
			SyncMinInterval:   capabilities.Limits.SyncMinInterval,
			SyncMaxInterval:   capabilities.Limits.SyncMaxInterval,
			MaxWorkers:        capabilities.Limits.MaxWorkers,
			MaxResultsPerPage: capabilities.Limits.MaxResultsPerPage,
		},
	}, nil
}
