package services

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// syncTriggerCtxKey is the unexported key used to attach a TaskTrigger to
// a request/worker context. The value travels across the orchestrator
// without anyone having to add the trigger to the SyncOrchestrator's
// public API — downstream observers can pull it out of context as needed.
//
// The key is intentionally unexported and instance-typed so foreign
// packages can't collide with it; callers must use the WithSyncTrigger
// / SyncTriggerFromContext helpers below.
type syncTriggerCtxKey struct{}

// WithSyncTrigger returns a child context carrying the supplied trigger.
// Used by the worker after dequeuing a task: it reads the trigger off
// the task payload (Task.Trigger()) and threads it onto the orchestrator
// call's context so the rest of the pipeline can see it.
func WithSyncTrigger(ctx context.Context, trigger domain.TaskTrigger) context.Context {
	return context.WithValue(ctx, syncTriggerCtxKey{}, trigger)
}

// SyncTriggerFromContext returns the trigger attached via WithSyncTrigger.
// When no trigger has been attached the helper returns
// TaskTriggerScheduled — matches the historical default so rows logged
// from non-orchestrator code paths (e.g. backfills, tests) get a
// sensible label rather than empty string.
func SyncTriggerFromContext(ctx context.Context) domain.TaskTrigger {
	v, ok := ctx.Value(syncTriggerCtxKey{}).(domain.TaskTrigger)
	if !ok || v == "" {
		return domain.TaskTriggerScheduled
	}
	return v
}
