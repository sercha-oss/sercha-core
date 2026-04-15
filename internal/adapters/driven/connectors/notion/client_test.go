package notion

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_Search(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %q, want POST", r.Method)
		}

		if r.URL.Path != "/v1/search" {
			t.Errorf("Path = %q, want /v1/search", r.URL.Path)
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", authHeader)
		}

		versionHeader := r.Header.Get("Notion-Version")
		if versionHeader != "2022-06-28" {
			t.Errorf("Notion-Version = %q, want 2022-06-28", versionHeader)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(SearchResponse{
			Results: []SearchResult{
				{
					Object:         "page",
					ID:             "page-123",
					CreatedTime:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					LastEditedTime: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
					URL:            "https://notion.so/page-123",
				},
			},
			HasMore:    false,
			NextCursor: "",
		})
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	client := NewClient(&stubTokenProvider{}, cfg)

	resp, err := client.Search(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(resp.Results) != 1 {
		t.Errorf("Results length = %d, want 1", len(resp.Results))
	}

	if resp.Results[0].ID != "page-123" {
		t.Errorf("Result ID = %q, want page-123", resp.Results[0].ID)
	}

	if resp.HasMore {
		t.Error("HasMore = true, want false")
	}
}

func TestClient_Search_WithFilter(t *testing.T) {
	filterApplied := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			if filter, ok := body["filter"].(map[string]interface{}); ok {
				// Notion Search API only supports "object" type filter
				if filter["property"] == "object" && filter["value"] == "page" {
					filterApplied = true
				}
			}
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(SearchResponse{
			Results: []SearchResult{},
			HasMore: false,
		})
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	client := NewClient(&stubTokenProvider{}, cfg)

	// Note: Notion Search API only supports filtering by object type (page/database)
	filter := &SearchFilter{
		Property: "object",
		Value:    "page",
	}

	_, err := client.Search(context.Background(), filter, "")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if !filterApplied {
		t.Error("filter was not applied in search request")
	}
}

func TestClient_Search_Pagination(t *testing.T) {
	cursorChecked := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			if cursor, ok := body["start_cursor"].(string); ok && cursor == "test-cursor" {
				cursorChecked = true
			}
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(SearchResponse{
			Results:    []SearchResult{},
			HasMore:    false,
			NextCursor: "",
		})
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	client := NewClient(&stubTokenProvider{}, cfg)

	_, err := client.Search(context.Background(), nil, "test-cursor")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if !cursorChecked {
		t.Error("cursor was not applied in search request")
	}
}

func TestClient_GetPage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Method = %q, want GET", r.Method)
		}

		if r.URL.Path != "/v1/pages/page-123" {
			t.Errorf("Path = %q, want /v1/pages/page-123", r.URL.Path)
		}

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
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	client := NewClient(&stubTokenProvider{}, cfg)

	page, err := client.GetPage(context.Background(), "page-123")
	if err != nil {
		t.Fatalf("GetPage() error = %v", err)
	}

	if page.ID != "page-123" {
		t.Errorf("ID = %q, want page-123", page.ID)
	}

	if page.Object != "page" {
		t.Errorf("Object = %q, want page", page.Object)
	}
}

func TestClient_GetBlocks(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Method = %q, want GET", r.Method)
		}

		if !strings.HasPrefix(r.URL.Path, "/v1/blocks/page-123/children") {
			t.Errorf("Path = %q, want /v1/blocks/page-123/children", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(BlocksResponse{
			Results: []Block{
				{
					ID:   "block-1",
					Type: "paragraph",
					Paragraph: &ParagraphBlock{
						RichText: []RichText{{PlainText: "Content"}},
					},
				},
			},
			HasMore:    false,
			NextCursor: "",
		})
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	client := NewClient(&stubTokenProvider{}, cfg)

	resp, err := client.GetBlocks(context.Background(), "page-123", "")
	if err != nil {
		t.Fatalf("GetBlocks() error = %v", err)
	}

	if len(resp.Results) != 1 {
		t.Errorf("Results length = %d, want 1", len(resp.Results))
	}

	if resp.Results[0].Type != "paragraph" {
		t.Errorf("Block type = %q, want paragraph", resp.Results[0].Type)
	}
}

func TestClient_GetBlocksRecursive(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		if strings.Contains(r.URL.Path, "/v1/blocks/parent-block/children") {
			// Return a block with children
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(BlocksResponse{
				Results: []Block{
					{
						ID:          "child-block",
						Type:        "paragraph",
						HasChildren: true,
						Paragraph: &ParagraphBlock{
							RichText: []RichText{{PlainText: "Parent content"}},
						},
					},
				},
				HasMore: false,
			})
			return
		}

		if strings.Contains(r.URL.Path, "/v1/blocks/child-block/children") {
			// Return nested children
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(BlocksResponse{
				Results: []Block{
					{
						ID:          "nested-block",
						Type:        "paragraph",
						HasChildren: false,
						Paragraph: &ParagraphBlock{
							RichText: []RichText{{PlainText: "Nested content"}},
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
	client := NewClient(&stubTokenProvider{}, cfg)

	blocks, err := client.GetBlocksRecursive(context.Background(), "parent-block", 10)
	if err != nil {
		t.Fatalf("GetBlocksRecursive() error = %v", err)
	}

	if len(blocks) != 2 {
		t.Errorf("Blocks length = %d, want 2 (parent + nested)", len(blocks))
	}

	if callCount != 2 {
		t.Errorf("API calls = %d, want 2 (parent + child)", callCount)
	}
}

func TestClient_GetBlocksRecursive_MaxDepth(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(BlocksResponse{
			Results: []Block{
				{
					ID:          "block-" + string(rune(callCount)),
					Type:        "paragraph",
					HasChildren: true,
					Paragraph: &ParagraphBlock{
						RichText: []RichText{{PlainText: "Content"}},
					},
				},
			},
			HasMore: false,
		})
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	client := NewClient(&stubTokenProvider{}, cfg)

	maxDepth := 2
	_, err := client.GetBlocksRecursive(context.Background(), "root-block", maxDepth)
	if err != nil {
		t.Fatalf("GetBlocksRecursive() error = %v", err)
	}

	if callCount > maxDepth {
		t.Errorf("API calls = %d, want <= %d (max depth limit)", callCount, maxDepth)
	}
}

func TestClient_QueryDatabase(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %q, want POST", r.Method)
		}

		if r.URL.Path != "/v1/databases/db-123/query" {
			t.Errorf("Path = %q, want /v1/databases/db-123/query", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(QueryDatabaseResponse{
			Results: []Page{
				{
					Object: "page",
					ID:     "entry-1",
					Properties: Properties{
						"Name": Property{
							Type:  "title",
							Title: MustMarshalRichText([]RichText{{PlainText: "Entry 1"}}),
						},
					},
				},
			},
			HasMore:    false,
			NextCursor: "",
		})
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	client := NewClient(&stubTokenProvider{}, cfg)

	resp, err := client.QueryDatabase(context.Background(), "db-123", nil, "")
	if err != nil {
		t.Fatalf("QueryDatabase() error = %v", err)
	}

	if len(resp.Results) != 1 {
		t.Errorf("Results length = %d, want 1", len(resp.Results))
	}

	if resp.Results[0].ID != "entry-1" {
		t.Errorf("Result ID = %q, want entry-1", resp.Results[0].ID)
	}
}

func TestClient_GetUser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Method = %q, want GET", r.Method)
		}

		if r.URL.Path != "/v1/users/me" {
			t.Errorf("Path = %q, want /v1/users/me", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(UserResponse{
			Object: "user",
			ID:     "user-123",
			Type:   "bot",
			Name:   "Test Bot",
		})
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	client := NewClient(&stubTokenProvider{}, cfg)

	user, err := client.GetUser(context.Background())
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}

	if user.ID != "user-123" {
		t.Errorf("ID = %q, want user-123", user.ID)
	}

	if user.Type != "bot" {
		t.Errorf("Type = %q, want bot", user.Type)
	}
}

func TestClient_RateLimiting(t *testing.T) {
	requestTimes := []time.Time{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestTimes = append(requestTimes, time.Now())
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(SearchResponse{
			Results: []SearchResult{},
			HasMore: false,
		})
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	cfg.RateLimitRPS = 2.0 // 2 requests per second for faster test
	client := NewClient(&stubTokenProvider{}, cfg)

	// Make 3 requests
	for i := 0; i < 3; i++ {
		_, err := client.Search(context.Background(), nil, "")
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
	}

	if len(requestTimes) != 3 {
		t.Fatalf("Expected 3 requests, got %d", len(requestTimes))
	}

	// Check that rate limiting was applied
	// With 2 RPS, the third request should be delayed by ~500ms from the second
	delay := requestTimes[2].Sub(requestTimes[1])
	minDelay := 400 * time.Millisecond
	if delay < minDelay {
		t.Errorf("Rate limiting not applied: delay = %v, want >= %v", delay, minDelay)
	}
}

func TestClient_RetryOnServerError(t *testing.T) {
	attemptCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			// Return 500 on first two attempts
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(ErrorResponse{
				Object:  "error",
				Status:  500,
				Code:    "internal_server_error",
				Message: "Server error",
			})
			return
		}

		// Success on third attempt
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(SearchResponse{
			Results: []SearchResult{},
			HasMore: false,
		})
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.APIBaseURL = ts.URL + "/v1"
	cfg.MaxRetries = 3
	client := NewClient(&stubTokenProvider{}, cfg)

	_, err := client.Search(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("Search() error = %v (should succeed after retries)", err)
	}

	if attemptCount != 3 {
		t.Errorf("Attempt count = %d, want 3 (2 retries + 1 success)", attemptCount)
	}
}

func TestClient_ErrorResponse(t *testing.T) {
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
	client := NewClient(&stubTokenProvider{}, cfg)

	_, err := client.Search(context.Background(), nil, "")
	if err == nil {
		t.Fatal("Search() expected error, got nil")
	}

	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("error = %q, want to contain 'unauthorized'", err.Error())
	}
}

func TestExtractPlainText(t *testing.T) {
	tests := []struct {
		name     string
		richText []RichText
		expected string
	}{
		{
			name:     "empty",
			richText: []RichText{},
			expected: "",
		},
		{
			name: "single text",
			richText: []RichText{
				{PlainText: "Hello"},
			},
			expected: "Hello",
		},
		{
			name: "multiple texts",
			richText: []RichText{
				{PlainText: "Hello "},
				{PlainText: "world"},
			},
			expected: "Hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractPlainText(tt.richText)
			if result != tt.expected {
				t.Errorf("ExtractPlainText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractBlockContent(t *testing.T) {
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
					RichText: []RichText{{PlainText: "Test paragraph"}},
				},
			},
			expected: "Test paragraph",
		},
		{
			name: "heading_1",
			block: Block{
				Type: "heading_1",
				Heading1: &HeadingBlock{
					RichText: []RichText{{PlainText: "Main Heading"}},
				},
			},
			expected: "# Main Heading",
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
			name: "bulleted_list",
			block: Block{
				Type: "bulleted_list_item",
				BulletedListItem: &ListItemBlock{
					RichText: []RichText{{PlainText: "Item"}},
				},
			},
			expected: "- Item",
		},
		{
			name: "numbered_list",
			block: Block{
				Type: "numbered_list_item",
				NumberedListItem: &ListItemBlock{
					RichText: []RichText{{PlainText: "First"}},
				},
			},
			expected: "1. First",
		},
		{
			name: "todo unchecked",
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
			name: "todo checked",
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
			name: "code block",
			block: Block{
				Type: "code",
				Code: &CodeBlock{
					RichText: []RichText{{PlainText: "fmt.Println()"}},
					Language: "go",
				},
			},
			expected: "```go\nfmt.Println()\n```",
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
			name: "callout",
			block: Block{
				Type: "callout",
				Callout: &CalloutBlock{
					RichText: []RichText{{PlainText: "Important note"}},
				},
			},
			expected: "Important note",
		},
		{
			name: "toggle",
			block: Block{
				Type: "toggle",
				Toggle: &ToggleBlock{
					RichText: []RichText{{PlainText: "Toggle content"}},
				},
			},
			expected: "Toggle content",
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
		{
			name: "unknown block type",
			block: Block{
				Type: "unknown",
			},
			expected: "",
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

func TestGetPageTitle(t *testing.T) {
	tests := []struct {
		name       string
		properties Properties
		expected   string
	}{
		{
			name:       "no properties",
			properties: Properties{},
			expected:   "Untitled",
		},
		{
			name: "with title property",
			properties: Properties{
				"title": Property{
					Type:  "title",
					Title: MustMarshalRichText([]RichText{{PlainText: "My Page"}}),
				},
			},
			expected: "My Page",
		},
		{
			name: "no title property",
			properties: Properties{
				"Name": Property{
					Type:     "rich_text",
					RichText: MustMarshalRichText([]RichText{{PlainText: "Not a title"}}),
				},
			},
			expected: "Untitled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPageTitle(tt.properties)
			if result != tt.expected {
				t.Errorf("GetPageTitle() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetDatabaseTitle(t *testing.T) {
	tests := []struct {
		name     string
		database *Database
		expected string
	}{
		{
			name: "with title",
			database: &Database{
				Title: []RichText{{PlainText: "My Database"}},
			},
			expected: "My Database",
		},
		{
			name: "no title",
			database: &Database{
				Title: []RichText{},
			},
			expected: "Untitled Database",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetDatabaseTitle(tt.database)
			if result != tt.expected {
				t.Errorf("GetDatabaseTitle() = %q, want %q", result, tt.expected)
			}
		})
	}
}
