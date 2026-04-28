package mocks

import (
	"context"
	"strings"
	"sync"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// MockSearchEngine is a mock implementation of SearchEngine for testing
type MockSearchEngine struct {
	mu   sync.RWMutex
	docs map[string]*docEntry // keyed by document_id
}

type docEntry struct {
	DocumentID string
	SourceID   string
	Title      string
	Content    string
	Path       string
	MimeType   string
}

// NewMockSearchEngine creates a new MockSearchEngine
func NewMockSearchEngine() *MockSearchEngine {
	return &MockSearchEngine{
		docs: make(map[string]*docEntry),
	}
}

func (m *MockSearchEngine) IndexDocument(ctx context.Context, doc *domain.DocumentContent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.docs[doc.DocumentID] = &docEntry{
		DocumentID: doc.DocumentID,
		SourceID:   doc.SourceID,
		Title:      doc.Title,
		Content:    doc.Body,
		Path:       doc.Path,
		MimeType:   doc.MimeType,
	}
	return nil
}

func (m *MockSearchEngine) SearchDocuments(ctx context.Context, query string, opts domain.SearchOptions) ([]driven.DocumentResult, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []driven.DocumentResult
	queryLower := strings.ToLower(query)

	for _, doc := range m.docs {
		if len(opts.SourceIDs) > 0 {
			found := false
			for _, sourceID := range opts.SourceIDs {
				if doc.SourceID == sourceID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if strings.Contains(strings.ToLower(doc.Content), queryLower) || strings.Contains(strings.ToLower(doc.Title), queryLower) {
			results = append(results, driven.DocumentResult{
				DocumentID: doc.DocumentID,
				SourceID:   doc.SourceID,
				Title:      doc.Title,
				Content:    doc.Content,
				Score:      1.0,
			})
		}
	}

	total := len(results)
	if opts.Offset >= len(results) {
		return []driven.DocumentResult{}, total, nil
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

func (m *MockSearchEngine) DeleteByDocument(ctx context.Context, documentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, doc := range m.docs {
		if doc.DocumentID == documentID {
			delete(m.docs, id)
		}
	}
	return nil
}

func (m *MockSearchEngine) DeleteByDocuments(ctx context.Context, documentIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, documentID := range documentIDs {
		for id, doc := range m.docs {
			if doc.DocumentID == documentID {
				delete(m.docs, id)
			}
		}
	}
	return nil
}

func (m *MockSearchEngine) DeleteBySource(ctx context.Context, sourceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, doc := range m.docs {
		if doc.SourceID == sourceID {
			delete(m.docs, id)
		}
	}
	return nil
}

func (m *MockSearchEngine) DeleteBySourceAndContainer(ctx context.Context, sourceID, containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, doc := range m.docs {
		if doc.SourceID == sourceID {
			// In a real implementation, we'd check doc.Metadata["container_id"] == containerID
			// For the mock, we'll just match by source_id
			delete(m.docs, id)
		}
	}
	return nil
}

func (m *MockSearchEngine) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *MockSearchEngine) Count(ctx context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.docs)), nil
}

func (m *MockSearchEngine) GetDocument(ctx context.Context, documentID string) (*domain.DocumentContent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	doc, ok := m.docs[documentID]
	if !ok {
		return nil, domain.ErrNotFound
	}

	return &domain.DocumentContent{
		DocumentID: doc.DocumentID,
		SourceID:   doc.SourceID,
		Title:      doc.Title,
		Body:       doc.Content,
		Path:       doc.Path,
		MimeType:   doc.MimeType,
		Metadata:   map[string]string{},
	}, nil
}

// Helper methods for testing

func (m *MockSearchEngine) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.docs = make(map[string]*docEntry)
}

// MockVectorIndex is a mock implementation of VectorIndex for testing
type MockVectorIndex struct {
	mu          sync.RWMutex
	embeddings  map[string][]float32
	documentIDs map[string]string // chunk_id -> document_id
	sourceIDs   map[string]string // chunk_id -> source_id
	contents    map[string]string // chunk_id -> content
}

// NewMockVectorIndex creates a new MockVectorIndex
func NewMockVectorIndex() *MockVectorIndex {
	return &MockVectorIndex{
		embeddings:  make(map[string][]float32),
		documentIDs: make(map[string]string),
		sourceIDs:   make(map[string]string),
		contents:    make(map[string]string),
	}
}

func (m *MockVectorIndex) Index(ctx context.Context, id string, documentID string, embedding []float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.embeddings[id] = embedding
	m.documentIDs[id] = documentID
	return nil
}

func (m *MockVectorIndex) IndexBatch(ctx context.Context, ids []string, documentIDs []string, sourceIDs []string, contents []string, embeddings [][]float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, id := range ids {
		if i < len(embeddings) {
			m.embeddings[id] = embeddings[i]
		}
		if i < len(documentIDs) {
			m.documentIDs[id] = documentIDs[i]
		}
		if i < len(sourceIDs) {
			m.sourceIDs[id] = sourceIDs[i]
		}
		if i < len(contents) {
			m.contents[id] = contents[i]
		}
	}
	return nil
}

func (m *MockVectorIndex) Search(ctx context.Context, embedding []float32, k int) ([]string, []float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

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

func (m *MockVectorIndex) SearchWithContent(ctx context.Context, embedding []float32, k int, sourceIDs []string, documentFilter *domain.DocumentIDFilter) ([]driven.VectorSearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Deny-all short-circuit: authoritative filter with zero allowed IDs returns nothing.
	if documentFilter.IsDenyAll() {
		return nil, nil
	}

	var results []driven.VectorSearchResult
	for id := range m.embeddings {
		// Apply source filter if specified
		if len(sourceIDs) > 0 {
			chunkSourceID := m.sourceIDs[id]
			found := false
			for _, sid := range sourceIDs {
				if chunkSourceID == sid {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		// Apply document ID allow-list filter if specified.
		if documentFilter.IsAllowList() {
			chunkDocumentID := m.documentIDs[id]
			found := false
			for _, did := range documentFilter.IDs {
				if chunkDocumentID == did {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		results = append(results, driven.VectorSearchResult{
			ChunkID:    id,
			DocumentID: m.documentIDs[id],
			Content:    m.contents[id],
			Distance:   0.1, // Small distance = high similarity
		})
		if len(results) >= k {
			break
		}
	}
	return results, nil
}

func (m *MockVectorIndex) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.embeddings, id)
	delete(m.documentIDs, id)
	delete(m.contents, id)
	return nil
}

func (m *MockVectorIndex) DeleteBatch(ctx context.Context, ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range ids {
		delete(m.embeddings, id)
		delete(m.documentIDs, id)
		delete(m.contents, id)
	}
	return nil
}

func (m *MockVectorIndex) DeleteByDocument(ctx context.Context, documentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, docID := range m.documentIDs {
		if docID == documentID {
			delete(m.embeddings, id)
			delete(m.documentIDs, id)
			delete(m.contents, id)
		}
	}
	return nil
}

func (m *MockVectorIndex) DeleteByDocuments(ctx context.Context, documentIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, documentID := range documentIDs {
		for id, docID := range m.documentIDs {
			if docID == documentID {
				delete(m.embeddings, id)
				delete(m.documentIDs, id)
				delete(m.contents, id)
			}
		}
	}
	return nil
}

func (m *MockVectorIndex) DeleteBySourceAndContainer(ctx context.Context, sourceID, containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, srcID := range m.sourceIDs {
		if srcID == sourceID {
			// In a real implementation, we'd check document metadata for container_id
			// For the mock, we'll just match by source_id
			delete(m.embeddings, id)
			delete(m.documentIDs, id)
			delete(m.sourceIDs, id)
			delete(m.contents, id)
		}
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
	m.documentIDs = make(map[string]string)
	m.sourceIDs = make(map[string]string)
	m.contents = make(map[string]string)
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
