package notion

import (
	"encoding/json"
	"time"
)

// SearchResponse represents the response from Notion Search API.
type SearchResponse struct {
	Results    []SearchResult `json:"results"`
	HasMore    bool           `json:"has_more"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

// SearchResult represents a single search result (page or database).
type SearchResult struct {
	Object         string     `json:"object"` // "page" or "database"
	ID             string     `json:"id"`
	CreatedTime    time.Time  `json:"created_time"`
	LastEditedTime time.Time  `json:"last_edited_time"`
	Archived       bool       `json:"archived"`
	Parent         Parent     `json:"parent"`
	Properties     Properties `json:"properties,omitempty"`
	Title          []RichText `json:"title,omitempty"` // For databases
	URL            string     `json:"url"`
}

// Parent represents the parent of a page or database.
type Parent struct {
	Type       string `json:"type"` // "workspace", "page_id", "database_id"
	Workspace  bool   `json:"workspace,omitempty"`
	PageID     string `json:"page_id,omitempty"`
	DatabaseID string `json:"database_id,omitempty"`
}

// Properties represents page or database properties.
// This is a map where keys are property names and values are property objects.
type Properties map[string]Property

// Property represents a single property value or schema definition.
// Note: For database schema, type-specific fields are empty objects {}.
// For page values, they contain actual data (arrays, objects, primitives).
// We use json.RawMessage for fields that differ between schema and values.
type Property struct {
	ID             string          `json:"id"`
	Type           string          `json:"type"`
	Name           string          `json:"name,omitempty"` // Present in database schema
	Title          json.RawMessage `json:"title,omitempty"`
	RichText       json.RawMessage `json:"rich_text,omitempty"`
	Number         json.RawMessage `json:"number,omitempty"`
	Select         json.RawMessage `json:"select,omitempty"`
	MultiSelect    json.RawMessage `json:"multi_select,omitempty"`
	Date           json.RawMessage `json:"date,omitempty"`
	People         json.RawMessage `json:"people,omitempty"`
	Files          json.RawMessage `json:"files,omitempty"`
	Checkbox       json.RawMessage `json:"checkbox,omitempty"`
	URL            json.RawMessage `json:"url,omitempty"`
	Email          json.RawMessage `json:"email,omitempty"`
	PhoneNumber    json.RawMessage `json:"phone_number,omitempty"`
	Formula        json.RawMessage `json:"formula,omitempty"`
	Relation       json.RawMessage `json:"relation,omitempty"`
	Rollup         json.RawMessage `json:"rollup,omitempty"`
	CreatedTime    json.RawMessage `json:"created_time,omitempty"`
	CreatedBy      json.RawMessage `json:"created_by,omitempty"`
	LastEditedTime json.RawMessage `json:"last_edited_time,omitempty"`
	LastEditedBy   json.RawMessage `json:"last_edited_by,omitempty"`
}

// GetTitle extracts title RichText array from the property.
// Returns nil if the property is a schema definition (empty object) or not a title type.
func (p Property) GetTitle() []RichText {
	if len(p.Title) == 0 || string(p.Title) == "{}" || string(p.Title) == "null" {
		return nil
	}
	var result []RichText
	if err := json.Unmarshal(p.Title, &result); err != nil {
		return nil
	}
	return result
}

// GetRichText extracts rich_text array from the property.
// Returns nil if the property is a schema definition (empty object) or not a rich_text type.
func (p Property) GetRichText() []RichText {
	if len(p.RichText) == 0 || string(p.RichText) == "{}" || string(p.RichText) == "null" {
		return nil
	}
	var result []RichText
	if err := json.Unmarshal(p.RichText, &result); err != nil {
		return nil
	}
	return result
}

// GetNumber extracts number value from the property.
func (p Property) GetNumber() *float64 {
	if len(p.Number) == 0 || string(p.Number) == "{}" || string(p.Number) == "null" {
		return nil
	}
	var result float64
	if err := json.Unmarshal(p.Number, &result); err != nil {
		return nil
	}
	return &result
}

// GetSelect extracts select value from the property.
func (p Property) GetSelect() *SelectValue {
	if len(p.Select) == 0 || string(p.Select) == "{}" || string(p.Select) == "null" {
		return nil
	}
	var result SelectValue
	if err := json.Unmarshal(p.Select, &result); err != nil {
		return nil
	}
	return &result
}

// GetDate extracts date value from the property.
func (p Property) GetDate() *DateValue {
	if len(p.Date) == 0 || string(p.Date) == "{}" || string(p.Date) == "null" {
		return nil
	}
	var result DateValue
	if err := json.Unmarshal(p.Date, &result); err != nil {
		return nil
	}
	return &result
}

// GetCheckbox extracts checkbox value from the property.
func (p Property) GetCheckbox() *bool {
	if len(p.Checkbox) == 0 || string(p.Checkbox) == "{}" || string(p.Checkbox) == "null" {
		return nil
	}
	var result bool
	if err := json.Unmarshal(p.Checkbox, &result); err != nil {
		return nil
	}
	return &result
}

// GetString extracts a string value from json.RawMessage.
func (p Property) GetString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return ""
	}
	var result string
	if err := json.Unmarshal(raw, &result); err != nil {
		return ""
	}
	return result
}

// MustMarshalRichText converts a []RichText to json.RawMessage.
// Panics on error (for use in tests and static initialization).
func MustMarshalRichText(rt []RichText) json.RawMessage {
	data, err := json.Marshal(rt)
	if err != nil {
		panic(err)
	}
	return data
}

// MustMarshal converts any value to json.RawMessage.
// Panics on error (for use in tests and static initialization).
func MustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

// RichText represents rich text content.
type RichText struct {
	Type        string       `json:"type"` // "text", "mention", "equation"
	Text        *TextContent `json:"text,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
	PlainText   string       `json:"plain_text"`
	Href        string       `json:"href,omitempty"`
}

// TextContent represents text content.
type TextContent struct {
	Content string `json:"content"`
	Link    *Link  `json:"link,omitempty"`
}

// Link represents a link in rich text.
type Link struct {
	URL string `json:"url"`
}

// Annotations represents text formatting.
type Annotations struct {
	Bold          bool   `json:"bold"`
	Italic        bool   `json:"italic"`
	Strikethrough bool   `json:"strikethrough"`
	Underline     bool   `json:"underline"`
	Code          bool   `json:"code"`
	Color         string `json:"color"`
}

// SelectValue represents a select property value.
type SelectValue struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// DateValue represents a date property value.
type DateValue struct {
	Start    string `json:"start"`
	End      string `json:"end,omitempty"`
	TimeZone string `json:"time_zone,omitempty"`
}

// FileValue represents a file property value.
type FileValue struct {
	Name     string        `json:"name"`
	Type     string        `json:"type"` // "external" or "file"
	External *ExternalFile `json:"external,omitempty"`
	File     *InternalFile `json:"file,omitempty"`
}

// ExternalFile represents an external file reference.
type ExternalFile struct {
	URL string `json:"url"`
}

// InternalFile represents a Notion-hosted file.
type InternalFile struct {
	URL        string    `json:"url"`
	ExpiryTime time.Time `json:"expiry_time"`
}

// Page represents a Notion page.
type Page struct {
	Object         string     `json:"object"` // "page"
	ID             string     `json:"id"`
	CreatedTime    time.Time  `json:"created_time"`
	LastEditedTime time.Time  `json:"last_edited_time"`
	Archived       bool       `json:"archived"`
	Parent         Parent     `json:"parent"`
	Properties     Properties `json:"properties"`
	URL            string     `json:"url"`
}

// Database represents a Notion database.
type Database struct {
	Object         string                      `json:"object"` // "database"
	ID             string                      `json:"id"`
	CreatedTime    time.Time                   `json:"created_time"`
	LastEditedTime time.Time                   `json:"last_edited_time"`
	Title          []RichText                  `json:"title"`
	Description    []RichText                  `json:"description"`
	Properties     map[string]DatabaseProperty `json:"properties"`
	Parent         Parent                      `json:"parent"`
	Archived       bool                        `json:"archived"`
	URL            string                      `json:"url"`
}

// DatabaseProperty represents a database property definition.
type DatabaseProperty struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// BlocksResponse represents a list of blocks.
type BlocksResponse struct {
	Results    []Block `json:"results"`
	HasMore    bool    `json:"has_more"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

// Block represents a Notion block.
type Block struct {
	Object         string    `json:"object"` // "block"
	ID             string    `json:"id"`
	Type           string    `json:"type"`
	CreatedTime    time.Time `json:"created_time"`
	LastEditedTime time.Time `json:"last_edited_time"`
	Archived       bool      `json:"archived"`
	HasChildren    bool      `json:"has_children"`

	// Block type-specific content
	Paragraph        *ParagraphBlock     `json:"paragraph,omitempty"`
	Heading1         *HeadingBlock       `json:"heading_1,omitempty"`
	Heading2         *HeadingBlock       `json:"heading_2,omitempty"`
	Heading3         *HeadingBlock       `json:"heading_3,omitempty"`
	BulletedListItem *ListItemBlock      `json:"bulleted_list_item,omitempty"`
	NumberedListItem *ListItemBlock      `json:"numbered_list_item,omitempty"`
	ToDo             *ToDoBlock          `json:"to_do,omitempty"`
	Toggle           *ToggleBlock        `json:"toggle,omitempty"`
	Code             *CodeBlock          `json:"code,omitempty"`
	Quote            *QuoteBlock         `json:"quote,omitempty"`
	Callout          *CalloutBlock       `json:"callout,omitempty"`
	ChildPage        *ChildPageBlock     `json:"child_page,omitempty"`
	ChildDatabase    *ChildDatabaseBlock `json:"child_database,omitempty"`
}

// ParagraphBlock represents a paragraph block.
type ParagraphBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color"`
}

// HeadingBlock represents a heading block.
type HeadingBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color"`
}

// ListItemBlock represents a list item block.
type ListItemBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color"`
}

// ToDoBlock represents a to-do block.
type ToDoBlock struct {
	RichText []RichText `json:"rich_text"`
	Checked  bool       `json:"checked"`
	Color    string     `json:"color"`
}

// ToggleBlock represents a toggle block.
type ToggleBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color"`
}

// CodeBlock represents a code block.
type CodeBlock struct {
	RichText []RichText `json:"rich_text"`
	Language string     `json:"language"`
	Caption  []RichText `json:"caption,omitempty"`
}

// QuoteBlock represents a quote block.
type QuoteBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color"`
}

// CalloutBlock represents a callout block.
type CalloutBlock struct {
	RichText []RichText `json:"rich_text"`
	Icon     *Icon      `json:"icon,omitempty"`
	Color    string     `json:"color"`
}

// Icon represents an icon (emoji or external).
type Icon struct {
	Type     string        `json:"type"` // "emoji" or "external"
	Emoji    string        `json:"emoji,omitempty"`
	External *ExternalFile `json:"external,omitempty"`
}

// ChildPageBlock represents a child page block.
type ChildPageBlock struct {
	Title string `json:"title"`
}

// ChildDatabaseBlock represents a child database block.
type ChildDatabaseBlock struct {
	Title string `json:"title"`
}

// QueryDatabaseResponse represents the response from querying a database.
type QueryDatabaseResponse struct {
	Results    []Page `json:"results"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// User represents a Notion user.
type User struct {
	Object    string  `json:"object"` // "user"
	ID        string  `json:"id"`
	Type      string  `json:"type"` // "person" or "bot"
	Name      string  `json:"name,omitempty"`
	AvatarURL string  `json:"avatar_url,omitempty"`
	Person    *Person `json:"person,omitempty"`
	Bot       *Bot    `json:"bot,omitempty"`
}

// Person represents a person user.
type Person struct {
	Email string `json:"email"`
}

// Bot represents a bot user.
type Bot struct {
	Owner BotOwner `json:"owner"`
}

// BotOwner represents the owner of a bot.
type BotOwner struct {
	Type      string `json:"type"` // "workspace"
	Workspace bool   `json:"workspace"`
}

// UserResponse represents the /v1/users/me response.
type UserResponse struct {
	Object    string        `json:"object"` // "user"
	ID        string        `json:"id"`
	Type      string        `json:"type"` // "person" or "bot"
	Name      string        `json:"name,omitempty"`
	AvatarURL string        `json:"avatar_url,omitempty"`
	Person    *Person       `json:"person,omitempty"`
	Bot       *BotOwnerInfo `json:"bot,omitempty"`
}

// BotOwnerInfo represents bot owner information.
type BotOwnerInfo struct {
	Owner struct {
		Type      string `json:"type"`
		Workspace bool   `json:"workspace"`
	} `json:"owner"`
	WorkspaceName string `json:"workspace_name,omitempty"`
}

// TokenResponse represents the OAuth token exchange response.
type TokenResponse struct {
	AccessToken   string    `json:"access_token"`
	TokenType     string    `json:"token_type"`
	BotID         string    `json:"bot_id"`
	WorkspaceID   string    `json:"workspace_id,omitempty"`
	WorkspaceName string    `json:"workspace_name,omitempty"`
	WorkspaceIcon string    `json:"workspace_icon,omitempty"`
	Owner         *UserInfo `json:"owner,omitempty"`
}

// UserInfo represents user information in the OAuth response.
type UserInfo struct {
	Type      string `json:"type"` // "user"
	User      *User  `json:"user,omitempty"`
	Workspace bool   `json:"workspace,omitempty"`
}

// ErrorResponse represents a Notion API error.
type ErrorResponse struct {
	Object  string `json:"object"` // "error"
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
