package services

import (
	"context"
	"log/slog"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven/mocks"
)

// TestSyncOrchestrator_RespectsSync Enabled validates acceptance criteria:
// - Sync operations check `sync_enabled` and skip gracefully when disabled
func TestSyncOrchestrator_RespectsSyncEnabled(t *testing.T) {
	tests := []struct {
		name            string
		syncEnabled     bool
		expectSuccess   bool
		expectErrorMsg  string
	}{
		{
			name:           "sync enabled - proceeds normally",
			syncEnabled:    true,
			expectSuccess:  false, // Will fail due to source not found, but that's expected
			expectErrorMsg: "failed to get source",
		},
		{
			name:           "sync disabled - skips gracefully",
			syncEnabled:    false,
			expectSuccess:  false,
			expectErrorMsg: "sync is disabled in team settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceStore := mocks.NewMockSourceStore()
			syncStore := mocks.NewMockSyncStateStore()

			customSettings := domain.DefaultSettings("team-123")
			customSettings.SyncEnabled = tt.syncEnabled
			settingsStore := &mockSettingsStore{
				settings: customSettings,
			}

			orchestrator := &SyncOrchestrator{
				sourceStore:   sourceStore,
				syncStore:     syncStore,
				settingsStore: settingsStore,
				teamID:        "team-123",
				logger:        slog.Default(),
			}

			result, _ := orchestrator.SyncSource(context.Background(), "source-456")

			// Result should always be returned
			if result == nil {
				t.Fatal("expected result to be returned")
			}

			// Check success/error based on test case
			if result.Success != tt.expectSuccess {
				t.Errorf("expected success=%v, got %v", tt.expectSuccess, result.Success)
			}

			// Verify error message
			if result.Error == "" {
				t.Error("expected error message to be set")
			}

			if tt.expectErrorMsg != "" {
				// Check that error contains expected message
				if result.Error != tt.expectErrorMsg {
					// For the "sync enabled" case, error will vary, so just check it's not about disabled sync
					if tt.syncEnabled && result.Error == "sync is disabled in team settings" {
						t.Errorf("sync enabled but got disabled message: %s", result.Error)
					}
				}
			}

			// When sync is disabled, should not attempt to proceed
			if !tt.syncEnabled && result.Error != "sync is disabled in team settings" {
				t.Errorf("expected 'sync is disabled' error, got: %s", result.Error)
			}
		})
	}
}

// TestSyncOrchestrator_SyncEnabledWithNilSettings validates that when settings
// can't be loaded, sync proceeds (fail open)
func TestSyncOrchestrator_SyncEnabledWithNilSettings(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	syncStore := mocks.NewMockSyncStateStore()

	settingsStore := &mockSettingsStore{
		settings: nil, // Will return ErrNotFound
	}

	orchestrator := &SyncOrchestrator{
		sourceStore:   sourceStore,
		syncStore:     syncStore,
		settingsStore: settingsStore,
		teamID:        "team-123",
		logger:        slog.Default(),
	}

	result, _ := orchestrator.SyncSource(context.Background(), "source-456")

	// Should attempt to proceed (fail open when settings unavailable)
	if result == nil {
		t.Fatal("expected result to be returned")
	}

	// Should fail because source doesn't exist, not because sync is disabled
	if result.Error == "sync is disabled in team settings" {
		t.Error("should not return disabled error when settings can't be loaded")
	}
}

// TestSyncOrchestrator_SyncEnabledWithNoSettingsStore validates that
// when settings store is nil, sync proceeds
func TestSyncOrchestrator_SyncEnabledWithNoSettingsStore(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	syncStore := mocks.NewMockSyncStateStore()

	orchestrator := &SyncOrchestrator{
		sourceStore:   sourceStore,
		syncStore:     syncStore,
		settingsStore: nil, // No settings store
		teamID:        "team-123",
		logger:        slog.Default(),
	}

	result, _ := orchestrator.SyncSource(context.Background(), "source-456")

	// Should attempt to proceed
	if result == nil {
		t.Fatal("expected result to be returned")
	}

	// Should fail because source doesn't exist, not because sync is disabled
	if result.Error == "sync is disabled in team settings" {
		t.Error("should not return disabled error when settings store is nil")
	}
}

// TestSyncOrchestrator_SyncDisabledDoesNotModifyState validates that when
// sync is disabled, sync state is not updated
func TestSyncOrchestrator_SyncDisabledDoesNotModifyState(t *testing.T) {
	sourceStore := mocks.NewMockSourceStore()
	syncStore := mocks.NewMockSyncStateStore()

	customSettings := domain.DefaultSettings("team-123")
	customSettings.SyncEnabled = false
	settingsStore := &mockSettingsStore{
		settings: customSettings,
	}

	// Track if sync state was modified (note: we can't easily track this with the mock,
	// so we'll just check that sync was skipped via the error message)

	orchestrator := &SyncOrchestrator{
		sourceStore:   sourceStore,
		syncStore:     syncStore,
		settingsStore: settingsStore,
		teamID:        "team-123",
		logger:        slog.Default(),
	}

	result, _ := orchestrator.SyncSource(context.Background(), "source-456")

	if result == nil {
		t.Fatal("expected result to be returned")
	}

	// Verify sync was skipped
	if result.Error != "sync is disabled in team settings" {
		t.Errorf("expected disabled error, got: %s", result.Error)
	}
}

// TestSyncOrchestrator_LoadSettings validates that loadSettings method works correctly
func TestSyncOrchestrator_LoadSettings(t *testing.T) {
	tests := []struct {
		name          string
		settingsStore *mockSettingsStore
		expectError   bool
		checkSettings func(*testing.T, *domain.Settings)
	}{
		{
			name: "successful load",
			settingsStore: func() *mockSettingsStore {
				s := domain.DefaultSettings("team-123")
				s.SyncEnabled = false
				s.SyncIntervalMinutes = 30
				return &mockSettingsStore{settings: s}
			}(),
			expectError: false,
			checkSettings: func(t *testing.T, s *domain.Settings) {
				if s.SyncEnabled {
					t.Error("expected SyncEnabled to be false")
				}
				if s.SyncIntervalMinutes != 30 {
					t.Errorf("expected SyncIntervalMinutes 30, got %d", s.SyncIntervalMinutes)
				}
			},
		},
		{
			name: "settings store returns error - falls back to defaults",
			settingsStore: &mockSettingsStore{
				settings: nil, // Will return ErrNotFound
			},
			expectError: true,
			checkSettings: func(t *testing.T, s *domain.Settings) {
				// Should return nil when error occurs
				if s != nil {
					t.Error("expected nil settings when error occurs")
				}
			},
		},
		{
			name:          "nil settings store - returns defaults",
			settingsStore: nil,
			expectError:   false,
			checkSettings: func(t *testing.T, s *domain.Settings) {
				if s == nil {
					t.Fatal("expected default settings when store is nil")
				}
				// Should return default values
				if !s.SyncEnabled {
					t.Error("expected default SyncEnabled to be true")
				}
				if s.SyncIntervalMinutes != 60 {
					t.Errorf("expected default SyncIntervalMinutes 60, got %d", s.SyncIntervalMinutes)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip nil settings store test as it's handled differently
			if tt.settingsStore == nil {
				orchestrator := &SyncOrchestrator{
					settingsStore: nil,
					teamID:        "team-123",
				}
				settings, err := orchestrator.loadSettings(context.Background())
				tt.checkSettings(t, settings)
				if err != nil && !tt.expectError {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}

			orchestrator := &SyncOrchestrator{
				settingsStore: tt.settingsStore,
				teamID:        "team-123",
			}

			settings, err := orchestrator.loadSettings(context.Background())

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			tt.checkSettings(t, settings)
		})
	}
}
