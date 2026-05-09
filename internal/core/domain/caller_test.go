package domain

import (
	"encoding/json"
	"testing"
)

func TestCallerSourceConstants(t *testing.T) {
	tests := []struct {
		name     string
		source   CallerSource
		expected string
	}{
		{
			name:     "CallerSourceDirect",
			source:   CallerSourceDirect,
			expected: "direct",
		},
		{
			name:     "CallerSourceMCP",
			source:   CallerSourceMCP,
			expected: "mcp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.source) != tt.expected {
				t.Errorf("got %q, want %q", string(tt.source), tt.expected)
			}
		})
	}
}

func TestCallerJSONMarshaling(t *testing.T) {
	tests := []struct {
		name    string
		caller  *Caller
		wantErr bool
		check   func(t *testing.T, data []byte)
	}{
		{
			name: "direct caller with user",
			caller: &Caller{
				Source: CallerSourceDirect,
				UserID: "user123",
			},
			wantErr: false,
			check: func(t *testing.T, data []byte) {
				var m map[string]interface{}
				if err := json.Unmarshal(data, &m); err != nil {
					t.Fatalf("unmarshal failed: %v", err)
				}
				if m["source"] != "direct" {
					t.Errorf("source: got %v, want %q", m["source"], "direct")
				}
				if m["user_id"] != "user123" {
					t.Errorf("user_id: got %v, want %q", m["user_id"], "user123")
				}
			},
		},
		{
			name: "mcp caller with empty user",
			caller: &Caller{
				Source: CallerSourceMCP,
				UserID: "",
			},
			wantErr: false,
			check: func(t *testing.T, data []byte) {
				var m map[string]interface{}
				if err := json.Unmarshal(data, &m); err != nil {
					t.Fatalf("unmarshal failed: %v", err)
				}
				if m["source"] != "mcp" {
					t.Errorf("source: got %v, want %q", m["source"], "mcp")
				}
				if m["user_id"] != "" {
					t.Errorf("user_id: got %v, want %q", m["user_id"], "")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.caller)
			if (err != nil) != tt.wantErr {
				t.Errorf("marshal error: got %v, want error=%v", err, tt.wantErr)
			}
			if err == nil && tt.check != nil {
				tt.check(t, data)
			}
		})
	}
}

func TestCallerJSONUnmarshaling(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Caller
		wantErr bool
	}{
		{
			name:  "valid direct caller",
			input: `{"source":"direct","user_id":"user456"}`,
			want: &Caller{
				Source: CallerSourceDirect,
				UserID: "user456",
			},
			wantErr: false,
		},
		{
			name:  "valid mcp caller",
			input: `{"source":"mcp","user_id":"user789"}`,
			want: &Caller{
				Source: CallerSourceMCP,
				UserID: "user789",
			},
			wantErr: false,
		},
		{
			name:    "invalid json",
			input:   `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Caller
			err := json.Unmarshal([]byte(tt.input), &c)
			if (err != nil) != tt.wantErr {
				t.Errorf("unmarshal error: got %v, want error=%v", err, tt.wantErr)
			}
			if err == nil {
				if c.Source != tt.want.Source {
					t.Errorf("source: got %q, want %q", c.Source, tt.want.Source)
				}
				if c.UserID != tt.want.UserID {
					t.Errorf("user_id: got %q, want %q", c.UserID, tt.want.UserID)
				}
			}
		})
	}
}

func TestCallerRoundTrip(t *testing.T) {
	original := &Caller{
		Source: CallerSourceMCP,
		UserID: "user-uuid-123",
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Unmarshal
	var restored Caller
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if restored.Source != original.Source {
		t.Errorf("source: got %q, want %q", restored.Source, original.Source)
	}
	if restored.UserID != original.UserID {
		t.Errorf("user_id: got %q, want %q", restored.UserID, original.UserID)
	}
}

func TestCallerFieldZeroValues(t *testing.T) {
	c := &Caller{}

	if c.Source != "" {
		t.Errorf("zero Source: got %q, want empty string", c.Source)
	}
	if c.UserID != "" {
		t.Errorf("zero UserID: got %q, want empty string", c.UserID)
	}
}

func TestCallerEquality(t *testing.T) {
	tests := []struct {
		name  string
		a     *Caller
		b     *Caller
		equal bool
	}{
		{
			name:  "identical callers",
			a:     &Caller{Source: CallerSourceDirect, UserID: "user1"},
			b:     &Caller{Source: CallerSourceDirect, UserID: "user1"},
			equal: true,
		},
		{
			name:  "different source",
			a:     &Caller{Source: CallerSourceDirect, UserID: "user1"},
			b:     &Caller{Source: CallerSourceMCP, UserID: "user1"},
			equal: false,
		},
		{
			name:  "different user_id",
			a:     &Caller{Source: CallerSourceDirect, UserID: "user1"},
			b:     &Caller{Source: CallerSourceDirect, UserID: "user2"},
			equal: false,
		},
		{
			name:  "both empty",
			a:     &Caller{},
			b:     &Caller{},
			equal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			equals := (tt.a.Source == tt.b.Source && tt.a.UserID == tt.b.UserID)
			if equals != tt.equal {
				t.Errorf("equality: got %v, want %v", equals, tt.equal)
			}
		})
	}
}
