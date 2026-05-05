package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/ratelimited"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure OpenAIEmbedding implements EmbeddingService
var _ driven.EmbeddingService = (*OpenAIEmbedding)(nil)

// OpenAIEmbedding implements EmbeddingService using OpenAI's embedding API.
type OpenAIEmbedding struct {
	apiKey     string
	model      string
	baseURL    string
	dimensions int
	client     *http.Client
	transport  *ratelimited.Transport
}

// OpenAIEmbeddingOption configures an OpenAIEmbedding at construction time.
// Use the With* functions to create options.
type OpenAIEmbeddingOption func(*OpenAIEmbedding)

// WithEmbeddingTPMLimit sets the tokens-per-minute budget for the embedding
// client's rate-limiter bucket. The default is read from OPENAI_TPM_LIMIT
// (200000 if unset).
func WithEmbeddingTPMLimit(tpm int64) OpenAIEmbeddingOption {
	return func(e *OpenAIEmbedding) {
		refillPerSec := float64(tpm) / 60.0
		bucket := ratelimited.NewBucket(tpm, refillPerSec)
		e.transport.Limiter = bucket
	}
}

// WithEmbeddingMaxRetries sets the maximum number of retry attempts for the
// embedding client. The default is read from OPENAI_MAX_RETRIES (5 if unset).
func WithEmbeddingMaxRetries(n int) OpenAIEmbeddingOption {
	return func(e *OpenAIEmbedding) {
		e.transport.MaxRetries = n
	}
}

// WithEmbeddingMaxRetryElapsed sets the maximum total elapsed time for
// retries. The default is read from OPENAI_MAX_RETRY_ELAPSED_SEC (60 if
// unset).
func WithEmbeddingMaxRetryElapsed(d time.Duration) OpenAIEmbeddingOption {
	return func(e *OpenAIEmbedding) {
		e.transport.MaxRetryElapsed = d
	}
}

// WithEmbeddingTransportSleep replaces the Transport's sleep function with fn.
// This is intended for tests that need to control or eliminate sleep delays
// without relying on real wall-clock time.
func WithEmbeddingTransportSleep(fn func(ctx context.Context, d time.Duration) error) OpenAIEmbeddingOption {
	return func(e *OpenAIEmbedding) {
		e.transport.Sleep = fn
	}
}

// Model dimensions for OpenAI embedding models.
var openAIModelDimensions = map[string]int{
	"text-embedding-3-small": 1536,
	"text-embedding-3-large": 3072,
	"text-embedding-ada-002": 1536,
}

// NewOpenAIEmbedding creates a new OpenAI embedding service.
//
// The constructor reads three env vars to configure the rate-limiter and retry
// behaviour:
//
//   - OPENAI_TPM_LIMIT (default 200000): initial token-per-minute budget
//   - OPENAI_MAX_RETRIES (default 5): max retry attempts on 429/5xx
//   - OPENAI_MAX_RETRY_ELAPSED_SEC (default 60): max total seconds spent retrying
//
// These defaults are conservative and work without tuning. Pass opts to
// override any of them programmatically (e.g. in tests).
//
// The public surface (Embed, EmbedQuery, Dimensions, Model, HealthCheck,
// Close) is unchanged from the previous version.
func NewOpenAIEmbedding(apiKey, model, baseURL string, opts ...OpenAIEmbeddingOption) (driven.EmbeddingService, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	if model == "" {
		model = "text-embedding-3-small"
	}

	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	dimensions, ok := openAIModelDimensions[model]
	if !ok {
		// Default to 1536 for unknown models
		dimensions = 1536
	}

	tpm := int64(getEnvIntAI("OPENAI_TPM_LIMIT", 200000))
	maxRetries := getEnvIntAI("OPENAI_MAX_RETRIES", 5)
	maxElapsedSec := getEnvIntAI("OPENAI_MAX_RETRY_ELAPSED_SEC", 60)

	refillPerSec := float64(tpm) / 60.0
	bucket := ratelimited.NewBucket(tpm, refillPerSec)

	transport := &ratelimited.Transport{
		Base:            http.DefaultTransport,
		Limiter:         bucket,
		Weight:          ratelimited.WeightFromHeader,
		ParseLimit:      parseOpenAILimits,
		MaxRetries:      maxRetries,
		MaxRetryElapsed: time.Duration(maxElapsedSec) * time.Second,
	}

	e := &OpenAIEmbedding{
		apiKey:     apiKey,
		model:      model,
		baseURL:    baseURL,
		dimensions: dimensions,
		transport:  transport,
		client: &http.Client{
			Timeout:   60 * time.Second,
			Transport: transport,
		},
	}

	for _, opt := range opts {
		opt(e)
	}

	// Ensure client transport stays in sync if opts replaced the transport fields.
	e.client.Transport = e.transport

	return e, nil
}

// embeddingRequest is the request body for OpenAI embedding API.
type embeddingRequest struct {
	Input          interface{} `json:"input"` // string or []string
	Model          string      `json:"model"`
	EncodingFormat string      `json:"encoding_format,omitempty"`
}

// embeddingResponse is the response from OpenAI embedding API.
type embeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// Embed generates embeddings for multiple texts.
func (e *OpenAIEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Estimate token weight: sum of character lengths / 4 (chars per token heuristic).
	var charTotal int
	for _, t := range texts {
		charTotal += len(t)
	}
	weight := int64(charTotal/4) + 1

	reqBody := embeddingRequest{
		Input:          texts,
		Model:          e.model,
		EncodingFormat: "float",
	}

	resp, err := e.doRequest(ctx, reqBody, weight)
	if err != nil {
		return nil, err
	}

	// Sort by index to ensure order matches input.
	embeddings := make([][]float32, len(texts))
	for _, d := range resp.Data {
		if d.Index < len(embeddings) {
			embeddings[d.Index] = d.Embedding
		}
	}

	return embeddings, nil
}

// EmbedQuery generates an embedding for a search query.
func (e *OpenAIEmbedding) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	embeddings, err := e.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned for query")
	}
	return embeddings[0], nil
}

// Dimensions returns the embedding dimension size.
func (e *OpenAIEmbedding) Dimensions() int {
	return e.dimensions
}

// Model returns the model name being used.
func (e *OpenAIEmbedding) Model() string {
	return e.model
}

// HealthCheck verifies the embedding service is available.
func (e *OpenAIEmbedding) HealthCheck(ctx context.Context) error {
	_, err := e.EmbedQuery(ctx, "health check")
	return err
}

// Close releases resources held by the embedding service.
func (e *OpenAIEmbedding) Close() error {
	e.client.CloseIdleConnections()
	return nil
}

// doRequest makes a request to the OpenAI embedding API. The weight parameter
// is set as the X-Sercha-Token-Weight header so the transport's Weight
// callback can acquire the correct budget before the attempt.
func (e *OpenAIEmbedding) doRequest(ctx context.Context, reqBody embeddingRequest, weight int64) (*embeddingResponse, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set(ratelimited.HeaderTokenWeight, strconv.FormatInt(weight, 10))

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if embResp.Error != nil {
		return nil, fmt.Errorf("OpenAI API error: %s (type: %s, code: %s)",
			embResp.Error.Message, embResp.Error.Type, embResp.Error.Code)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API returned status %d", resp.StatusCode)
	}

	return &embResp, nil
}

// getEnvIntAI reads an integer environment variable, returning defaultValue if
// the variable is absent or cannot be parsed. Scoped to the ai package so as
// not to conflict with config.getEnvInt.
func getEnvIntAI(key string, defaultValue int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return defaultValue
	}
	return n
}
