package services

import (
	"context"
	"fmt"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
)

// Ensure setupService implements SetupService
var _ driving.SetupService = (*setupService)(nil)

// setupService implements the SetupService interface
type setupService struct {
	userStore   driven.UserStore
	sourceStore driven.SourceStore
	teamID      string
}

// NewSetupService creates a new SetupService
func NewSetupService(
	userStore driven.UserStore,
	sourceStore driven.SourceStore,
	teamID string,
) driving.SetupService {
	return &setupService{
		userStore:   userStore,
		sourceStore: sourceStore,
		teamID:      teamID,
	}
}

// GetStatus returns the current setup status for FTUE flow
func (s *setupService) GetStatus(ctx context.Context) (*driving.SetupStatusResponse, error) {
	// Check if users exist
	users, err := s.userStore.List(ctx, s.teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	hasUsers := len(users) > 0

	// Check if sources exist
	sources, err := s.sourceStore.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}
	hasSources := len(sources) > 0

	// Setup is complete once the first admin user has been created.
	// Sources are configured separately after initial setup.
	setupComplete := hasUsers

	return &driving.SetupStatusResponse{
		SetupComplete: setupComplete,
		HasUsers:      hasUsers,
		HasSources:    hasSources,
	}, nil
}
