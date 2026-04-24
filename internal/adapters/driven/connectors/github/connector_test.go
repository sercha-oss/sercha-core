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
				ID:        1,
				Number:    42,
				Title:     "Test Issue",
				Body:      "Issue body content",
				State:     "open",
				HTMLURL:   "https://github.com/owner/repo/issues/42",
				User:      &User{Login: "testuser"},
				Labels:    []Label{{Name: "bug"}},
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
				ID:        2,
				Number:    10,
				Title:     "Test PR",
				Body:      "PR description",
				State:     "closed",
				HTMLURL:   "https://github.com/owner/repo/pull/10",
				User:      &User{Login: "prauthor"},
				Head:      &PRBranch{Ref: "feature-branch", SHA: "abc123"},
				Base:      &PRBranch{Ref: "main", SHA: "def456"},
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

func TestIssue_IsPR(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var i *Issue
		if i.IsPR() {
			t.Error("nil Issue should not report as PR")
		}
	})
	t.Run("absent pull_request key", func(t *testing.T) {
		var i Issue
		if err := json.Unmarshal([]byte(`{"number":1,"title":"bug"}`), &i); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if i.IsPR() {
			t.Error("issue without pull_request key should not report as PR")
		}
	})
	t.Run("present pull_request key", func(t *testing.T) {
		var i Issue
		if err := json.Unmarshal([]byte(`{"number":2,"title":"PR","pull_request":{"url":"..."}}`), &i); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !i.IsPR() {
			t.Error("issue with pull_request key should report as PR")
		}
	})
}

// TestConnector_FetchChanges_SkipsPRsInIssuesLoop asserts that the issues
// list response — which GitHub returns with PRs mixed in — does not produce
// a second indexed copy of each PR. Without the IsPR filter, a PR appears
// twice: once as `issue-N`, once as `pr-N`.
func TestConnector_FetchChanges_SkipsPRsInIssuesLoop(t *testing.T) {
	issuesCalled := 0
	prsCalled := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/issues"):
			issuesCalled++
			w.WriteHeader(http.StatusOK)
			pr := json.RawMessage(`{"url":"https://api.github.com/repos/owner/repo/pulls/7"}`)
			_ = json.NewEncoder(w).Encode([]*Issue{
				{
					ID:        1,
					Number:    5,
					Title:     "real issue",
					State:     "open",
					UpdatedAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					ID:          2,
					Number:      7,
					Title:       "really a PR",
					State:       "open",
					UpdatedAt:   time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
					PullRequest: &pr,
				},
			})
		case strings.Contains(r.URL.Path, "/pulls"):
			prsCalled++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]*PullRequest{
				{
					ID:        2,
					Number:    7,
					Title:     "really a PR",
					State:     "open",
					Head:      &PRBranch{Ref: "feature"},
					Base:      &PRBranch{Ref: "main"},
					UpdatedAt: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL
	cfg.IncludeIssues = true
	cfg.IncludePRs = true
	cfg.IncludeFiles = false
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)

	changes, _, err := c.FetchChanges(context.Background(), nil, "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("FetchChanges: %v", err)
	}

	if issuesCalled == 0 || prsCalled == 0 {
		t.Fatalf("expected both /issues and /pulls to be called; issues=%d pulls=%d", issuesCalled, prsCalled)
	}

	var issueIDs, prIDs int
	for _, ch := range changes {
		switch {
		case strings.HasPrefix(ch.ExternalID, "issue-"):
			issueIDs++
			if ch.ExternalID == "issue-7" {
				t.Errorf("PR #7 was indexed as an issue — should be filtered by IsPR()")
			}
		case strings.HasPrefix(ch.ExternalID, "pr-"):
			prIDs++
		}
	}
	if issueIDs != 1 {
		t.Errorf("want 1 issue change (the real issue #5), got %d", issueIDs)
	}
	if prIDs != 1 {
		t.Errorf("want 1 PR change (#7 from /pulls), got %d", prIDs)
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
