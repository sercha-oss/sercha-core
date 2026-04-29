package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// UserCreateObserver is invoked once after a user has been successfully
// persisted by UserService.Create. Implementations may perform arbitrary
// post-write work: cache invalidation, telemetry, derived-state updates,
// reconciliation tied to a user's identity, etc.
//
// Contract:
//   - Called synchronously after userStore.Save succeeds.
//   - Not called when userStore.Save fails.
//   - Returned errors are logged and ignored — observer failure does NOT
//     fail the user create. This matches the nil-guarded log-and-continue
//     pattern used by DocumentIngestObserver and DocumentDeleteObserver.
type UserCreateObserver interface {
	OnUserCreated(ctx context.Context, user *domain.User) error
}
