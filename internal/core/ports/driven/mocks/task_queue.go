package mocks

import (
	"context"
	"fmt"
	"sync"

	"github.com/custodia-labs/sercha-core/internal/core/domain"
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
)

// MockTaskQueue is a mock implementation of TaskQueue for testing
type MockTaskQueue struct {
	mu                sync.RWMutex
	tasks             map[string]*domain.Task
	enqueueCalls      int
	enqueueBatchCalls int
	enqueueError      error
	listTasksFunc     func(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error)
	countTasksFunc    func(ctx context.Context, filter driven.TaskFilter) (int64, error)
	getJobStatsFunc   func(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.JobStats, error)
	getTaskFunc       func(ctx context.Context, taskID string) (*domain.Task, error)
}

// NewMockTaskQueue creates a new MockTaskQueue
func NewMockTaskQueue() *MockTaskQueue {
	return &MockTaskQueue{
		tasks: make(map[string]*domain.Task),
	}
}

// SetEnqueueError sets an error to return from Enqueue/EnqueueBatch
func (m *MockTaskQueue) SetEnqueueError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueueError = err
}

// SetListTasksFunc sets a custom function for ListTasks
func (m *MockTaskQueue) SetListTasksFunc(fn func(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listTasksFunc = fn
}

// SetCountTasksFunc sets a custom function for CountTasks
func (m *MockTaskQueue) SetCountTasksFunc(fn func(ctx context.Context, filter driven.TaskFilter) (int64, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.countTasksFunc = fn
}

// SetGetJobStatsFunc sets a custom function for GetJobStats
func (m *MockTaskQueue) SetGetJobStatsFunc(fn func(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.JobStats, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getJobStatsFunc = fn
}

// SetGetTaskFunc sets a custom function for GetTask
func (m *MockTaskQueue) SetGetTaskFunc(fn func(ctx context.Context, taskID string) (*domain.Task, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getTaskFunc = fn
}

// GetEnqueueCalls returns the number of times Enqueue was called
func (m *MockTaskQueue) GetEnqueueCalls() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enqueueCalls
}

// GetEnqueueBatchCalls returns the number of times EnqueueBatch was called
func (m *MockTaskQueue) GetEnqueueBatchCalls() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enqueueBatchCalls
}

// GetTasks returns all tasks
func (m *MockTaskQueue) GetTasks() []*domain.Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.Task
	for _, task := range m.tasks {
		result = append(result, task)
	}
	return result
}

func (m *MockTaskQueue) Enqueue(ctx context.Context, task *domain.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueueCalls++
	if m.enqueueError != nil {
		return m.enqueueError
	}
	m.tasks[task.ID] = task
	return nil
}

func (m *MockTaskQueue) EnqueueBatch(ctx context.Context, tasks []*domain.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueueBatchCalls++
	if m.enqueueError != nil {
		return m.enqueueError
	}
	for _, task := range tasks {
		m.tasks[task.ID] = task
	}
	return nil
}

func (m *MockTaskQueue) Dequeue(ctx context.Context) (*domain.Task, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockTaskQueue) DequeueWithTimeout(ctx context.Context, timeout int) (*domain.Task, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockTaskQueue) Ack(ctx context.Context, taskID string) error {
	return fmt.Errorf("not implemented")
}

func (m *MockTaskQueue) Nack(ctx context.Context, taskID string, reason string) error {
	return fmt.Errorf("not implemented")
}

func (m *MockTaskQueue) GetTask(ctx context.Context, taskID string) (*domain.Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getTaskFunc != nil {
		return m.getTaskFunc(ctx, taskID)
	}
	task, ok := m.tasks[taskID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return task, nil
}

func (m *MockTaskQueue) ListTasks(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.listTasksFunc != nil {
		return m.listTasksFunc(ctx, filter)
	}
	// Default implementation: return all tasks
	var result []*domain.Task
	for _, task := range m.tasks {
		result = append(result, task)
	}
	return result, nil
}

func (m *MockTaskQueue) CancelTask(ctx context.Context, taskID string) error {
	return fmt.Errorf("not implemented")
}

func (m *MockTaskQueue) PurgeTasks(ctx context.Context, olderThan int) (int, error) {
	return 0, fmt.Errorf("not implemented")
}

func (m *MockTaskQueue) Stats(ctx context.Context) (*driven.QueueStats, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockTaskQueue) Ping(ctx context.Context) error {
	return nil
}

func (m *MockTaskQueue) GetJobStats(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.JobStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getJobStatsFunc != nil {
		return m.getJobStatsFunc(ctx, teamID, period)
	}
	// Default: return empty stats
	return domain.NewJobStats(period), nil
}

func (m *MockTaskQueue) CountTasks(ctx context.Context, filter driven.TaskFilter) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.countTasksFunc != nil {
		return m.countTasksFunc(ctx, filter)
	}
	// Default: count all tasks
	return int64(len(m.tasks)), nil
}

func (m *MockTaskQueue) Close() error {
	return nil
}
