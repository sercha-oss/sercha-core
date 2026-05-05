package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/ratelimited"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure OpenAILLM implements LLMService
var _ driven.LLMService = (*OpenAILLM)(nil)

// OpenAILLM implements LLMService using OpenAI's chat completion API.
type OpenAILLM struct {
	apiKey    string
	model     string
	baseURL   string
	client    *http.Client
	transport *ratelimited.Transport
}

// OpenAILLMOption configures an OpenAILLM at construction time.
// Use the With* functions to create options.
type OpenAILLMOption func(*OpenAILLM)

// WithLLMTPMLimit sets the tokens-per-minute budget for the LLM client's
// rate-limiter bucket. The default is read from OPENAI_TPM_LIMIT (200000 if
// unset).
func WithLLMTPMLimit(tpm int64) OpenAILLMOption {
	return func(l *OpenAILLM) {
		refillPerSec := float64(tpm) / 60.0
		bucket := ratelimited.NewBucket(tpm, refillPerSec)
		l.transport.Limiter = bucket
	}
}

// WithLLMMaxRetries sets the maximum number of retry attempts for the LLM
// client. The default is read from OPENAI_MAX_RETRIES (5 if unset).
func WithLLMMaxRetries(n int) OpenAILLMOption {
	return func(l *OpenAILLM) {
		l.transport.MaxRetries = n
	}
}

// WithLLMMaxRetryElapsed sets the maximum total elapsed time for retries. The
// default is read from OPENAI_MAX_RETRY_ELAPSED_SEC (60 if unset).
func WithLLMMaxRetryElapsed(d time.Duration) OpenAILLMOption {
	return func(l *OpenAILLM) {
		l.transport.MaxRetryElapsed = d
	}
}

// WithLLMTransportSleep replaces the Transport's sleep function with fn.
// This is intended for tests that need to control or eliminate sleep delays
// without relying on real wall-clock time.
func WithLLMTransportSleep(fn func(ctx context.Context, d time.Duration) error) OpenAILLMOption {
	return func(l *OpenAILLM) {
		l.transport.Sleep = fn
	}
}

// NewOpenAILLM creates a new OpenAI LLM service.
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
// The public surface (Complete, Model, Ping, Close) is unchanged from the
// previous version.
func NewOpenAILLM(apiKey, model, baseURL string, opts ...OpenAILLMOption) (driven.LLMService, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	if model == "" {
		model = "gpt-4o"
	}

	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
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

	l := &OpenAILLM{
		apiKey:    apiKey,
		model:     model,
		baseURL:   baseURL,
		transport: transport,
		client: &http.Client{
			Timeout:   120 * time.Second,
			Transport: transport,
		},
	}

	for _, opt := range opts {
		opt(l)
	}

	// Ensure client transport stays in sync if opts replaced the transport fields.
	l.client.Transport = l.transport

	return l, nil
}

// chatCompletionRequest is the request body for OpenAI chat completion API.
type chatCompletionRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float64         `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

// chatMessage represents a message in the chat.
type chatMessage struct {
	Role    string `json:"role"` // "system", "user", or "assistant"
	Content string `json:"content"`
}

// responseFormat specifies the output format for structured responses.
type responseFormat struct {
	Type       string                `json:"type"` // "json_schema" for structured output
	JSONSchema *jsonSchemaDefinition `json:"json_schema,omitempty"`
}

// jsonSchemaDefinition wraps a JSON schema with name and strict mode.
type jsonSchemaDefinition struct {
	Name   string `json:"name"`
	Schema any    `json:"schema"`
	Strict bool   `json:"strict"`
}

// chatCompletionResponse is the response from OpenAI chat completion API.
type chatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
		Param   string `json:"param,omitempty"`
	} `json:"error,omitempty"`
}

// Complete sends a completion request to the LLM and returns the response.
func (l *OpenAILLM) Complete(ctx context.Context, req domain.CompletionRequest) (domain.CompletionResponse, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return domain.CompletionResponse{}, fmt.Errorf("invalid request: %w", err)
	}

	// Build messages
	messages := []chatMessage{}
	if req.SystemPrompt != "" {
		messages = append(messages, chatMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}
	messages = append(messages, chatMessage{
		Role:    "user",
		Content: req.UserPrompt,
	})

	// Build request body
	reqBody := chatCompletionRequest{
		Model:    l.model,
		Messages: messages,
	}

	// Add optional parameters
	if req.Temperature > 0 {
		reqBody.Temperature = req.Temperature
	}
	if req.MaxTokens > 0 {
		reqBody.MaxTokens = req.MaxTokens
	}

	// Add response format for structured output if schema is provided
	if req.ResponseSchema != nil {
		reqBody.ResponseFormat = &responseFormat{
			Type: "json_schema",
			JSONSchema: &jsonSchemaDefinition{
				Name:   "response",
				Schema: req.ResponseSchema,
				Strict: true,
			},
		}
	}

	// Estimate token weight: input tokens (chars/4) + max_tokens output estimate.
	var inputChars int
	for _, m := range messages {
		inputChars += len(m.Content)
	}
	weight := int64(inputChars/4) + 1
	if req.MaxTokens > 0 {
		weight += int64(req.MaxTokens)
	}

	resp, err := l.doRequest(ctx, reqBody, weight)
	if err != nil {
		return domain.CompletionResponse{}, err
	}

	// Extract content from first choice
	if len(resp.Choices) == 0 {
		return domain.CompletionResponse{}, fmt.Errorf("no choices returned from LLM")
	}

	return domain.CompletionResponse{
		Content: resp.Choices[0].Message.Content,
		Usage: domain.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

// Model returns the model name being used.
func (l *OpenAILLM) Model() string {
	return l.model
}

// Ping verifies the LLM service is reachable and properly configured.
func (l *OpenAILLM) Ping(ctx context.Context) error {
	req := domain.NewCompletionRequest("", "ping")
	req = req.WithMaxTokens(1)
	_, err := l.Complete(ctx, req)
	return err
}

// Close releases resources held by the LLM service.
func (l *OpenAILLM) Close() error {
	l.client.CloseIdleConnections()
	return nil
}

// doRequest makes a request to the OpenAI chat completion API. The weight
// parameter is set as the X-Sercha-Token-Weight header so the transport's
// Weight callback can acquire the correct budget before the attempt.
func (l *OpenAILLM) doRequest(ctx context.Context, reqBody chatCompletionRequest, weight int64) (*chatCompletionResponse, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", l.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.apiKey)
	req.Header.Set(ratelimited.HeaderTokenWeight, strconv.FormatInt(weight, 10))

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrServiceUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var chatResp chatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Handle API errors
	if chatResp.Error != nil {
		return nil, mapOpenAIError(chatResp.Error)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return &chatResp, nil
}

// mapOpenAIError maps OpenAI API errors to domain errors.
func mapOpenAIError(apiErr *struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
	Param   string `json:"param,omitempty"`
}) error {
	message := apiErr.Message
	errorType := apiErr.Type
	code := apiErr.Code

	// Map specific error types to domain errors
	switch {
	case code == "rate_limit_exceeded" || errorType == "rate_limit_error":
		return fmt.Errorf("%w: %s", domain.ErrRateLimitExceeded, message)
	case code == "context_length_exceeded" || strings.Contains(message, "maximum context length"):
		return fmt.Errorf("%w: %s", domain.ErrContextLengthExceeded, message)
	case code == "model_not_found" || code == "invalid_model" || errorType == "invalid_model_error":
		return fmt.Errorf("%w: %s", domain.ErrInvalidModel, message)
	case code == "invalid_api_key" || code == "invalid_request_error":
		return fmt.Errorf("OpenAI API error (%s): %s", code, message)
	default:
		return fmt.Errorf("%w: %s (type: %s, code: %s)", domain.ErrServiceUnavailable, message, errorType, code)
	}
}
