package mocks

import (
	"context"
	"sync"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// MockChunkStore is a mock implementation of ChunkStore for testing
type MockChunkStore struct {
	mu         sync.RWMutex
	chunks     map[string]*domain.Chunk
	byDocument map[string][]*domain.Chunk
	bySource   map[string][]*domain.Chunk
}

// NewMockChunkStore creates a new MockChunkStore
func NewMockChunkStore() *MockChunkStore {
	return &MockChunkStore{
		chunks:     make(map[string]*domain.Chunk),
		byDocument: make(map[string][]*domain.Chunk),
		bySource:   make(map[string][]*domain.Chunk),
	}
}

func (m *MockChunkStore) Save(ctx context.Context, chunk *domain.Chunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chunks[chunk.ID] = chunk

	// Update byDocument index
	found := false
	for i, c := range m.byDocument[chunk.DocumentID] {
		if c.ID == chunk.ID {
			m.byDocument[chunk.DocumentID][i] = chunk
			found = true
			break
		}
	}
	if !found {
		m.byDocument[chunk.DocumentID] = append(m.byDocument[chunk.DocumentID], chunk)
	}

	// Update bySource index
	found = false
	for i, c := range m.bySource[chunk.SourceID] {
		if c.ID == chunk.ID {
			m.bySource[chunk.SourceID][i] = chunk
			found = true
			break
		}
	}
	if !found {
		m.bySource[chunk.SourceID] = append(m.bySource[chunk.SourceID], chunk)
	}

	return nil
}

func (m *MockChunkStore) SaveBatch(ctx context.Context, chunks []*domain.Chunk) error {
	for _, chunk := range chunks {
		if err := m.Save(ctx, chunk); err != nil {
			return err
		}
	}
	return nil
}

func (m *MockChunkStore) GetByDocument(ctx context.Context, documentID string) ([]*domain.Chunk, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.byDocument[documentID], nil
}

func (m *MockChunkStore) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	chunk, ok := m.chunks[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(m.chunks, id)

	// Update byDocument index
	docs := m.byDocument[chunk.DocumentID]
	for i, c := range docs {
		if c.ID == id {
			m.byDocument[chunk.DocumentID] = append(docs[:i], docs[i+1:]...)
			break
		}
	}

	// Update bySource index
	sources := m.bySource[chunk.SourceID]
	for i, c := range sources {
		if c.ID == id {
			m.bySource[chunk.SourceID] = append(sources[:i], sources[i+1:]...)
			break
		}
	}

	return nil
}

func (m *MockChunkStore) DeleteByDocument(ctx context.Context, documentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	chunks := m.byDocument[documentID]
	for _, chunk := range chunks {
		delete(m.chunks, chunk.ID)
	}
	delete(m.byDocument, documentID)
	return nil
}

func (m *MockChunkStore) DeleteBySource(ctx context.Context, sourceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	chunks := m.bySource[sourceID]
	for _, chunk := range chunks {
		delete(m.chunks, chunk.ID)

		// Update byDocument index
		docs := m.byDocument[chunk.DocumentID]
		for i, c := range docs {
			if c.ID == chunk.ID {
				m.byDocument[chunk.DocumentID] = append(docs[:i], docs[i+1:]...)
				break
			}
		}
	}
	delete(m.bySource, sourceID)
	return nil
}

func (m *MockChunkStore) DeleteChunksBySourceAndContainer(ctx context.Context, sourceID, containerID string) error {
	// Mock implementation: For testing purposes, we would need access to documents
	// to check container_id from metadata. For now, this is a simplified implementation
	// that deletes all chunks from the source (same as DeleteBySource)
	return m.DeleteBySource(ctx, sourceID)
}

// Helper methods for testing

func (m *MockChunkStore) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chunks = make(map[string]*domain.Chunk)
	m.byDocument = make(map[string][]*domain.Chunk)
	m.bySource = make(map[string][]*domain.Chunk)
}

func (m *MockChunkStore) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.chunks)
}
