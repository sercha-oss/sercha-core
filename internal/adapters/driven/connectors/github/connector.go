package github

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
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

// Type returns the provider type.
func (c *Connector) Type() domain.ProviderType {
	return domain.ProviderTypeGitHub
}

// ValidateConfig validates source configuration.
func (c *Connector) ValidateConfig(config domain.SourceConfig) error {
	// No special validation needed for GitHub
	return nil
}

// FetchChanges fetches document changes from the repository.
// For initial sync (empty cursor), it fetches all content.
// For incremental sync, it fetches changes since the cursor timestamp.
func (c *Connector) FetchChanges(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
	var changes []*domain.Change
	var lastModified time.Time

	// Parse cursor to get since timestamp
	var since *time.Time
	if cursor != "" {
		parsed, err := time.Parse(time.RFC3339, cursor)
		if err == nil {
			since = &parsed
		}
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

	// Fetch pull requests if enabled
	if c.config.IncludePRs {
		prChanges, err := c.fetchPRChanges(ctx)
		if err != nil {
			return nil, "", fmt.Errorf("fetch PRs: %w", err)
		}
		changes = append(changes, prChanges...)
		for _, change := range prChanges {
			if change.Document != nil && change.Document.UpdatedAt.After(lastModified) {
				lastModified = change.Document.UpdatedAt
			}
		}
	}

	// Fetch files if enabled (only on initial sync or if explicitly requested)
	if c.config.IncludeFiles && since == nil {
		fileChanges, err := c.fetchFileChanges(ctx)
		if err != nil {
			return nil, "", fmt.Errorf("fetch files: %w", err)
		}
		changes = append(changes, fileChanges...)
	}

	// Update cursor to the latest modified time
	newCursor := ""
	if !lastModified.IsZero() {
		newCursor = lastModified.Format(time.RFC3339)
	} else if len(changes) > 0 {
		newCursor = time.Now().Format(time.RFC3339)
	}

	return changes, newCursor, nil
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
			// Skip pull requests (they come from ListPullRequests)
			// GitHub issues API returns PRs too, identified by presence of pull_request field
			// We check by number in the ListIssues response structure

			change := &domain.Change{
				Type:       domain.ChangeTypeModified,
				ExternalID: fmt.Sprintf("issue-%d", issue.Number),
				Document:   c.issueToDocument(issue),
				Content:    c.formatIssueContent(issue),
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
			change := &domain.Change{
				Type:       domain.ChangeTypeModified,
				ExternalID: fmt.Sprintf("pr-%d", pr.Number),
				Document:   c.prToDocument(pr),
				Content:    c.formatPRContent(pr),
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

// fetchFileChanges fetches file changes from the repository with concurrent content fetching.
func (c *Connector) fetchFileChanges(ctx context.Context) ([]*domain.Change, error) {
	repoInfo, err := c.client.GetRepository(ctx, c.owner, c.repo)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	tree, err := c.client.GetTree(ctx, c.owner, c.repo, repoInfo.DefaultBranch)
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	// Filter tree entries before fetching content
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
			return changes, ctx.Err()
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
				Type:       domain.ChangeTypeAdded,
				ExternalID: fmt.Sprintf("file-%s", entry.SHA),
				Document:   c.fileToDocument(entry, content),
				Content:    decodedContent,
			}

			mu.Lock()
			changes = append(changes, change)
			mu.Unlock()
		}(entry)
	}

	wg.Wait()
	return changes, nil
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
		content := c.formatIssueContent(issue)
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
		content := c.formatPRContent(pr)
		contentHash := computeContentHash(content)
		return doc, contentHash, nil

	case "file":
		blob, err := c.client.GetBlob(ctx, c.owner, c.repo, identifier)
		if err != nil {
			return nil, "", fmt.Errorf("fetch file blob %s: %w", identifier, err)
		}
		// Decode base64 content
		decodedContent := ""
		if blob.Encoding == "base64" {
			decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(blob.Content, "\n", ""))
			if err != nil {
				return nil, "", fmt.Errorf("decode blob content: %w", err)
			}
			decodedContent = string(decoded)
		} else {
			decodedContent = blob.Content
		}
		// Build document - we don't have the file path from just a SHA,
		// so use the SHA as the title and build minimal metadata
		doc := &domain.Document{
			Title:    fmt.Sprintf("file-%s", identifier[:8]),
			Path:     fmt.Sprintf("https://github.com/%s/%s/blob/HEAD/%s", c.owner, c.repo, identifier),
			MimeType: "application/octet-stream",
			Metadata: map[string]string{
				"sha":  identifier,
				"size": fmt.Sprintf("%d", blob.Size),
				"repo": FormatContainerID(c.owner, c.repo),
			},
		}
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

// formatIssueContent formats issue content for indexing.
func (c *Connector) formatIssueContent(issue *Issue) string {
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

	return sb.String()
}

// formatPRContent formats pull request content for indexing.
func (c *Connector) formatPRContent(pr *PullRequest) string {
	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(pr.Title)
	sb.WriteString("\n\n")

	fmt.Fprintf(&sb, "Branch: %s → %s\n\n", pr.Head.Ref, pr.Base.Ref)

	if pr.Body != "" {
		sb.WriteString(pr.Body)
	}

	return sb.String()
}

// computeContentHash computes a SHA256 hash of content for change detection.
func computeContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}
