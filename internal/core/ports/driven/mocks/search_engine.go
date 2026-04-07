package mocks

import (
	"context"
	"strings"
	"sync"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// MockSearchEngine is a mock implementation of SearchEngine for testing
type MockSearchEngine struct {
	mu       sync.RWMutex
	chunks   map[string]*domain.Chunk
	byDoc    map[string][]*domain.Chunk
	bySource map[string][]*domain.Chunk
}

// NewMockSearchEngine creates a new MockSearchEngine
func NewMockSearchEngine() *MockSearchEngine {
	return &MockSearchEngine{
		chunks:   make(map[string]*domain.Chunk),
		byDoc:    make(map[string][]*domain.Chunk),
		bySource: make(map[string][]*domain.Chunk),
	}
}

func (m *MockSearchEngine) Index(ctx context.Context, chunks []*domain.Chunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, chunk := range chunks {
		m.chunks[chunk.ID] = chunk
		m.byDoc[chunk.DocumentID] = append(m.byDoc[chunk.DocumentID], chunk)
		m.bySource[chunk.SourceID] = append(m.bySource[chunk.SourceID], chunk)
	}
	return nil
}

func (m *MockSearchEngine) Search(ctx context.Context, query string, queryEmbedding []float32, opts domain.SearchOptions) ([]*domain.RankedChunk, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*domain.RankedChunk
	queryLower := strings.ToLower(query)

	for _, chunk := range m.chunks {
		// Filter by source if specified
		if len(opts.SourceIDs) > 0 {
			found := false
			for _, sourceID := range opts.SourceIDs {
				if chunk.SourceID == sourceID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Simple text matching for mock
		if strings.Contains(strings.ToLower(chunk.Content), queryLower) {
			results = append(results, &domain.RankedChunk{
				Chunk:      chunk,
				Score:      1.0,
				Highlights: []string{chunk.Content},
			})
		}
	}

	// Apply pagination
	total := len(results)
	if opts.Offset >= len(results) {
		return []*domain.RankedChunk{}, total, nil
	}
	end := opts.Offset + opts.Limit
	if end > len(results) {
		end = len(results)
	}
	if opts.Limit <= 0 {
		end = len(results)
	}

	return results[opts.Offset:end], total, nil
}

func (m *MockSearchEngine) Delete(ctx context.Context, chunkIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range chunkIDs {
		delete(m.chunks, id)
	}
	return nil
}

func (m *MockSearchEngine) DeleteByDocument(ctx context.Context, documentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	chunks := m.byDoc[documentID]
	for _, chunk := range chunks {
		delete(m.chunks, chunk.ID)
	}
	delete(m.byDoc, documentID)
	return nil
}

func (m *MockSearchEngine) DeleteBySource(ctx context.Context, sourceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	chunks := m.bySource[sourceID]
	for _, chunk := range chunks {
		delete(m.chunks, chunk.ID)
		delete(m.byDoc, chunk.DocumentID)
	}
	delete(m.bySource, sourceID)
	return nil
}

func (m *MockSearchEngine) HealthCheck(ctx context.Context) error {
	return nil
}

// Helper methods for testing

func (m *MockSearchEngine) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chunks = make(map[string]*domain.Chunk)
	m.byDoc = make(map[string][]*domain.Chunk)
	m.bySource = make(map[string][]*domain.Chunk)
}

func (m *MockSearchEngine) Count(ctx context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.chunks)), nil
}

// MockVectorIndex is a mock implementation of VectorIndex for testing
type MockVectorIndex struct {
	mu         sync.RWMutex
	embeddings map[string][]float32
}

// NewMockVectorIndex creates a new MockVectorIndex
func NewMockVectorIndex() *MockVectorIndex {
	return &MockVectorIndex{
		embeddings: make(map[string][]float32),
	}
}

func (m *MockVectorIndex) Index(ctx context.Context, id string, embedding []float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.embeddings[id] = embedding
	return nil
}

func (m *MockVectorIndex) IndexBatch(ctx context.Context, ids []string, embeddings [][]float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, id := range ids {
		if i < len(embeddings) {
			m.embeddings[id] = embeddings[i]
		}
	}
	return nil
}

func (m *MockVectorIndex) Search(ctx context.Context, embedding []float32, k int) ([]string, []float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Simple mock: return all stored IDs with distance 0.0
	var ids []string
	var distances []float64
	for id := range m.embeddings {
		ids = append(ids, id)
		distances = append(distances, 0.0)
		if len(ids) >= k {
			break
		}
	}
	return ids, distances, nil
}

func (m *MockVectorIndex) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.embeddings, id)
	return nil
}

func (m *MockVectorIndex) DeleteBatch(ctx context.Context, ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range ids {
		delete(m.embeddings, id)
	}
	return nil
}

func (m *MockVectorIndex) HealthCheck(ctx context.Context) error {
	return nil
}

// Helper methods for testing

func (m *MockVectorIndex) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.embeddings = make(map[string][]float32)
}

func (m *MockVectorIndex) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.embeddings)
}

func (m *MockVectorIndex) GetEmbedding(id string) ([]float32, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	emb, ok := m.embeddings[id]
	return emb, ok
}
