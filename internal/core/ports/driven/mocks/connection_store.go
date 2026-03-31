package mocks

import (
	"context"
	"sync"
	"time"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// MockConnectionStore is a mock implementation of ConnectionStore for testing
type MockConnectionStore struct {
	mu            sync.RWMutex
	connections   map[string]*domain.Connection
	byProvider    map[domain.ProviderType]map[string]*domain.Connection
	byAccount     map[string]*domain.Connection // key: providerType:accountID
}

// NewMockConnectionStore creates a new MockConnectionStore
func NewMockConnectionStore() *MockConnectionStore {
	return &MockConnectionStore{
		connections: make(map[string]*domain.Connection),
		byProvider:  make(map[domain.ProviderType]map[string]*domain.Connection),
		byAccount:   make(map[string]*domain.Connection),
	}
}

func (m *MockConnectionStore) Save(ctx context.Context, conn *domain.Connection) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.connections[conn.ID] = conn

	if m.byProvider[conn.ProviderType] == nil {
		m.byProvider[conn.ProviderType] = make(map[string]*domain.Connection)
	}
	m.byProvider[conn.ProviderType][conn.ID] = conn

	if conn.AccountID != "" {
		key := string(conn.ProviderType) + ":" + conn.AccountID
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
	delete(m.byProvider[conn.ProviderType], id)

	if conn.AccountID != "" {
		key := string(conn.ProviderType) + ":" + conn.AccountID
		delete(m.byAccount, key)
	}

	return nil
}

func (m *MockConnectionStore) GetByProvider(ctx context.Context, providerType domain.ProviderType) ([]*domain.ConnectionSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*domain.ConnectionSummary
	for _, conn := range m.byProvider[providerType] {
		result = append(result, conn.ToSummary())
	}
	return result, nil
}

func (m *MockConnectionStore) GetByAccountID(ctx context.Context, providerType domain.ProviderType, accountID string) (*domain.Connection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := string(providerType) + ":" + accountID
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
	m.byProvider = make(map[domain.ProviderType]map[string]*domain.Connection)
	m.byAccount = make(map[string]*domain.Connection)
}

func (m *MockConnectionStore) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.connections)
}
