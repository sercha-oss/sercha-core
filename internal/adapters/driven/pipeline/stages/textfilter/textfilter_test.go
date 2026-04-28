package textfilter

import (
	"strings"
	"testing"
)

func TestIsLikelyNonText(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "normal english text",
			content: "This is a normal piece of English text with spaces and punctuation. It should not be filtered out.",
			want:    false,
		},
		{
			name:    "markdown content",
			content: "# Heading\n\nThis is a paragraph with **bold** and *italic* text.\n\n- List item one\n- List item two\n",
			want:    false,
		},
		{
			name:    "JSON content",
			content: `{"name": "test", "value": 42, "nested": {"key": "value"}, "array": [1, 2, 3]}`,
			want:    false,
		},
		{
			name:    "code content",
			content: "func main() {\n\tfmt.Println(\"hello world\")\n\tfor i := 0; i < 10; i++ {\n\t\tfmt.Println(i)\n\t}\n}\n",
			want:    false,
		},
		{
			name:    "base64 encoded blob",
			content: "eJztWG1vGjkQ/ivWfmolXvJyJ1V8IzS9Sy9tokJ00qURMl4DTrz2nu0loYj/3hnbC4aF6vqlukqVomDGL/PMeOaZMass55YZUTqhVdbLBoZTxwklij8ToayjUlKcI1NtiNKqfdOv3JwwrRRnThtLXvVvr8gTX7ZISd28PaGW56875A4+/KZkqRRPnEjNqJzaTtbKDP+34tZd6HyZ9VYZrHRcORzSspSCec3dR4vQVpllc15QHO1i/hROIU4TVsOPSmvgESKB4RZkggywlEaX3DjBbdAvxrChqQxO+gsOEpa4",
			want:    true,
		},
		{
			name:    "compact encoded data no whitespace",
			content: strings.Repeat("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/", 3),
			want:    true,
		},
		{
			name:    "binary content with control chars",
			content: "normal start\x00\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\x0e\x0f" + strings.Repeat("\x00\x01binary", 20),
			want:    true,
		},
		{
			name:    "short content below threshold",
			content: "short",
			want:    false,
		},
		{
			name:    "empty content",
			content: "",
			want:    false,
		},
		{
			name:    "base64 blob at minimum detectable length",
			content: "eJztWG1vGjkQivWfmolXvJyJ1V8IzS9Sy9tokJ00qURMl4DTrz2nu0leJztWG1vGjkQ",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsLikelyNonText(tt.content)
			if got != tt.want {
				t.Errorf("IsLikelyNonText() = %v, want %v", got, tt.want)
			}
		})
	}
}

// IsLikelyNonTextWithMime should bypass the whitespace heuristic for known
// structured-text MIME types — a minified bundle or a packed package-lock.json
// has near-zero whitespace and would otherwise be flagged as binary.
func TestIsLikelyNonTextWithMime_KnownTextBypassesHeuristic(t *testing.T) {
	// 192 chars of valid minified-JSON-shape content with <2% whitespace.
	minifiedJSON := `{"a":1,"b":[2,3,4,5,6,7,8,9,10],"c":{"d":"e","f":"g","h":"i"},"j":"k","l":"m","n":"o","p":"q","r":"s","t":"u","v":"w","x":"y","z":"aa","bb":"cc","dd":"ee","ff":"gg"}`

	if !IsLikelyNonText(minifiedJSON) {
		t.Fatalf("test premise broken: minified JSON should trip the bare heuristic")
	}

	cases := []struct {
		mime string
		want bool
	}{
		{"application/json", false},
		{"application/json; charset=utf-8", false},
		{"text/json", false},
		{"application/javascript", false},
		{"text/javascript", false},
		{"application/x-javascript", false},
		{"text/css", false},
		{"application/xml", false},
		{"text/xml", false},
		{"application/vnd.api+json", false},
		{"image/svg+xml", false},
		{"APPLICATION/JSON", false},
		// Free-form text formats keep the heuristic — base64-zlib blobs in
		// .api.mdx files were the original bug it was written for.
		{"text/markdown", true},
		{"text/html", true},
		{"text/plain", true},
		{"", true},
		{"application/octet-stream", true},
	}

	for _, tc := range cases {
		t.Run(tc.mime, func(t *testing.T) {
			got := IsLikelyNonTextWithMime(minifiedJSON, tc.mime)
			if got != tc.want {
				t.Errorf("IsLikelyNonTextWithMime(_, %q) = %v, want %v", tc.mime, got, tc.want)
			}
		})
	}
}

// Genuinely binary content (>5% non-printable) is filtered regardless of MIME —
// the allowlist only bypasses the whitespace heuristic, not the binary check.
func TestIsLikelyNonTextWithMime_BinaryContentStillFiltered(t *testing.T) {
	binary := "normal start\x00\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\x0e\x0f" + strings.Repeat("\x00\x01binary", 20)

	if !IsLikelyNonTextWithMime(binary, "application/json") {
		t.Error("binary content with control chars should be filtered even with allowlisted MIME")
	}
}
