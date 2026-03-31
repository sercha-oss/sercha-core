package domain

import "time"

// Source represents a data source to index
type Source struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	ProviderType ProviderType `json:"provider_type"`
	Config       SourceConfig `json:"config"`
	Enabled      bool         `json:"enabled"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
	CreatedBy    string       `json:"created_by"` // User ID of creator

	// ConnectionID references the connector connection for authentication
	ConnectionID string `json:"connection_id,omitempty"`

	// Containers lists the containers to index (typed)
	// Empty means index all accessible containers
	// Examples: repos for GitHub, channels for Slack, folders for Drive
	Containers []Container `json:"containers,omitempty"`
}

// Container represents a selectable unit within a connection.
// Examples: a repository, a Slack channel, a Drive folder, a Notion database.
type Container struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Type     string            `json:"type,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// SourceConfig holds provider-specific configuration
type SourceConfig struct {
	// Common fields
	CredentialID string `json:"credential_id,omitempty"`

	// GitHub/GitLab
	Owner      string   `json:"owner,omitempty"`
	Repository string   `json:"repository,omitempty"`
	Branch     string   `json:"branch,omitempty"`
	Paths      []string `json:"paths,omitempty"`

	// Slack
	Channels []string `json:"channels,omitempty"`

	// Notion
	DatabaseIDs []string `json:"database_ids,omitempty"`
	PageIDs     []string `json:"page_ids,omitempty"`

	// Confluence
	SpaceKeys []string `json:"space_keys,omitempty"`

	// Google Drive
	FolderIDs []string `json:"folder_ids,omitempty"`

	// Jira
	ProjectKeys []string `json:"project_keys,omitempty"`
	JQL         string   `json:"jql,omitempty"`

	// Generic
	BaseURL string            `json:"base_url,omitempty"`
	Extra   map[string]string `json:"extra,omitempty"`
}

// SourceSummary provides a summary of a source's state
type SourceSummary struct {
	Source        *Source    `json:"source"`
	DocumentCount int        `json:"document_count"`
	LastSyncAt    *time.Time `json:"last_sync_at,omitempty"`
	SyncStatus    string     `json:"sync_status"`
}
