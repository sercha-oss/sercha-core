package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// UserDeleteObserver is invoked once after a user has been successfully
// removed by UserService.Delete. Implementations may perform arbitrary
// cleanup: cache invalidation, audit writes, derived-state pruning,
// reconciliation tied to a user's identity, etc.
//
// Contract:
//   - Called synchronously after userStore.Delete succeeds.
//   - Not called when userStore.Delete fails.
//   - The user value passed in is captured before the underlying delete
//     and is safe to read inside the observer.
//   - Returned errors are logged and ignored — observer failure does NOT
//     fail the user delete. This matches the nil-guarded log-and-continue
//     pattern used by DocumentIngestObserver and DocumentDeleteObserver.
type UserDeleteObserver interface {
	OnUserDeleted(ctx context.Context, user *domain.User) error
}
