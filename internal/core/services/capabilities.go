package services

import (
	"context"
	"fmt"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
)

// Ensure capabilitiesService implements CapabilitiesService.
var _ driving.CapabilitiesService = (*capabilitiesService)(nil)

// capabilitiesService is registry-driven: the set of known capabilities
// is whatever has been registered with the CapabilityRegistry, and
// availability is computed via an AvailabilityResolver. Adding a new
// capability requires only registering a descriptor at startup; no
// service-side switch cases or type-specific code.
type capabilitiesService struct {
	configProvider driven.ConfigProvider
	store          driven.CapabilityStore
	registry       domain.CapabilityRegistry
	resolver       domain.AvailabilityResolver
}

// NewCapabilitiesService wires the dependencies. The registry must already
// have descriptors registered by the time this service handles requests.
func NewCapabilitiesService(
	configProvider driven.ConfigProvider,
	store driven.CapabilityStore,
	registry domain.CapabilityRegistry,
	resolver domain.AvailabilityResolver,
) driving.CapabilitiesService {
	return &capabilitiesService{
		configProvider: configProvider,
		store:          store,
		registry:       registry,
		resolver:       resolver,
	}
}

// GetCapabilities returns the full capability snapshot — providers,
// descriptors, resolved feature status, and limits. Iterates the registry
// generically.
func (s *capabilitiesService) GetCapabilities(ctx context.Context, teamID string) (*driving.CapabilitiesResponse, error) {
	cfg := s.configProvider.GetCapabilities()

	descriptors := s.registry.All()

	// Build the availability map by asking the resolver for each
	// registered type. The resolver knows about backend wiring; the
	// service stays generic.
	available := make(map[domain.CapabilityType]bool, len(descriptors))
	for _, d := range descriptors {
		available[d.Type] = s.resolver.IsAvailable(ctx, d.Type)
	}

	// Per-team prefs (best-effort — if the store fails we treat it as
	// "no preferences set" and fall back to descriptor defaults).
	var prefs *domain.CapabilityPreferences
	if teamID != "" {
		if p, err := s.store.GetPreferences(ctx, teamID); err == nil {
			prefs = p
		}
	}

	resolved := domain.ResolveCapabilities(descriptors, available, prefs)

	features := make(map[domain.CapabilityType]driving.CapabilityStatus, len(resolved))
	for _, c := range resolved {
		features[c.Type] = driving.CapabilityStatus{
			Available: c.Available,
			Enabled:   c.Enabled,
			Active:    c.IsActive(),
		}
	}

	return &driving.CapabilitiesResponse{
		OAuthProviders: cfg.OAuthProviders,
		AIProviders: driving.AIProvidersCapability{
			Embedding: cfg.EmbeddingProviders,
			LLM:       cfg.LLMProviders,
		},
		Descriptors: descriptors,
		Features:    features,
		Limits: driving.LimitsCapability{
			SyncMinInterval:   cfg.Limits.SyncMinInterval,
			SyncMaxInterval:   cfg.Limits.SyncMaxInterval,
			MaxWorkers:        cfg.Limits.MaxWorkers,
			MaxResultsPerPage: cfg.Limits.MaxResultsPerPage,
		},
	}, nil
}

// GetCapabilityPreferences returns the persisted toggles for a team.
func (s *capabilitiesService) GetCapabilityPreferences(ctx context.Context, teamID string) (*domain.CapabilityPreferences, error) {
	prefs, err := s.store.GetPreferences(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("get capability preferences: %w", err)
	}
	return prefs, nil
}

// UpdateCapabilityPreferences applies a partial-toggle update. Validates
// that every toggle key in the request corresponds to a registered
// capability — unknown types are rejected so typos and stale clients
// don't silently persist garbage.
func (s *capabilitiesService) UpdateCapabilityPreferences(ctx context.Context, teamID string, req driving.UpdateCapabilityPreferencesRequest) (*domain.CapabilityPreferences, error) {
	if len(req.Toggles) > 0 {
		for capType := range req.Toggles {
			if _, ok := s.registry.Get(capType); !ok {
				return nil, fmt.Errorf("unknown capability: %q", capType)
			}
		}
		if err := s.store.SetToggles(ctx, teamID, req.Toggles); err != nil {
			return nil, fmt.Errorf("save capability preferences: %w", err)
		}
	}
	return s.store.GetPreferences(ctx, teamID)
}
