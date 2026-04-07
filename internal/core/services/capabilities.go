package services

import (
	"context"
	"fmt"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
)

// Ensure capabilitiesService implements CapabilitiesService
var _ driving.CapabilitiesService = (*capabilitiesService)(nil)

// capabilitiesService implements the CapabilitiesService interface.
// It exposes information about what features are available based on environment configuration
// and manages per-team capability preferences.
type capabilitiesService struct {
	configProvider  driven.ConfigProvider
	capabilityStore driven.CapabilityStore
}

// NewCapabilitiesService creates a new CapabilitiesService.
func NewCapabilitiesService(
	configProvider driven.ConfigProvider,
	capabilityStore driven.CapabilityStore,
) driving.CapabilitiesService {
	return &capabilitiesService{
		configProvider:  configProvider,
		capabilityStore: capabilityStore,
	}
}

// GetCapabilities returns information about available features.
func (s *capabilitiesService) GetCapabilities(ctx context.Context) (*driving.CapabilitiesResponse, error) {
	capabilities := s.configProvider.GetCapabilities()

	hasEmbeddings := len(capabilities.EmbeddingProviders) > 0

	return &driving.CapabilitiesResponse{
		OAuthProviders: capabilities.OAuthProviders,
		AIProviders: driving.AIProvidersCapability{
			Embedding: capabilities.EmbeddingProviders,
			LLM:       capabilities.LLMProviders,
		},
		Features: driving.FeaturesCapability{
			TextIndexing:      capabilities.SearchEngineAvailable,
			EmbeddingIndexing: hasEmbeddings && capabilities.VectorStoreAvailable,
			BM25Search:        capabilities.SearchEngineAvailable,
			VectorSearch:      hasEmbeddings && capabilities.VectorStoreAvailable,
		},
		Limits: driving.LimitsCapability{
			SyncMinInterval:   capabilities.Limits.SyncMinInterval,
			SyncMaxInterval:   capabilities.Limits.SyncMaxInterval,
			MaxWorkers:        capabilities.Limits.MaxWorkers,
			MaxResultsPerPage: capabilities.Limits.MaxResultsPerPage,
		},
	}, nil
}

// GetCapabilityPreferences retrieves capability preferences for a team.
func (s *capabilitiesService) GetCapabilityPreferences(ctx context.Context, teamID string) (*domain.CapabilityPreferences, error) {
	prefs, err := s.capabilityStore.GetPreferences(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("get capability preferences: %w", err)
	}
	return prefs, nil
}

// UpdateCapabilityPreferences updates capability preferences for a team.
// Uses partial update semantics - only non-nil fields are applied.
func (s *capabilitiesService) UpdateCapabilityPreferences(ctx context.Context, teamID string, req driving.UpdateCapabilityPreferencesRequest) (*domain.CapabilityPreferences, error) {
	// Load existing preferences or create defaults
	prefs, err := s.capabilityStore.GetPreferences(ctx, teamID)
	if err != nil {
		// If preferences don't exist, start with defaults
		prefs = domain.DefaultCapabilityPreferences(teamID)
	}

	// Apply partial updates using domain methods where appropriate
	if req.TextIndexingEnabled != nil {
		if *req.TextIndexingEnabled {
			prefs.EnableTextIndexing()
		} else {
			prefs.DisableTextIndexing()
		}
	}

	if req.EmbeddingIndexingEnabled != nil {
		if *req.EmbeddingIndexingEnabled {
			prefs.EnableEmbeddingIndexing()
		} else {
			prefs.DisableEmbeddingIndexing()
		}
	}

	// Allow fine-grained control of search preferences
	// (but note: disabling indexing above will also disable the search)
	if req.BM25SearchEnabled != nil {
		prefs.BM25SearchEnabled = *req.BM25SearchEnabled
		prefs.UpdatedAt = time.Now()
	}

	if req.VectorSearchEnabled != nil {
		prefs.VectorSearchEnabled = *req.VectorSearchEnabled
		prefs.UpdatedAt = time.Now()
	}

	// Save updated preferences
	if err := s.capabilityStore.SavePreferences(ctx, prefs); err != nil {
		return nil, fmt.Errorf("save capability preferences: %w", err)
	}

	return prefs, nil
}
