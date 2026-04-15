package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/services"
)

// Orchestrator defines the sync operations needed by the worker.
// This is a minimal interface to allow for testing.
type Orchestrator interface {
	SyncSource(ctx context.Context, sourceID string) (*domain.SyncResult, error)
	SyncAll(ctx context.Context) ([]*domain.SyncResult, error)
}

// Worker processes tasks from the task queue.
// It runs the sync orchestrator for each sync task.
type Worker struct {
	taskQueue    driven.TaskQueue
	orchestrator Orchestrator
	scheduler    *services.Scheduler
	logger       *slog.Logger

	// Configuration
	concurrency    int
	dequeueTimeout int // seconds

	// Internal state
	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// WorkerConfig holds configuration for the worker.
type WorkerConfig struct {
	TaskQueue      driven.TaskQueue
	Orchestrator   Orchestrator
	Scheduler      *services.Scheduler
	Logger         *slog.Logger
	Concurrency    int // Number of concurrent task processors
	DequeueTimeout int // Seconds to wait for a task before checking again
}

// NewWorker creates a new task worker.
func NewWorker(cfg WorkerConfig) *Worker {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	dequeueTimeout := cfg.DequeueTimeout
	if dequeueTimeout <= 0 {
		dequeueTimeout = 5
	}

	return &Worker{
		taskQueue:      cfg.TaskQueue,
		orchestrator:   cfg.Orchestrator,
		scheduler:      cfg.Scheduler,
		logger:         logger,
		concurrency:    concurrency,
		dequeueTimeout: dequeueTimeout,
	}
}

// Start begins the worker loop.
// It runs until Stop is called or context is cancelled.
func (w *Worker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.stopCh = make(chan struct{})
	w.doneCh = make(chan struct{})
	w.mu.Unlock()

	w.logger.Info("worker starting",
		"concurrency", w.concurrency,
		"dequeue_timeout", w.dequeueTimeout,
	)

	// Start the scheduler if provided
	if w.scheduler != nil {
		if err := w.scheduler.Start(ctx); err != nil {
			w.logger.Error("failed to start scheduler", "error", err)
		}
	}

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < w.concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			w.processLoop(ctx, workerID)
		}(i)
	}

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(w.doneCh)
	}()

	return nil
}

// Stop gracefully stops the worker.
func (w *Worker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	close(w.stopCh)
	w.mu.Unlock()

	// Stop the scheduler
	if w.scheduler != nil {
		w.scheduler.Stop()
	}

	// Wait for workers to finish
	<-w.doneCh

	w.mu.Lock()
	w.running = false
	w.mu.Unlock()

	w.logger.Info("worker stopped")
}

// Wait blocks until the worker stops.
func (w *Worker) Wait() {
	<-w.doneCh
}

// processLoop is the main processing loop for a worker goroutine.
func (w *Worker) processLoop(ctx context.Context, workerID int) {
	logger := w.logger.With("worker_id", workerID)
	logger.Info("worker goroutine started")

	for {
		select {
		case <-ctx.Done():
			logger.Info("worker context cancelled")
			return
		case <-w.stopCh:
			logger.Info("worker stop signal received")
			return
		default:
		}

		// Dequeue a task with timeout
		task, err := w.taskQueue.DequeueWithTimeout(ctx, w.dequeueTimeout)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			logger.Error("failed to dequeue task", "error", err)
			time.Sleep(time.Second) // Back off on error
			continue
		}

		if task == nil {
			// No task available, continue
			continue
		}

		// Process the task
		w.processTask(ctx, task, logger)
	}
}

// processTask processes a single task.
func (w *Worker) processTask(ctx context.Context, task *domain.Task, logger *slog.Logger) {
	logger = logger.With("task_id", task.ID, "task_type", task.Type, "team_id", task.TeamID)
	logger.Info("processing task")

	startTime := time.Now()
	var err error

	switch task.Type {
	case domain.TaskTypeSyncSource:
		err = w.handleSyncSource(ctx, task)
	case domain.TaskTypeSyncAll:
		err = w.handleSyncAll(ctx, task)
	case domain.TaskTypeSyncContainer:
		err = w.handleSyncContainer(ctx, task)
	default:
		err = fmt.Errorf("unknown task type: %s", task.Type)
	}

	duration := time.Since(startTime)

	if err != nil {
		logger.Error("task failed",
			"duration", duration,
			"error", err,
		)

		// Nack the task so it can be retried
		if nackErr := w.taskQueue.Nack(ctx, task.ID, err.Error()); nackErr != nil {
			logger.Error("failed to nack task", "nack_error", nackErr)
		}
		return
	}

	logger.Info("task completed", "duration", duration)

	// Ack the task
	if ackErr := w.taskQueue.Ack(ctx, task.ID); ackErr != nil {
		logger.Error("failed to ack task", "ack_error", ackErr)
	}
}

// handleSyncSource handles a sync_source task.
func (w *Worker) handleSyncSource(ctx context.Context, task *domain.Task) error {
	sourceID := task.SourceID()
	if sourceID == "" {
		return fmt.Errorf("source_id not found in task payload")
	}

	result, err := w.orchestrator.SyncSource(ctx, sourceID)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("sync failed: %s", result.Error)
	}

	return nil
}

// handleSyncAll handles a sync_all task.
func (w *Worker) handleSyncAll(ctx context.Context, task *domain.Task) error {
	results, err := w.orchestrator.SyncAll(ctx)
	if err != nil {
		return err
	}

	// Check if any sync failed
	var failures []string
	for _, result := range results {
		if !result.Success {
			failures = append(failures, fmt.Sprintf("%s: %s", result.SourceID, result.Error))
		}
	}

	if len(failures) > 0 {
		w.logger.Warn("some syncs failed",
			"total", len(results),
			"failed", len(failures),
		)
		// We still consider the task successful if at least some sources synced
		// The individual failures are logged and can be investigated
	}

	return nil
}

// handleSyncContainer handles a sync_container task.
func (w *Worker) handleSyncContainer(ctx context.Context, task *domain.Task) error {
	sourceID := task.SourceID()
	if sourceID == "" {
		return fmt.Errorf("source_id not found in task payload")
	}

	containerID := task.ContainerID()
	if containerID == "" {
		return fmt.Errorf("container_id not found in task payload")
	}

	// Type assert to access SyncContainer method
	type containerSyncer interface {
		SyncContainer(ctx context.Context, sourceID, containerID string) (*domain.SyncResult, error)
	}

	syncer, ok := w.orchestrator.(containerSyncer)
	if !ok {
		return fmt.Errorf("orchestrator does not support container sync")
	}

	result, err := syncer.SyncContainer(ctx, sourceID, containerID)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("sync failed: %s", result.Error)
	}

	return nil
}

// Health returns health status of the worker.
type Health struct {
	Running     bool   `json:"running"`
	QueueHealth bool   `json:"queue_health"`
	Error       string `json:"error,omitempty"`
}

// Health returns the health status of the worker.
func (w *Worker) Health(ctx context.Context) Health {
	w.mu.RLock()
	running := w.running
	w.mu.RUnlock()

	health := Health{
		Running: running,
	}

	// Check queue health
	if err := w.taskQueue.Ping(ctx); err != nil {
		health.QueueHealth = false
		health.Error = err.Error()
	} else {
		health.QueueHealth = true
	}

	return health
}
