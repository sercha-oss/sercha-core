package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// stubTokenProvider returns a static token for testing.
type stubTokenProvider struct{}

func (s *stubTokenProvider) GetAccessToken(ctx context.Context) (string, error) {
	return "test-token", nil
}
func (s *stubTokenProvider) GetCredentials(ctx context.Context) (*domain.Credentials, error) {
	return &domain.Credentials{}, nil
}
func (s *stubTokenProvider) AuthMethod() domain.AuthMethod {
	return domain.AuthMethodPAT
}
func (s *stubTokenProvider) IsValid(ctx context.Context) bool {
	return true
}

func TestConnector_Type(t *testing.T) {
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", nil)
	if c.Type() != domain.ProviderTypeGitHub {
		t.Errorf("expected ProviderTypeGitHub, got %v", c.Type())
	}
}

func TestConnector_FetchDocument_Issue(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/repos/owner/repo/issues/42") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Issue{
				ID:      1,
				Number:  42,
				Title:   "Test Issue",
				Body:    "Issue body content",
				State:   "open",
				HTMLURL: "https://github.com/owner/repo/issues/42",
				User:    &User{Login: "testuser"},
				Labels:  []Label{{Name: "bug"}},
				CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)

	doc, contentHash, err := c.FetchDocument(context.Background(), nil, "issue-42")
	if err != nil {
		t.Fatalf("FetchDocument() error = %v", err)
	}
	if doc.Title != "Test Issue" {
		t.Errorf("Title = %q, want Test Issue", doc.Title)
	}
	if doc.MimeType != "application/x-github-issue" {
		t.Errorf("MimeType = %q, want application/x-github-issue", doc.MimeType)
	}
	if contentHash == "" {
		t.Error("expected non-empty content hash")
	}
	if doc.Metadata["number"] != "42" {
		t.Errorf("Metadata[number] = %q, want 42", doc.Metadata["number"])
	}
	if doc.Metadata["state"] != "open" {
		t.Errorf("Metadata[state] = %q, want open", doc.Metadata["state"])
	}
}

func TestConnector_FetchDocument_PR(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/repos/owner/repo/pulls/10") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(PullRequest{
				ID:      2,
				Number:  10,
				Title:   "Test PR",
				Body:    "PR description",
				State:   "closed",
				HTMLURL: "https://github.com/owner/repo/pull/10",
				User:    &User{Login: "prauthor"},
				Head:    &PRBranch{Ref: "feature-branch", SHA: "abc123"},
				Base:    &PRBranch{Ref: "main", SHA: "def456"},
				CreatedAt: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2024, 2, 5, 0, 0, 0, 0, time.UTC),
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)

	doc, contentHash, err := c.FetchDocument(context.Background(), nil, "pr-10")
	if err != nil {
		t.Fatalf("FetchDocument() error = %v", err)
	}
	if doc.Title != "Test PR" {
		t.Errorf("Title = %q, want Test PR", doc.Title)
	}
	if doc.MimeType != "application/x-github-pr" {
		t.Errorf("MimeType = %q, want application/x-github-pr", doc.MimeType)
	}
	if contentHash == "" {
		t.Error("expected non-empty content hash")
	}
	if doc.Metadata["head_branch"] != "feature-branch" {
		t.Errorf("Metadata[head_branch] = %q, want feature-branch", doc.Metadata["head_branch"])
	}
}

func TestConnector_FetchDocument_File(t *testing.T) {
	fileContent := "package main\n\nfunc main() {}\n"
	encodedContent := base64.StdEncoding.EncodeToString([]byte(fileContent))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/repos/owner/repo/git/blobs/abc123def") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(BlobContent{
				SHA:      "abc123def",
				Content:  encodedContent,
				Encoding: "base64",
				Size:     int64(len(fileContent)),
				URL:      "https://api.github.com/repos/owner/repo/git/blobs/abc123def",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)

	doc, contentHash, err := c.FetchDocument(context.Background(), nil, "file-abc123def")
	if err != nil {
		t.Fatalf("FetchDocument() error = %v", err)
	}
	if doc.Title != "file-abc123de" {
		t.Errorf("Title = %q, want file-abc123de (first 8 chars of SHA)", doc.Title)
	}
	if contentHash == "" {
		t.Error("expected non-empty content hash")
	}
	if doc.Metadata["sha"] != "abc123def" {
		t.Errorf("Metadata[sha] = %q, want abc123def", doc.Metadata["sha"])
	}
}

func TestConnector_FetchDocument_InvalidExternalID(t *testing.T) {
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", nil)

	tests := []struct {
		name       string
		externalID string
	}{
		{"no separator", "invalidformat"},
		{"unknown type", "unknown-123"},
		{"invalid issue number", "issue-abc"},
		{"invalid PR number", "pr-xyz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := c.FetchDocument(context.Background(), nil, tt.externalID)
			if err == nil {
				t.Errorf("expected error for external ID %q", tt.externalID)
			}
		})
	}
}

func TestConnector_FetchDocument_ContentHashDeterministic(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/repos/owner/repo/issues/1") {
			callCount++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Issue{
				Number:    1,
				Title:     "Consistent Issue",
				Body:      "Same body every time",
				State:     "open",
				HTMLURL:   "https://github.com/owner/repo/issues/1",
				CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)

	_, hash1, err := c.FetchDocument(context.Background(), nil, "issue-1")
	if err != nil {
		t.Fatalf("first FetchDocument() error = %v", err)
	}
	_, hash2, err := c.FetchDocument(context.Background(), nil, "issue-1")
	if err != nil {
		t.Fatalf("second FetchDocument() error = %v", err)
	}
	if hash1 != hash2 {
		t.Errorf("content hash not deterministic: %q != %q", hash1, hash2)
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
}

func TestComputeContentHash(t *testing.T) {
	// Hash should be deterministic
	hash1 := computeContentHash("hello world")
	hash2 := computeContentHash("hello world")
	if hash1 != hash2 {
		t.Errorf("hash not deterministic: %q != %q", hash1, hash2)
	}

	// Different content should produce different hashes
	hash3 := computeContentHash("different content")
	if hash1 == hash3 {
		t.Error("different content produced same hash")
	}

	// Hash should be 64 hex chars (SHA256)
	if len(hash1) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash1))
	}
}
