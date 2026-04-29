package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven/mocks"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
)

func newTestUserService() (*mocks.MockUserStore, *mocks.MockSessionStore, *userService) {
	userStore := mocks.NewMockUserStore()
	sessionStore := mocks.NewMockSessionStore()
	authAdapter := mocks.NewMockAuthAdapter()
	svc := NewUserService(UserServiceConfig{
		UserStore:    userStore,
		SessionStore: sessionStore,
		AuthAdapter:  authAdapter,
		TeamID:       "team-123",
	}).(*userService)
	return userStore, sessionStore, svc
}

// spyUserCreateObserver records OnUserCreated calls and can return a
// configured error to verify observer failures don't break Create.
type spyUserCreateObserver struct {
	mu        sync.Mutex
	calls     []*domain.User
	returnErr error
}

func (s *spyUserCreateObserver) OnUserCreated(_ context.Context, user *domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, user)
	return s.returnErr
}

func (s *spyUserCreateObserver) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *spyUserCreateObserver) lastCall() *domain.User {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return nil
	}
	return s.calls[len(s.calls)-1]
}

// spyUserDeleteObserver records OnUserDeleted calls and can return a
// configured error to verify observer failures don't break Delete.
type spyUserDeleteObserver struct {
	mu        sync.Mutex
	calls     []*domain.User
	returnErr error
}

func (s *spyUserDeleteObserver) OnUserDeleted(_ context.Context, user *domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, user)
	return s.returnErr
}

func (s *spyUserDeleteObserver) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *spyUserDeleteObserver) lastCall() *domain.User {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return nil
	}
	return s.calls[len(s.calls)-1]
}

// newTestUserServiceWithObservers builds a userService with the given
// optional observers. Either observer may be nil.
func newTestUserServiceWithObservers(
	createObs driven.UserCreateObserver,
	deleteObs driven.UserDeleteObserver,
) (*mocks.MockUserStore, *mocks.MockSessionStore, *userService) {
	userStore := mocks.NewMockUserStore()
	sessionStore := mocks.NewMockSessionStore()
	authAdapter := mocks.NewMockAuthAdapter()
	svc := NewUserService(UserServiceConfig{
		UserStore:          userStore,
		SessionStore:       sessionStore,
		AuthAdapter:        authAdapter,
		TeamID:             "team-123",
		UserCreateObserver: createObs,
		UserDeleteObserver: deleteObs,
	}).(*userService)
	return userStore, sessionStore, svc
}

func TestUserService_Create(t *testing.T) {
	_, _, svc := newTestUserService()

	tests := []struct {
		name    string
		req     driving.CreateUserRequest
		wantErr error
	}{
		{
			name: "valid user",
			req: driving.CreateUserRequest{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "Test User",
				Role:     domain.RoleMember,
			},
			wantErr: nil,
		},
		{
			name: "missing email",
			req: driving.CreateUserRequest{
				Email:    "",
				Password: "password123",
				Name:     "Test User",
				Role:     domain.RoleMember,
			},
			wantErr: domain.ErrInvalidInput,
		},
		{
			name: "missing password",
			req: driving.CreateUserRequest{
				Email:    "test2@example.com",
				Password: "",
				Name:     "Test User",
				Role:     domain.RoleMember,
			},
			wantErr: domain.ErrInvalidInput,
		},
		{
			name: "missing name",
			req: driving.CreateUserRequest{
				Email:    "test3@example.com",
				Password: "password123",
				Name:     "",
				Role:     domain.RoleMember,
			},
			wantErr: domain.ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := svc.Create(context.Background(), tt.req)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if user == nil {
				t.Fatal("expected user to be returned")
			}
			if user.Email != tt.req.Email {
				t.Errorf("expected email %s, got %s", tt.req.Email, user.Email)
			}
			if user.Name != tt.req.Name {
				t.Errorf("expected name %s, got %s", tt.req.Name, user.Name)
			}
			if user.Role != tt.req.Role {
				t.Errorf("expected role %s, got %s", tt.req.Role, user.Role)
			}
			if user.TeamID != "team-123" {
				t.Errorf("expected team ID team-123, got %s", user.TeamID)
			}
			if !user.Active {
				t.Error("expected user to be active")
			}
		})
	}
}

func TestUserService_Create_DuplicateEmail(t *testing.T) {
	_, _, svc := newTestUserService()

	req := driving.CreateUserRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "Test User",
		Role:     domain.RoleMember,
	}

	// Create first user
	_, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Try to create duplicate
	_, err = svc.Create(context.Background(), req)
	if err != domain.ErrAlreadyExists {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestUserService_Get(t *testing.T) {
	userStore, _, svc := newTestUserService()

	// Create a user
	user := &domain.User{
		ID:     "user-123",
		Email:  "test@example.com",
		Name:   "Test User",
		Role:   domain.RoleMember,
		TeamID: "team-123",
		Active: true,
	}
	_ = userStore.Save(context.Background(), user)

	// Get the user
	result, err := svc.Get(context.Background(), "user-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != user.ID {
		t.Errorf("expected user ID %s, got %s", user.ID, result.ID)
	}

	// Get non-existent user
	_, err = svc.Get(context.Background(), "non-existent")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUserService_Get_WrongTeam(t *testing.T) {
	userStore, _, svc := newTestUserService()

	// Create a user in a different team
	user := &domain.User{
		ID:     "user-456",
		Email:  "other@example.com",
		Name:   "Other User",
		Role:   domain.RoleMember,
		TeamID: "other-team",
		Active: true,
	}
	_ = userStore.Save(context.Background(), user)

	// Try to get user from different team
	_, err := svc.Get(context.Background(), "user-456")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound for user in different team, got %v", err)
	}
}

func TestUserService_GetByEmail(t *testing.T) {
	userStore, _, svc := newTestUserService()

	// Create a user
	user := &domain.User{
		ID:     "user-123",
		Email:  "test@example.com",
		Name:   "Test User",
		Role:   domain.RoleMember,
		TeamID: "team-123",
		Active: true,
	}
	_ = userStore.Save(context.Background(), user)

	// Get by email
	result, err := svc.GetByEmail(context.Background(), "test@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Email != user.Email {
		t.Errorf("expected email %s, got %s", user.Email, result.Email)
	}

	// Get non-existent email
	_, err = svc.GetByEmail(context.Background(), "nonexistent@example.com")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUserService_List(t *testing.T) {
	userStore, _, svc := newTestUserService()

	// Create users
	for i := 0; i < 3; i++ {
		user := &domain.User{
			ID:     generateID(),
			Email:  "user" + string(rune('0'+i)) + "@example.com",
			Name:   "User " + string(rune('0'+i)),
			Role:   domain.RoleMember,
			TeamID: "team-123",
			Active: true,
		}
		_ = userStore.Save(context.Background(), user)
	}

	// List users
	users, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 3 {
		t.Errorf("expected 3 users, got %d", len(users))
	}
}

func TestUserService_Update(t *testing.T) {
	userStore, _, svc := newTestUserService()

	// Create a user
	user := &domain.User{
		ID:        "user-123",
		Email:     "test@example.com",
		Name:      "Test User",
		Role:      domain.RoleMember,
		TeamID:    "team-123",
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = userStore.Save(context.Background(), user)

	// Update name
	newName := "Updated Name"
	updated, err := svc.Update(context.Background(), "user-123", driving.UpdateUserRequest{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != newName {
		t.Errorf("expected name %s, got %s", newName, updated.Name)
	}

	// Update role
	newRole := domain.RoleAdmin
	updated, err = svc.Update(context.Background(), "user-123", driving.UpdateUserRequest{
		Role: &newRole,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Role != newRole {
		t.Errorf("expected role %s, got %s", newRole, updated.Role)
	}
}

func TestUserService_Update_DeactivateUser(t *testing.T) {
	userStore, sessionStore, svc := newTestUserService()

	// Create a user with sessions
	user := &domain.User{
		ID:     "user-123",
		Email:  "test@example.com",
		Name:   "Test User",
		Role:   domain.RoleMember,
		TeamID: "team-123",
		Active: true,
	}
	_ = userStore.Save(context.Background(), user)

	session := &domain.Session{
		ID:     "session-123",
		UserID: "user-123",
		Token:  "token-123",
	}
	_ = sessionStore.Save(context.Background(), session)

	// Deactivate user
	active := false
	_, err := svc.Update(context.Background(), "user-123", driving.UpdateUserRequest{
		Active: &active,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check sessions were deleted
	sessions, _ := sessionStore.ListByUser(context.Background(), "user-123")
	if len(sessions) != 0 {
		t.Error("expected sessions to be deleted when user is deactivated")
	}
}

func TestUserService_Delete(t *testing.T) {
	userStore, sessionStore, svc := newTestUserService()

	// Create a user with sessions
	user := &domain.User{
		ID:     "user-123",
		Email:  "test@example.com",
		Name:   "Test User",
		Role:   domain.RoleMember,
		TeamID: "team-123",
		Active: true,
	}
	_ = userStore.Save(context.Background(), user)

	session := &domain.Session{
		ID:     "session-123",
		UserID: "user-123",
		Token:  "token-123",
	}
	_ = sessionStore.Save(context.Background(), session)

	// Delete user
	err := svc.Delete(context.Background(), "user-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify user is deleted
	_, err = svc.Get(context.Background(), "user-123")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound after deletion, got %v", err)
	}

	// Verify sessions were deleted
	if sessionStore.Count() != 0 {
		t.Error("expected sessions to be deleted")
	}
}

func TestUserService_SetPassword(t *testing.T) {
	userStore, sessionStore, svc := newTestUserService()

	// Create a user
	user := &domain.User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: "old-hash",
		Name:         "Test User",
		Role:         domain.RoleMember,
		TeamID:       "team-123",
		Active:       true,
	}
	_ = userStore.Save(context.Background(), user)

	// Add a session
	session := &domain.Session{
		ID:     "session-123",
		UserID: "user-123",
		Token:  "token-123",
	}
	_ = sessionStore.Save(context.Background(), session)

	// Set new password
	err := svc.SetPassword(context.Background(), "user-123", "new-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify sessions were deleted (force re-login)
	if sessionStore.Count() != 0 {
		t.Error("expected sessions to be deleted after password change")
	}

	// Test empty password
	err = svc.SetPassword(context.Background(), "user-123", "")
	if err != domain.ErrInvalidInput {
		t.Errorf("expected ErrInvalidInput for empty password, got %v", err)
	}
}

func TestUserService_Create_FiresObserver(t *testing.T) {
	createObs := &spyUserCreateObserver{}
	_, _, svc := newTestUserServiceWithObservers(createObs, nil)

	user, err := svc.Create(context.Background(), driving.CreateUserRequest{
		Email:    "obs@example.com",
		Password: "password123",
		Name:     "Obs User",
		Role:     domain.RoleMember,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := createObs.callCount(), 1; got != want {
		t.Fatalf("expected %d observer calls, got %d", want, got)
	}
	last := createObs.lastCall()
	if last == nil || last.ID != user.ID {
		t.Errorf("expected observer to receive created user (id=%s), got %+v", user.ID, last)
	}
}

func TestUserService_Create_NilObserver(t *testing.T) {
	_, _, svc := newTestUserServiceWithObservers(nil, nil)

	user, err := svc.Create(context.Background(), driving.CreateUserRequest{
		Email:    "noobs@example.com",
		Password: "password123",
		Name:     "Noobs User",
		Role:     domain.RoleMember,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user to be returned")
	}
}

func TestUserService_Create_ObserverErrorIgnored(t *testing.T) {
	createObs := &spyUserCreateObserver{returnErr: errors.New("observer boom")}
	_, _, svc := newTestUserServiceWithObservers(createObs, nil)

	user, err := svc.Create(context.Background(), driving.CreateUserRequest{
		Email:    "boom@example.com",
		Password: "password123",
		Name:     "Boom User",
		Role:     domain.RoleMember,
	})
	if err != nil {
		t.Fatalf("expected create to succeed despite observer error, got %v", err)
	}
	if user == nil {
		t.Fatal("expected user to be returned")
	}
	if got := createObs.callCount(); got != 1 {
		t.Errorf("expected observer to fire once, got %d", got)
	}
}

func TestUserService_Delete_FiresObserver(t *testing.T) {
	deleteObs := &spyUserDeleteObserver{}
	userStore, _, svc := newTestUserServiceWithObservers(nil, deleteObs)

	user := &domain.User{
		ID:     "user-del-1",
		Email:  "del@example.com",
		Name:   "Del User",
		Role:   domain.RoleMember,
		TeamID: "team-123",
		Active: true,
	}
	_ = userStore.Save(context.Background(), user)

	if err := svc.Delete(context.Background(), user.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := deleteObs.callCount(), 1; got != want {
		t.Fatalf("expected %d observer calls, got %d", want, got)
	}
	last := deleteObs.lastCall()
	if last == nil || last.ID != user.ID {
		t.Errorf("expected observer to receive deleted user (id=%s), got %+v", user.ID, last)
	}
}

func TestUserService_Delete_NilObserver(t *testing.T) {
	userStore, _, svc := newTestUserServiceWithObservers(nil, nil)

	user := &domain.User{
		ID:     "user-del-2",
		Email:  "del2@example.com",
		Name:   "Del User",
		Role:   domain.RoleMember,
		TeamID: "team-123",
		Active: true,
	}
	_ = userStore.Save(context.Background(), user)

	if err := svc.Delete(context.Background(), user.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.Get(context.Background(), user.ID); err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound after deletion, got %v", err)
	}
}

func TestUserService_Delete_ObserverErrorIgnored(t *testing.T) {
	deleteObs := &spyUserDeleteObserver{returnErr: errors.New("observer boom")}
	userStore, _, svc := newTestUserServiceWithObservers(nil, deleteObs)

	user := &domain.User{
		ID:     "user-del-3",
		Email:  "del3@example.com",
		Name:   "Del User",
		Role:   domain.RoleMember,
		TeamID: "team-123",
		Active: true,
	}
	_ = userStore.Save(context.Background(), user)

	if err := svc.Delete(context.Background(), user.ID); err != nil {
		t.Fatalf("expected delete to succeed despite observer error, got %v", err)
	}
	if got := deleteObs.callCount(); got != 1 {
		t.Errorf("expected observer to fire once, got %d", got)
	}
	if _, err := svc.Get(context.Background(), user.ID); err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound after deletion, got %v", err)
	}
}
