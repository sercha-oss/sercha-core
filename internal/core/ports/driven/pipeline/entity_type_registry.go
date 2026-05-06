package pipeline

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
)

// EntityTypeRegistry is the source of truth for the active entity taxonomy.
// It manages the set of EntityTypeMetadata records that define which named-entity
// categories are recognised by the system.
//
// Implementations may be backed by any store (in-memory, database, distributed
// cache, etc.). List() and Get() are not assumed to be cheap; consumers should
// not call them in hot loops.
type EntityTypeRegistry interface {
	// Register adds a new entity type to the taxonomy.
	//
	// It applies a three-case contract on the outcome:
	//
	//   - success (nil error): metadata.ID was not previously registered and the
	//     record has been persisted. Subsequent Get calls will return it.
	//
	//   - duplicate ID (non-nil error): metadata.ID is already registered.
	//     The existing record is unchanged. Callers that want to mutate an
	//     existing record must use Update.
	//
	//   - storage error (non-nil error): a problem with the underlying store
	//     prevented the write. The taxonomy state is undefined for this ID.
	Register(ctx context.Context, metadata pipeline.EntityTypeMetadata) error

	// Update mutates an existing entity type in the taxonomy.
	//
	// It applies a three-case contract on the outcome:
	//
	//   - success (nil error): metadata.ID was already registered and the stored
	//     record has been replaced with the supplied value.
	//
	//   - unknown ID (non-nil error): metadata.ID is not registered. The call is
	//     a no-op. Callers that want to add a new record must use Register.
	//
	//   - storage error (non-nil error): a problem with the underlying store
	//     prevented the write. The previous record may or may not have been
	//     overwritten; callers should not assume the taxonomy is consistent.
	Update(ctx context.Context, metadata pipeline.EntityTypeMetadata) error

	// Delete removes an entity type from the taxonomy.
	//
	// It applies a three-case contract on the outcome:
	//
	//   - success (nil error): id was registered and the record has been removed.
	//     Subsequent Get calls for id will return found=false.
	//
	//   - unknown ID (non-nil error): id is not registered. The taxonomy is
	//     unchanged.
	//
	//   - storage error (non-nil error): a problem with the underlying store
	//     prevented the deletion. The record may or may not have been removed.
	Delete(ctx context.Context, id pipeline.EntityType) error

	// Get retrieves an entity type by its ID.
	//
	// It applies a three-case contract on the return:
	//
	//   - hit (metadata, true, nil): id is registered and the stored record is
	//     returned.
	//
	//   - miss (zero-value metadata, false, nil): id is not registered. This is
	//     not an error; callers should treat a false found as "unknown category".
	//
	//   - storage error (zero-value metadata, false, non-nil error): a problem
	//     with the underlying store prevented the read. The returned found value
	//     is false and must not be trusted.
	Get(ctx context.Context, id pipeline.EntityType) (pipeline.EntityTypeMetadata, bool, error)

	// List returns all registered entity types. The order of results is not
	// guaranteed to be stable across calls.
	//
	// Because implementations may be backed by a remote store, List() is not
	// assumed to be cheap. Consumers should cache the result where appropriate
	// and must not call List in hot loops.
	//
	// A nil error and an empty slice indicate that the taxonomy is empty, not
	// that there was a problem.
	List(ctx context.Context) ([]pipeline.EntityTypeMetadata, error)

	// SetOwningDetector claims ownership of an entity category for the given
	// detector.
	//
	// It applies a four-case contract on the outcome:
	//
	//   - success — no prior owner (nil error): id was registered and had no
	//     owner (OwningDetector == ""). The OwningDetector field of the stored
	//     record is now set to detectorID.
	//
	//   - success — same owner (nil error): id was registered and its
	//     OwningDetector already equals detectorID. The call is idempotent and
	//     treated as success; the record is unchanged.
	//
	//   - success — detectorID is empty (nil error): passing an empty detectorID
	//     clears the ownership of the category. The OwningDetector field of the
	//     stored record is set to "". Clearing an already-unowned category is
	//     also idempotent and treated as success.
	//
	//   - owner conflict (non-nil error): id was registered but is already owned
	//     by a different detector (OwningDetector != "" && OwningDetector !=
	//     detectorID). The record is unchanged. Callers must not reassign
	//     ownership without first clearing it via a call with an empty detectorID.
	//
	//   - unknown ID (non-nil error): id is not registered. The call is a no-op.
	//
	//   - storage error (non-nil error): a problem with the underlying store
	//     prevented the write.
	SetOwningDetector(ctx context.Context, id pipeline.EntityType, detectorID string) error
}
