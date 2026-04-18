package services

import (
	"context"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

func TestContentFilterService_GetMimeType(t *testing.T) {
	service := NewContentFilterService()

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		// Markup / documentation
		{"markdown", "README.md", "text/markdown"},
		{"markdown alt", "docs/guide.markdown", "text/markdown"},
		{"mdx", "component.mdx", "text/markdown"},
		{"plain text", "notes.txt", "text/plain"},
		{"restructured text", "doc.rst", "text/x-rst"},
		{"asciidoc", "manual.adoc", "text/asciidoc"},

		// Web
		{"html", "index.html", "text/html"},
		{"css", "styles.css", "text/css"},
		{"scss", "app.scss", "text/x-scss"},
		{"javascript", "app.js", "application/javascript"},
		{"jsx", "Component.jsx", "text/javascript-jsx"},
		{"typescript", "main.ts", "application/typescript"},
		{"tsx", "App.tsx", "text/typescript-jsx"},
		{"vue", "App.vue", "text/html"},
		{"svelte", "App.svelte", "text/html"},

		// Data
		{"json", "package.json", "application/json"},
		{"yaml", "config.yaml", "text/yaml"},
		{"yml", "docker-compose.yml", "text/yaml"},
		{"toml", "Cargo.toml", "text/x-toml"},
		{"xml", "pom.xml", "application/xml"},
		{"csv", "data.csv", "text/csv"},

		// Programming languages
		{"go", "main.go", "text/x-go"},
		{"python", "app.py", "text/x-python"},
		{"rust", "main.rs", "text/x-rust"},
		{"java", "Main.java", "text/x-java"},
		{"kotlin", "Main.kt", "text/x-kotlin"},
		{"c", "main.c", "text/x-c"},
		{"cpp", "main.cpp", "text/x-c++"},
		{"csharp", "Program.cs", "text/x-csharp"},
		{"swift", "App.swift", "text/x-swift"},
		{"php", "index.php", "text/x-php"},

		// Shell / scripts
		{"shell", "script.sh", "text/x-shellscript"},
		{"bash", "build.bash", "text/x-shellscript"},
		{"powershell", "deploy.ps1", "text/x-powershell"},

		// Build / config
		{"terraform", "main.tf", "text/x-hcl"},
		{"proto", "service.proto", "text/x-protobuf"},
		{"graphql", "schema.graphql", "text/x-graphql"},
		{"sql", "schema.sql", "application/sql"},

		// Images
		{"png", "logo.png", "image/png"},
		{"jpeg", "photo.jpg", "image/jpeg"},
		{"gif", "animation.gif", "image/gif"},
		{"svg", "icon.svg", "image/svg+xml"},
		{"ico", "favicon.ico", "image/x-icon"},

		// Binary / archives
		{"zip", "archive.zip", "application/zip"},
		{"tar", "backup.tar", "application/x-tar"},
		{"gzip", "data.gz", "application/gzip"},
		{"exe", "setup.exe", "application/x-msdownload"},
		{"dll", "library.dll", "application/x-msdownload"},
		{"so", "libfoo.so", "application/x-sharedlib"},
		{"wasm", "app.wasm", "application/wasm"},

		// Fonts
		{"woff", "font.woff", "font/woff"},
		{"woff2", "font.woff2", "font/woff2"},
		{"ttf", "font.ttf", "font/ttf"},
		{"otf", "font.otf", "font/otf"},

		// Extensionless names
		{"dockerfile", "Dockerfile", "text/x-dockerfile"},
		{"dockerfile lower", "dockerfile", "text/x-dockerfile"},
		{"makefile", "Makefile", "text/x-makefile"},
		{"makefile lower", "makefile", "text/x-makefile"},
		{"readme", "README", "text/plain"},
		{"license", "LICENSE", "text/plain"},
		{"gitignore", ".gitignore", "text/plain"},
		{"dockerignore", ".dockerignore", "text/plain"},

		// Unknown extension
		{"unknown", "file.unknown", "text/plain"},
		{"no extension", "file", "text/plain"},

		// Case insensitive
		{"uppercase extension", "file.PNG", "image/png"},
		{"mixed case extension", "file.JpEg", "image/jpeg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.GetMimeType(tt.path)
			if got != tt.expected {
				t.Errorf("GetMimeType(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestContentFilterService_ShouldFetchContent_NoSettings(t *testing.T) {
	service := NewContentFilterService()
	ctx := context.Background()

	tests := []struct {
		name      string
		path      string
		wantFetch bool
		wantMime  string
	}{
		{"text file", "README.md", true, "text/markdown"},
		{"image file", "logo.png", true, "image/png"},
		{"binary file", "app.exe", true, "application/x-msdownload"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFetch, gotMime := service.ShouldFetchContent(ctx, tt.path, nil)
			if gotFetch != tt.wantFetch {
				t.Errorf("ShouldFetchContent(%q, nil) fetch = %v, want %v", tt.path, gotFetch, tt.wantFetch)
			}
			if gotMime != tt.wantMime {
				t.Errorf("ShouldFetchContent(%q, nil) mime = %q, want %q", tt.path, gotMime, tt.wantMime)
			}
		})
	}
}

func TestContentFilterService_ShouldFetchContent_PathPatterns(t *testing.T) {
	service := NewContentFilterService()
	ctx := context.Background()

	settings := &domain.SyncExclusionSettings{
		EnabledPatterns: []string{
			"*.png",
			"*.jpg",
			".git/",
			"node_modules/",
			".DS_Store",
		},
	}

	tests := []struct {
		name      string
		path      string
		wantFetch bool
	}{
		// Should exclude
		{"exclude png", "logo.png", false},
		{"exclude jpg", "photo.jpg", false},
		{"exclude git folder", ".git/config", false},
		{"exclude nested git", "foo/.git/hooks/pre-commit", false},
		{"exclude node_modules", "node_modules/package/index.js", false},
		{"exclude DS_Store", ".DS_Store", false},
		{"exclude DS_Store in folder", "folder/.DS_Store", false},

		// Should allow
		{"allow markdown", "README.md", true},
		{"allow typescript", "app.ts", true},
		{"allow svg", "icon.svg", true},
		{"allow go", "main.go", true},
		{"allow json", "package.json", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFetch, _ := service.ShouldFetchContent(ctx, tt.path, settings)
			if gotFetch != tt.wantFetch {
				t.Errorf("ShouldFetchContent(%q) = %v, want %v", tt.path, gotFetch, tt.wantFetch)
			}
		})
	}
}

func TestContentFilterService_ShouldFetchContent_MimePatterns(t *testing.T) {
	service := NewContentFilterService()
	ctx := context.Background()

	settings := &domain.SyncExclusionSettings{
		MimeExclusions: []string{
			"image/*",
			"font/*",
			"application/zip",
		},
	}

	tests := []struct {
		name      string
		path      string
		wantFetch bool
		wantMime  string
	}{
		// Should exclude (image/*)
		{"exclude png", "logo.png", false, "image/png"},
		{"exclude jpg", "photo.jpg", false, "image/jpeg"},
		{"exclude gif", "anim.gif", false, "image/gif"},
		{"exclude svg", "icon.svg", false, "image/svg+xml"},

		// Should exclude (font/*)
		{"exclude woff", "font.woff", false, "font/woff"},
		{"exclude woff2", "font.woff2", false, "font/woff2"},
		{"exclude ttf", "font.ttf", false, "font/ttf"},

		// Should exclude (application/zip)
		{"exclude zip", "archive.zip", false, "application/zip"},

		// Should allow
		{"allow markdown", "README.md", true, "text/markdown"},
		{"allow typescript", "app.ts", true, "application/typescript"},
		{"allow go", "main.go", true, "text/x-go"},
		{"allow tar", "backup.tar", true, "application/x-tar"},     // Not excluded
		{"allow exe", "app.exe", true, "application/x-msdownload"}, // Not excluded
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFetch, gotMime := service.ShouldFetchContent(ctx, tt.path, settings)
			if gotFetch != tt.wantFetch {
				t.Errorf("ShouldFetchContent(%q) fetch = %v, want %v", tt.path, gotFetch, tt.wantFetch)
			}
			if gotMime != tt.wantMime {
				t.Errorf("ShouldFetchContent(%q) mime = %q, want %q", tt.path, gotMime, tt.wantMime)
			}
		})
	}
}

func TestContentFilterService_ShouldFetchContent_CombinedFilters(t *testing.T) {
	service := NewContentFilterService()
	ctx := context.Background()

	settings := &domain.SyncExclusionSettings{
		EnabledPatterns: []string{
			"*.log",
			".git/",
		},
		MimeExclusions: []string{
			"image/*",
			"font/*",
		},
	}

	tests := []struct {
		name      string
		path      string
		wantFetch bool
	}{
		// Excluded by path pattern
		{"exclude by path - log", "app.log", false},
		{"exclude by path - git", ".git/config", false},

		// Excluded by MIME pattern
		{"exclude by mime - png", "logo.png", false},
		{"exclude by mime - font", "font.woff2", false},

		// Allowed
		{"allow markdown", "README.md", true},
		{"allow typescript", "app.ts", true},
		{"allow tar", "backup.tar", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFetch, _ := service.ShouldFetchContent(ctx, tt.path, settings)
			if gotFetch != tt.wantFetch {
				t.Errorf("ShouldFetchContent(%q) = %v, want %v", tt.path, gotFetch, tt.wantFetch)
			}
		})
	}
}

func TestContentFilterService_ShouldFetchContent_CustomPatterns(t *testing.T) {
	service := NewContentFilterService()
	ctx := context.Background()

	settings := &domain.SyncExclusionSettings{
		CustomPatterns: []string{
			"*.test.ts",
			"*.spec.js",
			"__tests__/",
		},
	}

	tests := []struct {
		name      string
		path      string
		wantFetch bool
	}{
		{"exclude test file", "app.test.ts", false},
		{"exclude spec file", "component.spec.js", false},
		{"exclude tests folder", "__tests__/unit/app.test.js", false},
		{"allow normal typescript", "app.ts", true},
		{"allow normal javascript", "app.js", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFetch, _ := service.ShouldFetchContent(ctx, tt.path, settings)
			if gotFetch != tt.wantFetch {
				t.Errorf("ShouldFetchContent(%q) = %v, want %v", tt.path, gotFetch, tt.wantFetch)
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		pattern string
		want    bool
	}{
		// Folder patterns
		{"folder at root", ".git/config", ".git/", true},
		{"folder nested", "foo/.git/config", ".git/", true},
		{"folder nested deep", "foo/bar/.git/hooks/pre-commit", ".git/", true},
		{"folder not match", "foo/bar/file.txt", ".git/", false},
		{"node_modules at root", "node_modules/pkg/index.js", "node_modules/", true},
		{"node_modules nested", "app/node_modules/pkg/index.js", "node_modules/", true},

		// Exact filename matches
		{"exact match", ".DS_Store", ".DS_Store", true},
		{"exact match in folder", "foo/.DS_Store", ".DS_Store", true},
		{"exact match nested", "foo/bar/.DS_Store", ".DS_Store", true},
		{"exact no match", "foo.txt", ".DS_Store", false},

		// Glob patterns
		{"glob png", "logo.png", "*.png", true},
		{"glob jpg", "photo.jpg", "*.jpg", true},
		{"glob txt", "notes.txt", "*.txt", true},
		{"glob log", "app.log", "*.log", true},
		{"glob no match", "file.txt", "*.log", false},
		{"glob in folder", "folder/file.png", "*.png", true},
		{"glob nested", "a/b/c/file.png", "*.png", true},

		// Complex patterns
		{"test file", "app.test.ts", "*.test.ts", true},
		{"spec file", "component.spec.js", "*.spec.js", true},
		{"test file no match", "app.ts", "*.test.ts", false},

		// Edge cases
		{"empty path", "", "*.txt", false},
		{"empty pattern", "file.txt", "", false},
		{"pattern longer than path", "f.t", "*.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPattern(tt.path, tt.pattern)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.path, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestMatchesExclusionPattern(t *testing.T) {
	patterns := []string{
		"*.png",
		"*.jpg",
		".git/",
		"node_modules/",
		".DS_Store",
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"match png", "logo.png", true},
		{"match jpg", "photo.jpg", true},
		{"match git", ".git/config", true},
		{"match node_modules", "node_modules/pkg/index.js", true},
		{"match DS_Store", ".DS_Store", true},
		{"no match", "README.md", false},
		{"no match ts", "app.ts", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesExclusionPattern(tt.path, patterns)
			if got != tt.want {
				t.Errorf("matchesExclusionPattern(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestContentFilterService_DefaultExclusions(t *testing.T) {
	service := NewContentFilterService()
	ctx := context.Background()
	settings := domain.DefaultSyncExclusions()

	tests := []struct {
		name      string
		path      string
		wantFetch bool
		reason    string
	}{
		// Should exclude by default path patterns
		{"git folder", ".git/config", false, "version control"},
		{"node_modules", "node_modules/pkg/index.js", false, "dependencies"},
		{"DS_Store", ".DS_Store", false, "system files"},

		// Should exclude by default MIME patterns
		{"png image", "logo.png", false, "image MIME type"},
		{"jpg image", "photo.jpg", false, "image MIME type"},
		{"font woff2", "font.woff2", false, "font MIME type"},
		{"zip archive", "archive.zip", false, "archive MIME type"},

		// Should allow
		{"markdown", "README.md", true, "text content"},
		{"typescript", "app.ts", true, "code"},
		{"go", "main.go", true, "code"},
		{"json", "package.json", true, "config"},
		{"svg", "icon.svg", false, "SVG is image/* MIME type"}, // SVG matches image/*
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFetch, gotMime := service.ShouldFetchContent(ctx, tt.path, settings)
			if gotFetch != tt.wantFetch {
				t.Errorf("ShouldFetchContent(%q) = %v (mime: %s), want %v (reason: %s)",
					tt.path, gotFetch, gotMime, tt.wantFetch, tt.reason)
			}
		})
	}
}
