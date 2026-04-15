package mocks

import (
	"context"
	"sync"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// MockDocumentStore is a mock implementation of DocumentStore for testing
type MockDocumentStore struct {
	mu         sync.RWMutex
	documents  map[string]*domain.Document
	bySource   map[string][]*domain.Document
	byExternal map[string]*domain.Document // key: sourceID:externalID
}

// NewMockDocumentStore creates a new MockDocumentStore
func NewMockDocumentStore() *MockDocumentStore {
	return &MockDocumentStore{
		documents:  make(map[string]*domain.Document),
		bySource:   make(map[string][]*domain.Document),
		byExternal: make(map[string]*domain.Document),
	}
}

func (m *MockDocumentStore) Save(ctx context.Context, doc *domain.Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.documents[doc.ID] = doc
	m.byExternal[doc.SourceID+":"+doc.ExternalID] = doc

	// Update bySource index
	found := false
	for i, d := range m.bySource[doc.SourceID] {
		if d.ID == doc.ID {
			m.bySource[doc.SourceID][i] = doc
			found = true
			break
		}
	}
	if !found {
		m.bySource[doc.SourceID] = append(m.bySource[doc.SourceID], doc)
	}
	return nil
}

func (m *MockDocumentStore) SaveBatch(ctx context.Context, docs []*domain.Document) error {
	for _, doc := range docs {
		if err := m.Save(ctx, doc); err != nil {
			return err
		}
	}
	return nil
}

func (m *MockDocumentStore) Get(ctx context.Context, id string) (*domain.Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	doc, ok := m.documents[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return doc, nil
}

func (m *MockDocumentStore) GetByExternalID(ctx context.Context, sourceID, externalID string) (*domain.Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	doc, ok := m.byExternal[sourceID+":"+externalID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return doc, nil
}

func (m *MockDocumentStore) GetBySource(ctx context.Context, sourceID string, limit, offset int) ([]*domain.Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	docs := m.bySource[sourceID]
	if offset >= len(docs) {
		return []*domain.Document{}, nil
	}
	end := offset + limit
	if end > len(docs) {
		end = len(docs)
	}
	return docs[offset:end], nil
}

func (m *MockDocumentStore) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	doc, ok := m.documents[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(m.byExternal, doc.SourceID+":"+doc.ExternalID)
	delete(m.documents, id)

	// Update bySource index
	docs := m.bySource[doc.SourceID]
	for i, d := range docs {
		if d.ID == id {
			m.bySource[doc.SourceID] = append(docs[:i], docs[i+1:]...)
			break
		}
	}
	return nil
}

func (m *MockDocumentStore) DeleteBySource(ctx context.Context, sourceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	docs := m.bySource[sourceID]
	for _, doc := range docs {
		delete(m.documents, doc.ID)
		delete(m.byExternal, doc.SourceID+":"+doc.ExternalID)
	}
	delete(m.bySource, sourceID)
	return nil
}

func (m *MockDocumentStore) DeleteBySourceAndContainer(ctx context.Context, sourceID, containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	docs := m.bySource[sourceID]
	var remaining []*domain.Document
	for _, doc := range docs {
		// Check if document metadata has matching container_id
		if doc.Metadata != nil && doc.Metadata["container_id"] == containerID {
			delete(m.documents, doc.ID)
			delete(m.byExternal, doc.SourceID+":"+doc.ExternalID)
		} else {
			remaining = append(remaining, doc)
		}
	}
	m.bySource[sourceID] = remaining
	return nil
}

func (m *MockDocumentStore) DeleteBatch(ctx context.Context, ids []string) error {
	for _, id := range ids {
		_ = m.Delete(ctx, id)
	}
	return nil
}

func (m *MockDocumentStore) Count(ctx context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.documents), nil
}

func (m *MockDocumentStore) CountBySource(ctx context.Context, sourceID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.bySource[sourceID]), nil
}

func (m *MockDocumentStore) ListExternalIDs(ctx context.Context, sourceID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var ids []string
	for _, doc := range m.bySource[sourceID] {
		ids = append(ids, doc.ExternalID)
	}
	return ids, nil
}

// Helper methods for testing

func (m *MockDocumentStore) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.documents = make(map[string]*domain.Document)
	m.bySource = make(map[string][]*domain.Document)
	m.byExternal = make(map[string]*domain.Document)
}
