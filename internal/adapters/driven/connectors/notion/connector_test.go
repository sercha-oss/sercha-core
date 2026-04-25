package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
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
	return domain.AuthMethodOAuth2
}
func (s *stubTokenProvider) IsValid(ctx context.Context) bool {
	return true
}

func TestConnector_Type(t *testing.T) {
	c := NewConnector(&stubTokenProvider{}, "", nil)
	if c.Type() != domain.ProviderTypeNotion {
		t.Errorf("expected ProviderTypeNotion, got %v", c.Type())
	}
}

func TestConnector_ValidateConfig(t *testing.T) {
	c := NewConnector(&stubTokenProvider{}, "", nil)
	err := c.ValidateConfig(domain.SourceConfig{})
	if err != nil {
		t.Errorf("ValidateConfig() error = %v, want nil", err)
	}
}

func TestConnector_FetchChanges_InitialSync(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method == "POST" && r.URL.Path == "/v1/search" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(SearchResponse{
				Results: []SearchResult{
					{
						Object:         "page",
						ID:             "page-123",
						CreatedTime:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
						LastEditedTime: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
						Archived:       false,
						URL:            "https://notion.so/page-123",
						Properties: Properties{
							"title": Property{
								Type:  "title",
								Title: MustMarshalRichText([]RichText{{PlainText: "Test Page"}}),
							},
						},
					},
				},
				HasMore:    false,
				NextCursor: "",
			})
			return
		}

		if r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/pages/page-123") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Page{
				Object:         "page",
				ID:             "page-123",
				CreatedTime:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				LastEditedTime: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
				URL:            "https://notion.so/page-123",
				Properties: Properties{
					"title": Property{
						Type:  "title",
						Title: MustMarshalRichText([]RichText{{PlainText: "Test Page"}}),
					},
				},
			})
			return
		}

		if r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/blocks/page-123/children") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(BlocksResponse{
				Results: []Block{
					{
						ID:   "block-1",
						Type: "paragraph",
						Paragraph: &ParagraphBlock{
							RichText: []RichText{{PlainText: "Page content"}},
						},
					},
				},
				HasMore: false,
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	c := NewConnector(&stubTokenProvider{}, "", cfg)

	changes, cursor, err := c.FetchChanges(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("FetchChanges() error = %v", err)
	}

	if len(changes) != 1 {
		t.Errorf("expected 1 change, got %d", len(changes))
	}

	if changes[0].ExternalID != "page-page-123" {
		t.Errorf("ExternalID = %q, want page-page-123", changes[0].ExternalID)
	}

	if changes[0].Type != domain.ChangeTypeModified {
		t.Errorf("Type = %v, want ChangeTypeModified", changes[0].Type)
	}

	if !strings.Contains(changes[0].Content, "Test Page") {
		t.Errorf("Content missing page title, got: %s", changes[0].Content)
	}

	if !strings.Contains(changes[0].Content, "Page content") {
		t.Errorf("Content missing paragraph text, got: %s", changes[0].Content)
	}

	if cursor == "" {
		t.Error("expected non-empty cursor")
	}
}

func TestConnector_FetchChanges_IncrementalSync(t *testing.T) {
	// Cursor represents the last sync time
	lastSync := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create test data: one page edited before cursor (should be skipped),
	// one page edited after cursor (should be included)
	oldPage := SearchResult{
		Object:         "page",
		ID:             "old-page",
		LastEditedTime: time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC), // Before cursor
		Parent:         Parent{Type: "workspace"},
	}
	newPage := SearchResult{
		Object:         "page",
		ID:             "new-page",
		LastEditedTime: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), // After cursor
		Parent:         Parent{Type: "workspace"},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/v1/search" {
			// Verify no filter is sent (Notion Search API doesn't support timestamp filter)
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				if _, hasFilter := body["filter"]; hasFilter {
					t.Error("unexpected filter in search request - Notion Search API doesn't support timestamp filter")
				}
			}

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(SearchResponse{
				Results:    []SearchResult{oldPage, newPage},
				HasMore:    false,
				NextCursor: "",
			})
			return
		}

		// Handle GetPage calls
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/pages/new-page") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Page{
				Object:         "page",
				ID:             "new-page",
				CreatedTime:    time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
				LastEditedTime: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
				Parent:         Parent{Type: "workspace"},
				URL:            "https://notion.so/new-page",
				Properties: Properties{
					"title": Property{
						Type:  "title",
						Title: json.RawMessage(`[{"plain_text": "New Page"}]`),
					},
				},
			})
			return
		}

		// Handle GetBlocks calls
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/blocks/new-page/children") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(BlocksResponse{
				Results: []Block{},
				HasMore: false,
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	c := NewConnector(&stubTokenProvider{}, "", cfg)

	cursor := lastSync.Format(time.RFC3339)
	changes, _, err := c.FetchChanges(context.Background(), nil, cursor)
	if err != nil {
		t.Fatalf("FetchChanges() error = %v", err)
	}

	// Should only return the new page (client-side filtered)
	if len(changes) != 1 {
		t.Errorf("expected 1 change (filtered by cursor), got %d", len(changes))
	}
}

func TestConnector_FetchDocument_Page(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/pages/page-123") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Page{
				Object:         "page",
				ID:             "page-123",
				CreatedTime:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				LastEditedTime: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
				URL:            "https://notion.so/page-123",
				Properties: Properties{
					"title": Property{
						Type:  "title",
						Title: MustMarshalRichText([]RichText{{PlainText: "Test Page"}}),
					},
				},
			})
			return
		}

		if r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/blocks/page-123/children") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(BlocksResponse{
				Results: []Block{
					{
						ID:   "block-1",
						Type: "heading_1",
						Heading1: &HeadingBlock{
							RichText: []RichText{{PlainText: "Heading"}},
						},
					},
					{
						ID:   "block-2",
						Type: "paragraph",
						Paragraph: &ParagraphBlock{
							RichText: []RichText{{PlainText: "Content"}},
						},
					},
				},
				HasMore: false,
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	c := NewConnector(&stubTokenProvider{}, "", cfg)

	doc, contentHash, err := c.FetchDocument(context.Background(), nil, "page-page-123")
	if err != nil {
		t.Fatalf("FetchDocument() error = %v", err)
	}

	if doc.Title != "Test Page" {
		t.Errorf("Title = %q, want Test Page", doc.Title)
	}

	if doc.MimeType != "application/x-notion-page" {
		t.Errorf("MimeType = %q, want application/x-notion-page", doc.MimeType)
	}

	if contentHash == "" {
		t.Error("expected non-empty content hash")
	}

	if doc.Metadata["page_id"] != "page-123" {
		t.Errorf("Metadata[page_id] = %q, want page-123", doc.Metadata["page_id"])
	}

	if doc.Metadata["type"] != "page" {
		t.Errorf("Metadata[type] = %q, want page", doc.Metadata["type"])
	}
}

func TestConnector_FetchDocument_DatabaseEntry(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/pages/entry-456") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Page{
				Object:         "page",
				ID:             "entry-456",
				CreatedTime:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				LastEditedTime: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
				URL:            "https://notion.so/entry-456",
				Parent: Parent{
					Type:       "database_id",
					DatabaseID: "db-789",
				},
				Properties: Properties{
					"Name": Property{
						Type:  "title",
						Title: MustMarshalRichText([]RichText{{PlainText: "Database Entry"}}),
					},
					"Status": Property{
						Type:   "select",
						Select: MustMarshal(&SelectValue{Name: "In Progress"}),
					},
				},
			})
			return
		}

		if r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/databases/db-789") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Database{
				Object: "database",
				ID:     "db-789",
				Title:  []RichText{{PlainText: "My Database"}},
			})
			return
		}

		if r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/blocks/entry-456/children") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(BlocksResponse{
				Results: []Block{},
				HasMore: false,
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	c := NewConnector(&stubTokenProvider{}, "", cfg)

	doc, contentHash, err := c.FetchDocument(context.Background(), nil, "database-entry-entry-456")
	if err != nil {
		t.Fatalf("FetchDocument() error = %v", err)
	}

	if doc.Title != "Database Entry" {
		t.Errorf("Title = %q, want Database Entry", doc.Title)
	}

	if doc.MimeType != "application/x-notion-database-entry" {
		t.Errorf("MimeType = %q, want application/x-notion-database-entry", doc.MimeType)
	}

	if contentHash == "" {
		t.Error("expected non-empty content hash")
	}

	if doc.Metadata["type"] != "database_entry" {
		t.Errorf("Metadata[type] = %q, want database_entry", doc.Metadata["type"])
	}

	if doc.Metadata["database_id"] != "db-789" {
		t.Errorf("Metadata[database_id] = %q, want db-789", doc.Metadata["database_id"])
	}

	if doc.Metadata["database_name"] != "My Database" {
		t.Errorf("Metadata[database_name] = %q, want My Database", doc.Metadata["database_name"])
	}

	if doc.Metadata["prop_Status"] != "In Progress" {
		t.Errorf("Metadata[prop_Status] = %q, want In Progress", doc.Metadata["prop_Status"])
	}
}

func TestConnector_FetchDocument_InvalidExternalID(t *testing.T) {
	c := NewConnector(&stubTokenProvider{}, "", nil)

	tests := []struct {
		name       string
		externalID string
	}{
		{"no separator", "invalidformat"},
		{"unknown type", "unknown-123"},
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

func TestConnector_TestConnection(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/v1/users/me" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(UserResponse{
				Object: "user",
				ID:     "user-123",
				Type:   "bot",
				Name:   "Test Bot",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	c := NewConnector(&stubTokenProvider{}, "", cfg)

	err := c.TestConnection(context.Background(), nil)
	if err != nil {
		t.Errorf("TestConnection() error = %v, want nil", err)
	}
}

func TestConnector_TestConnection_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(ErrorResponse{
			Object:  "error",
			Status:  401,
			Code:    "unauthorized",
			Message: "Invalid token",
		})
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	c := NewConnector(&stubTokenProvider{}, "", cfg)

	err := c.TestConnection(context.Background(), nil)
	if err == nil {
		t.Error("TestConnection() expected error, got nil")
	}
}

func TestConnector_ExtractBlockContent(t *testing.T) {
	tests := []struct {
		name     string
		block    Block
		expected string
	}{
		{
			name: "paragraph",
			block: Block{
				Type: "paragraph",
				Paragraph: &ParagraphBlock{
					RichText: []RichText{{PlainText: "Hello world"}},
				},
			},
			expected: "Hello world",
		},
		{
			name: "heading_1",
			block: Block{
				Type: "heading_1",
				Heading1: &HeadingBlock{
					RichText: []RichText{{PlainText: "Heading"}},
				},
			},
			expected: "# Heading",
		},
		{
			name: "heading_2",
			block: Block{
				Type: "heading_2",
				Heading2: &HeadingBlock{
					RichText: []RichText{{PlainText: "Subheading"}},
				},
			},
			expected: "## Subheading",
		},
		{
			name: "heading_3",
			block: Block{
				Type: "heading_3",
				Heading3: &HeadingBlock{
					RichText: []RichText{{PlainText: "Minor heading"}},
				},
			},
			expected: "### Minor heading",
		},
		{
			name: "bulleted_list_item",
			block: Block{
				Type: "bulleted_list_item",
				BulletedListItem: &ListItemBlock{
					RichText: []RichText{{PlainText: "List item"}},
				},
			},
			expected: "- List item",
		},
		{
			name: "numbered_list_item",
			block: Block{
				Type: "numbered_list_item",
				NumberedListItem: &ListItemBlock{
					RichText: []RichText{{PlainText: "Numbered item"}},
				},
			},
			expected: "1. Numbered item",
		},
		{
			name: "to_do unchecked",
			block: Block{
				Type: "to_do",
				ToDo: &ToDoBlock{
					RichText: []RichText{{PlainText: "Task"}},
					Checked:  false,
				},
			},
			expected: "[ ] Task",
		},
		{
			name: "to_do checked",
			block: Block{
				Type: "to_do",
				ToDo: &ToDoBlock{
					RichText: []RichText{{PlainText: "Done"}},
					Checked:  true,
				},
			},
			expected: "[x] Done",
		},
		{
			name: "code",
			block: Block{
				Type: "code",
				Code: &CodeBlock{
					RichText: []RichText{{PlainText: "fmt.Println(\"hello\")"}},
					Language: "go",
				},
			},
			expected: "```go\nfmt.Println(\"hello\")\n```",
		},
		{
			name: "quote",
			block: Block{
				Type: "quote",
				Quote: &QuoteBlock{
					RichText: []RichText{{PlainText: "Quote text"}},
				},
			},
			expected: "> Quote text",
		},
		{
			name: "child_page",
			block: Block{
				Type: "child_page",
				ChildPage: &ChildPageBlock{
					Title: "Nested Page",
				},
			},
			expected: "[[Nested Page]]",
		},
		{
			name: "child_database",
			block: Block{
				Type: "child_database",
				ChildDatabase: &ChildDatabaseBlock{
					Title: "Nested DB",
				},
			},
			expected: "[[Database: Nested DB]]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractBlockContent(tt.block)
			if result != tt.expected {
				t.Errorf("ExtractBlockContent() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConnector_ContentHashDeterministic(t *testing.T) {
	hash1 := computeContentHash("hello world")
	hash2 := computeContentHash("hello world")
	if hash1 != hash2 {
		t.Errorf("content hash not deterministic: %q != %q", hash1, hash2)
	}

	hash3 := computeContentHash("different content")
	if hash1 == hash3 {
		t.Error("different content produced same hash")
	}

	if len(hash1) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash1))
	}
}

func TestConnector_FetchChanges_Pagination(t *testing.T) {
	callNum := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/v1/search" {
			callNum++
			if callNum == 1 {
				// First page
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(SearchResponse{
					Results: []SearchResult{
						{
							Object:         "page",
							ID:             "page-1",
							CreatedTime:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
							LastEditedTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
							URL:            "https://notion.so/page-1",
							Properties: Properties{
								"title": Property{
									Type:  "title",
									Title: MustMarshalRichText([]RichText{{PlainText: "Page 1"}}),
								},
							},
						},
					},
					HasMore:    true,
					NextCursor: "cursor-1",
				})
			} else {
				// Second page
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(SearchResponse{
					Results: []SearchResult{
						{
							Object:         "page",
							ID:             "page-2",
							CreatedTime:    time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
							LastEditedTime: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
							URL:            "https://notion.so/page-2",
							Properties: Properties{
								"title": Property{
									Type:  "title",
									Title: MustMarshalRichText([]RichText{{PlainText: "Page 2"}}),
								},
							},
						},
					},
					HasMore:    false,
					NextCursor: "",
				})
			}
			return
		}

		if r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/pages/") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Page{
				Object:      "page",
				ID:          "page-1",
				CreatedTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				URL:         "https://notion.so/page-1",
				Properties: Properties{
					"title": Property{Type: "title", Title: MustMarshalRichText([]RichText{{PlainText: "Page"}})},
				},
			})
			return
		}

		if r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/blocks/") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(BlocksResponse{Results: []Block{}, HasMore: false})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	c := NewConnector(&stubTokenProvider{}, "", cfg)

	changes, _, err := c.FetchChanges(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("FetchChanges() error = %v", err)
	}

	if len(changes) != 2 {
		t.Errorf("expected 2 changes from pagination, got %d", len(changes))
	}
}

func TestConnector_FetchChanges_SkipArchived(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/v1/search" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(SearchResponse{
				Results: []SearchResult{
					{
						Object:         "page",
						ID:             "page-active",
						CreatedTime:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
						LastEditedTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
						Archived:       false,
						URL:            "https://notion.so/page-active",
						Properties: Properties{
							"title": Property{Type: "title", Title: MustMarshalRichText([]RichText{{PlainText: "Active"}})},
						},
					},
					{
						Object:         "page",
						ID:             "page-archived",
						CreatedTime:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
						LastEditedTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
						Archived:       true,
						URL:            "https://notion.so/page-archived",
						Properties: Properties{
							"title": Property{Type: "title", Title: MustMarshalRichText([]RichText{{PlainText: "Archived"}})},
						},
					},
				},
				HasMore: false,
			})
			return
		}

		if r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/pages/page-active") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Page{
				Object: "page",
				ID:     "page-active",
				URL:    "https://notion.so/page-active",
				Properties: Properties{
					"title": Property{Type: "title", Title: MustMarshalRichText([]RichText{{PlainText: "Active"}})},
				},
			})
			return
		}

		if r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/blocks/") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(BlocksResponse{Results: []Block{}, HasMore: false})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	c := NewConnector(&stubTokenProvider{}, "", cfg)

	changes, _, err := c.FetchChanges(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("FetchChanges() error = %v", err)
	}

	if len(changes) != 1 {
		t.Errorf("expected 1 change (archived skipped), got %d", len(changes))
	}

	if changes[0].ExternalID != "page-page-active" {
		t.Errorf("ExternalID = %q, want page-page-active", changes[0].ExternalID)
	}
}

// TestReconciliationScopes_Notion — Notion has no native delete signal,
// so both prefixes the connector emits must be in scope.
func TestReconciliationScopes_Notion(t *testing.T) {
	c := NewConnector(&stubTokenProvider{}, "", DefaultConfig())
	scopes := c.ReconciliationScopes()
	if len(scopes) != 2 {
		t.Fatalf("want 2 scopes, got %v", scopes)
	}
	want := map[string]bool{"page-": false, "database-entry-": false}
	for _, s := range scopes {
		if _, ok := want[s]; ok {
			want[s] = true
		} else {
			t.Errorf("unexpected scope %q", s)
		}
	}
	for s, found := range want {
		if !found {
			t.Errorf("missing scope %q", s)
		}
	}
}

// notionInventoryServer collects /search and /databases/.../query
// responses, dispatching by request body's filter.value to mimic
// Notion's object-typed Search filter.
type notionInventoryServer struct {
	// Search responses keyed by filter object value: "page" or "database".
	pages     []SearchResult
	databases []SearchResult
	// QueryDatabase entries keyed by database ID.
	entries map[string][]Page

	pageCalls   int
	dbCalls     int
	queryCalls  map[string]int
	queryErrFor string
}

func newNotionInventoryServer() *notionInventoryServer {
	return &notionInventoryServer{
		entries:    map[string][]Page{},
		queryCalls: map[string]int{},
	}
}

func (n *notionInventoryServer) handler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/v1/search"):
			var body struct {
				Filter *SearchFilter `json:"filter"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			val := ""
			if body.Filter != nil {
				if v, ok := body.Filter.Value.(string); ok {
					val = v
				}
			}
			w.WriteHeader(http.StatusOK)
			switch val {
			case "page":
				n.pageCalls++
				_ = json.NewEncoder(w).Encode(SearchResponse{Results: n.pages})
			case "database":
				n.dbCalls++
				_ = json.NewEncoder(w).Encode(SearchResponse{Results: n.databases})
			default:
				_ = json.NewEncoder(w).Encode(SearchResponse{})
			}
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/v1/databases/") && strings.HasSuffix(r.URL.Path, "/query"):
			// /v1/databases/<id>/query
			parts := strings.Split(r.URL.Path, "/")
			dbID := parts[len(parts)-2]
			n.queryCalls[dbID]++
			if n.queryErrFor == dbID {
				// 403 is non-retryable; avoids the client's 5xx backoff loop
				// turning this into a multi-second test.
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(QueryDatabaseResponse{Results: n.entries[dbID]})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

// TestInventory_Pages_WorkspaceWide — Inventory("page-") returns
// canonical IDs for non-archived workspace pages, skipping pages whose
// parent is a database (those belong to "database-entry-").
func TestInventory_Pages_WorkspaceWide(t *testing.T) {
	srv := newNotionInventoryServer()
	srv.pages = []SearchResult{
		{Object: "page", ID: "p-keep", Parent: Parent{Type: "workspace"}},
		{Object: "page", ID: "p-archived", Parent: Parent{Type: "workspace"}, Archived: true},
		{Object: "page", ID: "p-in-db", Parent: Parent{Type: "database_id", DatabaseID: "db-1"}},
	}

	ts := httptest.NewServer(srv.handler(t))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	c := NewConnector(&stubTokenProvider{}, "", cfg)

	ids, err := c.Inventory(context.Background(), nil, "page-")
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if len(ids) != 1 || ids[0] != "page-p-keep" {
		t.Errorf("inventory = %v, want [page-p-keep]", ids)
	}
}

// TestInventory_DatabaseEntries_WorkspaceWide — Inventory("database-entry-")
// walks Search-for-databases then QueryDatabase per database, accumulating
// non-archived entries.
func TestInventory_DatabaseEntries_WorkspaceWide(t *testing.T) {
	srv := newNotionInventoryServer()
	srv.databases = []SearchResult{
		{Object: "database", ID: "db-1"},
		{Object: "database", ID: "db-2"},
	}
	srv.entries["db-1"] = []Page{
		{ID: "entry-a", Parent: Parent{Type: "database_id", DatabaseID: "db-1"}},
		{ID: "entry-archived", Parent: Parent{Type: "database_id", DatabaseID: "db-1"}, Archived: true},
	}
	srv.entries["db-2"] = []Page{
		{ID: "entry-b", Parent: Parent{Type: "database_id", DatabaseID: "db-2"}},
	}

	ts := httptest.NewServer(srv.handler(t))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	c := NewConnector(&stubTokenProvider{}, "", cfg)

	ids, err := c.Inventory(context.Background(), nil, "database-entry-")
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	got := map[string]bool{}
	for _, id := range ids {
		got[id] = true
	}
	want := []string{"database-entry-entry-a", "database-entry-entry-b"}
	for _, w := range want {
		if !got[w] {
			t.Errorf("inventory missing %q (got %v)", w, ids)
		}
	}
	if got["database-entry-entry-archived"] {
		t.Errorf("archived entry leaked into inventory: %v", ids)
	}
}

// TestInventory_QueryDatabaseFailureSkipsThatDB — a single database
// failing should not poison the rest of the inventory walk; we still
// reconcile against what we did manage to enumerate.
func TestInventory_QueryDatabaseFailureSkipsThatDB(t *testing.T) {
	srv := newNotionInventoryServer()
	srv.databases = []SearchResult{
		{Object: "database", ID: "db-broken"},
		{Object: "database", ID: "db-ok"},
	}
	srv.queryErrFor = "db-broken"
	srv.entries["db-ok"] = []Page{
		{ID: "entry-ok", Parent: Parent{Type: "database_id", DatabaseID: "db-ok"}},
	}

	ts := httptest.NewServer(srv.handler(t))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	c := NewConnector(&stubTokenProvider{}, "", cfg)

	ids, err := c.Inventory(context.Background(), nil, "database-entry-")
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if len(ids) != 1 || ids[0] != "database-entry-entry-ok" {
		t.Errorf("inventory = %v, want [database-entry-entry-ok]", ids)
	}
}

// TestInventory_FailsWhenSearchPagesError — the page-listing search
// itself failing must surface an error and not return a partial set.
func TestInventory_FailsWhenSearchPagesError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	c := NewConnector(&stubTokenProvider{}, "", cfg)

	ids, err := c.Inventory(context.Background(), nil, "page-")
	if err == nil {
		t.Fatalf("expected error, got %d ids and nil err", len(ids))
	}
	if ids != nil {
		t.Errorf("partial inventory leaked: %v", ids)
	}
}

// TestInventory_PagesWarnsOnPagination asserts that the Notion Search
// ordering risk (#100 finding 7) is surfaced as a runtime warning when
// — and only when — the inventory walk actually paginates. The risk
// window only opens with multi-page walks; single-page walks are
// inherently consistent and should produce no warning noise.
func TestInventory_PagesWarnsOnPagination(t *testing.T) {
	t.Run("single page emits no warning", func(t *testing.T) {
		prev := slog.Default()
		t.Cleanup(func() { slog.SetDefault(prev) })
		var buf bytes.Buffer
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(SearchResponse{
				Results: []SearchResult{{Object: "page", ID: "p", Parent: Parent{Type: "workspace"}}},
			})
		}))
		defer ts.Close()

		cfg := DefaultConfig()
		cfg.APIBaseURL = ts.URL + "/v1"
		c := NewConnector(&stubTokenProvider{}, "", cfg)
		if _, err := c.Inventory(context.Background(), nil, "page-"); err != nil {
			t.Fatalf("Inventory: %v", err)
		}
		if strings.Contains(buf.String(), "Search ordering") {
			t.Errorf("single-page walk should not warn, got: %q", buf.String())
		}
	})

	t.Run("multi-page emits warning", func(t *testing.T) {
		prev := slog.Default()
		t.Cleanup(func() { slog.SetDefault(prev) })
		var buf bytes.Buffer
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))

		hits := 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits++
			w.WriteHeader(http.StatusOK)
			if hits == 1 {
				_ = json.NewEncoder(w).Encode(SearchResponse{
					Results:    []SearchResult{{Object: "page", ID: "p1", Parent: Parent{Type: "workspace"}}},
					HasMore:    true,
					NextCursor: "next",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(SearchResponse{
				Results: []SearchResult{{Object: "page", ID: "p2", Parent: Parent{Type: "workspace"}}},
			})
		}))
		defer ts.Close()

		cfg := DefaultConfig()
		cfg.APIBaseURL = ts.URL + "/v1"
		c := NewConnector(&stubTokenProvider{}, "", cfg)
		ids, err := c.Inventory(context.Background(), nil, "page-")
		if err != nil {
			t.Fatalf("Inventory: %v", err)
		}
		if len(ids) != 2 {
			t.Errorf("want 2 ids, got %v", ids)
		}
		if !strings.Contains(buf.String(), "Search ordering is not stable") {
			t.Errorf("multi-page walk should warn about ordering risk, got: %q", buf.String())
		}
		if !strings.Contains(buf.String(), "pages_fetched=2") {
			t.Errorf("warning should report page count, got: %q", buf.String())
		}
	})
}

// TestFetchChanges_LogsAndSkipsFailedItem — when one item's per-item
// fetch fails, the loop logs at WARN and continues. The successful
// item is still emitted; the cursor advances to the successful item's
// timestamp only, so the failed item gets retried on the next tick.
func TestFetchChanges_LogsAndSkipsFailedItem(t *testing.T) {
	// Capture slog output through the package default logger.
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/v1/search"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(SearchResponse{
				Results: []SearchResult{
					{Object: "page", ID: "page-broken", Parent: Parent{Type: "workspace"},
						LastEditedTime: time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)},
					{Object: "page", ID: "page-good", Parent: Parent{Type: "workspace"},
						LastEditedTime: time.Date(2026, 4, 25, 11, 0, 0, 0, time.UTC)},
				},
			})
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/pages/page-broken"):
			// Non-retryable: avoids the client's 5xx backoff.
			w.WriteHeader(http.StatusForbidden)
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/pages/page-good"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Page{
				Object:         "page",
				ID:             "page-good",
				LastEditedTime: time.Date(2026, 4, 25, 11, 0, 0, 0, time.UTC),
			})
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/blocks/page-good/children"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(BlocksResponse{HasMore: false})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	c := NewConnector(&stubTokenProvider{}, "", cfg)

	changes, _, err := c.FetchChanges(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("FetchChanges: %v", err)
	}

	if len(changes) != 1 || changes[0].ExternalID != "page-page-good" {
		t.Errorf("want 1 change for page-good, got %v", changes)
	}

	logged := buf.String()
	if !strings.Contains(logged, "notion: per-item fetch failed") {
		t.Errorf("expected warning log, got: %q", logged)
	}
	if !strings.Contains(logged, "page-broken") {
		t.Errorf("warning should mention failed item id, got: %q", logged)
	}
}

// TestFetchChanges_CursorPreservesSubSecondPrecision — the cursor must
// retain sub-second precision so that two pages edited within the same
// wall-clock second don't collide on the !After cursor comparison and
// drop one of them.
func TestFetchChanges_CursorPreservesSubSecondPrecision(t *testing.T) {
	subSec := time.Date(2026, 4, 25, 12, 34, 56, 789_000_000, time.UTC) // .789

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/v1/search"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(SearchResponse{
				Results: []SearchResult{{
					Object:         "page",
					ID:             "p-1",
					Parent:         Parent{Type: "workspace"},
					LastEditedTime: subSec,
				}},
			})
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/pages/p-1"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Page{Object: "page", ID: "p-1", LastEditedTime: subSec})
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/blocks/p-1/children"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(BlocksResponse{HasMore: false})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	c := NewConnector(&stubTokenProvider{}, "", cfg)

	_, cursor, err := c.FetchChanges(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("FetchChanges: %v", err)
	}
	if !strings.Contains(cursor, ".789") {
		t.Errorf("cursor %q lost sub-second precision (expected nanos)", cursor)
	}
	// And — critically — the legacy parser still reads it. RFC3339-style
	// time.Parse accepts the nano-suffixed form.
	parsed, err := time.Parse(time.RFC3339, cursor)
	if err != nil {
		t.Fatalf("legacy time.RFC3339 parser rejected nano cursor %q: %v", cursor, err)
	}
	if !parsed.Equal(subSec) {
		t.Errorf("round-trip lost data: got %v, want %v", parsed, subSec)
	}
}
