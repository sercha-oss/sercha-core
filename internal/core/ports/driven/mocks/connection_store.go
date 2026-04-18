package mocks

import (
	"context"
	"sync"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// MockConnectionStore is a mock implementation of ConnectionStore for testing
type MockConnectionStore struct {
	mu          sync.RWMutex
	connections map[string]*domain.Connection
	byPlatform  map[domain.PlatformType]map[string]*domain.Connection
	byAccount   map[string]*domain.Connection // key: platform:accountID
}

// NewMockConnectionStore creates a new MockConnectionStore
func NewMockConnectionStore() *MockConnectionStore {
	return &MockConnectionStore{
		connections: make(map[string]*domain.Connection),
		byPlatform:  make(map[domain.PlatformType]map[string]*domain.Connection),
		byAccount:   make(map[string]*domain.Connection),
	}
}

func (m *MockConnectionStore) Save(ctx context.Context, conn *domain.Connection) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.connections[conn.ID] = conn

	if m.byPlatform[conn.Platform] == nil {
		m.byPlatform[conn.Platform] = make(map[string]*domain.Connection)
	}
	m.byPlatform[conn.Platform][conn.ID] = conn

	if conn.AccountID != "" {
		key := string(conn.Platform) + ":" + conn.AccountID
		m.byAccount[key] = conn
	}

	return nil
}

func (m *MockConnectionStore) Get(ctx context.Context, id string) (*domain.Connection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, ok := m.connections[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return conn, nil
}

func (m *MockConnectionStore) List(ctx context.Context) ([]*domain.ConnectionSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*domain.ConnectionSummary
	for _, conn := range m.connections {
		result = append(result, conn.ToSummary())
	}
	return result, nil
}

func (m *MockConnectionStore) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, ok := m.connections[id]
	if !ok {
		return domain.ErrNotFound
	}

	delete(m.connections, id)
	delete(m.byPlatform[conn.Platform], id)

	if conn.AccountID != "" {
		key := string(conn.Platform) + ":" + conn.AccountID
		delete(m.byAccount, key)
	}

	return nil
}

func (m *MockConnectionStore) GetByPlatform(ctx context.Context, platform domain.PlatformType) ([]*domain.ConnectionSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*domain.ConnectionSummary
	for _, conn := range m.byPlatform[platform] {
		result = append(result, conn.ToSummary())
	}
	return result, nil
}

func (m *MockConnectionStore) GetByAccountID(ctx context.Context, platform domain.PlatformType, accountID string) (*domain.Connection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := string(platform) + ":" + accountID
	conn, ok := m.byAccount[key]
	if !ok {
		return nil, nil
	}
	return conn, nil
}

func (m *MockConnectionStore) UpdateSecrets(ctx context.Context, id string, secrets *domain.ConnectionSecrets, expiry *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, ok := m.connections[id]
	if !ok {
		return domain.ErrNotFound
	}

	conn.Secrets = secrets
	conn.OAuthExpiry = expiry
	conn.UpdatedAt = time.Now()

	return nil
}

func (m *MockConnectionStore) UpdateLastUsed(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, ok := m.connections[id]
	if !ok {
		return domain.ErrNotFound
	}

	now := time.Now()
	conn.LastUsedAt = &now

	return nil
}

// Helper methods for testing

func (m *MockConnectionStore) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connections = make(map[string]*domain.Connection)
	m.byPlatform = make(map[domain.PlatformType]map[string]*domain.Connection)
	m.byAccount = make(map[string]*domain.Connection)
}

func (m *MockConnectionStore) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.connections)
}
