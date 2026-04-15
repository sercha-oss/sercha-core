package notion

import (
	"context"
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
	lastSync := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	filterChecked := false

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/v1/search" {
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				if filter, ok := body["filter"].(map[string]interface{}); ok {
					if filter["property"] == "last_edited_time" {
						filterChecked = true
					}
				}
			}

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(SearchResponse{
				Results:    []SearchResult{},
				HasMore:    false,
				NextCursor: "",
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
	_, _, err := c.FetchChanges(context.Background(), nil, cursor)
	if err != nil {
		t.Fatalf("FetchChanges() error = %v", err)
	}

	if !filterChecked {
		t.Error("expected filter by last_edited_time to be applied")
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
