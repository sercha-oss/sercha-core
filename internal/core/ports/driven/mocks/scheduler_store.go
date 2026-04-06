package mocks

import (
	"context"
	"fmt"
	"sync"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
)

// MockSchedulerStore is a mock implementation of SchedulerStore for testing
type MockSchedulerStore struct {
	mu                    sync.RWMutex
	scheduledTasks        map[string]*domain.ScheduledTask
	listScheduledTasksFunc func(ctx context.Context, teamID string) ([]*domain.ScheduledTask, error)
}

// NewMockSchedulerStore creates a new MockSchedulerStore
func NewMockSchedulerStore() *MockSchedulerStore {
	return &MockSchedulerStore{
		scheduledTasks: make(map[string]*domain.ScheduledTask),
	}
}

// SetListScheduledTasksFunc sets a custom function for ListScheduledTasks
func (m *MockSchedulerStore) SetListScheduledTasksFunc(fn func(ctx context.Context, teamID string) ([]*domain.ScheduledTask, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listScheduledTasksFunc = fn
}

// AddScheduledTask adds a scheduled task to the mock store
func (m *MockSchedulerStore) AddScheduledTask(task *domain.ScheduledTask) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scheduledTasks[task.ID] = task
}

func (m *MockSchedulerStore) GetScheduledTask(ctx context.Context, id string) (*domain.ScheduledTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.scheduledTasks[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return task, nil
}

func (m *MockSchedulerStore) ListScheduledTasks(ctx context.Context, teamID string) ([]*domain.ScheduledTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.listScheduledTasksFunc != nil {
		return m.listScheduledTasksFunc(ctx, teamID)
	}
	var result []*domain.ScheduledTask
	for _, task := range m.scheduledTasks {
		if task.TeamID == teamID {
			result = append(result, task)
		}
	}
	return result, nil
}

func (m *MockSchedulerStore) SaveScheduledTask(ctx context.Context, task *domain.ScheduledTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scheduledTasks[task.ID] = task
	return nil
}

func (m *MockSchedulerStore) DeleteScheduledTask(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.scheduledTasks, id)
	return nil
}

func (m *MockSchedulerStore) GetDueScheduledTasks(ctx context.Context) ([]*domain.ScheduledTask, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockSchedulerStore) UpdateLastRun(ctx context.Context, id string, lastError string) error {
	return fmt.Errorf("not implemented")
}
