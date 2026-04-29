package localfs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/connectors"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

func TestConnector_Type(t *testing.T) {
	c := NewConnector("/tmp", "", nil)
	if c.Type() != domain.ProviderTypeLocalFS {
		t.Errorf("expected ProviderTypeLocalFS, got %v", c.Type())
	}
}

func TestConnector_TestConnection(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	c := NewConnector(tmpDir, "", nil)
	err := c.TestConnection(context.Background(), nil)
	if err != nil {
		t.Errorf("expected no error for existing directory, got %v", err)
	}

	// Test non-existent directory
	c2 := NewConnector("/nonexistent/path/12345", "", nil)
	err = c2.TestConnection(context.Background(), nil)
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

func TestConnector_FetchChanges(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()

	// Create test files
	testFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(testFile, []byte("# Test\n\nThis is a test file."), 0644); err != nil {
		t.Fatal(err)
	}

	goFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(goFile, []byte("package main\n\nfunc main() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory with a file
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	subFile := filepath.Join(subDir, "nested.txt")
	if err := os.WriteFile(subFile, []byte("Nested content"), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewConnector(tmpDir, "", nil)

	changes, cursor, err := c.FetchChanges(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find 3 files
	if len(changes) != 3 {
		t.Errorf("expected 3 changes, got %d", len(changes))
	}

	// Cursor should be set
	if cursor == "" {
		t.Error("expected non-empty cursor")
	}

	// All changes should be "added" type on initial sync
	for _, change := range changes {
		if change.Type != domain.ChangeTypeAdded {
			t.Errorf("expected ChangeTypeAdded, got %v", change.Type)
		}
		if change.Document == nil {
			t.Error("expected document to be set")
		}
		if change.LoadContent == nil {
			t.Error("expected LoadContent thunk to be set")
			continue
		}
		loaded, err := change.LoadContent(context.Background())
		if err != nil {
			t.Errorf("LoadContent error = %v", err)
		}
		if loaded == "" {
			t.Error("expected loaded content to be non-empty")
		}
	}
}

func TestConnector_ShouldExcludeDir(t *testing.T) {
	c := NewConnector("/tmp", "", DefaultConfig())

	tests := []struct {
		path     string
		expected bool
	}{
		{"/tmp/node_modules", true},
		{"/tmp/.git", true},
		{"/tmp/vendor", true},
		{"/tmp/src", false},
		{"/tmp/docs", false},
	}

	for _, tt := range tests {
		got := c.shouldExcludeDir(tt.path)
		if got != tt.expected {
			t.Errorf("shouldExcludeDir(%q) = %v, expected %v", tt.path, got, tt.expected)
		}
	}
}

func TestConnector_FetchDocument(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(testFile, []byte("# Test\n\nThis is a test file."), 0644); err != nil {
		t.Fatal(err)
	}

	goFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(goFile, []byte("package main\n\nfunc main() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewConnector(tmpDir, "test-container", nil)

	// First, fetch changes to discover external IDs
	changes, _, err := c.FetchChanges(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("FetchChanges() error = %v", err)
	}
	if len(changes) == 0 {
		t.Fatal("expected at least one change")
	}

	// Find the external ID for test.md
	var testExternalID string
	for _, change := range changes {
		if change.Document != nil && change.Document.Title == "test.md" {
			testExternalID = change.ExternalID
			break
		}
	}
	if testExternalID == "" {
		t.Fatal("could not find external ID for test.md")
	}

	t.Run("fetch existing document", func(t *testing.T) {
		doc, contentHash, err := c.FetchDocument(context.Background(), nil, testExternalID)
		if err != nil {
			t.Fatalf("FetchDocument() error = %v", err)
		}
		if doc.Title != "test.md" {
			t.Errorf("Title = %q, want test.md", doc.Title)
		}
		if contentHash == "" {
			t.Error("expected non-empty content hash")
		}
	})

	t.Run("fetch non-existent document", func(t *testing.T) {
		_, _, err := c.FetchDocument(context.Background(), nil, "file-0000000000000000")
		if err == nil {
			t.Error("expected error for non-existent document")
		}
	})

	t.Run("invalid external ID format", func(t *testing.T) {
		_, _, err := c.FetchDocument(context.Background(), nil, "invalid-format")
		if err == nil {
			t.Error("expected error for invalid external ID format")
		}
	})

	t.Run("content hash is deterministic", func(t *testing.T) {
		_, hash1, err := c.FetchDocument(context.Background(), nil, testExternalID)
		if err != nil {
			t.Fatalf("first FetchDocument() error = %v", err)
		}
		_, hash2, err := c.FetchDocument(context.Background(), nil, testExternalID)
		if err != nil {
			t.Fatalf("second FetchDocument() error = %v", err)
		}
		if hash1 != hash2 {
			t.Errorf("content hash not deterministic: %q != %q", hash1, hash2)
		}
	})
}

func TestGuessMimeType(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"test.md", "text/markdown"},
		{"test.go", "text/x-go"},
		{"test.py", "text/x-python"},
		{"test.js", "application/javascript"},
		{"test.ts", "application/typescript"},
		{"test.json", "application/json"},
		{"test.yaml", "text/yaml"},
		{"test.yml", "text/yaml"},
		{"test.txt", "text/plain"},
		{"test.unknown", "text/plain"},
		{"Dockerfile", "text/x-dockerfile"},
		{"Makefile", "text/x-makefile"},
	}

	for _, tt := range tests {
		got := connectors.GuessMimeType(tt.path)
		if got != tt.expected {
			t.Errorf("guessMimeType(%q) = %q, expected %q", tt.path, got, tt.expected)
		}
	}
}
