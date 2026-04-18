package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Verify interface compliance
var _ driven.DistributedLock = (*Lock)(nil)

const lockPrefix = "sercha:lock:"

// Lock implements DistributedLock using Redis SETNX with TTL.
// It uses a unique owner ID to prevent accidental release by other instances.
type Lock struct {
	client  *redis.Client
	ownerID string
}

// NewLock creates a new Redis-backed distributed lock.
// The owner ID is automatically generated to uniquely identify this instance.
func NewLock(client *redis.Client) *Lock {
	return &Lock{
		client:  client,
		ownerID: generateOwnerID(),
	}
}

// generateOwnerID creates a unique identifier for this lock holder.
// Format: hostname:pid:random
func generateOwnerID() string {
	hostname, _ := os.Hostname()
	randomBytes := make([]byte, 8)
	_, _ = rand.Read(randomBytes)
	return fmt.Sprintf("%s:%d:%s", hostname, os.Getpid(), hex.EncodeToString(randomBytes))
}

// Acquire attempts to acquire a named lock with the given TTL.
// Uses Redis SETNX (SET if Not eXists) for atomic lock acquisition.
// Returns true if acquired, false if already held by another instance.
func (l *Lock) Acquire(ctx context.Context, name string, ttl time.Duration) (bool, error) {
	key := lockPrefix + name
	result, err := l.client.SetNX(ctx, key, l.ownerID, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("acquire lock %s: %w", name, err)
	}
	return result, nil
}

// releaseScript is a Lua script for safe lock release.
// It only deletes the lock if the current owner matches, preventing
// accidental release of locks held by other instances.
var releaseScript = redis.NewScript(`
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("del", KEYS[1])
	else
		return 0
	end
`)

// Release releases a named lock if held by this instance.
// Uses a Lua script to atomically check ownership and delete.
// Safe to call even if the lock is not held or has expired.
func (l *Lock) Release(ctx context.Context, name string) error {
	key := lockPrefix + name
	_, err := releaseScript.Run(ctx, l.client, []string{key}, l.ownerID).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("release lock %s: %w", name, err)
	}
	return nil
}

// extendScript is a Lua script for safe lock TTL extension.
// It only extends the TTL if the current owner matches.
var extendScript = redis.NewScript(`
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("pexpire", KEYS[1], ARGV[2])
	else
		return 0
	end
`)

// Extend extends the TTL of a currently held lock.
// Returns error if the lock is not held by this instance.
func (l *Lock) Extend(ctx context.Context, name string, ttl time.Duration) error {
	key := lockPrefix + name
	result, err := extendScript.Run(ctx, l.client, []string{key}, l.ownerID, ttl.Milliseconds()).Result()
	if err != nil {
		return fmt.Errorf("extend lock %s: %w", name, err)
	}
	if result.(int64) == 0 {
		return fmt.Errorf("lock %s not held by this instance", name)
	}
	return nil
}

// Ping checks if the Redis backend is healthy.
func (l *Lock) Ping(ctx context.Context) error {
	return l.client.Ping(ctx).Err()
}

// OwnerID returns the unique identifier for this lock instance.
// Useful for debugging and logging.
func (l *Lock) OwnerID() string {
	return l.ownerID
}
