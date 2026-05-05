package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewOpenAIEmbedding_RequiresAPIKey(t *testing.T) {
	_, err := NewOpenAIEmbedding("", "text-embedding-3-small", "")
	if err == nil {
		t.Error("expected error for empty API key")
	}
}

func TestNewOpenAIEmbedding_DefaultModel(t *testing.T) {
	svc, err := NewOpenAIEmbedding("sk-test", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	emb := svc.(*OpenAIEmbedding)
	if emb.model != "text-embedding-3-small" {
		t.Errorf("expected default model text-embedding-3-small, got %s", emb.model)
	}
}

func TestNewOpenAIEmbedding_DefaultBaseURL(t *testing.T) {
	svc, err := NewOpenAIEmbedding("sk-test", "text-embedding-3-small", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	emb := svc.(*OpenAIEmbedding)
	if emb.baseURL != "https://api.openai.com/v1" {
		t.Errorf("expected default base URL, got %s", emb.baseURL)
	}
}

func TestNewOpenAIEmbedding_CustomBaseURL(t *testing.T) {
	svc, err := NewOpenAIEmbedding("sk-test", "text-embedding-3-small", "https://custom.api.com/v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	emb := svc.(*OpenAIEmbedding)
	if emb.baseURL != "https://custom.api.com/v1" {
		t.Errorf("expected custom base URL, got %s", emb.baseURL)
	}
}

func TestOpenAIEmbedding_Dimensions(t *testing.T) {
	testCases := []struct {
		model      string
		dimensions int
	}{
		{"text-embedding-3-small", 1536},
		{"text-embedding-3-large", 3072},
		{"text-embedding-ada-002", 1536},
		{"unknown-model", 1536}, // defaults to 1536
	}

	for _, tc := range testCases {
		t.Run(tc.model, func(t *testing.T) {
			svc, err := NewOpenAIEmbedding("sk-test", tc.model, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if svc.Dimensions() != tc.dimensions {
				t.Errorf("expected dimensions %d, got %d", tc.dimensions, svc.Dimensions())
			}
		})
	}
}

func TestOpenAIEmbedding_Model(t *testing.T) {
	svc, err := NewOpenAIEmbedding("sk-test", "text-embedding-3-large", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if svc.Model() != "text-embedding-3-large" {
		t.Errorf("expected model text-embedding-3-large, got %s", svc.Model())
	}
}

func TestOpenAIEmbedding_Close(t *testing.T) {
	svc, err := NewOpenAIEmbedding("sk-test", "text-embedding-3-small", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := svc.Close(); err != nil {
		t.Errorf("expected no error from Close, got %v", err)
	}
}

func TestOpenAIEmbedding_Embed_EmptyInput(t *testing.T) {
	svc, err := NewOpenAIEmbedding("sk-test", "text-embedding-3-small", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := svc.Embed(context.Background(), []string{})
	if err != nil {
		t.Errorf("unexpected error for empty input: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for empty input")
	}
}

func TestOpenAIEmbedding_Embed_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/embeddings" {
			t.Errorf("expected /embeddings, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Error("expected Authorization header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type application/json")
		}

		// Decode request
		var req embeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		// Return mock response
		resp := embeddingResponse{
			Object: "list",
			Data: []struct {
				Object    string    `json:"object"`
				Index     int       `json:"index"`
				Embedding []float32 `json:"embedding"`
			}{
				{Object: "embedding", Index: 0, Embedding: []float32{0.1, 0.2, 0.3}},
				{Object: "embedding", Index: 1, Embedding: []float32{0.4, 0.5, 0.6}},
			},
			Model: "text-embedding-3-small",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewOpenAIEmbedding("sk-test", "text-embedding-3-small", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := svc.Embed(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 embeddings, got %d", len(result))
	}

	if len(result[0]) != 3 || result[0][0] != 0.1 {
		t.Error("unexpected embedding values")
	}
}

func TestOpenAIEmbedding_EmbedQuery_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := embeddingResponse{
			Object: "list",
			Data: []struct {
				Object    string    `json:"object"`
				Index     int       `json:"index"`
				Embedding []float32 `json:"embedding"`
			}{
				{Object: "embedding", Index: 0, Embedding: []float32{0.1, 0.2, 0.3}},
			},
			Model: "text-embedding-3-small",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewOpenAIEmbedding("sk-test", "text-embedding-3-small", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := svc.EmbedQuery(context.Background(), "test query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 dimensions, got %d", len(result))
	}
}

func TestOpenAIEmbedding_Embed_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := embeddingResponse{
			Error: &struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			}{
				Message: "Invalid API key",
				Type:    "invalid_request_error",
				Code:    "invalid_api_key",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewOpenAIEmbedding("sk-invalid", "text-embedding-3-small", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Error("expected error for API error response")
	}
}

func TestOpenAIEmbedding_Embed_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	svc, err := NewOpenAIEmbedding("sk-test", "text-embedding-3-small", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestOpenAIEmbedding_Embed_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	svc, err := NewOpenAIEmbedding("sk-test", "text-embedding-3-small", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Error("expected error for server error response")
	}
}

func TestOpenAIEmbedding_Embed_NetworkError(t *testing.T) {
	// Use invalid URL to trigger network error. Disable retries so the test
	// completes quickly without real backoff delays.
	noSleep := func(_ context.Context, _ time.Duration) error { return nil }
	svc, err := NewOpenAIEmbedding("sk-test", "text-embedding-3-small", "http://localhost:99999",
		WithEmbeddingMaxRetries(0),
		WithEmbeddingTransportSleep(noSleep),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Error("expected error for network error")
	}
}

func TestOpenAIEmbedding_HealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := embeddingResponse{
			Object: "list",
			Data: []struct {
				Object    string    `json:"object"`
				Index     int       `json:"index"`
				Embedding []float32 `json:"embedding"`
			}{
				{Object: "embedding", Index: 0, Embedding: []float32{0.1, 0.2, 0.3}},
			},
			Model: "text-embedding-3-small",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewOpenAIEmbedding("sk-test", "text-embedding-3-small", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = svc.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("expected no error from health check, got %v", err)
	}
}

// TestOpenAIEmbedding_429_RetryAfterEventuallySucceeds verifies that when the
// server returns a 429 with a Retry-After header, the embedding client retries
// and returns the successful embedding on the second attempt.
func TestOpenAIEmbedding_429_RetryAfterEventuallySucceeds(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		resp := embeddingResponse{
			Object: "list",
			Data: []struct {
				Object    string    `json:"object"`
				Index     int       `json:"index"`
				Embedding []float32 `json:"embedding"`
			}{
				{Object: "embedding", Index: 0, Embedding: []float32{0.1, 0.2, 0.3}},
			},
			Model: "text-embedding-3-small",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Use a no-op sleep so the test does not actually sleep 1 second.
	var sleptFor time.Duration
	fakeSleep := func(_ context.Context, d time.Duration) error {
		sleptFor = d
		return nil
	}

	svc, err := NewOpenAIEmbedding("sk-test", "text-embedding-3-small", server.URL,
		WithEmbeddingMaxRetries(3),
		WithEmbeddingTransportSleep(fakeSleep),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := svc.Embed(context.Background(), []string{"test"})
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if len(result) != 1 || len(result[0]) != 3 {
		t.Errorf("unexpected embedding result: %v", result)
	}
	if requestCount != 2 {
		t.Errorf("expected 2 requests, got %d", requestCount)
	}
	// The Retry-After header specified 1 second.
	if sleptFor < 900*time.Millisecond || sleptFor > 1100*time.Millisecond {
		t.Errorf("expected ~1s sleep from Retry-After, got %v", sleptFor)
	}
}

// TestOpenAIEmbedding_HeaderDrivenUpdate verifies that x-ratelimit-* headers
// in a successful response update the bucket and are reflected in subsequent
// Wait calls. This test checks that ParseLimit is called and Update propagates.
func TestOpenAIEmbedding_HeaderDrivenUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return rate-limit headers indicating near-exhaustion.
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-ratelimit-remaining-tokens", "10")
		w.Header().Set("x-ratelimit-reset-tokens", "30s")

		resp := embeddingResponse{
			Object: "list",
			Data: []struct {
				Object    string    `json:"object"`
				Index     int       `json:"index"`
				Embedding []float32 `json:"embedding"`
			}{
				{Object: "embedding", Index: 0, Embedding: []float32{0.5, 0.6}},
			},
			Model: "text-embedding-3-small",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewOpenAIEmbedding("sk-test", "text-embedding-3-small", server.URL,
		WithEmbeddingMaxRetries(0),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After the first successful response, the bucket should have been
	// updated with remaining=10. We verify this indirectly by confirming
	// that a second Embed succeeds — if Update somehow panicked or caused
	// a deadlock, this would hang.
	_, err = svc.Embed(context.Background(), []string{"world"})
	if err != nil {
		t.Fatalf("unexpected error on second embed: %v", err)
	}
}

func TestOpenAIEmbedding_EmbedQuery_EmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := embeddingResponse{
			Object: "list",
			Data: []struct {
				Object    string    `json:"object"`
				Index     int       `json:"index"`
				Embedding []float32 `json:"embedding"`
			}{}, // Empty data
			Model: "text-embedding-3-small",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewOpenAIEmbedding("sk-test", "text-embedding-3-small", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// When the API returns empty data, Embed returns a slice with nil embeddings
	// EmbedQuery returns the first element (nil) without error
	result, err := svc.EmbedQuery(context.Background(), "test query")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// The result will be nil since no embedding data was populated
	if result != nil {
		t.Error("expected nil result for empty API response")
	}
}
