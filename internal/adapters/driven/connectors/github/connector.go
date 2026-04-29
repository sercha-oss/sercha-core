package github

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/connectors"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure Connector implements the interface.
var _ driven.Connector = (*Connector)(nil)

// Connector fetches documents from a single GitHub repository.
type Connector struct {
	tokenProvider driven.TokenProvider
	owner         string
	repo          string
	client        *Client
	config        *Config
}

// NewConnector creates a GitHub connector scoped to a specific repository.
func NewConnector(tokenProvider driven.TokenProvider, owner, repo string, config *Config) *Connector {
	if config == nil {
		config = DefaultConfig()
	}
	return &Connector{
		tokenProvider: tokenProvider,
		owner:         owner,
		repo:          repo,
		client:        NewClient(tokenProvider, config.APIBaseURL),
		config:        config,
	}
}

// cursorState is the persisted shape of the GitHub connector's cursor.
// LastModified drives the issues since-filter; LastSHA anchors the Compare
// API for incremental file sync. Empty fields fall back safely: no
// LastModified → fetch all issues; no LastSHA → tree snapshot for files.
//
// For back-compat, a bare RFC3339 timestamp (the pre-reconciliation cursor
// format) decodes as LastModified only.
type cursorState struct {
	LastModified time.Time `json:"lastModified,omitempty"`
	LastSHA      string    `json:"lastSHA,omitempty"`
}

// parseCursor handles both the JSON struct format and the legacy bare
// RFC3339 timestamp format.
func parseCursor(cursor string) cursorState {
	var st cursorState
	if cursor == "" {
		return st
	}
	if cursor[0] == '{' {
		_ = json.Unmarshal([]byte(cursor), &st)
		return st
	}
	if t, err := time.Parse(time.RFC3339, cursor); err == nil {
		st.LastModified = t
	}
	return st
}

func (s cursorState) encode() string {
	if s.LastModified.IsZero() && s.LastSHA == "" {
		return ""
	}
	b, _ := json.Marshal(s)
	return string(b)
}

// Type returns the provider type.
func (c *Connector) Type() domain.ProviderType {
	return domain.ProviderTypeGitHub
}

// RESTClient implements driven.Connector. Returns the embedded GitHub Client,
// which satisfies driven.RESTClient natively.
func (c *Connector) RESTClient() driven.RESTClient {
	return c.client
}

// ValidateConfig validates source configuration.
func (c *Connector) ValidateConfig(config domain.SourceConfig) error {
	// No special validation needed for GitHub
	return nil
}

// FetchChanges fetches document changes from the repository.
// For initial sync (empty cursor), it fetches all content.
// For incremental sync, it fetches issue/PR changes since the cursor
// timestamp and file changes via the Compare API anchored on the stored
// head SHA.
func (c *Connector) FetchChanges(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
	var changes []*domain.Change
	var lastModified time.Time

	state := parseCursor(cursor)
	var since *time.Time
	if !state.LastModified.IsZero() {
		since = &state.LastModified
	}

	// Fetch issues if enabled
	if c.config.IncludeIssues {
		issueChanges, err := c.fetchIssueChanges(ctx, since)
		if err != nil {
			return nil, "", fmt.Errorf("fetch issues: %w", err)
		}
		changes = append(changes, issueChanges...)
		for _, change := range issueChanges {
			if change.Document != nil && change.Document.UpdatedAt.After(lastModified) {
				lastModified = change.Document.UpdatedAt
			}
		}
	}

	// Fetch pull requests if enabled. Note we do *not* let PR UpdatedAt
	// advance lastModified: that watermark is only used as `since` on the
	// next ListIssues call, and the PR endpoint is re-fetched in full
	// every sync (no `since` filter on /pulls). Folding PRs into the
	// issue cursor would push it past unmodified-but-still-present
	// issues, who'd then drop out of the next sync's window forever.
	if c.config.IncludePRs {
		prChanges, err := c.fetchPRChanges(ctx)
		if err != nil {
			return nil, "", fmt.Errorf("fetch PRs: %w", err)
		}
		changes = append(changes, prChanges...)
	}

	// Fetch files every tick. When we have a previous head SHA we use the
	// Compare API for cheap incremental delta (including native delete and
	// rename signals). Without one — initial sync, or force-push invalidated
	// the stored SHA — we fall back to a full tree snapshot.
	newHeadSHA := state.LastSHA
	if c.config.IncludeFiles {
		fileChanges, headSHA, err := c.fetchFileChanges(ctx, state.LastSHA)
		if err != nil {
			return nil, "", fmt.Errorf("fetch files: %w", err)
		}
		changes = append(changes, fileChanges...)
		if headSHA != "" {
			newHeadSHA = headSHA
		}
	}

	newState := cursorState{LastSHA: newHeadSHA}
	switch {
	case !lastModified.IsZero():
		newState.LastModified = lastModified
	case len(changes) > 0:
		newState.LastModified = time.Now()
	default:
		newState.LastModified = state.LastModified
	}
	return changes, newState.encode(), nil
}

// fetchIssueChanges fetches issue changes.
func (c *Connector) fetchIssueChanges(ctx context.Context, since *time.Time) ([]*domain.Change, error) {
	var allChanges []*domain.Change
	cursor := ""

	for {
		issues, nextCursor, err := c.client.ListIssues(ctx, c.owner, c.repo, since, cursor)
		if err != nil {
			return nil, err
		}

		for _, issue := range issues {
			// GitHub's list-issues endpoint returns pull requests mixed in.
			// Indexing them here would produce a second copy alongside the
			// one fetchPRChanges emits.
			if issue.IsPR() {
				continue
			}

			// Defer comment fetch into LoadContent so the listing path
			// returns metadata-only and the comment GETs parallelise
			// across the orchestrator's worker pool.
			issue := issue
			change := &domain.Change{
				Type:       domain.ChangeTypeModified,
				ExternalID: fmt.Sprintf("issue-%d", issue.Number),
				Document:   c.issueToDocument(issue),
				LoadContent: func(ctx context.Context) (string, error) {
					return c.formatIssueContent(ctx, issue), nil
				},
			}
			if since == nil {
				change.Type = domain.ChangeTypeAdded
			}
			allChanges = append(allChanges, change)
		}

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return allChanges, nil
}

// fetchPRChanges fetches pull request changes.
func (c *Connector) fetchPRChanges(ctx context.Context) ([]*domain.Change, error) {
	var allChanges []*domain.Change
	cursor := ""

	for {
		prs, nextCursor, err := c.client.ListPullRequests(ctx, c.owner, c.repo, cursor)
		if err != nil {
			return nil, err
		}

		for _, pr := range prs {
			// Defer comment + review fetch into LoadContent (same
			// rationale as fetchIssueChanges).
			pr := pr
			change := &domain.Change{
				Type:       domain.ChangeTypeModified,
				ExternalID: fmt.Sprintf("pr-%d", pr.Number),
				Document:   c.prToDocument(pr),
				LoadContent: func(ctx context.Context) (string, error) {
					return c.formatPRContent(ctx, pr), nil
				},
			}
			allChanges = append(allChanges, change)
		}

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return allChanges, nil
}

// fetchFileChanges returns file-level changes and the head SHA for storage
// in the cursor. When baseSHA is empty (initial sync) or the stored base is
// no longer reachable (force-push), it falls back to a full tree snapshot.
// Otherwise it uses the Compare API, which gives add / modify / delete /
// rename statuses directly and costs one request.
func (c *Connector) fetchFileChanges(ctx context.Context, baseSHA string) ([]*domain.Change, string, error) {
	repoInfo, err := c.client.GetRepository(ctx, c.owner, c.repo)
	if err != nil {
		return nil, "", fmt.Errorf("get repository: %w", err)
	}
	headSHA, err := c.client.GetCommitSHA(ctx, c.owner, c.repo, repoInfo.DefaultBranch)
	if err != nil {
		return nil, "", fmt.Errorf("resolve head of %s: %w", repoInfo.DefaultBranch, err)
	}

	// Initial sync: no base to compare against.
	if baseSHA == "" {
		changes, err := c.snapshotFileChanges(ctx, repoInfo.DefaultBranch)
		if err != nil {
			return nil, "", err
		}
		return changes, headSHA, nil
	}
	// Nothing pushed since the last tick.
	if baseSHA == headSHA {
		return nil, headSHA, nil
	}

	cmp, err := c.client.CompareCommits(ctx, c.owner, c.repo, baseSHA, headSHA)
	if err != nil {
		if errors.Is(err, ErrCompareBaseNotFound) {
			// Stored SHA no longer reachable — usually a force-push. Full
			// snapshot re-seeds the cursor safely; orphan deletes are
			// caught by phase-1 reconciliation if we ever add `file-` to
			// that scope (we don't today — compare is the mechanism).
			changes, snapErr := c.snapshotFileChanges(ctx, repoInfo.DefaultBranch)
			if snapErr != nil {
				return nil, "", snapErr
			}
			return changes, headSHA, nil
		}
		return nil, "", fmt.Errorf("compare %s...%s: %w", baseSHA, headSHA, err)
	}

	changes, err := c.compareToChanges(ctx, cmp)
	if err != nil {
		return nil, "", err
	}
	return changes, headSHA, nil
}

// snapshotFileChanges walks the full repository tree on the given ref and
// emits one ChangeTypeModified per file. Modified (not Added) so the
// orchestrator's cleanup-on-update path fires on re-sync.
func (c *Connector) snapshotFileChanges(ctx context.Context, ref string) ([]*domain.Change, error) {
	tree, err := c.client.GetTree(ctx, c.owner, c.repo, ref)
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	var toFetch []*TreeEntry
	for _, entry := range tree {
		if entry.Size > c.config.MaxFileSize {
			continue
		}
		if !c.shouldIncludeFile(entry.Path) {
			continue
		}
		toFetch = append(toFetch, entry)
	}
	return c.fetchFilesConcurrent(ctx, toFetch), nil
}

// compareToChanges translates a Compare API response into domain changes.
// Per-file status is the source of truth: added/modified/changed/copied
// trigger a content re-fetch; removed emits a delete keyed on the filename;
// renamed emits both a delete of the old path and a modify of the new path
// (treating rename as delete + add is the connector's documented crude
// stance — we re-fetch content rather than track path-only updates).
func (c *Connector) compareToChanges(ctx context.Context, cmp *CompareResponse) ([]*domain.Change, error) {
	var toFetch []*TreeEntry
	var changes []*domain.Change

	for _, f := range cmp.Files {
		switch f.Status {
		case "removed":
			if !c.shouldIncludeFile(f.Filename) {
				continue
			}
			changes = append(changes, &domain.Change{
				Type:       domain.ChangeTypeDeleted,
				ExternalID: fmt.Sprintf("file-%s", f.Filename),
			})
		case "renamed":
			// Retire the old path.
			if c.shouldIncludeFile(f.PreviousFilename) {
				changes = append(changes, &domain.Change{
					Type:       domain.ChangeTypeDeleted,
					ExternalID: fmt.Sprintf("file-%s", f.PreviousFilename),
				})
			}
			// Re-index under the new path — content fetched below.
			if c.shouldIncludeFile(f.Filename) && f.Size <= c.config.MaxFileSize {
				toFetch = append(toFetch, &TreeEntry{Path: f.Filename, SHA: f.SHA, Size: f.Size, Type: "blob"})
			}
		case "added", "modified", "changed", "copied":
			if !c.shouldIncludeFile(f.Filename) {
				continue
			}
			if f.Size > c.config.MaxFileSize {
				continue
			}
			toFetch = append(toFetch, &TreeEntry{Path: f.Filename, SHA: f.SHA, Size: f.Size, Type: "blob"})
		case "unchanged":
			// No-op. Compare occasionally reports unchanged entries.
		default:
			// Unknown status — skip rather than misclassify.
			continue
		}
	}

	changes = append(changes, c.fetchFilesConcurrent(ctx, toFetch)...)
	return changes, nil
}

// fetchFilesConcurrent fetches content for each entry and builds a
// ChangeTypeModified per file. Unchanged files still emit a Modified change;
// the orchestrator dedups by external ID.
func (c *Connector) fetchFilesConcurrent(ctx context.Context, toFetch []*TreeEntry) []*domain.Change {
	concurrency := c.config.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		sem     = make(chan struct{}, concurrency)
		changes []*domain.Change
	)

	for _, entry := range toFetch {
		select {
		case <-ctx.Done():
			return changes
		default:
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(entry *TreeEntry) {
			defer wg.Done()
			defer func() { <-sem }()

			content, err := c.client.GetFileContent(ctx, c.owner, c.repo, entry.Path)
			if err != nil {
				return
			}

			decodedContent := ""
			if content.Encoding == "base64" {
				decoded, err := base64.StdEncoding.DecodeString(content.Content)
				if err == nil {
					decodedContent = string(decoded)
				}
			} else {
				decodedContent = content.Content
			}

			change := &domain.Change{
				Type:       domain.ChangeTypeModified,
				ExternalID: fmt.Sprintf("file-%s", entry.Path),
				Document:   c.fileToDocument(entry, content),
				Content:    decodedContent,
			}

			mu.Lock()
			changes = append(changes, change)
			mu.Unlock()
		}(entry)
	}

	wg.Wait()
	return changes
}

// shouldIncludeFile checks if a file should be included based on configuration.
func (c *Connector) shouldIncludeFile(path string) bool {
	// Check excluded paths
	for _, exclude := range c.config.ExcludePaths {
		matched, _ := filepath.Match(exclude, path)
		if matched {
			return false
		}
		// Check if path starts with excluded directory
		if strings.HasSuffix(exclude, "/") && strings.HasPrefix(path, exclude) {
			return false
		}
	}

	// Check file extensions
	if len(c.config.FileExtensions) > 0 {
		ext := filepath.Ext(path)
		found := false
		for _, allowedExt := range c.config.FileExtensions {
			if ext == allowedExt || ext == "."+allowedExt {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// FetchDocument fetches a single document by external ID.
func (c *Connector) FetchDocument(ctx context.Context, source *domain.Source, externalID string) (*domain.Document, string, error) {
	parts := strings.SplitN(externalID, "-", 2)
	if len(parts) != 2 {
		return nil, "", fmt.Errorf("invalid external ID format: %s", externalID)
	}

	docType := parts[0]
	identifier := parts[1]

	switch docType {
	case "issue":
		number, err := strconv.Atoi(identifier)
		if err != nil {
			return nil, "", fmt.Errorf("invalid issue number: %s", identifier)
		}
		issue, err := c.client.GetIssue(ctx, c.owner, c.repo, number)
		if err != nil {
			return nil, "", fmt.Errorf("fetch issue %d: %w", number, err)
		}
		doc := c.issueToDocument(issue)
		content := c.formatIssueContent(ctx, issue)
		contentHash := computeContentHash(content)
		return doc, contentHash, nil

	case "pr":
		number, err := strconv.Atoi(identifier)
		if err != nil {
			return nil, "", fmt.Errorf("invalid PR number: %s", identifier)
		}
		pr, err := c.client.GetPullRequest(ctx, c.owner, c.repo, number)
		if err != nil {
			return nil, "", fmt.Errorf("fetch PR %d: %w", number, err)
		}
		doc := c.prToDocument(pr)
		content := c.formatPRContent(ctx, pr)
		contentHash := computeContentHash(content)
		return doc, contentHash, nil

	case "file":
		// identifier is the repo-relative path. Fetch by path so we can
		// surface the real name and URL rather than a blob SHA.
		content, err := c.client.GetFileContent(ctx, c.owner, c.repo, identifier)
		if err != nil {
			return nil, "", fmt.Errorf("fetch file %s: %w", identifier, err)
		}
		decodedContent := ""
		if content.Encoding == "base64" {
			decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(content.Content, "\n", ""))
			if err != nil {
				return nil, "", fmt.Errorf("decode file content: %w", err)
			}
			decodedContent = string(decoded)
		} else {
			decodedContent = content.Content
		}
		entry := &TreeEntry{Path: identifier, SHA: content.SHA, Size: content.Size, Type: "blob"}
		doc := c.fileToDocument(entry, content)
		contentHash := computeContentHash(decodedContent)
		return doc, contentHash, nil

	default:
		return nil, "", fmt.Errorf("unknown document type: %s", docType)
	}
}

// TestConnection tests the connection to the repository.
func (c *Connector) TestConnection(ctx context.Context, source *domain.Source) error {
	return c.client.ValidateRepoAccess(ctx, c.owner, c.repo)
}

// issueToDocument converts a GitHub issue to a domain document.
func (c *Connector) issueToDocument(issue *Issue) *domain.Document {
	metadata := map[string]string{
		"number":   fmt.Sprintf("%d", issue.Number),
		"state":    issue.State,
		"comments": fmt.Sprintf("%d", issue.Comments),
		"repo":     FormatContainerID(c.owner, c.repo),
	}

	if issue.User != nil {
		metadata["author"] = issue.User.Login
	}

	labels := make([]string, len(issue.Labels))
	for i, l := range issue.Labels {
		labels[i] = l.Name
	}
	if len(labels) > 0 {
		metadata["labels"] = strings.Join(labels, ",")
	}

	return &domain.Document{
		Title:     issue.Title,
		Path:      issue.HTMLURL,
		MimeType:  "application/x-github-issue",
		Metadata:  metadata,
		CreatedAt: issue.CreatedAt,
		UpdatedAt: issue.UpdatedAt,
	}
}

// prToDocument converts a GitHub pull request to a domain document.
func (c *Connector) prToDocument(pr *PullRequest) *domain.Document {
	metadata := map[string]string{
		"number":      fmt.Sprintf("%d", pr.Number),
		"state":       pr.State,
		"repo":        FormatContainerID(c.owner, c.repo),
		"head_branch": pr.Head.Ref,
		"base_branch": pr.Base.Ref,
	}

	if pr.User != nil {
		metadata["author"] = pr.User.Login
	}

	if pr.MergedAt != nil {
		metadata["merged"] = "true"
	}

	return &domain.Document{
		Title:     pr.Title,
		Path:      pr.HTMLURL,
		MimeType:  "application/x-github-pr",
		Metadata:  metadata,
		CreatedAt: pr.CreatedAt,
		UpdatedAt: pr.UpdatedAt,
	}
}

// fileToDocument converts a GitHub file to a domain document.
func (c *Connector) fileToDocument(entry *TreeEntry, content *FileContent) *domain.Document {
	mimeType := connectors.GuessMimeType(entry.Path)

	return &domain.Document{
		Title:    entry.Path,
		Path:     content.HTMLURL,
		MimeType: mimeType,
		Metadata: map[string]string{
			"file_path": entry.Path,
			"sha":       entry.SHA,
			"size":      fmt.Sprintf("%d", entry.Size),
			"repo":      FormatContainerID(c.owner, c.repo),
		},
	}
}

// formatIssueContent formats issue content for indexing. Conversation
// comments are fetched and appended as a `## Comments` section so the
// section-aware chunker can split on them; the discussion thread is
// usually where the actual context lives. A failed comments fetch logs
// and falls through — the issue body alone is still better than nothing.
func (c *Connector) formatIssueContent(ctx context.Context, issue *Issue) string {
	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(issue.Title)
	sb.WriteString("\n\n")

	if len(issue.Labels) > 0 {
		sb.WriteString("Labels: ")
		for i, l := range issue.Labels {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(l.Name)
		}
		sb.WriteString("\n\n")
	}

	if issue.Body != "" {
		sb.WriteString(issue.Body)
	}

	// Issue.Comments is the count from the list endpoint; skip the API
	// call when there's nothing to fetch.
	if issue.Comments > 0 {
		comments, err := c.client.ListIssueComments(ctx, c.owner, c.repo, issue.Number)
		if err != nil {
			slog.Warn("github: failed to fetch issue comments",
				"repo", FormatContainerID(c.owner, c.repo),
				"issue", issue.Number,
				"error", err)
		} else {
			appendComments(&sb, comments)
		}
	}

	return sb.String()
}

// formatPRContent formats pull request content for indexing. Three extra
// buckets are appended: conversation comments (the issue-style timeline,
// served by the same /issues/{n}/comments endpoint), review comments
// (line-level on the diff), and reviews (approve/request-changes
// envelopes). Each bucket is a `## ` heading so the chunker can
// section-split. PR list responses don't carry a comment count, so we
// always attempt the fetch — the API returns an empty slice cheaply.
func (c *Connector) formatPRContent(ctx context.Context, pr *PullRequest) string {
	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(pr.Title)
	sb.WriteString("\n\n")

	fmt.Fprintf(&sb, "Branch: %s → %s\n\n", pr.Head.Ref, pr.Base.Ref)

	if pr.Body != "" {
		sb.WriteString(pr.Body)
	}

	if comments, err := c.client.ListIssueComments(ctx, c.owner, c.repo, pr.Number); err != nil {
		slog.Warn("github: failed to fetch PR conversation comments",
			"repo", FormatContainerID(c.owner, c.repo), "pr", pr.Number, "error", err)
	} else {
		appendComments(&sb, comments)
	}

	if reviewComments, err := c.client.ListPRReviewComments(ctx, c.owner, c.repo, pr.Number); err != nil {
		slog.Warn("github: failed to fetch PR review comments",
			"repo", FormatContainerID(c.owner, c.repo), "pr", pr.Number, "error", err)
	} else {
		appendReviewComments(&sb, reviewComments)
	}

	if reviews, err := c.client.ListPRReviews(ctx, c.owner, c.repo, pr.Number); err != nil {
		slog.Warn("github: failed to fetch PR reviews",
			"repo", FormatContainerID(c.owner, c.repo), "pr", pr.Number, "error", err)
	} else {
		appendReviews(&sb, reviews)
	}

	return sb.String()
}

func appendComments(sb *strings.Builder, comments []*IssueComment) {
	if len(comments) == 0 {
		return
	}
	sb.WriteString("\n\n## Comments\n\n")
	for _, c := range comments {
		body := strings.TrimSpace(c.Body)
		if body == "" {
			continue
		}
		author := "unknown"
		if c.User != nil {
			author = c.User.Login
		}
		fmt.Fprintf(sb, "### @%s (%s)\n\n%s\n\n", author, c.CreatedAt.Format("2006-01-02"), body)
	}
}

func appendReviewComments(sb *strings.Builder, comments []*PRReviewComment) {
	if len(comments) == 0 {
		return
	}
	sb.WriteString("\n\n## Review comments\n\n")
	for _, c := range comments {
		body := strings.TrimSpace(c.Body)
		if body == "" {
			continue
		}
		author := "unknown"
		if c.User != nil {
			author = c.User.Login
		}
		// Path/line context goes into the heading so retrieval can match
		// queries like "review of pkg/foo/bar.go".
		loc := c.Path
		if loc != "" && c.Line > 0 {
			loc = fmt.Sprintf("%s:%d", c.Path, c.Line)
		}
		if loc != "" {
			fmt.Fprintf(sb, "### @%s on %s (%s)\n\n%s\n\n", author, loc, c.CreatedAt.Format("2006-01-02"), body)
		} else {
			fmt.Fprintf(sb, "### @%s (%s)\n\n%s\n\n", author, c.CreatedAt.Format("2006-01-02"), body)
		}
	}
}

func appendReviews(sb *strings.Builder, reviews []*PRReview) {
	// Filter empty reviews — GitHub records every approval click as a
	// review, most have no body and would dilute the section.
	hasBody := false
	for _, r := range reviews {
		if strings.TrimSpace(r.Body) != "" {
			hasBody = true
			break
		}
	}
	if !hasBody {
		return
	}
	sb.WriteString("\n\n## Reviews\n\n")
	for _, r := range reviews {
		body := strings.TrimSpace(r.Body)
		if body == "" {
			continue
		}
		author := "unknown"
		if r.User != nil {
			author = r.User.Login
		}
		state := strings.ToLower(strings.ReplaceAll(r.State, "_", " "))
		fmt.Fprintf(sb, "### @%s %s (%s)\n\n%s\n\n", author, state, r.SubmittedAt.Format("2006-01-02"), body)
	}
}

// computeContentHash computes a SHA256 hash of content for change detection.
func computeContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// ReconciliationScopes declares which canonical-ID prefixes this connector
// snapshot-enumerates for delete detection.
//
// Issues and PRs go through phase-1 reconciliation because GitHub's REST
// API has no deletion signal — a deleted issue or transferred PR simply
// stops appearing in subsequent list responses. Files do NOT need
// reconciliation: the Compare API used in fetchFileChanges already emits
// per-file removed/renamed statuses natively, so deletes are caught
// in-band during phase 2.
func (c *Connector) ReconciliationScopes() []string {
	var scopes []string
	if c.config.IncludeIssues {
		scopes = append(scopes, "issue-")
	}
	if c.config.IncludePRs {
		scopes = append(scopes, "pr-")
	}
	return scopes
}

// Inventory enumerates every canonical ID currently present upstream
// within the given scope. Pagination is "complete-or-error": any page
// failure aborts the whole walk so the orchestrator never sees a partial
// inventory and falsely deletes documents.
func (c *Connector) Inventory(ctx context.Context, source *domain.Source, scope string) ([]string, error) {
	switch scope {
	case "issue-":
		return c.inventoryIssues(ctx)
	case "pr-":
		return c.inventoryPRs(ctx)
	default:
		return nil, fmt.Errorf("github: unknown reconciliation scope %q", scope)
	}
}

func (c *Connector) inventoryIssues(ctx context.Context) ([]string, error) {
	var ids []string
	cursor := ""
	for {
		// since=nil → enumerate every issue, every page. The loop below
		// returns immediately on any error, never producing a short slice.
		issues, nextCursor, err := c.client.ListIssues(ctx, c.owner, c.repo, nil, cursor)
		if err != nil {
			return nil, fmt.Errorf("inventory issues: %w", err)
		}
		for _, issue := range issues {
			if issue.IsPR() {
				continue
			}
			ids = append(ids, fmt.Sprintf("issue-%d", issue.Number))
		}
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}
	return ids, nil
}

func (c *Connector) inventoryPRs(ctx context.Context) ([]string, error) {
	var ids []string
	cursor := ""
	for {
		prs, nextCursor, err := c.client.ListPullRequests(ctx, c.owner, c.repo, cursor)
		if err != nil {
			return nil, fmt.Errorf("inventory PRs: %w", err)
		}
		for _, pr := range prs {
			ids = append(ids, fmt.Sprintf("pr-%d", pr.Number))
		}
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}
	return ids, nil
}
