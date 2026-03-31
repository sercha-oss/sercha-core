package mocks

import (
	"context"
	"sync"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// MockSourceStore is a mock implementation of SourceStore for testing
type MockSourceStore struct {
	mu      sync.RWMutex
	sources map[string]*domain.Source
	byName  map[string]*domain.Source
}

// NewMockSourceStore creates a new MockSourceStore
func NewMockSourceStore() *MockSourceStore {
	return &MockSourceStore{
		sources: make(map[string]*domain.Source),
		byName:  make(map[string]*domain.Source),
	}
}

func (m *MockSourceStore) Save(ctx context.Context, source *domain.Source) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources[source.ID] = source
	m.byName[source.Name] = source
	return nil
}

func (m *MockSourceStore) Get(ctx context.Context, id string) (*domain.Source, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	source, ok := m.sources[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return source, nil
}

func (m *MockSourceStore) GetByName(ctx context.Context, name string) (*domain.Source, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	source, ok := m.byName[name]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return source, nil
}

func (m *MockSourceStore) List(ctx context.Context) ([]*domain.Source, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.Source
	for _, source := range m.sources {
		result = append(result, source)
	}
	return result, nil
}

func (m *MockSourceStore) ListEnabled(ctx context.Context) ([]*domain.Source, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.Source
	for _, source := range m.sources {
		if source.Enabled {
			result = append(result, source)
		}
	}
	return result, nil
}

func (m *MockSourceStore) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	source, ok := m.sources[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(m.byName, source.Name)
	delete(m.sources, id)
	return nil
}

func (m *MockSourceStore) SetEnabled(ctx context.Context, id string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	source, ok := m.sources[id]
	if !ok {
		return domain.ErrNotFound
	}
	source.Enabled = enabled
	return nil
}

func (m *MockSourceStore) CountByConnection(ctx context.Context, connectionID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, source := range m.sources {
		if source.ConnectionID == connectionID {
			count++
		}
	}
	return count, nil
}

func (m *MockSourceStore) ListByConnection(ctx context.Context, connectionID string) ([]*domain.Source, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.Source
	for _, source := range m.sources {
		if source.ConnectionID == connectionID {
			result = append(result, source)
		}
	}
	return result, nil
}

func (m *MockSourceStore) UpdateContainers(ctx context.Context, id string, containers []domain.Container) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	source, ok := m.sources[id]
	if !ok {
		return domain.ErrNotFound
	}
	source.Containers = containers
	return nil
}

// Helper methods for testing

func (m *MockSourceStore) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources = make(map[string]*domain.Source)
	m.byName = make(map[string]*domain.Source)
}

func (m *MockSourceStore) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sources)
}
