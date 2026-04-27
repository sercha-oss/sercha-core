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

// Issue conversation comments are appended as a `## Comments` section,
// each one as a `### @author (date)` sub-heading. The section markers
// let the chunker split per-comment downstream.
func TestConnector_FetchDocument_Issue_AppendsComments(t *testing.T) {
	var commentsRequested bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/issues/42/comments"):
			commentsRequested = true
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]IssueComment{
				{
					ID:        100,
					Body:      "first comment body",
					User:      &User{Login: "alice"},
					CreatedAt: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC),
				},
				{
					ID:        101,
					Body:      "second comment body",
					User:      &User{Login: "bob"},
					CreatedAt: time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC),
				},
			})
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/issues/42"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Issue{
				Number:    42,
				Title:     "Test Issue",
				Body:      "Issue body content",
				State:     "open",
				User:      &User{Login: "testuser"},
				Comments:  2, // > 0 triggers the comment fetch
				CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)

	// FetchDocument doesn't return content directly — it returns a hash.
	// Reach into formatIssueContent via the same path the connector uses.
	issue, err := c.client.GetIssue(context.Background(), "owner", "repo", 42)
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	body := c.formatIssueContent(context.Background(), issue)

	if !commentsRequested {
		t.Error("comments endpoint was not called")
	}
	for _, want := range []string{
		"## Comments",
		"### @alice (2024-01-03)",
		"first comment body",
		"### @bob (2024-01-04)",
		"second comment body",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in body:\n%s", want, body)
		}
	}
}

// When Issue.Comments is 0, the connector skips the API call entirely —
// no point fetching empty pages for the common case.
func TestConnector_FetchDocument_Issue_SkipsCommentFetchWhenCountIsZero(t *testing.T) {
	var commentsRequested bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/comments") {
			commentsRequested = true
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]IssueComment{})
			return
		}
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/issues/42") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Issue{
				Number:   42,
				Title:    "T",
				Comments: 0,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)

	issue, _ := c.client.GetIssue(context.Background(), "owner", "repo", 42)
	_ = c.formatIssueContent(context.Background(), issue)
	if commentsRequested {
		t.Error("comments endpoint should not be called when Comments == 0")
	}
}

// A failing comments fetch must not abort the sync — the issue body
// alone is still useful. The connector logs and returns content
// without comments.
func TestConnector_FetchDocument_Issue_CommentFetchErrorIsTolerated(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/comments") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/issues/42") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Issue{Number: 42, Title: "T", Body: "main body", Comments: 5})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)

	issue, _ := c.client.GetIssue(context.Background(), "owner", "repo", 42)
	body := c.formatIssueContent(context.Background(), issue)

	if !strings.Contains(body, "main body") {
		t.Errorf("expected issue body to survive even on comment fetch failure:\n%s", body)
	}
	if strings.Contains(body, "## Comments") {
		t.Errorf("did not expect Comments section when fetch failed:\n%s", body)
	}
}

// PRs get three buckets: conversation comments (issues/{n}/comments),
// review comments (pulls/{n}/comments), and reviews (pulls/{n}/reviews).
// Each appears as its own ## section.
func TestConnector_FetchDocument_PR_AppendsAllCommentBuckets(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/issues/10/comments"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]IssueComment{{
				ID: 1, Body: "conversation comment", User: &User{Login: "alice"},
				CreatedAt: time.Date(2024, 2, 6, 0, 0, 0, 0, time.UTC),
			}})
		case strings.HasSuffix(r.URL.Path, "/pulls/10/comments"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]PRReviewComment{{
				ID: 2, Body: "review comment on a line", User: &User{Login: "bob"},
				Path: "src/main.go", Line: 42,
				CreatedAt: time.Date(2024, 2, 7, 0, 0, 0, 0, time.UTC),
			}})
		case strings.HasSuffix(r.URL.Path, "/pulls/10/reviews"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]PRReview{{
				ID: 3, Body: "looks good to me", State: "APPROVED", User: &User{Login: "charlie"},
				SubmittedAt: time.Date(2024, 2, 8, 0, 0, 0, 0, time.UTC),
			}})
		case strings.HasSuffix(r.URL.Path, "/pulls/10"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(PullRequest{
				Number: 10, Title: "Test PR", Body: "PR description",
				Head: &PRBranch{Ref: "feat"}, Base: &PRBranch{Ref: "main"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)

	pr, err := c.client.GetPullRequest(context.Background(), "owner", "repo", 10)
	if err != nil {
		t.Fatalf("GetPullRequest: %v", err)
	}
	body := c.formatPRContent(context.Background(), pr)

	for _, want := range []string{
		"## Comments",
		"conversation comment",
		"## Review comments",
		"### @bob on src/main.go:42",
		"review comment on a line",
		"## Reviews",
		"### @charlie approved (2024-02-08)",
		"looks good to me",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in PR body:\n%s", want, body)
		}
	}
}

// Reviews with empty bodies (every "approve" click GitHub records as a
// review even with no message) must not produce an empty section.
func TestConnector_AppendReviews_DropsEmptyBodyReviews(t *testing.T) {
	var sb strings.Builder
	appendReviews(&sb, []*PRReview{
		{User: &User{Login: "alice"}, State: "APPROVED", Body: ""},
		{User: &User{Login: "bob"}, State: "APPROVED", Body: ""},
	})
	if sb.Len() != 0 {
		t.Errorf("empty-body reviews should not produce a section, got:\n%s", sb.String())
	}
}

func TestConnector_FetchDocument_File(t *testing.T) {
	fileContent := "package main\n\nfunc main() {}\n"
	encodedContent := base64.StdEncoding.EncodeToString([]byte(fileContent))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/repos/owner/repo/contents/src/main.go") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(FileContent{
				Name:     "main.go",
				Path:     "src/main.go",
				SHA:      "abc123def",
				Content:  encodedContent,
				Encoding: "base64",
				Size:     int64(len(fileContent)),
				HTMLURL:  "https://github.com/owner/repo/blob/HEAD/src/main.go",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)

	doc, contentHash, err := c.FetchDocument(context.Background(), nil, "file-src/main.go")
	if err != nil {
		t.Fatalf("FetchDocument() error = %v", err)
	}
	if doc.Title != "src/main.go" {
		t.Errorf("Title = %q, want src/main.go", doc.Title)
	}
	if contentHash == "" {
		t.Error("expected non-empty content hash")
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

// TestConnector_FetchChanges_PRTimestampDoesNotAdvanceIssueCursor —
// the persisted cursor's LastModified is reused as `since` on the next
// ListIssues call. If a PR's UpdatedAt (which is typically newer than
// any issue's, since PRs churn faster) bumps that watermark, real
// issues older than the freshest PR drop out of the next sync's window.
// /pulls is fetched in full each tick (no `since` filter), so it has no
// business advancing the issue cursor.
func TestConnector_FetchChanges_PRTimestampDoesNotAdvanceIssueCursor(t *testing.T) {
	issueUpdatedAt := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	prUpdatedAt := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC) // newer than the issue
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/issues"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]*Issue{
				{ID: 1, Number: 5, Title: "real issue", State: "open", UpdatedAt: issueUpdatedAt},
			})
		case strings.Contains(r.URL.Path, "/pulls"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]*PullRequest{
				{
					ID: 2, Number: 7, Title: "newer PR", State: "open",
					Head:      &PRBranch{Ref: "feature"},
					Base:      &PRBranch{Ref: "main"},
					UpdatedAt: prUpdatedAt,
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

	_, cursor, err := c.FetchChanges(context.Background(), nil, "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("FetchChanges: %v", err)
	}

	got := parseCursor(cursor)
	if !got.LastModified.Equal(issueUpdatedAt) {
		t.Errorf("LastModified = %s, want %s (issue's UpdatedAt; PRs must not advance the issue cursor)",
			got.LastModified.Format(time.RFC3339), issueUpdatedAt.Format(time.RFC3339))
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

// fileTestServer stands up an httptest server mimicking the subset of the
// GitHub API the file-sync path exercises: /repos/{o}/{r}, /commits/{ref},
// /compare/{a}...{b}, /git/trees/{ref}, and /contents/{path}. Each handler
// is wired through the same shared state so tests can assert what the
// connector actually hit.
type fileTestServer struct {
	headSHA       string
	defaultBranch string
	// tree returned for /git/trees/... keyed by ref.
	trees map[string][]*TreeEntry
	// contents keyed by path.
	contents map[string]FileContent
	// compare indexed by "base...head" → response.
	compares map[string]*CompareResponse
	// If compareNotFound is true, all /compare requests return 404.
	compareNotFound bool
	// Observed calls.
	comparesRequested []string
	treesRequested    []string
	contentsFetched   []string
}

func newFileTestServer(headSHA, defaultBranch string) *fileTestServer {
	return &fileTestServer{
		headSHA:       headSHA,
		defaultBranch: defaultBranch,
		trees:         map[string][]*TreeEntry{},
		contents:      map[string]FileContent{},
		compares:      map[string]*CompareResponse{},
	}
}

func (f *fileTestServer) handler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/repos/owner/repo"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Repository{
				Name:          "repo",
				FullName:      "owner/repo",
				DefaultBranch: f.defaultBranch,
			})
		case strings.Contains(path, "/repos/owner/repo/commits/"):
			// GetCommitSHA resolves a ref to a SHA.
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"sha": f.headSHA})
		case strings.Contains(path, "/repos/owner/repo/compare/"):
			spec := strings.TrimPrefix(path, "/repos/owner/repo/compare/")
			f.comparesRequested = append(f.comparesRequested, spec)
			if f.compareNotFound {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			resp, ok := f.compares[spec]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
		case strings.Contains(path, "/repos/owner/repo/git/trees/"):
			ref := strings.TrimPrefix(path, "/repos/owner/repo/git/trees/")
			if i := strings.Index(ref, "?"); i >= 0 {
				ref = ref[:i]
			}
			f.treesRequested = append(f.treesRequested, ref)
			tree, ok := f.trees[ref]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tree":      tree,
				"truncated": false,
			})
		case strings.Contains(path, "/repos/owner/repo/contents/"):
			p := strings.TrimPrefix(path, "/repos/owner/repo/contents/")
			f.contentsFetched = append(f.contentsFetched, p)
			content, ok := f.contents[p]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(content)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func encodeB64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// TestFileSync_InitialSyncUsesTreeSnapshot — with no base SHA stored
// the connector falls through to a full tree walk, emits path-keyed
// Modified changes for each file, and returns the current head SHA so
// the next tick can switch to incremental Compare.
func TestFileSync_InitialSyncUsesTreeSnapshot(t *testing.T) {
	srv := newFileTestServer("sha-1", "main")
	srv.trees["main"] = []*TreeEntry{
		{Path: "README.md", Type: "blob", SHA: "blob-readme", Size: 50},
		{Path: "src/main.go", Type: "blob", SHA: "blob-main", Size: 120},
	}
	srv.contents["README.md"] = FileContent{Path: "README.md", SHA: "blob-readme", Content: encodeB64("# readme"), Encoding: "base64"}
	srv.contents["src/main.go"] = FileContent{Path: "src/main.go", SHA: "blob-main", Content: encodeB64("package main"), Encoding: "base64"}

	ts := httptest.NewServer(srv.handler(t))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL
	cfg.IncludeIssues = false
	cfg.IncludePRs = false
	cfg.IncludeFiles = true
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)

	changes, cursor, err := c.FetchChanges(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("FetchChanges: %v", err)
	}

	want := map[string]bool{"file-README.md": true, "file-src/main.go": true}
	gotIDs := make(map[string]bool)
	for _, ch := range changes {
		if ch.Type != domain.ChangeTypeModified {
			t.Errorf("want ChangeTypeModified, got %q for %s", ch.Type, ch.ExternalID)
		}
		gotIDs[ch.ExternalID] = true
	}
	for id := range want {
		if !gotIDs[id] {
			t.Errorf("missing change for %s", id)
		}
	}

	if len(srv.comparesRequested) != 0 {
		t.Errorf("initial sync must not call compare, got %v", srv.comparesRequested)
	}
	if len(srv.treesRequested) == 0 {
		t.Error("initial sync should walk tree snapshot")
	}

	state := parseCursor(cursor)
	if state.LastSHA != "sha-1" {
		t.Errorf("cursor LastSHA = %q, want sha-1", state.LastSHA)
	}
}

// TestFileSync_CompareEmitsMixedChanges — with a base SHA stored, the
// connector uses Compare and translates each file status into the
// right Change type. Covers add, modify, remove, rename.
func TestFileSync_CompareEmitsMixedChanges(t *testing.T) {
	srv := newFileTestServer("sha-new", "main")
	srv.compares["sha-old...sha-new"] = &CompareResponse{
		Status: "ahead",
		Files: []*CompareFile{
			{Filename: "README.md", Status: "modified", SHA: "blob-r2", Size: 60},
			{Filename: "src/added.go", Status: "added", SHA: "blob-a1", Size: 40},
			{Filename: "src/gone.go", Status: "removed"},
			{Filename: "docs/new.md", PreviousFilename: "docs/old.md", Status: "renamed", SHA: "blob-rn", Size: 30},
		},
	}
	srv.contents["README.md"] = FileContent{Path: "README.md", SHA: "blob-r2", Content: encodeB64("# readme v2"), Encoding: "base64"}
	srv.contents["src/added.go"] = FileContent{Path: "src/added.go", SHA: "blob-a1", Content: encodeB64("new file"), Encoding: "base64"}
	srv.contents["docs/new.md"] = FileContent{Path: "docs/new.md", SHA: "blob-rn", Content: encodeB64("moved"), Encoding: "base64"}

	ts := httptest.NewServer(srv.handler(t))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL
	cfg.IncludeIssues = false
	cfg.IncludePRs = false
	cfg.IncludeFiles = true
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)

	priorCursor := cursorState{LastSHA: "sha-old"}.encode()
	changes, cursor, err := c.FetchChanges(context.Background(), nil, priorCursor)
	if err != nil {
		t.Fatalf("FetchChanges: %v", err)
	}

	type want struct {
		changeType domain.ChangeType
	}
	expected := map[string]want{
		"file-README.md":    {domain.ChangeTypeModified},
		"file-src/added.go": {domain.ChangeTypeModified},
		"file-src/gone.go":  {domain.ChangeTypeDeleted},
		"file-docs/old.md":  {domain.ChangeTypeDeleted},
		"file-docs/new.md":  {domain.ChangeTypeModified},
	}
	got := map[string]domain.ChangeType{}
	for _, ch := range changes {
		got[ch.ExternalID] = ch.Type
	}
	for id, w := range expected {
		if got[id] != w.changeType {
			t.Errorf("%s: got %q, want %q", id, got[id], w.changeType)
		}
	}
	if len(got) != len(expected) {
		t.Errorf("change count: got %d (%v), want %d (%v)", len(got), got, len(expected), expected)
	}
	if len(srv.treesRequested) != 0 {
		t.Errorf("compare path should not walk tree, but tree requested: %v", srv.treesRequested)
	}
	if parseCursor(cursor).LastSHA != "sha-new" {
		t.Errorf("cursor LastSHA = %q, want sha-new", parseCursor(cursor).LastSHA)
	}
}

// TestFileSync_ForcePushFallsBackToSnapshot — when Compare 404s (stored
// SHA no longer reachable, e.g. rebased main) the connector quietly
// re-seeds the cursor from the fresh tree walk.
func TestFileSync_ForcePushFallsBackToSnapshot(t *testing.T) {
	srv := newFileTestServer("sha-new", "main")
	srv.compareNotFound = true
	srv.trees["main"] = []*TreeEntry{
		{Path: "README.md", Type: "blob", SHA: "blob-r", Size: 10},
	}
	srv.contents["README.md"] = FileContent{Path: "README.md", SHA: "blob-r", Content: encodeB64("# readme"), Encoding: "base64"}

	ts := httptest.NewServer(srv.handler(t))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL
	cfg.IncludeIssues = false
	cfg.IncludePRs = false
	cfg.IncludeFiles = true
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)

	priorCursor := cursorState{LastSHA: "sha-gone"}.encode()
	changes, cursor, err := c.FetchChanges(context.Background(), nil, priorCursor)
	if err != nil {
		t.Fatalf("FetchChanges: %v", err)
	}

	if len(srv.comparesRequested) == 0 {
		t.Error("compare should have been attempted")
	}
	if len(srv.treesRequested) == 0 {
		t.Error("expected tree snapshot fallback after compare 404")
	}
	if len(changes) != 1 || changes[0].ExternalID != "file-README.md" {
		t.Errorf("unexpected changes after fallback: %v", changes)
	}
	if parseCursor(cursor).LastSHA != "sha-new" {
		t.Errorf("cursor should be re-seeded to current head, got %q", parseCursor(cursor).LastSHA)
	}
}

// TestFileSync_NoPushShortCircuits — when the repo head hasn't moved
// since the last cursor, no compare is issued and no changes emitted.
func TestFileSync_NoPushShortCircuits(t *testing.T) {
	srv := newFileTestServer("sha-same", "main")
	ts := httptest.NewServer(srv.handler(t))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL
	cfg.IncludeIssues = false
	cfg.IncludePRs = false
	cfg.IncludeFiles = true
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)

	priorCursor := cursorState{LastSHA: "sha-same"}.encode()
	changes, cursor, err := c.FetchChanges(context.Background(), nil, priorCursor)
	if err != nil {
		t.Fatalf("FetchChanges: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(changes))
	}
	if len(srv.comparesRequested) != 0 {
		t.Errorf("no-push path should not call compare, got %v", srv.comparesRequested)
	}
	if parseCursor(cursor).LastSHA != "sha-same" {
		t.Errorf("cursor should stay at same sha, got %q", parseCursor(cursor).LastSHA)
	}
}

// TestReconciliationScopes_RespectsConfig — declared scopes track which
// kinds of content the source actually syncs. Files are intentionally
// absent: the Compare API delta in fetchFileChanges already emits
// removed/renamed signals natively.
func TestReconciliationScopes_RespectsConfig(t *testing.T) {
	cases := []struct {
		name           string
		issues, prs    bool
		wantContains   []string
		mustNotContain []string
	}{
		{"both", true, true, []string{"issue-", "pr-"}, []string{"file-"}},
		{"issues only", true, false, []string{"issue-"}, []string{"pr-", "file-"}},
		{"prs only", false, true, []string{"pr-"}, []string{"issue-", "file-"}},
		{"neither", false, false, nil, []string{"issue-", "pr-", "file-"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.IncludeIssues = tc.issues
			cfg.IncludePRs = tc.prs
			c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)
			scopes := c.ReconciliationScopes()
			has := func(s string) bool {
				for _, x := range scopes {
					if x == s {
						return true
					}
				}
				return false
			}
			for _, w := range tc.wantContains {
				if !has(w) {
					t.Errorf("scopes %v missing %q", scopes, w)
				}
			}
			for _, n := range tc.mustNotContain {
				if has(n) {
					t.Errorf("scopes %v must not contain %q", scopes, n)
				}
			}
		})
	}
}

// TestInventory_IssuesPaginatesAndFiltersPRs — Inventory("issue-") walks
// every page of /issues, drops any rows whose pull_request key is set
// (those belong to "pr-"), and returns canonical IDs.
func TestInventory_IssuesPaginatesAndFiltersPRs(t *testing.T) {
	pageHits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/issues") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		pageHits++
		page := r.URL.Query().Get("page")
		switch page {
		case "1", "":
			// Return 100 issues and advertise a next page via the Link
			// header — the connector's pagination signal.
			w.Header().Set("Link", `<`+r.URL.String()+`&page=2>; rel="next"`)
			w.WriteHeader(http.StatusOK)
			items := make([]*Issue, 100)
			pr := json.RawMessage(`{"url":"x"}`)
			for i := 0; i < 100; i++ {
				items[i] = &Issue{Number: i + 1, State: "open"}
				if i == 99 {
					// Last one is a PR — must be excluded from inventory.
					items[i].PullRequest = &pr
				}
			}
			_ = json.NewEncoder(w).Encode(items)
		case "2":
			// Final page: one real issue + no Link rel="next", so probing stops.
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]*Issue{{Number: 200, State: "closed"}})
		default:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]*Issue{})
		}
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL
	cfg.IncludeIssues = true
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)

	ids, err := c.Inventory(context.Background(), nil, "issue-")
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if pageHits < 2 {
		t.Errorf("expected at least 2 pages requested, got %d", pageHits)
	}
	// 99 issues from page 1 (PR filtered) + 1 from page 2 = 100.
	if len(ids) != 100 {
		t.Errorf("want 100 inventory IDs, got %d", len(ids))
	}
	// PR's number was 100; must not appear in inventory.
	for _, id := range ids {
		if id == "issue-100" {
			t.Errorf("PR leaked into issue inventory: %q", id)
		}
	}
}

// TestInventory_FailsOnAnyPageError — the orchestrator depends on
// "complete-or-error". A mid-walk failure must not return a partial
// list, since that would drive false deletes.
func TestInventory_FailsOnAnyPageError(t *testing.T) {
	pageHits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/issues") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		pageHits++
		if pageHits == 1 {
			// First page succeeds with a full 100 items and a Link
			// rel="next" header so the connector asks for page 2.
			items := make([]*Issue, 100)
			for i := 0; i < 100; i++ {
				items[i] = &Issue{Number: i + 1, State: "open"}
			}
			w.Header().Set("Link", `<`+r.URL.String()+`&page=2>; rel="next"`)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(items)
			return
		}
		// Page 2 fails non-retryably (avoid the 5xx exponential backoff
		// the client would otherwise spin through).
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL
	cfg.IncludeIssues = true
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)

	ids, err := c.Inventory(context.Background(), nil, "issue-")
	if err == nil {
		t.Fatalf("expected error on page-2 failure, got %d ids and nil err", len(ids))
	}
	if ids != nil {
		t.Errorf("partial inventory leaked: %d ids returned alongside error", len(ids))
	}
}

// TestInventory_KeepsPagingWhenPageUnderHundred — regression for the
// `len == 100` heuristic: a full page with Link rel="next" must drive
// the next page request even if the *filtered* result is shorter.
// Previously this case stopped pagination silently, dropping issues and
// (worse, post-delete-detection) flagging them as upstream deletes.
func TestInventory_KeepsPagingWhenPageUnderHundred(t *testing.T) {
	pageHits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/issues") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		pageHits++
		page := r.URL.Query().Get("page")
		switch page {
		case "1", "":
			// 50 items + Link rel="next": fewer than 100, but more pages exist.
			w.Header().Set("Link", `<`+r.URL.String()+`&page=2>; rel="next"`)
			w.WriteHeader(http.StatusOK)
			items := make([]*Issue, 50)
			for i := 0; i < 50; i++ {
				items[i] = &Issue{Number: i + 1, State: "open"}
			}
			_ = json.NewEncoder(w).Encode(items)
		case "2":
			// No Link header → end of pagination.
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]*Issue{{Number: 100, State: "open"}})
		default:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]*Issue{})
		}
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL
	cfg.IncludeIssues = true
	c := NewConnector(&stubTokenProvider{}, "owner", "repo", cfg)

	ids, err := c.Inventory(context.Background(), nil, "issue-")
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if pageHits != 2 {
		t.Errorf("expected 2 page requests, got %d", pageHits)
	}
	if len(ids) != 51 {
		t.Errorf("want 51 inventory IDs, got %d", len(ids))
	}
}

// TestParseCursor_BackCompat — bare RFC3339 cursors from before the
// struct format are still understood as LastModified-only.
func TestParseCursor_BackCompat(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := parseCursor(""); !got.LastModified.IsZero() || got.LastSHA != "" {
			t.Errorf("empty cursor: got %+v, want zero value", got)
		}
	})
	t.Run("legacy RFC3339", func(t *testing.T) {
		got := parseCursor("2026-04-01T12:00:00Z")
		if got.LastModified.IsZero() {
			t.Error("legacy cursor: LastModified should be set")
		}
		if got.LastSHA != "" {
			t.Errorf("legacy cursor: LastSHA = %q, want empty", got.LastSHA)
		}
	})
	t.Run("struct cursor", func(t *testing.T) {
		enc := cursorState{
			LastModified: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
			LastSHA:      "abc123",
		}.encode()
		got := parseCursor(enc)
		if got.LastSHA != "abc123" {
			t.Errorf("LastSHA = %q, want abc123", got.LastSHA)
		}
	})
}
