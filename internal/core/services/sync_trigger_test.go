package services

import (
	"context"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

func TestSyncTriggerFromContext_DefaultIsScheduled(t *testing.T) {
	// A bare context (or any code path that didn't go through the worker)
	// should report "scheduled" — mirrors the historical default and
	// prevents the audit decorator from writing empty trigger labels.
	if got := SyncTriggerFromContext(context.Background()); got != domain.TaskTriggerScheduled {
		t.Errorf("SyncTriggerFromContext(empty) = %q, want %q", got, domain.TaskTriggerScheduled)
	}
}

func TestSyncTriggerFromContext_RoundTripsKnownValues(t *testing.T) {
	cases := []domain.TaskTrigger{
		domain.TaskTriggerManual,
		domain.TaskTriggerScheduled,
		domain.TaskTriggerWebhook,
	}
	for _, tr := range cases {
		t.Run(string(tr), func(t *testing.T) {
			ctx := WithSyncTrigger(context.Background(), tr)
			if got := SyncTriggerFromContext(ctx); got != tr {
				t.Errorf("round-trip %q: got %q", tr, got)
			}
		})
	}
}

func TestSyncTriggerFromContext_EmptyValueFallsBackToScheduled(t *testing.T) {
	// Defensive: if a caller threads an empty TaskTrigger onto context
	// (zero value) the helper must still return the safe default rather
	// than propagating the empty string. The audit decorator uses the
	// returned trigger directly to label rows.
	ctx := WithSyncTrigger(context.Background(), domain.TaskTrigger(""))
	if got := SyncTriggerFromContext(ctx); got != domain.TaskTriggerScheduled {
		t.Errorf("empty trigger: got %q, want %q", got, domain.TaskTriggerScheduled)
	}
}

func TestWithSyncTrigger_DoesNotMutateParent(t *testing.T) {
	// Standard context immutability guarantee, asserted to catch any
	// future refactor that might switch to a mutable carrier.
	parent := context.Background()
	child := WithSyncTrigger(parent, domain.TaskTriggerManual)
	if got := SyncTriggerFromContext(parent); got != domain.TaskTriggerScheduled {
		t.Errorf("parent ctx leaked trigger: got %q", got)
	}
	if got := SyncTriggerFromContext(child); got != domain.TaskTriggerManual {
		t.Errorf("child ctx missing trigger: got %q", got)
	}
}
