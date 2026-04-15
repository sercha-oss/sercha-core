package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) (*redis.Client, func()) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return client, func() {
		_ = client.Close()
		mr.Close()
	}
}

func TestNewLock(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	lock := NewLock(client)

	if lock.ownerID == "" {
		t.Error("expected non-empty owner ID")
	}
}

func TestLock_OwnerID_Unique(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	lock1 := NewLock(client)
	lock2 := NewLock(client)

	if lock1.OwnerID() == lock2.OwnerID() {
		t.Errorf("expected unique owner IDs, got same: %s", lock1.OwnerID())
	}
}

func TestLock_Acquire_Success(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	lock := NewLock(client)
	ctx := context.Background()

	acquired, err := lock.Acquire(ctx, "test-lock", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Error("expected to acquire lock")
	}
}

func TestLock_Acquire_AlreadyHeld(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	lock1 := NewLock(client)
	lock2 := NewLock(client)
	ctx := context.Background()

	// First lock acquires
	acquired, err := lock1.Acquire(ctx, "test-lock", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Error("expected first lock to acquire")
	}

	// Second lock cannot acquire
	acquired, err = lock2.Acquire(ctx, "test-lock", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acquired {
		t.Error("expected second lock to fail")
	}
}

func TestLock_Acquire_Reentrant(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	lock := NewLock(client)
	ctx := context.Background()

	// First acquire succeeds
	acquired, err := lock.Acquire(ctx, "test-lock", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Error("expected to acquire lock")
	}

	// Same instance cannot re-acquire (not reentrant)
	acquired, err = lock.Acquire(ctx, "test-lock", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acquired {
		t.Error("expected reentrant acquire to fail")
	}
}

func TestLock_Release_Success(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	lock := NewLock(client)
	ctx := context.Background()

	// Acquire lock
	acquired, err := lock.Acquire(ctx, "test-lock", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Error("expected to acquire lock")
	}

	// Release lock
	err = lock.Release(ctx, "test-lock")
	if err != nil {
		t.Fatalf("unexpected error on release: %v", err)
	}

	// Should be able to acquire again
	acquired, err = lock.Acquire(ctx, "test-lock", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Error("expected to acquire lock after release")
	}
}

func TestLock_Release_NotHeld(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	lock := NewLock(client)
	ctx := context.Background()

	// Release without acquire should not error
	err := lock.Release(ctx, "test-lock")
	if err != nil {
		t.Errorf("unexpected error releasing unheld lock: %v", err)
	}
}

func TestLock_Release_ByDifferentOwner(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	lock1 := NewLock(client)
	lock2 := NewLock(client)
	ctx := context.Background()

	// Lock1 acquires
	acquired, err := lock1.Acquire(ctx, "test-lock", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Error("expected to acquire lock")
	}

	// Lock2 tries to release - should not actually release
	err = lock2.Release(ctx, "test-lock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Lock2 still cannot acquire (lock1 still holds)
	acquired, err = lock2.Acquire(ctx, "test-lock", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acquired {
		t.Error("expected lock to still be held by lock1")
	}
}

func TestLock_Extend_Success(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	lock := NewLock(client)
	ctx := context.Background()

	// Acquire with short TTL
	acquired, err := lock.Acquire(ctx, "test-lock", 1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Error("expected to acquire lock")
	}

	// Extend TTL
	err = lock.Extend(ctx, "test-lock", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error on extend: %v", err)
	}
}

func TestLock_Extend_NotHeld(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	lock := NewLock(client)
	ctx := context.Background()

	// Extend without holding lock should fail
	err := lock.Extend(ctx, "test-lock", 10*time.Second)
	if err == nil {
		t.Error("expected error when extending unheld lock")
	}
}

func TestLock_Extend_ByDifferentOwner(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	lock1 := NewLock(client)
	lock2 := NewLock(client)
	ctx := context.Background()

	// Lock1 acquires
	acquired, err := lock1.Acquire(ctx, "test-lock", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Error("expected to acquire lock")
	}

	// Lock2 tries to extend - should fail
	err = lock2.Extend(ctx, "test-lock", 20*time.Second)
	if err == nil {
		t.Error("expected error when different owner tries to extend")
	}
}

func TestLock_Ping(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	lock := NewLock(client)
	ctx := context.Background()

	err := lock.Ping(ctx)
	if err != nil {
		t.Errorf("unexpected ping error: %v", err)
	}
}

func TestLock_DifferentLockNames(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	lock := NewLock(client)
	ctx := context.Background()

	// Acquire lock-a
	acquired, err := lock.Acquire(ctx, "lock-a", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Error("expected to acquire lock-a")
	}

	// Should be able to acquire lock-b
	acquired, err = lock.Acquire(ctx, "lock-b", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Error("expected to acquire lock-b")
	}
}
