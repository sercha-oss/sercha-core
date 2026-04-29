package services

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
)

// Ensure userService implements UserService
var _ driving.UserService = (*userService)(nil)

// userService implements the UserService interface
type userService struct {
	userStore          driven.UserStore
	sessionStore       driven.SessionStore
	authAdapter        driven.AuthAdapter
	teamID             string // Team context for this service instance
	logger             *slog.Logger
	userCreateObserver driven.UserCreateObserver // Optional; nil means no observer.
	userDeleteObserver driven.UserDeleteObserver // Optional; nil means no observer.
}

// UserServiceConfig holds dependencies for UserService.
type UserServiceConfig struct {
	UserStore          driven.UserStore
	SessionStore       driven.SessionStore
	AuthAdapter        driven.AuthAdapter
	TeamID             string
	Logger             *slog.Logger
	UserCreateObserver driven.UserCreateObserver // Optional; nil means no observer.
	UserDeleteObserver driven.UserDeleteObserver // Optional; nil means no observer.
}

// NewUserService creates a new UserService.
func NewUserService(cfg UserServiceConfig) driving.UserService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &userService{
		userStore:          cfg.UserStore,
		sessionStore:       cfg.SessionStore,
		authAdapter:        cfg.AuthAdapter,
		teamID:             cfg.TeamID,
		logger:             logger,
		userCreateObserver: cfg.UserCreateObserver,
		userDeleteObserver: cfg.UserDeleteObserver,
	}
}

// Setup creates the initial admin user (only works if no users exist)
func (s *userService) Setup(ctx context.Context, req driving.SetupRequest) (*driving.SetupResponse, error) {
	// Validate input
	if req.Email == "" || req.Password == "" || req.Name == "" {
		return nil, domain.ErrInvalidInput
	}

	// Check if any users exist
	users, err := s.userStore.List(ctx, s.teamID)
	if err != nil {
		return nil, err
	}
	if len(users) > 0 {
		return nil, domain.ErrForbidden
	}

	// Create the admin user
	user, err := s.Create(ctx, driving.CreateUserRequest{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
		Role:     domain.RoleAdmin,
	})
	if err != nil {
		return nil, err
	}

	return &driving.SetupResponse{
		User:    user,
		Message: "Setup complete. You can now log in.",
	}, nil
}

// Create creates a new user (admin only)
func (s *userService) Create(ctx context.Context, req driving.CreateUserRequest) (*domain.User, error) {
	// Validate input
	if err := s.validateCreateRequest(req); err != nil {
		return nil, err
	}

	// Check if email already exists
	existing, _ := s.userStore.GetByEmail(ctx, req.Email)
	if existing != nil {
		return nil, domain.ErrAlreadyExists
	}

	// Hash password
	passwordHash, err := s.authAdapter.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user := &domain.User{
		ID:           generateID(),
		Email:        strings.ToLower(strings.TrimSpace(req.Email)),
		PasswordHash: passwordHash,
		Name:         strings.TrimSpace(req.Name),
		Role:         req.Role,
		TeamID:       s.teamID,
		Active:       true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.userStore.Save(ctx, user); err != nil {
		return nil, err
	}

	// Fire observer after the underlying save succeeds. Failures are
	// logged and ignored — observer health must not affect create
	// correctness, mirroring the ingest/delete observer posture.
	if s.userCreateObserver != nil {
		if err := s.userCreateObserver.OnUserCreated(ctx, user); err != nil {
			s.logger.Warn("user create observer failed",
				"user_id", user.ID,
				"error", err,
			)
		}
	}

	return user, nil
}

// Get retrieves a user by ID
func (s *userService) Get(ctx context.Context, id string) (*domain.User, error) {
	user, err := s.userStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Ensure user belongs to this team
	if user.TeamID != s.teamID {
		return nil, domain.ErrNotFound
	}

	return user, nil
}

// GetByEmail retrieves a user by email
func (s *userService) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	user, err := s.userStore.GetByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return nil, err
	}

	// Ensure user belongs to this team
	if user.TeamID != s.teamID {
		return nil, domain.ErrNotFound
	}

	return user, nil
}

// List retrieves all users in the team
func (s *userService) List(ctx context.Context) ([]*domain.User, error) {
	return s.userStore.List(ctx, s.teamID)
}

// Update updates a user (admin only)
func (s *userService) Update(ctx context.Context, id string, req driving.UpdateUserRequest) (*domain.User, error) {
	user, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if req.Name != nil {
		user.Name = strings.TrimSpace(*req.Name)
	}
	if req.Role != nil {
		user.Role = *req.Role
	}
	if req.Active != nil {
		user.Active = *req.Active
	}
	user.UpdatedAt = time.Now()

	if err := s.userStore.Save(ctx, user); err != nil {
		return nil, err
	}

	// If user was deactivated, invalidate their sessions
	if req.Active != nil && !*req.Active {
		_ = s.sessionStore.DeleteByUser(ctx, id)
	}

	return user, nil
}

// Delete deletes a user (admin only)
func (s *userService) Delete(ctx context.Context, id string) error {
	// Capture by value so observers can read user metadata after
	// userStore.Delete below.
	user, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	// Invalidate all sessions first
	_ = s.sessionStore.DeleteByUser(ctx, user.ID)

	if err := s.userStore.Delete(ctx, id); err != nil {
		return err
	}

	// Fire observer after the underlying delete succeeds. Failures are
	// logged and ignored — observer health must not affect deletion
	// correctness, mirroring the ingest/delete observer posture.
	if s.userDeleteObserver != nil {
		if err := s.userDeleteObserver.OnUserDeleted(ctx, user); err != nil {
			s.logger.Warn("user delete observer failed",
				"user_id", user.ID,
				"error", err,
			)
		}
	}

	return nil
}

// SetPassword sets a new password for a user (admin only)
func (s *userService) SetPassword(ctx context.Context, id string, password string) error {
	if password == "" {
		return domain.ErrInvalidInput
	}

	user, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	passwordHash, err := s.authAdapter.HashPassword(password)
	if err != nil {
		return err
	}

	user.PasswordHash = passwordHash
	user.UpdatedAt = time.Now()

	if err := s.userStore.Save(ctx, user); err != nil {
		return err
	}

	// Invalidate all sessions (force re-login)
	return s.sessionStore.DeleteByUser(ctx, id)
}

// validateCreateRequest validates the create user request
func (s *userService) validateCreateRequest(req driving.CreateUserRequest) error {
	if req.Email == "" {
		return domain.ErrInvalidInput
	}
	if req.Password == "" {
		return domain.ErrInvalidInput
	}
	if req.Name == "" {
		return domain.ErrInvalidInput
	}
	if req.Role != domain.RoleAdmin && req.Role != domain.RoleMember && req.Role != domain.RoleViewer {
		return domain.ErrInvalidInput
	}
	return nil
}
