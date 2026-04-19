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
func (s *capabilitiesService) GetCapabilities(ctx context.Context, teamID string) (*driving.CapabilitiesResponse, error) {
	capabilities := s.configProvider.GetCapabilities()

	hasEmbeddings := len(capabilities.EmbeddingProviders) > 0
	hasLLM := len(capabilities.LLMProviders) > 0

	// Build availability map from backend status
	available := map[domain.CapabilityType]bool{
		domain.CapabilityTextIndexing:      capabilities.SearchEngineAvailable,
		domain.CapabilityEmbeddingIndexing: hasEmbeddings && capabilities.VectorStoreAvailable,
		domain.CapabilityBM25Search:        capabilities.SearchEngineAvailable,
		domain.CapabilityVectorSearch:      hasEmbeddings && capabilities.VectorStoreAvailable,
		domain.CapabilityQueryExpansion:    hasLLM,
		domain.CapabilityQueryRewriting:    false,
		domain.CapabilitySummarization:     false,
	}

	// Fetch team preferences to merge with availability
	var prefs *domain.CapabilityPreferences
	if teamID != "" {
		p, err := s.capabilityStore.GetPreferences(ctx, teamID)
		if err == nil {
			prefs = p
		}
		// If error or not found, prefs stays nil → ResolveCapabilities uses factory defaults
	}

	resolved := domain.ResolveCapabilities(prefs, available)

	// Build features from resolved capabilities
	features := driving.FeaturesCapability{}
	for _, cap := range resolved {
		status := driving.CapabilityStatus{
			Available: cap.Available,
			Enabled:   cap.Enabled,
			Active:    cap.IsActive(),
		}
		switch cap.Type {
		case domain.CapabilityTextIndexing:
			features.TextIndexing = status
		case domain.CapabilityEmbeddingIndexing:
			features.EmbeddingIndexing = status
		case domain.CapabilityBM25Search:
			features.BM25Search = status
		case domain.CapabilityVectorSearch:
			features.VectorSearch = status
		case domain.CapabilityQueryExpansion:
			features.QueryExpansion = status
		case domain.CapabilityQueryRewriting:
			features.QueryRewriting = status
		case domain.CapabilitySummarization:
			features.Summarization = status
		}
	}

	return &driving.CapabilitiesResponse{
		OAuthProviders: capabilities.OAuthProviders,
		AIProviders: driving.AIProvidersCapability{
			Embedding: capabilities.EmbeddingProviders,
			LLM:       capabilities.LLMProviders,
		},
		Features: features,
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

	// LLM-powered feature preferences
	if req.QueryExpansionEnabled != nil {
		prefs.QueryExpansionEnabled = *req.QueryExpansionEnabled
		prefs.UpdatedAt = time.Now()
	}

	if req.QueryRewritingEnabled != nil {
		prefs.QueryRewritingEnabled = *req.QueryRewritingEnabled
		prefs.UpdatedAt = time.Now()
	}

	if req.SummarizationEnabled != nil {
		prefs.SummarizationEnabled = *req.SummarizationEnabled
		prefs.UpdatedAt = time.Now()
	}

	// Save updated preferences
	if err := s.capabilityStore.SavePreferences(ctx, prefs); err != nil {
		return nil, fmt.Errorf("save capability preferences: %w", err)
	}

	return prefs, nil
}
