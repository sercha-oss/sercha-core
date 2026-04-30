package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// mockTaskQueue implements driven.TaskQueue for testing
type mockTaskQueue struct {
	mu           sync.Mutex
	tasks        []*domain.Task
	dequeueDelay time.Duration
	enqueueFn    func(*domain.Task) error
	dequeueFn    func() (*domain.Task, error)
	ackFn        func(string) error
	nackFn       func(string, string) error
	pingFn       func() error
}

func newMockTaskQueue() *mockTaskQueue {
	return &mockTaskQueue{
		tasks: make([]*domain.Task, 0),
	}
}

func (m *mockTaskQueue) Enqueue(ctx context.Context, task *domain.Task) error {
	if m.enqueueFn != nil {
		return m.enqueueFn(task)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks = append(m.tasks, task)
	return nil
}

func (m *mockTaskQueue) EnqueueBatch(ctx context.Context, tasks []*domain.Task) error {
	for _, t := range tasks {
		if err := m.Enqueue(ctx, t); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockTaskQueue) Dequeue(ctx context.Context) (*domain.Task, error) {
	if m.dequeueFn != nil {
		return m.dequeueFn()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.tasks) == 0 {
		return nil, nil
	}
	task := m.tasks[0]
	m.tasks = m.tasks[1:]
	return task, nil
}

func (m *mockTaskQueue) DequeueWithTimeout(ctx context.Context, timeout int) (*domain.Task, error) {
	if m.dequeueDelay > 0 {
		select {
		case <-time.After(m.dequeueDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return m.Dequeue(ctx)
}

func (m *mockTaskQueue) Ack(ctx context.Context, taskID string) error {
	if m.ackFn != nil {
		return m.ackFn(taskID)
	}
	return nil
}

func (m *mockTaskQueue) Nack(ctx context.Context, taskID string, reason string) error {
	if m.nackFn != nil {
		return m.nackFn(taskID, reason)
	}
	return nil
}

func (m *mockTaskQueue) GetTask(ctx context.Context, taskID string) (*domain.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tasks {
		if t.ID == taskID {
			return t, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockTaskQueue) ListTasks(ctx context.Context, filter driven.TaskFilter) ([]*domain.Task, error) {
	return m.tasks, nil
}

func (m *mockTaskQueue) CancelTask(ctx context.Context, taskID string) error {
	return nil
}

func (m *mockTaskQueue) PurgeTasks(ctx context.Context, olderThan int) (int, error) {
	return 0, nil
}

func (m *mockTaskQueue) Stats(ctx context.Context) (*driven.QueueStats, error) {
	return &driven.QueueStats{
		PendingCount: int64(len(m.tasks)),
	}, nil
}

func (m *mockTaskQueue) Ping(ctx context.Context) error {
	if m.pingFn != nil {
		return m.pingFn()
	}
	return nil
}

func (m *mockTaskQueue) Close() error {
	return nil
}

func (m *mockTaskQueue) GetJobStats(ctx context.Context, teamID string, period domain.AnalyticsPeriod) (*domain.JobStats, error) {
	return domain.NewJobStats(period), nil
}

func (m *mockTaskQueue) CountTasks(ctx context.Context, filter driven.TaskFilter) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int64(len(m.tasks)), nil
}

func TestNewWorker(t *testing.T) {
	queue := newMockTaskQueue()
	logger := slog.Default()

	w := NewWorker(WorkerConfig{
		TaskQueue:      queue,
		Logger:         logger,
		Concurrency:    2,
		DequeueTimeout: 5,
	})

	if w == nil {
		t.Fatal("expected non-nil worker")
	}
	if w.concurrency != 2 {
		t.Errorf("expected concurrency 2, got %d", w.concurrency)
	}
	if w.dequeueTimeout != 5 {
		t.Errorf("expected dequeue timeout 5, got %d", w.dequeueTimeout)
	}
}

func TestNewWorker_Defaults(t *testing.T) {
	queue := newMockTaskQueue()

	w := NewWorker(WorkerConfig{
		TaskQueue:      queue,
		Concurrency:    0, // Should default to 1
		DequeueTimeout: 0, // Should default to 5
	})

	if w.concurrency != 1 {
		t.Errorf("expected default concurrency 1, got %d", w.concurrency)
	}
	if w.dequeueTimeout != 5 {
		t.Errorf("expected default dequeue timeout 5, got %d", w.dequeueTimeout)
	}
	if w.logger == nil {
		t.Error("expected default logger")
	}
}

func TestWorker_StartStop(t *testing.T) {
	queue := newMockTaskQueue()
	// Add delay so workers don't spin too fast
	queue.dequeueDelay = 100 * time.Millisecond

	w := NewWorker(WorkerConfig{
		TaskQueue:      queue,
		Concurrency:    1,
		DequeueTimeout: 1,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := w.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start worker: %v", err)
	}

	// Verify worker is running
	health := w.Health(ctx)
	if !health.Running {
		t.Error("expected worker to be running")
	}

	// Start again should be no-op
	err = w.Start(ctx)
	if err != nil {
		t.Errorf("second start should not error: %v", err)
	}

	// Stop the worker
	w.Stop()

	// Verify worker is stopped
	health = w.Health(ctx)
	if health.Running {
		t.Error("expected worker to be stopped")
	}

	// Stop again should be no-op
	w.Stop() // Should not panic
}

func TestWorker_Health(t *testing.T) {
	queue := newMockTaskQueue()

	w := NewWorker(WorkerConfig{
		TaskQueue:   queue,
		Concurrency: 1,
	})

	ctx := context.Background()

	// Not running initially
	health := w.Health(ctx)
	if health.Running {
		t.Error("expected not running")
	}
	if !health.QueueHealth {
		t.Error("expected queue to be healthy")
	}
}

func TestWorker_Health_QueueError(t *testing.T) {
	queue := newMockTaskQueue()
	queue.pingFn = func() error {
		return errors.New("connection failed")
	}

	w := NewWorker(WorkerConfig{
		TaskQueue:   queue,
		Concurrency: 1,
	})

	ctx := context.Background()

	health := w.Health(ctx)
	if health.QueueHealth {
		t.Error("expected queue to be unhealthy")
	}
	if health.Error != "connection failed" {
		t.Errorf("expected error message, got %q", health.Error)
	}
}

func TestWorker_ProcessTask_UnknownType(t *testing.T) {
	queue := newMockTaskQueue()

	var nacked []string
	queue.nackFn = func(taskID, reason string) error {
		nacked = append(nacked, taskID)
		return nil
	}

	// Create task with unknown type
	task := &domain.Task{
		ID:     "task-123",
		Type:   domain.TaskType("unknown_type"),
		TeamID: "team-123",
	}

	w := NewWorker(WorkerConfig{
		TaskQueue:   queue,
		Concurrency: 1,
	})

	ctx := context.Background()

	// Process the task directly
	w.processTask(ctx, task, slog.Default())

	// Should be nacked due to unknown type
	if len(nacked) != 1 {
		t.Errorf("expected 1 nack for unknown type, got %d", len(nacked))
	}
}

func TestWorker_ProcessTask_MissingSourceID(t *testing.T) {
	queue := newMockTaskQueue()

	var nacked []string
	queue.nackFn = func(taskID, reason string) error {
		nacked = append(nacked, taskID)
		return nil
	}

	// Create sync_source task without source_id in payload
	task := &domain.Task{
		ID:      "task-123",
		Type:    domain.TaskTypeSyncSource,
		TeamID:  "team-123",
		Payload: nil, // No source_id
	}

	w := NewWorker(WorkerConfig{
		TaskQueue:   queue,
		Concurrency: 1,
	})

	ctx := context.Background()

	// Process the task - should fail due to missing source_id
	w.processTask(ctx, task, slog.Default())

	// Should be nacked due to missing source_id
	if len(nacked) != 1 {
		t.Errorf("expected 1 nack for missing source_id, got %d", len(nacked))
	}
}

func TestWorker_ContextCancellation(t *testing.T) {
	queue := newMockTaskQueue()
	// Slow dequeue so we can cancel
	queue.dequeueDelay = 500 * time.Millisecond

	w := NewWorker(WorkerConfig{
		TaskQueue:      queue,
		Concurrency:    1,
		DequeueTimeout: 10,
	})

	ctx, cancel := context.WithCancel(context.Background())

	err := w.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start worker: %v", err)
	}

	// Cancel context after short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// Wait for worker to stop
	done := make(chan struct{})
	go func() {
		w.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good, worker stopped
	case <-time.After(2 * time.Second):
		t.Error("worker did not stop after context cancellation")
		w.Stop() // Force stop
	}
}

func TestHealth_Struct(t *testing.T) {
	h := Health{
		Running:     true,
		QueueHealth: true,
		Error:       "",
	}

	if !h.Running {
		t.Error("expected running")
	}
	if !h.QueueHealth {
		t.Error("expected queue healthy")
	}

	h2 := Health{
		Running:     false,
		QueueHealth: false,
		Error:       "some error",
	}

	if h2.Running {
		t.Error("expected not running")
	}
	if h2.QueueHealth {
		t.Error("expected queue unhealthy")
	}
	if h2.Error != "some error" {
		t.Errorf("expected error 'some error', got %q", h2.Error)
	}
}

func TestWorkerConfig(t *testing.T) {
	queue := newMockTaskQueue()
	logger := slog.Default()

	cfg := WorkerConfig{
		TaskQueue:      queue,
		Orchestrator:   nil,
		Scheduler:      nil,
		Logger:         logger,
		Concurrency:    4,
		DequeueTimeout: 10,
	}

	if cfg.TaskQueue == nil {
		t.Error("expected task queue")
	}
	if cfg.Concurrency != 4 {
		t.Errorf("expected concurrency 4, got %d", cfg.Concurrency)
	}
	if cfg.DequeueTimeout != 10 {
		t.Errorf("expected dequeue timeout 10, got %d", cfg.DequeueTimeout)
	}
}

// Test that mock implements the interface
func TestMockTaskQueueInterface(t *testing.T) {
	var _ driven.TaskQueue = (*mockTaskQueue)(nil)
}

// mockOrchestrator implements Orchestrator for testing
type mockOrchestrator struct {
	syncSourceFn func(ctx context.Context, sourceID string) (*domain.SyncResult, error)
	syncAllFn    func(ctx context.Context) ([]*domain.SyncResult, error)
}

func (m *mockOrchestrator) SyncSource(ctx context.Context, sourceID string) (*domain.SyncResult, error) {
	if m.syncSourceFn != nil {
		return m.syncSourceFn(ctx, sourceID)
	}
	return &domain.SyncResult{Success: true, SourceID: sourceID}, nil
}

func (m *mockOrchestrator) SyncAll(ctx context.Context) ([]*domain.SyncResult, error) {
	if m.syncAllFn != nil {
		return m.syncAllFn(ctx)
	}
	return []*domain.SyncResult{{Success: true}}, nil
}

// Test that mock implements the interface
func TestMockOrchestratorInterface(t *testing.T) {
	var _ Orchestrator = (*mockOrchestrator)(nil)
}

func TestWorker_HandleSyncSource_Success(t *testing.T) {
	queue := newMockTaskQueue()
	orch := &mockOrchestrator{
		syncSourceFn: func(ctx context.Context, sourceID string) (*domain.SyncResult, error) {
			return &domain.SyncResult{
				Success:  true,
				SourceID: sourceID,
			}, nil
		},
	}

	var acked []string
	queue.ackFn = func(taskID string) error {
		acked = append(acked, taskID)
		return nil
	}

	// Create sync_source task with source_id
	task := &domain.Task{
		ID:      "task-123",
		Type:    domain.TaskTypeSyncSource,
		TeamID:  "team-123",
		Payload: map[string]string{"source_id": "source-456"},
	}

	w := NewWorker(WorkerConfig{
		TaskQueue:    queue,
		Orchestrator: orch,
		Concurrency:  1,
	})

	ctx := context.Background()
	w.processTask(ctx, task, slog.Default())

	// Should be acked since sync was successful
	if len(acked) != 1 {
		t.Errorf("expected 1 ack, got %d", len(acked))
	}
}

func TestWorker_HandleSyncSource_Error(t *testing.T) {
	queue := newMockTaskQueue()
	orch := &mockOrchestrator{
		syncSourceFn: func(ctx context.Context, sourceID string) (*domain.SyncResult, error) {
			return nil, errors.New("connection failed")
		},
	}

	var nacked []string
	queue.nackFn = func(taskID, reason string) error {
		nacked = append(nacked, taskID)
		return nil
	}

	task := &domain.Task{
		ID:      "task-123",
		Type:    domain.TaskTypeSyncSource,
		TeamID:  "team-123",
		Payload: map[string]string{"source_id": "source-456"},
	}

	w := NewWorker(WorkerConfig{
		TaskQueue:    queue,
		Orchestrator: orch,
		Concurrency:  1,
	})

	ctx := context.Background()
	w.processTask(ctx, task, slog.Default())

	// Should be nacked since sync failed
	if len(nacked) != 1 {
		t.Errorf("expected 1 nack, got %d", len(nacked))
	}
}

func TestWorker_HandleSyncSource_NotSuccessful(t *testing.T) {
	queue := newMockTaskQueue()
	orch := &mockOrchestrator{
		syncSourceFn: func(ctx context.Context, sourceID string) (*domain.SyncResult, error) {
			return &domain.SyncResult{
				Success:  false,
				SourceID: sourceID,
				Error:    "source is disabled",
			}, nil
		},
	}

	var nacked []string
	queue.nackFn = func(taskID, reason string) error {
		nacked = append(nacked, taskID)
		return nil
	}

	task := &domain.Task{
		ID:      "task-123",
		Type:    domain.TaskTypeSyncSource,
		TeamID:  "team-123",
		Payload: map[string]string{"source_id": "source-456"},
	}

	w := NewWorker(WorkerConfig{
		TaskQueue:    queue,
		Orchestrator: orch,
		Concurrency:  1,
	})

	ctx := context.Background()
	w.processTask(ctx, task, slog.Default())

	// Should be nacked since sync result indicates failure
	if len(nacked) != 1 {
		t.Errorf("expected 1 nack, got %d", len(nacked))
	}
}

func TestWorker_HandleSyncAll_Success(t *testing.T) {
	queue := newMockTaskQueue()
	orch := &mockOrchestrator{
		syncAllFn: func(ctx context.Context) ([]*domain.SyncResult, error) {
			return []*domain.SyncResult{
				{Success: true, SourceID: "source-1"},
				{Success: true, SourceID: "source-2"},
			}, nil
		},
	}

	var acked []string
	queue.ackFn = func(taskID string) error {
		acked = append(acked, taskID)
		return nil
	}

	task := &domain.Task{
		ID:     "task-123",
		Type:   domain.TaskTypeSyncAll,
		TeamID: "team-123",
	}

	w := NewWorker(WorkerConfig{
		TaskQueue:    queue,
		Orchestrator: orch,
		Concurrency:  1,
	})

	ctx := context.Background()
	w.processTask(ctx, task, slog.Default())

	// Should be acked
	if len(acked) != 1 {
		t.Errorf("expected 1 ack, got %d", len(acked))
	}
}

func TestWorker_HandleSyncAll_PartialFailure(t *testing.T) {
	queue := newMockTaskQueue()
	orch := &mockOrchestrator{
		syncAllFn: func(ctx context.Context) ([]*domain.SyncResult, error) {
			return []*domain.SyncResult{
				{Success: true, SourceID: "source-1"},
				{Success: false, SourceID: "source-2", Error: "failed"},
			}, nil
		},
	}

	var acked []string
	queue.ackFn = func(taskID string) error {
		acked = append(acked, taskID)
		return nil
	}

	task := &domain.Task{
		ID:     "task-123",
		Type:   domain.TaskTypeSyncAll,
		TeamID: "team-123",
	}

	w := NewWorker(WorkerConfig{
		TaskQueue:    queue,
		Orchestrator: orch,
		Concurrency:  1,
	})

	ctx := context.Background()
	w.processTask(ctx, task, slog.Default())

	// Should still be acked (partial failures are logged but task succeeds)
	if len(acked) != 1 {
		t.Errorf("expected 1 ack, got %d", len(acked))
	}
}

func TestWorker_HandleSyncAll_Error(t *testing.T) {
	queue := newMockTaskQueue()
	orch := &mockOrchestrator{
		syncAllFn: func(ctx context.Context) ([]*domain.SyncResult, error) {
			return nil, errors.New("database connection failed")
		},
	}

	var nacked []string
	queue.nackFn = func(taskID, reason string) error {
		nacked = append(nacked, taskID)
		return nil
	}

	task := &domain.Task{
		ID:     "task-123",
		Type:   domain.TaskTypeSyncAll,
		TeamID: "team-123",
	}

	w := NewWorker(WorkerConfig{
		TaskQueue:    queue,
		Orchestrator: orch,
		Concurrency:  1,
	})

	ctx := context.Background()
	w.processTask(ctx, task, slog.Default())

	// Should be nacked
	if len(nacked) != 1 {
		t.Errorf("expected 1 nack, got %d", len(nacked))
	}
}

func TestWorker_ProcessLoop_WithTasks(t *testing.T) {
	queue := newMockTaskQueue()
	orch := &mockOrchestrator{
		syncSourceFn: func(ctx context.Context, sourceID string) (*domain.SyncResult, error) {
			return &domain.SyncResult{Success: true, SourceID: sourceID}, nil
		},
	}

	// Queue up a task
	task := &domain.Task{
		ID:      "task-1",
		Type:    domain.TaskTypeSyncSource,
		TeamID:  "team-1",
		Payload: map[string]string{"source_id": "source-1"},
	}
	_ = queue.Enqueue(context.Background(), task)

	// ackedCount is read by the test goroutine in a busy-wait poll BEFORE
	// w.Stop() establishes a happens-before, so plain `[]string` append +
	// len() reads race under -race. atomic.Int32 keeps the assertion shape
	// without needing a mutex.
	var ackedCount atomic.Int32
	queue.ackFn = func(taskID string) error {
		ackedCount.Add(1)
		return nil
	}

	w := NewWorker(WorkerConfig{
		TaskQueue:      queue,
		Orchestrator:   orch,
		Concurrency:    1,
		DequeueTimeout: 1,
	})

	ctx, cancel := context.WithCancel(context.Background())

	err := w.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start worker: %v", err)
	}

	// Wait for task to be processed
	deadline := time.Now().Add(2 * time.Second)
	for ackedCount.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	w.Stop()

	if got := ackedCount.Load(); got != 1 {
		t.Errorf("expected 1 ack, got %d", got)
	}
}

func TestWorker_ProcessLoop_DequeueError(t *testing.T) {
	queue := newMockTaskQueue()
	callCount := 0
	queue.dequeueFn = func() (*domain.Task, error) {
		callCount++
		if callCount < 3 {
			return nil, errors.New("temporary error")
		}
		return nil, nil // No more errors
	}

	w := NewWorker(WorkerConfig{
		TaskQueue:      queue,
		Concurrency:    1,
		DequeueTimeout: 1,
	})

	// Use a longer timeout since there's a 1s backoff after errors
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := w.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start worker: %v", err)
	}

	// Wait for worker to process and handle errors (need time for backoff)
	time.Sleep(2 * time.Second)
	w.Stop()

	// Should have retried after errors
	if callCount < 2 {
		t.Errorf("expected at least 2 dequeue attempts, got %d", callCount)
	}
}

func TestWorker_Ack_Error(t *testing.T) {
	queue := newMockTaskQueue()
	orch := &mockOrchestrator{
		syncSourceFn: func(ctx context.Context, sourceID string) (*domain.SyncResult, error) {
			return &domain.SyncResult{Success: true, SourceID: sourceID}, nil
		},
	}

	ackCalled := false
	queue.ackFn = func(taskID string) error {
		ackCalled = true
		return errors.New("ack failed")
	}

	task := &domain.Task{
		ID:      "task-123",
		Type:    domain.TaskTypeSyncSource,
		TeamID:  "team-123",
		Payload: map[string]string{"source_id": "source-456"},
	}

	w := NewWorker(WorkerConfig{
		TaskQueue:    queue,
		Orchestrator: orch,
		Concurrency:  1,
	})

	ctx := context.Background()
	// This should not panic even if ack fails
	w.processTask(ctx, task, slog.Default())

	if !ackCalled {
		t.Error("expected ack to be called")
	}
}

func TestWorker_Nack_Error(t *testing.T) {
	queue := newMockTaskQueue()
	orch := &mockOrchestrator{
		syncSourceFn: func(ctx context.Context, sourceID string) (*domain.SyncResult, error) {
			return nil, errors.New("sync failed")
		},
	}

	nackCalled := false
	queue.nackFn = func(taskID, reason string) error {
		nackCalled = true
		return errors.New("nack failed")
	}

	task := &domain.Task{
		ID:      "task-123",
		Type:    domain.TaskTypeSyncSource,
		TeamID:  "team-123",
		Payload: map[string]string{"source_id": "source-456"},
	}

	w := NewWorker(WorkerConfig{
		TaskQueue:    queue,
		Orchestrator: orch,
		Concurrency:  1,
	})

	ctx := context.Background()
	// This should not panic even if nack fails
	w.processTask(ctx, task, slog.Default())

	if !nackCalled {
		t.Error("expected nack to be called")
	}
}
