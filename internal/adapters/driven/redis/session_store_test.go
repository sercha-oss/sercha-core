package redis

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// setupTestSessionStore creates a test Redis client and SessionStore
func setupTestSessionStore(t *testing.T) (*SessionStore, *miniredis.Miniredis, func()) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	store := NewSessionStore(client)

	return store, mr, func() {
		_ = client.Close()
		mr.Close()
	}
}

// createTestSession creates a test session with default values
func createTestSession(userID string) *domain.Session {
	return &domain.Session{
		ID:           "session-123",
		UserID:       userID,
		Token:        "token-abc",
		RefreshToken: "refresh-xyz",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		CreatedAt:    time.Now(),
		UserAgent:    "Mozilla/5.0",
		IPAddress:    "192.168.1.1",
	}
}

func TestNewSessionStore(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	store := NewSessionStore(client)

	if store == nil {
		t.Fatal("expected non-nil SessionStore")
	}
	if store.client == nil {
		t.Error("expected non-nil Redis client")
	}
}

func TestSessionStore_Save_Success(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()
	session := createTestSession("user-1")

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error saving session: %v", err)
	}

	// Verify session was saved by retrieving it
	retrieved, err := store.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("failed to retrieve saved session: %v", err)
	}

	if retrieved.ID != session.ID {
		t.Errorf("expected ID %s, got %s", session.ID, retrieved.ID)
	}
	if retrieved.UserID != session.UserID {
		t.Errorf("expected UserID %s, got %s", session.UserID, retrieved.UserID)
	}
	if retrieved.Token != session.Token {
		t.Errorf("expected Token %s, got %s", session.Token, retrieved.Token)
	}
}

func TestSessionStore_Save_ExpiredSession(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()
	session := createTestSession("user-1")
	session.ExpiresAt = time.Now().Add(-1 * time.Hour) // Already expired

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Session should not be saved since it's already expired
	_, err = store.Get(ctx, session.ID)
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound for expired session, got %v", err)
	}
}

func TestSessionStore_Save_CreatesIndexes(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()
	session := createTestSession("user-1")

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify token index exists
	tokenKey := sessionTokenPrefix + session.Token
	if !mr.Exists(tokenKey) {
		t.Error("expected token index to exist")
	}

	// Verify refresh token index exists
	refreshKey := sessionRefreshPrefix + session.RefreshToken
	if !mr.Exists(refreshKey) {
		t.Error("expected refresh token index to exist")
	}

	// Verify user session set exists
	userKey := sessionUserPrefix + session.UserID
	if !mr.Exists(userKey) {
		t.Error("expected user session set to exist")
	}

	// Verify session ID is in user's set
	members, err := mr.Members(userKey)
	if err != nil {
		t.Fatalf("failed to get members: %v", err)
	}
	found := false
	for _, member := range members {
		if member == session.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected session ID in user's session set")
	}
}

func TestSessionStore_Save_UpdatesExistingSession(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()
	session := createTestSession("user-1")

	// Save initial session
	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Update session
	session.UserAgent = "Updated User Agent"
	err = store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error updating session: %v", err)
	}

	// Verify updated data
	retrieved, err := store.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("failed to retrieve session: %v", err)
	}

	if retrieved.UserAgent != "Updated User Agent" {
		t.Errorf("expected UserAgent 'Updated User Agent', got %s", retrieved.UserAgent)
	}
}

func TestSessionStore_Get_Success(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()
	session := createTestSession("user-1")

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	retrieved, err := store.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.ID != session.ID {
		t.Errorf("expected ID %s, got %s", session.ID, retrieved.ID)
	}
	if retrieved.Token != session.Token {
		t.Errorf("expected Token %s, got %s", session.Token, retrieved.Token)
	}
}

func TestSessionStore_Get_NotFound(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent-session")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSessionStore_Get_InvalidJSON(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Manually set invalid JSON in Redis
	_ = mr.Set(sessionPrefix+"bad-session", "invalid json data")

	_, err := store.Get(ctx, "bad-session")
	if err == nil {
		t.Error("expected error unmarshaling invalid JSON")
	}
}

func TestSessionStore_GetByToken_Success(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()
	session := createTestSession("user-1")

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	retrieved, err := store.GetByToken(ctx, session.Token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.ID != session.ID {
		t.Errorf("expected ID %s, got %s", session.ID, retrieved.ID)
	}
	if retrieved.Token != session.Token {
		t.Errorf("expected Token %s, got %s", session.Token, retrieved.Token)
	}
}

func TestSessionStore_GetByToken_NotFound(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	_, err := store.GetByToken(ctx, "nonexistent-token")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSessionStore_GetByRefreshToken_Success(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()
	session := createTestSession("user-1")

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	retrieved, err := store.GetByRefreshToken(ctx, session.RefreshToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.ID != session.ID {
		t.Errorf("expected ID %s, got %s", session.ID, retrieved.ID)
	}
	if retrieved.RefreshToken != session.RefreshToken {
		t.Errorf("expected RefreshToken %s, got %s", session.RefreshToken, retrieved.RefreshToken)
	}
}

func TestSessionStore_GetByRefreshToken_NotFound(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	_, err := store.GetByRefreshToken(ctx, "nonexistent-refresh-token")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSessionStore_Delete_Success(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()
	session := createTestSession("user-1")

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = store.Delete(ctx, session.ID)
	if err != nil {
		t.Fatalf("unexpected error deleting session: %v", err)
	}

	// Verify session is deleted
	_, err = store.Get(ctx, session.ID)
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound after deletion, got %v", err)
	}
}

func TestSessionStore_Delete_RemovesIndexes(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()
	session := createTestSession("user-1")

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = store.Delete(ctx, session.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify token index is removed
	tokenKey := sessionTokenPrefix + session.Token
	if mr.Exists(tokenKey) {
		t.Error("expected token index to be removed")
	}

	// Verify refresh token index is removed
	refreshKey := sessionRefreshPrefix + session.RefreshToken
	if mr.Exists(refreshKey) {
		t.Error("expected refresh token index to be removed")
	}

	// Verify session ID removed from user's set
	userKey := sessionUserPrefix + session.UserID
	if mr.Exists(userKey) {
		members, err := mr.Members(userKey)
		if err != nil {
			t.Fatalf("failed to get members: %v", err)
		}
		for _, member := range members {
			if member == session.ID {
				t.Error("expected session ID to be removed from user's set")
			}
		}
	}
}

func TestSessionStore_Delete_NotFound(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Deleting non-existent session should not error
	err := store.Delete(ctx, "nonexistent-session")
	if err != nil {
		t.Errorf("unexpected error deleting non-existent session: %v", err)
	}
}

func TestSessionStore_DeleteByToken_Success(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()
	session := createTestSession("user-1")

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = store.DeleteByToken(ctx, session.Token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify session is deleted
	_, err = store.Get(ctx, session.ID)
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSessionStore_DeleteByToken_NotFound(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Deleting non-existent token should not error
	err := store.DeleteByToken(ctx, "nonexistent-token")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSessionStore_DeleteByUser_Success(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple sessions for the same user
	session1 := createTestSession("user-1")
	session1.ID = "session-1"
	session1.Token = "token-1"
	session1.RefreshToken = "refresh-1"

	session2 := createTestSession("user-1")
	session2.ID = "session-2"
	session2.Token = "token-2"
	session2.RefreshToken = "refresh-2"

	err := store.Save(ctx, session1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = store.Save(ctx, session2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Delete all sessions for user
	err = store.DeleteByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify both sessions are deleted
	_, err = store.Get(ctx, session1.ID)
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound for session1, got %v", err)
	}

	_, err = store.Get(ctx, session2.ID)
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound for session2, got %v", err)
	}
}

func TestSessionStore_DeleteByUser_NoSessions(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Deleting sessions for user with no sessions should not error
	err := store.DeleteByUser(ctx, "user-with-no-sessions")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSessionStore_DeleteByUser_PartiallyExpired(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create two sessions, manually expire one
	session1 := createTestSession("user-1")
	session1.ID = "session-1"
	session1.Token = "token-1"
	session1.RefreshToken = "refresh-1"

	session2 := createTestSession("user-1")
	session2.ID = "session-2"
	session2.Token = "token-2"
	session2.RefreshToken = "refresh-2"

	err := store.Save(ctx, session1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = store.Save(ctx, session2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Manually delete session1 from Redis (simulating expiration)
	mr.Del(sessionPrefix + session1.ID)

	// DeleteByUser should handle missing sessions gracefully
	err = store.DeleteByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Session2 should still be deleted
	_, err = store.Get(ctx, session2.ID)
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound for session2, got %v", err)
	}
}

func TestSessionStore_ListByUser_Success(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple sessions for the same user
	session1 := createTestSession("user-1")
	session1.ID = "session-1"
	session1.Token = "token-1"
	session1.RefreshToken = "refresh-1"

	session2 := createTestSession("user-1")
	session2.ID = "session-2"
	session2.Token = "token-2"
	session2.RefreshToken = "refresh-2"

	err := store.Save(ctx, session1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = store.Save(ctx, session2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// List sessions
	sessions, err := store.ListByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}

	// Verify sessions are correct
	sessionIDs := make(map[string]bool)
	for _, session := range sessions {
		sessionIDs[session.ID] = true
	}

	if !sessionIDs[session1.ID] {
		t.Error("expected session1 in list")
	}
	if !sessionIDs[session2.ID] {
		t.Error("expected session2 in list")
	}
}

func TestSessionStore_ListByUser_Empty(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	sessions, err := store.ListByUser(ctx, "user-with-no-sessions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestSessionStore_ListByUser_FiltersExpired(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create two sessions
	session1 := createTestSession("user-1")
	session1.ID = "session-1"
	session1.Token = "token-1"
	session1.RefreshToken = "refresh-1"

	session2 := createTestSession("user-1")
	session2.ID = "session-2"
	session2.Token = "token-2"
	session2.RefreshToken = "refresh-2"
	session2.ExpiresAt = time.Now().Add(-1 * time.Hour) // Expired

	err := store.Save(ctx, session1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = store.Save(ctx, session2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Manually delete session2 (simulating TTL expiration)
	mr.Del(sessionPrefix + session2.ID)

	// List should return only session1
	sessions, err := store.ListByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}

	if sessions[0].ID != session1.ID {
		t.Errorf("expected session1, got %s", sessions[0].ID)
	}
}

func TestSessionStore_ListByUser_CleansUpExpiredIDs(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create session
	session := createTestSession("user-1")

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Manually delete session (simulating expiration)
	mr.Del(sessionPrefix + session.ID)

	// List should clean up the expired ID from user's set
	sessions, err := store.ListByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}

	// Verify user's set is cleaned (should either not exist or be empty)
	userKey := sessionUserPrefix + session.UserID
	if mr.Exists(userKey) {
		members, err := mr.Members(userKey)
		if err != nil {
			t.Fatalf("failed to get members: %v", err)
		}
		if len(members) != 0 {
			t.Errorf("expected user's session set to be empty, got %d members", len(members))
		}
	}
}

func TestSessionStore_MultipleUsers(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create sessions for different users
	session1 := createTestSession("user-1")
	session1.ID = "session-1"
	session1.Token = "token-1"
	session1.RefreshToken = "refresh-1"

	session2 := createTestSession("user-2")
	session2.ID = "session-2"
	session2.Token = "token-2"
	session2.RefreshToken = "refresh-2"

	err := store.Save(ctx, session1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = store.Save(ctx, session2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// List sessions for user-1
	sessions1, err := store.ListByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions1) != 1 {
		t.Errorf("expected 1 session for user-1, got %d", len(sessions1))
	}

	// List sessions for user-2
	sessions2, err := store.ListByUser(ctx, "user-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions2) != 1 {
		t.Errorf("expected 1 session for user-2, got %d", len(sessions2))
	}

	// Delete user-1 sessions should not affect user-2
	err = store.DeleteByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sessions2After, err := store.ListByUser(ctx, "user-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions2After) != 1 {
		t.Errorf("expected user-2 sessions to remain, got %d", len(sessions2After))
	}
}

func TestSessionStore_TTL_Expiration(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create session with very short TTL
	session := createTestSession("user-1")
	session.ExpiresAt = time.Now().Add(2 * time.Second)

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify session exists
	_, err = store.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fast-forward time in miniredis
	mr.FastForward(3 * time.Second)

	// Session should be expired
	_, err = store.Get(ctx, session.ID)
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound for expired session, got %v", err)
	}
}

func TestSessionStore_ConcurrentAccess(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create session
	session := createTestSession("user-1")

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Simulate concurrent reads
	done := make(chan bool)
	errors := make(chan error, 5)

	for i := 0; i < 5; i++ {
		go func() {
			_, err := store.Get(ctx, session.ID)
			if err != nil {
				errors <- err
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("unexpected error in concurrent access: %v", err)
	}
}

func TestSessionStore_EmptyFields(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create session with empty optional fields
	session := &domain.Session{
		ID:           "session-123",
		UserID:       "user-1",
		Token:        "token-abc",
		RefreshToken: "refresh-xyz",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		CreatedAt:    time.Now(),
		UserAgent:    "",
		IPAddress:    "",
	}

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.UserAgent != "" {
		t.Errorf("expected empty UserAgent, got %s", retrieved.UserAgent)
	}
	if retrieved.IPAddress != "" {
		t.Errorf("expected empty IPAddress, got %s", retrieved.IPAddress)
	}
}

func TestSessionStore_GetByToken_SessionExpiredButIndexExists(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()
	session := createTestSession("user-1")

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Manually delete session but leave token index (simulating race condition)
	mr.Del(sessionPrefix + session.ID)

	// GetByToken should return ErrNotFound when session data is missing
	_, err = store.GetByToken(ctx, session.Token)
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSessionStore_GetByRefreshToken_SessionExpiredButIndexExists(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()
	session := createTestSession("user-1")

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Manually delete session but leave refresh token index
	mr.Del(sessionPrefix + session.ID)

	// GetByRefreshToken should return ErrNotFound when session data is missing
	_, err = store.GetByRefreshToken(ctx, session.RefreshToken)
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSessionStore_Delete_ErrorGettingSession(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Manually set invalid JSON to cause Get to fail
	_ = mr.Set(sessionPrefix+"bad-session", "invalid json")

	// Delete should fail when unable to get session
	err := store.Delete(ctx, "bad-session")
	if err == nil {
		t.Error("expected error when deleting session with invalid JSON")
	}
}

func TestSessionStore_DeleteByToken_ErrorGettingSession(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create valid token index pointing to invalid session
	_ = mr.Set(sessionTokenPrefix+"bad-token", "bad-session-id")
	_ = mr.Set(sessionPrefix+"bad-session-id", "invalid json")

	// DeleteByToken should fail when unable to get session
	err := store.DeleteByToken(ctx, "bad-token")
	if err == nil {
		t.Error("expected error when deleting session with invalid JSON")
	}
}

func TestSessionStore_ContextCancellation(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	session := createTestSession("user-1")

	// Operations should handle cancelled context
	err := store.Save(ctx, session)
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

func TestSessionStore_ListByUser_ErrorGettingSession(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create valid session first
	session1 := createTestSession("user-1")
	session1.ID = "session-1"
	session1.Token = "token-1"
	session1.RefreshToken = "refresh-1"

	err := store.Save(ctx, session1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Add invalid session to user's set
	_ = mr.Set(sessionPrefix+"bad-session", "invalid json")
	_, _ = mr.SAdd(sessionUserPrefix+"user-1", "bad-session")

	// ListByUser should fail when encountering invalid session
	_, err = store.ListByUser(ctx, "user-1")
	if err == nil {
		t.Error("expected error when listing sessions with invalid data")
	}
}

func TestSessionStore_Save_VeryShortTTL(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create session with very short TTL (1 millisecond)
	session := createTestSession("user-1")
	session.ExpiresAt = time.Now().Add(1 * time.Millisecond)

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fast-forward to expire it
	mr.FastForward(2 * time.Millisecond)

	// Session should be expired
	_, err = store.Get(ctx, session.ID)
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound for expired session, got %v", err)
	}
}

func TestSessionStore_MultipleSessionsPerUser_DeleteOne(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple sessions for the same user
	session1 := createTestSession("user-1")
	session1.ID = "session-1"
	session1.Token = "token-1"
	session1.RefreshToken = "refresh-1"

	session2 := createTestSession("user-1")
	session2.ID = "session-2"
	session2.Token = "token-2"
	session2.RefreshToken = "refresh-2"

	err := store.Save(ctx, session1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = store.Save(ctx, session2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Delete only session1
	err = store.Delete(ctx, session1.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// session2 should still exist
	sessions, err := store.ListByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("expected 1 session remaining, got %d", len(sessions))
	}

	if sessions[0].ID != session2.ID {
		t.Errorf("expected session2 to remain, got %s", sessions[0].ID)
	}
}

func TestSessionStore_SaveSameSessionTwice(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()
	session := createTestSession("user-1")

	// Save first time
	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error on first save: %v", err)
	}

	// Save again (update)
	session.UserAgent = "Updated Agent"
	err = store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error on second save: %v", err)
	}

	// Verify updated data
	retrieved, err := store.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.UserAgent != "Updated Agent" {
		t.Errorf("expected updated UserAgent, got %s", retrieved.UserAgent)
	}

	// Verify no duplicate entries in user's session set
	sessions, err := store.ListByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d (possible duplicate)", len(sessions))
	}
}

func TestSessionStore_DifferentTokensSameUser(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple sessions with different tokens for same user
	session1 := createTestSession("user-1")
	session1.ID = "session-1"
	session1.Token = "unique-token-1"
	session1.RefreshToken = "unique-refresh-1"

	session2 := createTestSession("user-1")
	session2.ID = "session-2"
	session2.Token = "unique-token-2"
	session2.RefreshToken = "unique-refresh-2"

	err := store.Save(ctx, session1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = store.Save(ctx, session2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Each token should retrieve its own session
	retrieved1, err := store.GetByToken(ctx, "unique-token-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retrieved1.ID != "session-1" {
		t.Errorf("expected session-1, got %s", retrieved1.ID)
	}

	retrieved2, err := store.GetByToken(ctx, "unique-token-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retrieved2.ID != "session-2" {
		t.Errorf("expected session-2, got %s", retrieved2.ID)
	}
}

func TestSessionStore_IndexesRemovedOnDelete(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()
	session := createTestSession("user-1")

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all indexes exist
	tokenKey := sessionTokenPrefix + session.Token
	refreshKey := sessionRefreshPrefix + session.RefreshToken

	if !mr.Exists(tokenKey) {
		t.Fatal("token index should exist before delete")
	}
	if !mr.Exists(refreshKey) {
		t.Fatal("refresh token index should exist before delete")
	}

	// Delete session
	err = store.Delete(ctx, session.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all indexes are removed
	if mr.Exists(tokenKey) {
		t.Error("token index should be removed after delete")
	}
	if mr.Exists(refreshKey) {
		t.Error("refresh token index should be removed after delete")
	}

	// Verify lookup by token fails
	_, err = store.GetByToken(ctx, session.Token)
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}

	// Verify lookup by refresh token fails
	_, err = store.GetByRefreshToken(ctx, session.RefreshToken)
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSessionStore_GetByToken_RedisError(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	// Close miniredis to simulate Redis connection error
	mr.Close()

	ctx := context.Background()

	// GetByToken should return error when Redis is down
	_, err := store.GetByToken(ctx, "some-token")
	if err == nil {
		t.Error("expected error when Redis is unavailable")
	}
	if err == domain.ErrNotFound {
		t.Error("expected Redis error, not ErrNotFound")
	}
}

func TestSessionStore_GetByRefreshToken_RedisError(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	// Close miniredis to simulate Redis connection error
	mr.Close()

	ctx := context.Background()

	// GetByRefreshToken should return error when Redis is down
	_, err := store.GetByRefreshToken(ctx, "some-refresh-token")
	if err == nil {
		t.Error("expected error when Redis is unavailable")
	}
	if err == domain.ErrNotFound {
		t.Error("expected Redis error, not ErrNotFound")
	}
}

func TestSessionStore_Get_RedisError(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	// Close miniredis to simulate Redis connection error
	mr.Close()

	ctx := context.Background()

	// Get should return error when Redis is down
	_, err := store.Get(ctx, "some-id")
	if err == nil {
		t.Error("expected error when Redis is unavailable")
	}
	if err == domain.ErrNotFound {
		t.Error("expected Redis error, not ErrNotFound")
	}
}

func TestSessionStore_DeleteByUser_ContinuesOnError(t *testing.T) {
	store, mr, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple sessions
	session1 := createTestSession("user-1")
	session1.ID = "session-1"
	session1.Token = "token-1"
	session1.RefreshToken = "refresh-1"

	session2 := createTestSession("user-1")
	session2.ID = "session-2"
	session2.Token = "token-2"
	session2.RefreshToken = "refresh-2"

	err := store.Save(ctx, session1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = store.Save(ctx, session2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Manually corrupt session-1 (invalid JSON) but keep it in user's set
	_ = mr.Set(sessionPrefix+"session-1", "corrupted data")

	// DeleteByUser should continue even when one session fails
	err = store.DeleteByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// session-2 should still be deleted
	_, err = store.Get(ctx, session2.ID)
	if err != domain.ErrNotFound {
		t.Errorf("expected session-2 to be deleted, got: %v", err)
	}
}

func TestSessionStore_ListByUser_FiltersExpiredByTime(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create a valid session
	session1 := createTestSession("user-1")
	session1.ID = "session-1"
	session1.Token = "token-1"
	session1.RefreshToken = "refresh-1"

	// Create an expired session (past ExpiresAt time)
	session2 := createTestSession("user-1")
	session2.ID = "session-2"
	session2.Token = "token-2"
	session2.RefreshToken = "refresh-2"
	session2.ExpiresAt = time.Now().Add(-1 * time.Hour) // Expired

	err := store.Save(ctx, session1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Manually save expired session (bypass TTL check in Save)
	data, _ := json.Marshal(session2)
	client := store.client
	client.Set(ctx, sessionPrefix+session2.ID, data, 10*time.Second)
	client.SAdd(ctx, sessionUserPrefix+session2.UserID, session2.ID)

	// ListByUser should filter out session2 based on IsExpired()
	sessions, err := store.ListByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("expected 1 active session, got %d", len(sessions))
	}

	if len(sessions) > 0 && sessions[0].ID != session1.ID {
		t.Errorf("expected session1, got %s", sessions[0].ID)
	}
}

func TestSessionStore_TimeFields_Preserved(t *testing.T) {
	store, _, cleanup := setupTestSessionStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create session with specific times (in the future)
	now := time.Now()
	createdAt := now.Add(-1 * time.Hour)
	expiresAt := now.Add(24 * time.Hour)

	session := &domain.Session{
		ID:           "session-123",
		UserID:       "user-1",
		Token:        "token-abc",
		RefreshToken: "refresh-xyz",
		ExpiresAt:    expiresAt,
		CreatedAt:    createdAt,
		UserAgent:    "Test Agent",
		IPAddress:    "192.168.1.1",
	}

	err := store.Save(ctx, session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Retrieve and verify time fields are preserved
	retrieved, err := store.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Compare times (truncate to second precision due to JSON serialization)
	createdAtTrunc := createdAt.Truncate(time.Second)
	expiresAtTrunc := expiresAt.Truncate(time.Second)
	retrievedCreatedTrunc := retrieved.CreatedAt.Truncate(time.Second)
	retrievedExpiresTrunc := retrieved.ExpiresAt.Truncate(time.Second)

	if !retrievedCreatedTrunc.Equal(createdAtTrunc) {
		t.Errorf("CreatedAt not preserved: expected %v, got %v", createdAtTrunc, retrievedCreatedTrunc)
	}
	if !retrievedExpiresTrunc.Equal(expiresAtTrunc) {
		t.Errorf("ExpiresAt not preserved: expected %v, got %v", expiresAtTrunc, retrievedExpiresTrunc)
	}
}
