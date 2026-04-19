package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

func TestNewOpenAILLM_RequiresAPIKey(t *testing.T) {
	_, err := NewOpenAILLM("", "gpt-4o", "")
	if err == nil {
		t.Error("expected error for empty API key")
	}
}

func TestNewOpenAILLM_DefaultModel(t *testing.T) {
	svc, err := NewOpenAILLM("sk-test", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	llm := svc.(*OpenAILLM)
	if llm.model != "gpt-4o" {
		t.Errorf("expected default model gpt-4o, got %s", llm.model)
	}
}

func TestNewOpenAILLM_DefaultBaseURL(t *testing.T) {
	svc, err := NewOpenAILLM("sk-test", "gpt-4o", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	llm := svc.(*OpenAILLM)
	if llm.baseURL != "https://api.openai.com/v1" {
		t.Errorf("expected default base URL, got %s", llm.baseURL)
	}
}

func TestNewOpenAILLM_CustomBaseURL(t *testing.T) {
	svc, err := NewOpenAILLM("sk-test", "gpt-4o", "https://custom.api.com/v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	llm := svc.(*OpenAILLM)
	if llm.baseURL != "https://custom.api.com/v1" {
		t.Errorf("expected custom base URL, got %s", llm.baseURL)
	}
}

func TestOpenAILLM_Model(t *testing.T) {
	svc, err := NewOpenAILLM("sk-test", "gpt-4o-mini", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if svc.Model() != "gpt-4o-mini" {
		t.Errorf("expected model gpt-4o-mini, got %s", svc.Model())
	}
}

func TestOpenAILLM_Close(t *testing.T) {
	svc, err := NewOpenAILLM("sk-test", "gpt-4o", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := svc.Close(); err != nil {
		t.Errorf("expected no error from Close, got %v", err)
	}
}

func TestOpenAILLM_Complete_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected /chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Error("expected Authorization header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type application/json")
		}

		// Decode request
		var req chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		// Verify messages structure
		if len(req.Messages) < 1 {
			t.Error("expected at least one message")
		}

		// Return mock response
		resp := chatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4o",
			Choices: []struct {
				Index        int         `json:"index"`
				Message      chatMessage `json:"message"`
				FinishReason string      `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: chatMessage{
						Role:    "assistant",
						Content: "This is a test response from the LLM.",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewOpenAILLM("sk-test", "gpt-4o", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := domain.NewCompletionRequest("You are a helpful assistant.", "What is 2+2?")
	result, err := svc.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Content != "This is a test response from the LLM." {
		t.Errorf("unexpected content: %s", result.Content)
	}

	if result.Usage.PromptTokens != 10 {
		t.Errorf("expected 10 prompt tokens, got %d", result.Usage.PromptTokens)
	}
	if result.Usage.CompletionTokens != 20 {
		t.Errorf("expected 20 completion tokens, got %d", result.Usage.CompletionTokens)
	}
	if result.Usage.TotalTokens != 30 {
		t.Errorf("expected 30 total tokens, got %d", result.Usage.TotalTokens)
	}
}

func TestOpenAILLM_Complete_WithSystemPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify system message is included
		if len(req.Messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("expected first message to be system, got %s", req.Messages[0].Role)
		}
		if req.Messages[1].Role != "user" {
			t.Errorf("expected second message to be user, got %s", req.Messages[1].Role)
		}

		resp := chatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4o",
			Choices: []struct {
				Index        int         `json:"index"`
				Message      chatMessage `json:"message"`
				FinishReason string      `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: chatMessage{
						Role:    "assistant",
						Content: "Response",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewOpenAILLM("sk-test", "gpt-4o", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := domain.NewCompletionRequest("You are a helpful assistant.", "Test query")
	_, err = svc.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenAILLM_Complete_WithoutSystemPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify only user message is included
		if len(req.Messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "user" {
			t.Errorf("expected message to be user, got %s", req.Messages[0].Role)
		}

		resp := chatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4o",
			Choices: []struct {
				Index        int         `json:"index"`
				Message      chatMessage `json:"message"`
				FinishReason string      `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: chatMessage{
						Role:    "assistant",
						Content: "Response",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     5,
				CompletionTokens: 5,
				TotalTokens:      10,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewOpenAILLM("sk-test", "gpt-4o", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := domain.NewCompletionRequest("", "Test query")
	_, err = svc.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenAILLM_Complete_WithTemperature(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify temperature is set
		if req.Temperature != 0.7 {
			t.Errorf("expected temperature 0.7, got %f", req.Temperature)
		}

		resp := chatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4o",
			Choices: []struct {
				Index        int         `json:"index"`
				Message      chatMessage `json:"message"`
				FinishReason string      `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: chatMessage{
						Role:    "assistant",
						Content: "Response",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     5,
				CompletionTokens: 5,
				TotalTokens:      10,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewOpenAILLM("sk-test", "gpt-4o", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := domain.NewCompletionRequest("", "Test query").WithTemperature(0.7)
	_, err = svc.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenAILLM_Complete_WithMaxTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify max tokens is set
		if req.MaxTokens != 100 {
			t.Errorf("expected max tokens 100, got %d", req.MaxTokens)
		}

		resp := chatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4o",
			Choices: []struct {
				Index        int         `json:"index"`
				Message      chatMessage `json:"message"`
				FinishReason string      `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: chatMessage{
						Role:    "assistant",
						Content: "Response",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     5,
				CompletionTokens: 5,
				TotalTokens:      10,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewOpenAILLM("sk-test", "gpt-4o", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := domain.NewCompletionRequest("", "Test query").WithMaxTokens(100)
	_, err = svc.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenAILLM_Complete_WithResponseSchema(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"answer": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"answer"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify response format is set
		if req.ResponseFormat == nil {
			t.Error("expected response format to be set")
		}
		if req.ResponseFormat.Type != "json_schema" {
			t.Errorf("expected response format type json_schema, got %s", req.ResponseFormat.Type)
		}
		if req.ResponseFormat.JSONSchema == nil {
			t.Error("expected json schema to be set")
		}

		resp := chatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4o",
			Choices: []struct {
				Index        int         `json:"index"`
				Message      chatMessage `json:"message"`
				FinishReason string      `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: chatMessage{
						Role:    "assistant",
						Content: `{"answer": "4"}`,
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     5,
				CompletionTokens: 5,
				TotalTokens:      10,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewOpenAILLM("sk-test", "gpt-4o", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := domain.NewCompletionRequest("", "What is 2+2?").WithResponseSchema(schema)
	result, err := svc.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Content != `{"answer": "4"}` {
		t.Errorf("unexpected content: %s", result.Content)
	}
}

func TestOpenAILLM_Complete_InvalidRequest(t *testing.T) {
	svc, err := NewOpenAILLM("sk-test", "gpt-4o", "http://localhost:99999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty user prompt should fail validation
	req := domain.NewCompletionRequest("", "")
	_, err = svc.Complete(context.Background(), req)
	if err == nil {
		t.Error("expected error for empty user prompt")
	}
}

func TestOpenAILLM_Complete_RateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatCompletionResponse{
			Error: &struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
				Param   string `json:"param,omitempty"`
			}{
				Message: "Rate limit exceeded",
				Type:    "rate_limit_error",
				Code:    "rate_limit_exceeded",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewOpenAILLM("sk-test", "gpt-4o", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := domain.NewCompletionRequest("", "Test")
	_, err = svc.Complete(context.Background(), req)
	if err == nil {
		t.Error("expected error for rate limit")
	}
	// Verify it's the right domain error
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("expected rate limit error, got: %v", err)
	}
}

func TestOpenAILLM_Complete_ContextLengthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatCompletionResponse{
			Error: &struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
				Param   string `json:"param,omitempty"`
			}{
				Message: "This model's maximum context length is 4096 tokens",
				Type:    "invalid_request_error",
				Code:    "context_length_exceeded",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewOpenAILLM("sk-test", "gpt-4o", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := domain.NewCompletionRequest("", "Test")
	_, err = svc.Complete(context.Background(), req)
	if err == nil {
		t.Error("expected error for context length exceeded")
	}
	// Verify it's the right domain error
	if !strings.Contains(err.Error(), "context length") {
		t.Errorf("expected context length error, got: %v", err)
	}
}

func TestOpenAILLM_Complete_InvalidModelError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatCompletionResponse{
			Error: &struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
				Param   string `json:"param,omitempty"`
			}{
				Message: "The model 'invalid-model' does not exist",
				Type:    "invalid_model_error",
				Code:    "model_not_found",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewOpenAILLM("sk-test", "invalid-model", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := domain.NewCompletionRequest("", "Test")
	_, err = svc.Complete(context.Background(), req)
	if err == nil {
		t.Error("expected error for invalid model")
	}
	// Verify it's the right domain error
	if !strings.Contains(err.Error(), "invalid model") {
		t.Errorf("expected invalid model error, got: %v", err)
	}
}

func TestOpenAILLM_Complete_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": {"message": "internal error", "type": "server_error", "code": "server_error"}}`))
	}))
	defer server.Close()

	svc, err := NewOpenAILLM("sk-test", "gpt-4o", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := domain.NewCompletionRequest("", "Test")
	_, err = svc.Complete(context.Background(), req)
	if err == nil {
		t.Error("expected error for server error response")
	}
}

func TestOpenAILLM_Complete_NetworkError(t *testing.T) {
	// Use invalid URL to trigger network error
	svc, err := NewOpenAILLM("sk-test", "gpt-4o", "http://localhost:99999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := domain.NewCompletionRequest("", "Test")
	_, err = svc.Complete(context.Background(), req)
	if err == nil {
		t.Error("expected error for network error")
	}
}

func TestOpenAILLM_Complete_NoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4o",
			Choices: []struct {
				Index        int         `json:"index"`
				Message      chatMessage `json:"message"`
				FinishReason string      `json:"finish_reason"`
			}{}, // Empty choices
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     5,
				CompletionTokens: 0,
				TotalTokens:      5,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewOpenAILLM("sk-test", "gpt-4o", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := domain.NewCompletionRequest("", "Test")
	_, err = svc.Complete(context.Background(), req)
	if err == nil {
		t.Error("expected error for response with no choices")
	}
}

func TestOpenAILLM_Complete_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	svc, err := NewOpenAILLM("sk-test", "gpt-4o", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := domain.NewCompletionRequest("", "Test")
	_, err = svc.Complete(context.Background(), req)
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestOpenAILLM_Ping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4o",
			Choices: []struct {
				Index        int         `json:"index"`
				Message      chatMessage `json:"message"`
				FinishReason string      `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: chatMessage{
						Role:    "assistant",
						Content: "pong",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     1,
				CompletionTokens: 1,
				TotalTokens:      2,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewOpenAILLM("sk-test", "gpt-4o", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = svc.Ping(context.Background())
	if err != nil {
		t.Errorf("expected no error from Ping, got %v", err)
	}
}
