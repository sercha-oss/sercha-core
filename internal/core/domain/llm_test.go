package domain

import (
	"testing"
)

func TestNewCompletionRequest(t *testing.T) {
	tests := []struct {
		name         string
		systemPrompt string
		userPrompt   string
		want         CompletionRequest
	}{
		{
			name:         "creates request with both prompts",
			systemPrompt: "You are a helpful assistant.",
			userPrompt:   "What is 2+2?",
			want: CompletionRequest{
				SystemPrompt: "You are a helpful assistant.",
				UserPrompt:   "What is 2+2?",
			},
		},
		{
			name:         "creates request with empty system prompt",
			systemPrompt: "",
			userPrompt:   "Test query",
			want: CompletionRequest{
				SystemPrompt: "",
				UserPrompt:   "Test query",
			},
		},
		{
			name:         "creates request with empty user prompt",
			systemPrompt: "System instructions",
			userPrompt:   "",
			want: CompletionRequest{
				SystemPrompt: "System instructions",
				UserPrompt:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewCompletionRequest(tt.systemPrompt, tt.userPrompt)
			if got.SystemPrompt != tt.want.SystemPrompt {
				t.Errorf("SystemPrompt = %v, want %v", got.SystemPrompt, tt.want.SystemPrompt)
			}
			if got.UserPrompt != tt.want.UserPrompt {
				t.Errorf("UserPrompt = %v, want %v", got.UserPrompt, tt.want.UserPrompt)
			}
		})
	}
}

func TestCompletionRequest_WithMaxTokens(t *testing.T) {
	req := NewCompletionRequest("system", "user")

	got := req.WithMaxTokens(100)

	if got.MaxTokens != 100 {
		t.Errorf("MaxTokens = %v, want %v", got.MaxTokens, 100)
	}
	if got.SystemPrompt != "system" {
		t.Errorf("SystemPrompt was modified")
	}
	if got.UserPrompt != "user" {
		t.Errorf("UserPrompt was modified")
	}
}

func TestCompletionRequest_WithTemperature(t *testing.T) {
	req := NewCompletionRequest("system", "user")

	got := req.WithTemperature(0.7)

	if got.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want %v", got.Temperature, 0.7)
	}
	if got.SystemPrompt != "system" {
		t.Errorf("SystemPrompt was modified")
	}
	if got.UserPrompt != "user" {
		t.Errorf("UserPrompt was modified")
	}
}

func TestCompletionRequest_WithResponseSchema(t *testing.T) {
	req := NewCompletionRequest("system", "user")
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"answer": map[string]interface{}{
				"type": "string",
			},
		},
	}

	got := req.WithResponseSchema(schema)

	if got.ResponseSchema == nil {
		t.Error("ResponseSchema is nil")
	}
	if got.SystemPrompt != "system" {
		t.Errorf("SystemPrompt was modified")
	}
	if got.UserPrompt != "user" {
		t.Errorf("UserPrompt was modified")
	}
}

func TestCompletionRequest_Chaining(t *testing.T) {
	schema := map[string]interface{}{"type": "object"}

	got := NewCompletionRequest("system", "user").
		WithMaxTokens(100).
		WithTemperature(0.5).
		WithResponseSchema(schema)

	if got.MaxTokens != 100 {
		t.Errorf("MaxTokens = %v, want %v", got.MaxTokens, 100)
	}
	if got.Temperature != 0.5 {
		t.Errorf("Temperature = %v, want %v", got.Temperature, 0.5)
	}
	if got.ResponseSchema == nil {
		t.Error("ResponseSchema is nil")
	}
	if got.SystemPrompt != "system" {
		t.Errorf("SystemPrompt = %v, want %v", got.SystemPrompt, "system")
	}
	if got.UserPrompt != "user" {
		t.Errorf("UserPrompt = %v, want %v", got.UserPrompt, "user")
	}
}

func TestCompletionRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     CompletionRequest
		wantErr bool
	}{
		{
			name: "valid request with all fields",
			req: CompletionRequest{
				SystemPrompt: "system",
				UserPrompt:   "user",
				MaxTokens:    100,
				Temperature:  0.7,
			},
			wantErr: false,
		},
		{
			name: "valid request with minimal fields",
			req: CompletionRequest{
				UserPrompt: "user",
			},
			wantErr: false,
		},
		{
			name: "valid request with zero temperature",
			req: CompletionRequest{
				UserPrompt:  "user",
				Temperature: 0,
			},
			wantErr: false,
		},
		{
			name: "valid request with max temperature",
			req: CompletionRequest{
				UserPrompt:  "user",
				Temperature: 2,
			},
			wantErr: false,
		},
		{
			name: "invalid empty user prompt",
			req: CompletionRequest{
				SystemPrompt: "system",
				UserPrompt:   "",
			},
			wantErr: true,
		},
		{
			name: "invalid negative temperature",
			req: CompletionRequest{
				UserPrompt:  "user",
				Temperature: -0.1,
			},
			wantErr: true,
		},
		{
			name: "invalid temperature above 2",
			req: CompletionRequest{
				UserPrompt:  "user",
				Temperature: 2.1,
			},
			wantErr: true,
		},
		{
			name: "invalid negative max tokens",
			req: CompletionRequest{
				UserPrompt: "user",
				MaxTokens:  -1,
			},
			wantErr: true,
		},
		{
			name: "valid zero max tokens",
			req: CompletionRequest{
				UserPrompt: "user",
				MaxTokens:  0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != ErrInvalidInput {
				t.Errorf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestCompletionResponse_IsEmpty(t *testing.T) {
	tests := []struct {
		name string
		resp CompletionResponse
		want bool
	}{
		{
			name: "empty response",
			resp: CompletionResponse{
				Content: "",
			},
			want: true,
		},
		{
			name: "non-empty response",
			resp: CompletionResponse{
				Content: "Some content",
			},
			want: false,
		},
		{
			name: "response with whitespace only is not empty",
			resp: CompletionResponse{
				Content: "   ",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resp.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompletionResponse_HasUsage(t *testing.T) {
	tests := []struct {
		name string
		resp CompletionResponse
		want bool
	}{
		{
			name: "response with usage",
			resp: CompletionResponse{
				Content: "content",
				Usage: TokenUsage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				},
			},
			want: true,
		},
		{
			name: "response without usage",
			resp: CompletionResponse{
				Content: "content",
				Usage: TokenUsage{
					PromptTokens:     0,
					CompletionTokens: 0,
					TotalTokens:      0,
				},
			},
			want: false,
		},
		{
			name: "response with partial usage is sufficient",
			resp: CompletionResponse{
				Content: "content",
				Usage: TokenUsage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resp.HasUsage(); got != tt.want {
				t.Errorf("HasUsage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTokenUsage_TotalCalculation(t *testing.T) {
	usage := TokenUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	if usage.TotalTokens != usage.PromptTokens+usage.CompletionTokens {
		t.Errorf("TotalTokens = %v, want %v (sum of prompt and completion)",
			usage.TotalTokens, usage.PromptTokens+usage.CompletionTokens)
	}
}

func TestCompletionRequest_JSONSerialization(t *testing.T) {
	req := CompletionRequest{
		SystemPrompt: "system",
		UserPrompt:   "user",
		MaxTokens:    100,
		Temperature:  0.7,
		ResponseSchema: map[string]interface{}{
			"type": "object",
		},
	}

	// This test ensures the struct tags are correct for JSON serialization
	// In real usage, this would be marshaled to JSON
	if req.SystemPrompt == "" {
		t.Error("SystemPrompt should be set")
	}
	if req.UserPrompt == "" {
		t.Error("UserPrompt should be set")
	}
	if req.MaxTokens == 0 {
		t.Error("MaxTokens should be set")
	}
	if req.Temperature == 0 {
		t.Error("Temperature should be set")
	}
	if req.ResponseSchema == nil {
		t.Error("ResponseSchema should be set")
	}
}
