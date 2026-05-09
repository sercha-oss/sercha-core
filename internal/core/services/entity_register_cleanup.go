package services

import (
	"context"
	"log/slog"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// EntityRegisterCleanupObserver implements DocumentDeleteObserver to keep
// the entity_register cache aligned with the document store. When a
// document is deleted (per-doc or via source cascade) the observer
// removes every entity_register row keyed on that document_id.
//
// Without this observer, cache rows accumulate across re-syncs as
// documents change ID — hundreds of orphaned rows in a typical
// deployment. The cache is keyed on (document_id, content_sha256,
// analyzer_version) so orphans never serve stale data, but they
// consume storage and pollute audits.
//
// Observer failures are logged-and-ignored by the caller per Core's
// observer contract. This implementation logs internally too so a
// repeated cache-cleanup failure is visible without grepping the
// orchestrator's log line.
type EntityRegisterCleanupObserver struct {
	register driven.EntityRegister
	logger   *slog.Logger
}

// NewEntityRegisterCleanupObserver wires the observer. register must be
// non-nil; the observer takes no action on a nil register (the field
// is checked at call time so a misconfigured wiring fails open with a
// log message rather than a nil-pointer panic).
func NewEntityRegisterCleanupObserver(register driven.EntityRegister, logger *slog.Logger) *EntityRegisterCleanupObserver {
	if logger == nil {
		logger = slog.Default()
	}
	return &EntityRegisterCleanupObserver{register: register, logger: logger}
}

// OnDocumentDeleted removes all entity_register rows for the deleted
// document. Errors propagate upward so the orchestrator can log them
// alongside other observer outcomes.
func (o *EntityRegisterCleanupObserver) OnDocumentDeleted(ctx context.Context, source *domain.Source, doc *domain.Document) error {
	if o.register == nil || doc == nil {
		return nil
	}
	if err := o.register.DeleteByDocument(ctx, doc.ID); err != nil {
		o.logger.WarnContext(ctx, "entity_register cleanup failed",
			"document_id", doc.ID,
			"source_id", source.ID,
			"error", err,
		)
		return err
	}
	return nil
}

// OnSourceDeleted is a no-op. Per Core's contract, OnDocumentDeleted
// fires for every document in a source before OnSourceDeleted fires —
// so per-doc cleanup has already run by the time we get here. Reserved
// as an extension point if a future shape needs source-level
// entity_register state.
func (o *EntityRegisterCleanupObserver) OnSourceDeleted(ctx context.Context, source *domain.Source) error {
	return nil
}

// Compile-time interface assertion.
var _ driven.DocumentDeleteObserver = (*EntityRegisterCleanupObserver)(nil)
