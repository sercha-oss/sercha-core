package services

import (
	"context"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// TestSyncOrchestrator_MatchPattern validates pattern matching logic
// for sync exclusions
func TestSyncOrchestrator_MatchPattern(t *testing.T) {
	// Create a minimal orchestrator just for testing pattern matching
	orchestrator := &SyncOrchestrator{}

	tests := []struct {
		name     string
		path     string
		pattern  string
		expected bool
	}{
		// Folder patterns (ending with /)
		{
			name:     "folder pattern - exact match at root",
			path:     ".git/config",
			pattern:  ".git/",
			expected: true,
		},
		{
			name:     "folder pattern - match in subdirectory",
			path:     "src/.git/config",
			pattern:  ".git/",
			expected: true,
		},
		{
			name:     "folder pattern - node_modules in subdirectory",
			path:     "packages/app/node_modules/react/index.js",
			pattern:  "node_modules/",
			expected: true,
		},
		{
			name:     "folder pattern - no match",
			path:     "src/gitconfig.js",
			pattern:  ".git/",
			expected: false,
		},
		{
			name:     "folder pattern - vendor at root",
			path:     "vendor/package/file.go",
			pattern:  "vendor/",
			expected: true,
		},
		{
			name:     "folder pattern - build directory",
			path:     "build/output.js",
			pattern:  "build/",
			expected: true,
		},

		// Exact filename matches
		{
			name:     "exact match - DS_Store at root",
			path:     ".DS_Store",
			pattern:  ".DS_Store",
			expected: true,
		},
		{
			name:     "exact match - DS_Store in subdirectory",
			path:     "src/components/.DS_Store",
			pattern:  ".DS_Store",
			expected: true,
		},
		{
			name:     "exact match - Thumbs.db",
			path:     "images/Thumbs.db",
			pattern:  "Thumbs.db",
			expected: true,
		},
		{
			name:     "exact match - no match",
			path:     "src/file.txt",
			pattern:  ".DS_Store",
			expected: false,
		},

		// Glob patterns (*.ext)
		{
			name:     "glob pattern - *.log at root",
			path:     "app.log",
			pattern:  "*.log",
			expected: true,
		},
		{
			name:     "glob pattern - *.log in subdirectory",
			path:     "logs/error.log",
			pattern:  "*.log",
			expected: true,
		},
		{
			name:     "glob pattern - *.tmp",
			path:     "cache/temp.tmp",
			pattern:  "*.tmp",
			expected: true,
		},
		{
			name:     "glob pattern - *.zip",
			path:     "archives/backup.zip",
			pattern:  "*.zip",
			expected: true,
		},
		{
			name:     "glob pattern - no match",
			path:     "src/file.txt",
			pattern:  "*.log",
			expected: false,
		},
		{
			name:     "glob pattern - partial extension no match",
			path:     "src/file.logger",
			pattern:  "*.log",
			expected: false,
		},
		{
			name:     "glob pattern - *.mp4",
			path:     "videos/recording.mp4",
			pattern:  "*.mp4",
			expected: true,
		},

		// Edge cases
		{
			name:     "empty path",
			path:     "",
			pattern:  "*.log",
			expected: false,
		},
		{
			name:     "path with no directory separator",
			path:     "file.txt",
			pattern:  ".git/",
			expected: false,
		},
		{
			name:     "deeply nested folder",
			path:     "a/b/c/d/.git/e/f",
			pattern:  ".git/",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := orchestrator.matchPattern(tt.path, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchPattern(%q, %q) = %v, expected %v", tt.path, tt.pattern, result, tt.expected)
			}
		})
	}
}

// TestSyncOrchestrator_MatchesExclusionPattern validates that a path is checked
// against multiple patterns
func TestSyncOrchestrator_MatchesExclusionPattern(t *testing.T) {
	orchestrator := &SyncOrchestrator{}

	patterns := []string{
		".git/",
		"node_modules/",
		"*.log",
		"*.tmp",
		".DS_Store",
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "matches folder pattern",
			path:     ".git/config",
			expected: true,
		},
		{
			name:     "matches glob pattern",
			path:     "app.log",
			expected: true,
		},
		{
			name:     "matches exact filename",
			path:     "src/.DS_Store",
			expected: true,
		},
		{
			name:     "no match",
			path:     "src/main.go",
			expected: false,
		},
		{
			name:     "matches node_modules in subdirectory",
			path:     "packages/app/node_modules/react/index.js",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := orchestrator.matchesExclusionPattern(tt.path, patterns)
			if result != tt.expected {
				t.Errorf("matchesExclusionPattern(%q, patterns) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestSyncOrchestrator_ShouldExclude validates acceptance criteria:
// - Sync orchestrator applies exclusion patterns
// - Custom patterns from UI are respected alongside enabled default patterns
func TestSyncOrchestrator_ShouldExclude(t *testing.T) {
	tests := []struct {
		name      string
		settings  *domain.Settings
		docPath   string
		expected  bool
	}{
		{
			name: "nil sync exclusions - no exclusion",
			settings: &domain.Settings{
				SyncExclusions: nil,
			},
			docPath:  "src/main.go",
			expected: false,
		},
		{
			name: "empty patterns - no exclusion",
			settings: &domain.Settings{
				SyncExclusions: &domain.SyncExclusionSettings{
					EnabledPatterns:  []string{},
					DisabledPatterns: []string{},
					CustomPatterns:   []string{},
				},
			},
			docPath:  "src/main.go",
			expected: false,
		},
		{
			name: "enabled pattern matches",
			settings: &domain.Settings{
				SyncExclusions: &domain.SyncExclusionSettings{
					EnabledPatterns: []string{".git/", "node_modules/"},
					CustomPatterns:  []string{},
				},
			},
			docPath:  ".git/config",
			expected: true,
		},
		{
			name: "custom pattern matches",
			settings: &domain.Settings{
				SyncExclusions: &domain.SyncExclusionSettings{
					EnabledPatterns: []string{},
					CustomPatterns:  []string{"*.secret", "private/"},
				},
			},
			docPath:  "config.secret",
			expected: true,
		},
		{
			name: "custom folder pattern matches",
			settings: &domain.Settings{
				SyncExclusions: &domain.SyncExclusionSettings{
					EnabledPatterns: []string{},
					CustomPatterns:  []string{"private/"},
				},
			},
			docPath:  "private/keys.txt",
			expected: true,
		},
		{
			name: "both enabled and custom patterns - enabled matches",
			settings: &domain.Settings{
				SyncExclusions: &domain.SyncExclusionSettings{
					EnabledPatterns: []string{".git/", "node_modules/"},
					CustomPatterns:  []string{"*.secret"},
				},
			},
			docPath:  "node_modules/react/index.js",
			expected: true,
		},
		{
			name: "both enabled and custom patterns - custom matches",
			settings: &domain.Settings{
				SyncExclusions: &domain.SyncExclusionSettings{
					EnabledPatterns: []string{".git/", "node_modules/"},
					CustomPatterns:  []string{"*.secret"},
				},
			},
			docPath:  "api/keys.secret",
			expected: true,
		},
		{
			name: "no match - file should be included",
			settings: &domain.Settings{
				SyncExclusions: &domain.SyncExclusionSettings{
					EnabledPatterns: []string{".git/", "node_modules/"},
					CustomPatterns:  []string{"*.secret"},
				},
			},
			docPath:  "src/main.go",
			expected: false,
		},
		{
			name: "disabled patterns are not applied",
			settings: &domain.Settings{
				SyncExclusions: &domain.SyncExclusionSettings{
					EnabledPatterns:  []string{".git/"},
					DisabledPatterns: []string{"node_modules/"}, // Should not be used
					CustomPatterns:   []string{},
				},
			},
			docPath:  "node_modules/package/file.js",
			expected: false, // node_modules is disabled, should not exclude
		},
		{
			name: "default exclusions - common patterns",
			settings: &domain.Settings{
				SyncExclusions: domain.DefaultSyncExclusions(),
			},
			docPath:  ".git/config",
			expected: true,
		},
		{
			name: "default exclusions - build artifacts",
			settings: &domain.Settings{
				SyncExclusions: domain.DefaultSyncExclusions(),
			},
			docPath:  "build/output.js",
			expected: true,
		},
		{
			name: "default exclusions - log files",
			settings: &domain.Settings{
				SyncExclusions: domain.DefaultSyncExclusions(),
			},
			docPath:  "app.log",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settingsStore := &mockSettingsStore{
				settings: tt.settings,
			}

			orchestrator := &SyncOrchestrator{
				settingsStore: settingsStore,
				teamID:        "team-123",
			}

			doc := &domain.Document{
				Path: tt.docPath,
			}

			result := orchestrator.shouldExclude(context.Background(), doc)
			if result != tt.expected {
				t.Errorf("shouldExclude(%q) = %v, expected %v", tt.docPath, result, tt.expected)
			}
		})
	}
}

// TestSyncOrchestrator_ShouldExcludeWithSettingsError validates that when
// settings can't be loaded, documents are NOT excluded (fail open)
func TestSyncOrchestrator_ShouldExcludeWithSettingsError(t *testing.T) {
	settingsStore := &mockSettingsStore{
		settings: nil, // Will return ErrNotFound
	}

	orchestrator := &SyncOrchestrator{
		settingsStore: settingsStore,
		teamID:        "team-123",
	}

	doc := &domain.Document{
		Path: ".git/config", // Would normally be excluded
	}

	// When settings fail to load, we fail open (don't exclude)
	result := orchestrator.shouldExclude(context.Background(), doc)
	if result {
		t.Error("expected shouldExclude to return false when settings can't be loaded")
	}
}

// TestSyncOrchestrator_ExclusionPatternsComprehensive tests comprehensive
// pattern matching across all default exclusion categories
func TestSyncOrchestrator_ExclusionPatternsComprehensive(t *testing.T) {
	orchestrator := &SyncOrchestrator{}

	defaultExclusions := domain.DefaultSyncExclusions()
	patterns := defaultExclusions.GetActivePatterns()

	tests := []struct {
		category string
		path     string
		expected bool
	}{
		// Version control
		{"version-control", ".git/HEAD", true},
		{"version-control", ".svn/entries", true},
		{"version-control", ".hg/store", true},

		// Dependencies
		{"dependencies", "node_modules/package/index.js", true},
		{"dependencies", "vendor/module/file.go", true},
		{"dependencies", "venv/bin/python", true},
		{"dependencies", ".venv/lib/module.py", true},

		// Build artifacts
		{"build", "build/main.js", true},
		{"build", "dist/bundle.js", true},
		{"build", "target/classes/Main.class", true},
		{"build", "out/production/app.jar", true},
		{"build", "bin/executable", true},

		// IDE/Editor
		{"ide", ".idea/workspace.xml", true},
		{"ide", ".vscode/settings.json", true},
		{"ide", ".vs/config.json", true},

		// OS files
		{"os", ".DS_Store", true},
		{"os", "folder/.DS_Store", true},
		{"os", "Thumbs.db", true},

		// Temporary files
		{"temp", "file.tmp", true},
		{"temp", "cache.temp", true},
		{"temp", "app.log", true},

		// Archives
		{"archive", "backup.zip", true},
		{"archive", "data.tar.gz", true},
		{"archive", "files.rar", true},

		// Media files
		{"media", "video.mp4", true},
		{"media", "clip.mov", true},
		{"media", "recording.avi", true},
		{"media", "song.mp3", true},
		{"media", "audio.wav", true},

		// Images
		{"image", "logo.png", true},
		{"image", "photo.jpg", true},
		{"image", "picture.jpeg", true},
		{"image", "animation.gif", true},
		{"image", "icon.svg", true},
		{"image", "assets/images/hero.webp", true},
		{"image", "favicon.ico", true},
		{"image", "screenshot.bmp", true},

		// Should NOT be excluded
		{"source", "src/main.go", false},
		{"source", "README.md", false},
		{"source", "package.json", false},
		{"config", "config.yaml", false},
		{"doc", "docs/guide.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.category+": "+tt.path, func(t *testing.T) {
			result := orchestrator.matchesExclusionPattern(tt.path, patterns)
			if result != tt.expected {
				t.Errorf("path %q: expected excluded=%v, got %v", tt.path, tt.expected, result)
			}
		})
	}
}
