package normalisers

import (
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Mock normaliser for testing
type mockNormaliser struct {
	name     string
	types    []string
	priority int
}

func (m *mockNormaliser) Normalise(content string, mimeType string) string {
	return content + "-" + m.name
}

func (m *mockNormaliser) SupportedTypes() []string {
	return m.types
}

func (m *mockNormaliser) Priority() int {
	return m.priority
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	mock := &mockNormaliser{name: "test", types: []string{"text/plain"}, priority: 50}

	r.Register(mock)

	types := r.List()
	if len(types) != 1 {
		t.Errorf("expected 1 type, got %d", len(types))
	}
	if types[0] != "text/plain" {
		t.Errorf("expected text/plain, got %s", types[0])
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	mock := &mockNormaliser{name: "test", types: []string{"text/plain"}, priority: 50}
	r.Register(mock)

	// Should find registered type
	n := r.Get("text/plain")
	if n == nil {
		t.Fatal("expected to find normaliser")
	}

	// Should not find unregistered type
	n = r.Get("application/json")
	if n != nil {
		t.Error("expected nil for unregistered type")
	}
}

func TestRegistry_Get_PrioritySelection(t *testing.T) {
	r := NewRegistry()

	lowPriority := &mockNormaliser{name: "low", types: []string{"text/plain"}, priority: 10}
	highPriority := &mockNormaliser{name: "high", types: []string{"text/plain"}, priority: 90}
	mediumPriority := &mockNormaliser{name: "medium", types: []string{"text/plain"}, priority: 50}

	// Register in random order
	r.Register(lowPriority)
	r.Register(highPriority)
	r.Register(mediumPriority)

	// Should return highest priority
	n := r.Get("text/plain")
	if n == nil {
		t.Fatal("expected to find normaliser")
	}

	result := n.Normalise("test", "text/plain")
	if result != "test-high" {
		t.Errorf("expected high priority normaliser, got %s", result)
	}
}

func TestRegistry_GetAll(t *testing.T) {
	r := NewRegistry()

	n1 := &mockNormaliser{name: "n1", types: []string{"text/plain"}, priority: 10}
	n2 := &mockNormaliser{name: "n2", types: []string{"text/plain"}, priority: 90}
	n3 := &mockNormaliser{name: "n3", types: []string{"text/html"}, priority: 50}

	r.Register(n1)
	r.Register(n2)
	r.Register(n3)

	// Should return 2 normalisers for text/plain, sorted by priority
	all := r.GetAll("text/plain")
	if len(all) != 2 {
		t.Fatalf("expected 2 normalisers, got %d", len(all))
	}

	// First should be highest priority
	if all[0].Priority() != 90 {
		t.Errorf("expected first priority 90, got %d", all[0].Priority())
	}
	if all[1].Priority() != 10 {
		t.Errorf("expected second priority 10, got %d", all[1].Priority())
	}

	// Should return 1 for text/html
	all = r.GetAll("text/html")
	if len(all) != 1 {
		t.Errorf("expected 1 normaliser for text/html, got %d", len(all))
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()

	r.Register(&mockNormaliser{name: "n1", types: []string{"text/plain", "text/csv"}, priority: 50})
	r.Register(&mockNormaliser{name: "n2", types: []string{"text/html"}, priority: 50})

	types := r.List()

	// Should have 3 unique types
	if len(types) != 3 {
		t.Errorf("expected 3 types, got %d", len(types))
	}

	// Should be sorted
	expected := []string{"text/csv", "text/html", "text/plain"}
	for i, exp := range expected {
		if types[i] != exp {
			t.Errorf("expected type %s at index %d, got %s", exp, i, types[i])
		}
	}
}

func TestRegistry_WildcardMatching(t *testing.T) {
	r := NewRegistry()

	// Register a wildcard normaliser
	wildcard := &mockNormaliser{name: "text-wildcard", types: []string{"text/*"}, priority: 20}
	specific := &mockNormaliser{name: "markdown", types: []string{"text/markdown"}, priority: 50}

	r.Register(wildcard)
	r.Register(specific)

	// text/markdown should match specific (higher priority)
	n := r.Get("text/markdown")
	if n == nil {
		t.Fatal("expected normaliser for text/markdown")
	}
	result := n.Normalise("test", "text/markdown")
	if result != "test-markdown" {
		t.Errorf("expected markdown normaliser, got %s", result)
	}

	// text/csv should match wildcard only
	n = r.Get("text/csv")
	if n == nil {
		t.Fatal("expected normaliser for text/csv")
	}
	result = n.Normalise("test", "text/csv")
	if result != "test-text-wildcard" {
		t.Errorf("expected text-wildcard normaliser, got %s", result)
	}
}

func TestRegistry_UniversalWildcard(t *testing.T) {
	r := NewRegistry()

	universal := &mockNormaliser{name: "universal", types: []string{"*/*"}, priority: 1}
	r.Register(universal)

	// Should match any type
	n := r.Get("application/octet-stream")
	if n == nil {
		t.Fatal("expected normaliser for any type")
	}
}

func TestMatchesMIMEType(t *testing.T) {
	tests := []struct {
		name      string
		supported []string
		mimeType  string
		expected  bool
	}{
		{"exact match", []string{"text/plain"}, "text/plain", true},
		{"case insensitive", []string{"TEXT/PLAIN"}, "text/plain", true},
		{"with charset", []string{"text/plain"}, "text/plain; charset=utf-8", true},
		{"wildcard subtype", []string{"text/*"}, "text/plain", true},
		{"wildcard subtype html", []string{"text/*"}, "text/html", true},
		{"wildcard no match", []string{"text/*"}, "application/json", false},
		{"universal wildcard", []string{"*/*"}, "anything/here", true},
		{"no match", []string{"text/plain"}, "text/html", false},
		{"multiple supported", []string{"text/plain", "text/html"}, "text/html", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesMIMEType(tt.supported, tt.mimeType)
			if result != tt.expected {
				t.Errorf("matchesMIMEType(%v, %s) = %v, want %v",
					tt.supported, tt.mimeType, result, tt.expected)
			}
		})
	}
}

func TestDefaultRegistry(t *testing.T) {
	r := DefaultRegistry()

	// Should have plaintext normaliser
	n := r.Get("text/plain")
	if n == nil {
		t.Error("expected plaintext normaliser")
	}

	// Should have markdown normaliser
	n = r.Get("text/markdown")
	if n == nil {
		t.Error("expected markdown normaliser")
	}

	// Should have HTML normaliser
	n = r.Get("text/html")
	if n == nil {
		t.Error("expected HTML normaliser")
	}

	// Plaintext should be fallback (priority 1)
	all := r.GetAll("text/plain")
	if len(all) == 0 {
		t.Fatal("expected at least one normaliser for text/plain")
	}
	// Should have the fallback with */* support
	foundFallback := false
	for _, norm := range all {
		if norm.Priority() == 1 {
			foundFallback = true
			break
		}
	}
	if !foundFallback {
		t.Error("expected to find fallback normaliser with priority 1")
	}
}

func TestPlaintextNormaliser(t *testing.T) {
	n := &PlaintextNormaliser{}

	// Test line ending normalization
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple text", "hello world", "hello world"},
		{"windows line endings", "hello\r\nworld", "hello\nworld"},
		{"old mac line endings", "hello\rworld", "hello\nworld"},
		{"mixed line endings", "a\r\nb\rc\n", "a\nb\nc"},
		{"trim whitespace", "  hello  ", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := n.Normalise(tt.input, "text/plain")
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}

	// Check supported types
	types := n.SupportedTypes()
	if len(types) != 2 {
		t.Errorf("expected 2 supported types, got %d", len(types))
	}

	// Check priority (should be 1 - fallback)
	if n.Priority() != 1 {
		t.Errorf("expected priority 1, got %d", n.Priority())
	}
}

func TestMarkdownNormaliser(t *testing.T) {
	n := &MarkdownNormaliser{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple markdown", "# Hello\nWorld", "# Hello\nWorld"},
		{"excessive blank lines", "a\n\n\n\nb", "a\n\nb"},
		{"windows line endings", "# Title\r\nContent", "# Title\nContent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := n.Normalise(tt.input, "text/markdown")
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}

	// Check priority (should be 50 - format-specific)
	if n.Priority() != 50 {
		t.Errorf("expected priority 50, got %d", n.Priority())
	}
}

func TestHTMLNormaliser(t *testing.T) {
	n := &HTMLNormaliser{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple html", "<p>Hello</p>", "Hello"},
		{"nested tags", "<div><p>Hello</p></div>", "Hello"},
		{"script removal", "<script>alert('x')</script>Text", "Text"},
		{"style removal", "<style>.a{}</style>Text", "Text"},
		{"entity decode", "&amp; &lt; &gt;", "& < >"},
		{"multiple spaces", "<p>Hello     World</p>", "Hello World"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := n.Normalise(tt.input, "text/html")
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}

	// Check priority (should be 50 - format-specific)
	if n.Priority() != 50 {
		t.Errorf("expected priority 50, got %d", n.Priority())
	}

	// Check supported types
	types := n.SupportedTypes()
	found := false
	for _, t := range types {
		if t == "text/html" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected text/html in supported types")
	}
}

func TestRemoveHTMLBlocks(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		tagName  string
		expected string
	}{
		{"script tag", "<script>code</script>text", "script", "text"},
		{"style tag", "<style>css</style>text", "style", "text"},
		{"multiple script tags", "<script>a</script>text<script>b</script>", "script", "text"},
		{"nested content", "before<script>alert();</script>after", "script", "beforeafter"},
		{"no matching tag", "<div>content</div>", "script", "<div>content</div>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeHTMLBlocks(tt.content, tt.tagName)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple tag", "<p>hello</p>", " hello "},
		{"nested tags", "<div><p>hello</p></div>", "  hello  "},
		{"self-closing", "hello<br/>world", "hello world"},
		{"attributes", "<a href='url'>link</a>", " link "},
		{"no tags", "plain text", "plain text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripHTMLTags(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestDecodeHTMLEntities(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"amp", "&amp;", "&"},
		{"lt gt", "&lt;&gt;", "<>"},
		{"nbsp", "&nbsp;", " "},
		{"quote", "&quot;", "\""},
		{"apos", "&apos;", "'"},
		{"multiple", "&amp; &lt;test&gt;", "& <test>"},
		{"no entities", "plain text", "plain text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := decodeHTMLEntities(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// Verify interface compliance
func TestInterfaceCompliance(t *testing.T) {
	var _ driven.NormaliserRegistry = (*Registry)(nil)
	var _ driven.Normaliser = (*PlaintextNormaliser)(nil)
	var _ driven.Normaliser = (*MarkdownNormaliser)(nil)
	var _ driven.Normaliser = (*HTMLNormaliser)(nil)
	var _ driven.Normaliser = (*GitHubIssueNormaliser)(nil)
	var _ driven.Normaliser = (*GitHubPRNormaliser)(nil)
}
