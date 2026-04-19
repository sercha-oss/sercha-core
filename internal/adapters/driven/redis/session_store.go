package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Verify interface compliance
var _ driven.SessionStore = (*SessionStore)(nil)

const (
	// Key prefixes for Redis
	sessionPrefix        = "session:"
	sessionTokenPrefix   = "session:token:"
	sessionRefreshPrefix = "session:refresh:"
	sessionUserPrefix    = "session:user:"
)

// SessionStore implements driven.SessionStore using Redis
// Sessions use Redis TTL for automatic expiration
type SessionStore struct {
	client *redis.Client
}

// NewSessionStore creates a new Redis-backed SessionStore
func NewSessionStore(client *redis.Client) *SessionStore {
	return &SessionStore{client: client}
}

// Save stores a session with TTL based on ExpiresAt
func (s *SessionStore) Save(ctx context.Context, session *domain.Session) error {
	// Calculate TTL from ExpiresAt
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		// Session already expired, don't save
		return nil
	}

	// Serialize session
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Use pipeline for atomic operations
	pipe := s.client.Pipeline()

	// Store session by ID
	pipe.Set(ctx, sessionPrefix+session.ID, data, ttl)

	// Index by token
	pipe.Set(ctx, sessionTokenPrefix+session.Token, session.ID, ttl)

	// Index by refresh token
	pipe.Set(ctx, sessionRefreshPrefix+session.RefreshToken, session.ID, ttl)

	// Add to user's session set
	pipe.SAdd(ctx, sessionUserPrefix+session.UserID, session.ID)
	pipe.Expire(ctx, sessionUserPrefix+session.UserID, 30*24*time.Hour) // Keep set for 30 days

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}

// Get retrieves a session by ID
func (s *SessionStore) Get(ctx context.Context, id string) (*domain.Session, error) {
	data, err := s.client.Get(ctx, sessionPrefix+id).Bytes()
	if err == redis.Nil {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session domain.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// GetByToken retrieves a session by token value
func (s *SessionStore) GetByToken(ctx context.Context, token string) (*domain.Session, error) {
	// Get session ID from token index
	sessionID, err := s.client.Get(ctx, sessionTokenPrefix+token).Result()
	if err == redis.Nil {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session by token: %w", err)
	}

	return s.Get(ctx, sessionID)
}

// GetByRefreshToken retrieves a session by refresh token value
func (s *SessionStore) GetByRefreshToken(ctx context.Context, refreshToken string) (*domain.Session, error) {
	// Get session ID from refresh token index
	sessionID, err := s.client.Get(ctx, sessionRefreshPrefix+refreshToken).Result()
	if err == redis.Nil {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session by refresh token: %w", err)
	}

	return s.Get(ctx, sessionID)
}

// Delete deletes a session
func (s *SessionStore) Delete(ctx context.Context, id string) error {
	// Get session first to clean up indexes
	session, err := s.Get(ctx, id)
	if err == domain.ErrNotFound {
		return nil // Already deleted
	}
	if err != nil {
		return err
	}

	return s.deleteSession(ctx, session)
}

// DeleteByToken deletes a session by token
func (s *SessionStore) DeleteByToken(ctx context.Context, token string) error {
	session, err := s.GetByToken(ctx, token)
	if err == domain.ErrNotFound {
		return nil // Already deleted
	}
	if err != nil {
		return err
	}

	return s.deleteSession(ctx, session)
}

// DeleteByUser deletes all sessions for a user (logout everywhere)
func (s *SessionStore) DeleteByUser(ctx context.Context, userID string) error {
	// Get all session IDs for user
	sessionIDs, err := s.client.SMembers(ctx, sessionUserPrefix+userID).Result()
	if err != nil {
		return fmt.Errorf("failed to get user sessions: %w", err)
	}

	// Delete each session
	for _, sessionID := range sessionIDs {
		if err := s.Delete(ctx, sessionID); err != nil {
			// Log but continue - some sessions may have already expired
			continue
		}
	}

	// Delete the user's session set
	s.client.Del(ctx, sessionUserPrefix+userID)

	return nil
}

// ListByUser lists all active sessions for a user
func (s *SessionStore) ListByUser(ctx context.Context, userID string) ([]*domain.Session, error) {
	// Get all session IDs for user
	sessionIDs, err := s.client.SMembers(ctx, sessionUserPrefix+userID).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}

	var sessions []*domain.Session
	var expiredIDs []string

	for _, sessionID := range sessionIDs {
		session, err := s.Get(ctx, sessionID)
		if err == domain.ErrNotFound {
			// Session expired, track for cleanup
			expiredIDs = append(expiredIDs, sessionID)
			continue
		}
		if err != nil {
			return nil, err
		}

		// Double-check expiration
		if !session.IsExpired() {
			sessions = append(sessions, session)
		} else {
			expiredIDs = append(expiredIDs, sessionID)
		}
	}

	// Clean up expired session IDs from user's set
	if len(expiredIDs) > 0 {
		s.client.SRem(ctx, sessionUserPrefix+userID, expiredIDs)
	}

	return sessions, nil
}

// deleteSession removes a session and all its indexes
func (s *SessionStore) deleteSession(ctx context.Context, session *domain.Session) error {
	pipe := s.client.Pipeline()

	// Delete session data
	pipe.Del(ctx, sessionPrefix+session.ID)

	// Delete token index
	pipe.Del(ctx, sessionTokenPrefix+session.Token)

	// Delete refresh token index
	pipe.Del(ctx, sessionRefreshPrefix+session.RefreshToken)

	// Remove from user's session set
	pipe.SRem(ctx, sessionUserPrefix+session.UserID, session.ID)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}
