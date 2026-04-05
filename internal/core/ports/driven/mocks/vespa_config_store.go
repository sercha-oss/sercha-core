package mocks

import (
	"context"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// MockVespaConfigStore is a mock implementation of driven.VespaConfigStore
type MockVespaConfigStore struct {
	GetVespaConfigFunc  func(ctx context.Context, teamID string) (*domain.VespaConfig, error)
	SaveVespaConfigFunc func(ctx context.Context, config *domain.VespaConfig) error
}

// NewMockVespaConfigStore creates a new mock VespaConfigStore
func NewMockVespaConfigStore() *MockVespaConfigStore {
	return &MockVespaConfigStore{
		GetVespaConfigFunc: func(ctx context.Context, teamID string) (*domain.VespaConfig, error) {
			return nil, domain.ErrNotFound
		},
		SaveVespaConfigFunc: func(ctx context.Context, config *domain.VespaConfig) error {
			return nil
		},
	}
}

func (m *MockVespaConfigStore) GetVespaConfig(ctx context.Context, teamID string) (*domain.VespaConfig, error) {
	return m.GetVespaConfigFunc(ctx, teamID)
}

func (m *MockVespaConfigStore) SaveVespaConfig(ctx context.Context, config *domain.VespaConfig) error {
	return m.SaveVespaConfigFunc(ctx, config)
}
