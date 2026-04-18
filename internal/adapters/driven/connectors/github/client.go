package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Client provides GitHub API operations.
type Client struct {
	tokenProvider driven.TokenProvider
	httpClient    *http.Client
	baseURL       string
	maxRetries    int
}

// NewClient creates a new GitHub API client.
func NewClient(tokenProvider driven.TokenProvider, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &Client{
		tokenProvider: tokenProvider,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		baseURL:       strings.TrimSuffix(baseURL, "/"),
		maxRetries:    3,
	}
}

// Repository represents a GitHub repository.
type Repository struct {
	ID            int64            `json:"id"`
	Owner         string           `json:"-"` // Populated from FullName, not JSON
	OwnerInfo     *RepositoryOwner `json:"owner"`
	Name          string           `json:"name"`
	FullName      string           `json:"full_name"`
	Description   string           `json:"description"`
	Private       bool             `json:"private"`
	Archived      bool             `json:"archived"`
	HTMLURL       string           `json:"html_url"`
	DefaultBranch string           `json:"default_branch"`
}

// RepositoryOwner represents the owner object in GitHub API responses.
type RepositoryOwner struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

// Issue represents a GitHub issue.
type Issue struct {
	ID        int64      `json:"id"`
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	State     string     `json:"state"`
	HTMLURL   string     `json:"html_url"`
	User      *User      `json:"user"`
	Labels    []Label    `json:"labels"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at"`
	Comments  int        `json:"comments"`
	IsPR      bool       `json:"-"` // Set based on pull_request field presence
}

// PullRequest represents a GitHub pull request.
type PullRequest struct {
	ID        int64      `json:"id"`
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	State     string     `json:"state"`
	HTMLURL   string     `json:"html_url"`
	User      *User      `json:"user"`
	Head      *PRBranch  `json:"head"`
	Base      *PRBranch  `json:"base"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	MergedAt  *time.Time `json:"merged_at"`
	ClosedAt  *time.Time `json:"closed_at"`
}

// PRBranch represents a PR branch reference.
type PRBranch struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

// User represents a GitHub user.
type User struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	HTMLURL   string `json:"html_url"`
	AvatarURL string `json:"avatar_url"`
}

// Label represents a GitHub label.
type Label struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// TreeEntry represents a file in a repository tree.
type TreeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"` // "blob" or "tree"
	Size int64  `json:"size"`
	SHA  string `json:"sha"`
	URL  string `json:"url"`
}

// FileContent represents file content from GitHub.
type FileContent struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	SHA      string `json:"sha"`
	Size     int64  `json:"size"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	HTMLURL  string `json:"html_url"`
}

// Commit represents a GitHub commit.
type Commit struct {
	SHA     string        `json:"sha"`
	Message string        `json:"message"`
	Author  *CommitAuthor `json:"author"`
	Date    time.Time     `json:"date"`
}

// CommitAuthor represents commit author info.
type CommitAuthor struct {
	Name  string    `json:"name"`
	Email string    `json:"email"`
	Date  time.Time `json:"date"`
}

// ListReposResponse is the response from listing repositories.
type ListReposResponse struct {
	Repos      []*Repository
	NextCursor string
}

// ListAccessibleRepos lists all repositories accessible to the authenticated user.
func (c *Client) ListAccessibleRepos(ctx context.Context, cursor string) (*ListReposResponse, error) {
	page := 1
	if cursor != "" {
		var err error
		page, err = strconv.Atoi(cursor)
		if err != nil {
			page = 1
		}
	}

	path := fmt.Sprintf("/user/repos?per_page=100&page=%d&affiliation=owner,collaborator,organization_member", page)

	var repos []*Repository
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("decode repos: %w", err)
	}

	// Parse owner from full_name
	for _, repo := range repos {
		parts := strings.SplitN(repo.FullName, "/", 2)
		if len(parts) == 2 {
			repo.Owner = parts[0]
			repo.Name = parts[1]
		}
	}

	// Check if there are more pages
	nextCursor := ""
	if len(repos) == 100 {
		nextCursor = strconv.Itoa(page + 1)
	}

	return &ListReposResponse{
		Repos:      repos,
		NextCursor: nextCursor,
	}, nil
}

// ValidateRepoAccess checks if the authenticated user has access to a repository.
func (c *Client) ValidateRepoAccess(ctx context.Context, owner, repo string) error {
	path := fmt.Sprintf("/repos/%s/%s", owner, repo)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

// GetRepository gets repository information.
func (c *Client) GetRepository(ctx context.Context, owner, repo string) (*Repository, error) {
	path := fmt.Sprintf("/repos/%s/%s", owner, repo)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var repository Repository
	if err := json.NewDecoder(resp.Body).Decode(&repository); err != nil {
		return nil, fmt.Errorf("decode repository: %w", err)
	}

	// Parse owner from full_name
	parts := strings.SplitN(repository.FullName, "/", 2)
	if len(parts) == 2 {
		repository.Owner = parts[0]
		repository.Name = parts[1]
	}

	return &repository, nil
}

// ListIssues lists issues for a repository.
func (c *Client) ListIssues(ctx context.Context, owner, repo string, since *time.Time, cursor string) ([]*Issue, string, error) {
	page := 1
	if cursor != "" {
		var err error
		page, err = strconv.Atoi(cursor)
		if err != nil {
			page = 1
		}
	}

	path := fmt.Sprintf("/repos/%s/%s/issues?per_page=100&page=%d&state=all&sort=updated&direction=desc",
		owner, repo, page)
	if since != nil {
		path += "&since=" + url.QueryEscape(since.Format(time.RFC3339))
	}

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var issues []*Issue
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, "", fmt.Errorf("decode issues: %w", err)
	}

	nextCursor := ""
	if len(issues) == 100 {
		nextCursor = strconv.Itoa(page + 1)
	}

	return issues, nextCursor, nil
}

// ListPullRequests lists pull requests for a repository.
func (c *Client) ListPullRequests(ctx context.Context, owner, repo string, cursor string) ([]*PullRequest, string, error) {
	page := 1
	if cursor != "" {
		var err error
		page, err = strconv.Atoi(cursor)
		if err != nil {
			page = 1
		}
	}

	path := fmt.Sprintf("/repos/%s/%s/pulls?per_page=100&page=%d&state=all&sort=updated&direction=desc",
		owner, repo, page)

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var prs []*PullRequest
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return nil, "", fmt.Errorf("decode pull requests: %w", err)
	}

	nextCursor := ""
	if len(prs) == 100 {
		nextCursor = strconv.Itoa(page + 1)
	}

	return prs, nextCursor, nil
}

// GetTree gets the repository tree (file listing).
func (c *Client) GetTree(ctx context.Context, owner, repo, sha string) ([]*TreeEntry, error) {
	path := fmt.Sprintf("/repos/%s/%s/git/trees/%s?recursive=1", owner, repo, sha)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Tree      []*TreeEntry `json:"tree"`
		Truncated bool         `json:"truncated"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode tree: %w", err)
	}

	// Filter to only blobs (files)
	var files []*TreeEntry
	for _, entry := range result.Tree {
		if entry.Type == "blob" {
			files = append(files, entry)
		}
	}

	return files, nil
}

// GetFileContent gets the content of a file.
func (c *Client) GetFileContent(ctx context.Context, owner, repo, path string) (*FileContent, error) {
	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, path)
	resp, err := c.doRequest(ctx, "GET", apiPath, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var content FileContent
	if err := json.NewDecoder(resp.Body).Decode(&content); err != nil {
		return nil, fmt.Errorf("decode file content: %w", err)
	}

	return &content, nil
}

// GetIssue gets a single issue by number.
func (c *Client) GetIssue(ctx context.Context, owner, repo string, number int) (*Issue, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, number)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var issue Issue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("decode issue: %w", err)
	}

	return &issue, nil
}

// GetPullRequest gets a single pull request by number.
func (c *Client) GetPullRequest(ctx context.Context, owner, repo string, number int) (*PullRequest, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var pr PullRequest
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("decode pull request: %w", err)
	}

	return &pr, nil
}

// BlobContent represents a git blob from GitHub.
type BlobContent struct {
	SHA      string `json:"sha"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	Size     int64  `json:"size"`
	URL      string `json:"url"`
}

// GetBlob gets a git blob by SHA (raw file content).
func (c *Client) GetBlob(ctx context.Context, owner, repo, sha string) (*BlobContent, error) {
	path := fmt.Sprintf("/repos/%s/%s/git/blobs/%s", owner, repo, sha)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var blob BlobContent
	if err := json.NewDecoder(resp.Body).Decode(&blob); err != nil {
		return nil, fmt.Errorf("decode blob: %w", err)
	}

	return &blob, nil
}

// GetUser gets the authenticated user's information.
func (c *Client) GetUser(ctx context.Context) (*User, error) {
	resp, err := c.doRequest(ctx, "GET", "/user", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode user: %w", err)
	}

	return &user, nil
}

// doRequest performs an authenticated HTTP request with retry logic.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	token, err := c.tokenProvider.GetAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	var resp *http.Response
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("do request: %w", err)
		}

		// Check for rate limiting
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			if resetHeader := resp.Header.Get("X-RateLimit-Reset"); resetHeader != "" {
				resetTime, _ := strconv.ParseInt(resetHeader, 10, 64)
				if resetTime > 0 {
					waitDuration := time.Until(time.Unix(resetTime, 0))
					if waitDuration > 0 && waitDuration < 5*time.Minute {
						_ = resp.Body.Close()
						select {
						case <-ctx.Done():
							return nil, ctx.Err()
						case <-time.After(waitDuration):
							continue
						}
					}
				}
			}
		}

		// Success or non-retryable error
		if resp.StatusCode < 500 {
			break
		}

		// Server error - retry with exponential backoff
		_ = resp.Body.Close()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * time.Second):
		}
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}
