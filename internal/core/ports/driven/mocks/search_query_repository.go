package mocks

import (
	"context"
	"sync"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// MockSearchQueryRepository is a mock implementation of SearchQueryRepository for testing
type MockSearchQueryRepository struct {
	mu                      sync.RWMutex
	queries                 []*domain.SearchQuery
	getSearchHistoryFunc    func(ctx context.Context, teamID string, limit int) ([]*domain.SearchQuery, error)
	getSearchAnalyticsFunc  func(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.SearchAnalytics, error)
	getSearchMetricsFunc    func(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.SearchMetrics, error)
}

// NewMockSearchQueryRepository creates a new MockSearchQueryRepository
func NewMockSearchQueryRepository() *MockSearchQueryRepository {
	return &MockSearchQueryRepository{
		queries: make([]*domain.SearchQuery, 0),
	}
}

// SetGetSearchHistoryFunc sets a custom function for GetSearchHistory
func (m *MockSearchQueryRepository) SetGetSearchHistoryFunc(fn func(ctx context.Context, teamID string, limit int) ([]*domain.SearchQuery, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getSearchHistoryFunc = fn
}

// SetGetSearchAnalyticsFunc sets a custom function for GetSearchAnalytics
func (m *MockSearchQueryRepository) SetGetSearchAnalyticsFunc(fn func(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.SearchAnalytics, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getSearchAnalyticsFunc = fn
}

// SetGetSearchMetricsFunc sets a custom function for GetSearchMetrics
func (m *MockSearchQueryRepository) SetGetSearchMetricsFunc(fn func(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.SearchMetrics, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getSearchMetricsFunc = fn
}

func (m *MockSearchQueryRepository) Save(ctx context.Context, query *domain.SearchQuery) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queries = append(m.queries, query)
	return nil
}

func (m *MockSearchQueryRepository) GetSearchHistory(ctx context.Context, teamID string, limit int) ([]*domain.SearchQuery, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getSearchHistoryFunc != nil {
		return m.getSearchHistoryFunc(ctx, teamID, limit)
	}
	// Default: return empty history
	return []*domain.SearchQuery{}, nil
}

func (m *MockSearchQueryRepository) GetSearchAnalytics(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.SearchAnalytics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getSearchAnalyticsFunc != nil {
		return m.getSearchAnalyticsFunc(ctx, teamID, period)
	}
	// Default: return empty analytics
	return &domain.SearchAnalytics{
		Period:         period,
		SearchesByMode: make(map[domain.SearchMode]int64),
	}, nil
}

func (m *MockSearchQueryRepository) GetSearchMetrics(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.SearchMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getSearchMetricsFunc != nil {
		return m.getSearchMetricsFunc(ctx, teamID, period)
	}
	// Default: return empty metrics
	return &domain.SearchMetrics{
		Period: period,
	}, nil
}
