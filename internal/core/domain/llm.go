package domain

// CompletionRequest represents a request to an LLM for text completion.
// This is a pure domain type with no external dependencies.
type CompletionRequest struct {
	// SystemPrompt sets the system-level instructions for the LLM
	SystemPrompt string `json:"system_prompt"`

	// UserPrompt is the user's input or query to the LLM
	UserPrompt string `json:"user_prompt"`

	// MaxTokens limits the length of the generated response
	// If 0, the LLM provider's default is used
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature controls randomness in the response (0.0 to 2.0)
	// Lower values make output more focused and deterministic
	// Higher values make output more creative and random
	Temperature float64 `json:"temperature,omitempty"`

	// ResponseSchema is an optional JSON Schema that constrains the LLM output
	// to a specific structure. Used for structured data extraction.
	// The actual schema format depends on the LLM provider.
	ResponseSchema any `json:"response_schema,omitempty"`
}

// CompletionResponse represents the LLM's response to a completion request.
type CompletionResponse struct {
	// Content is the generated text from the LLM
	Content string `json:"content"`

	// Usage tracks token consumption for this request
	Usage TokenUsage `json:"usage"`
}

// TokenUsage tracks token consumption for an LLM request.
// This is useful for cost tracking and rate limiting.
type TokenUsage struct {
	// PromptTokens is the number of tokens in the input prompt
	PromptTokens int `json:"prompt_tokens"`

	// CompletionTokens is the number of tokens in the generated response
	CompletionTokens int `json:"completion_tokens"`

	// TotalTokens is the sum of prompt and completion tokens
	TotalTokens int `json:"total_tokens"`
}

// NewCompletionRequest creates a simple completion request with just prompts.
// Use the struct directly for more control over optional parameters.
func NewCompletionRequest(systemPrompt, userPrompt string) CompletionRequest {
	return CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}
}

// WithMaxTokens returns a new request with MaxTokens set.
func (r CompletionRequest) WithMaxTokens(maxTokens int) CompletionRequest {
	r.MaxTokens = maxTokens
	return r
}

// WithTemperature returns a new request with Temperature set.
func (r CompletionRequest) WithTemperature(temperature float64) CompletionRequest {
	r.Temperature = temperature
	return r
}

// WithResponseSchema returns a new request with a JSON Schema for structured output.
func (r CompletionRequest) WithResponseSchema(schema any) CompletionRequest {
	r.ResponseSchema = schema
	return r
}

// Validate checks if the completion request is valid.
func (r CompletionRequest) Validate() error {
	if r.UserPrompt == "" {
		return ErrInvalidInput
	}
	if r.Temperature < 0 || r.Temperature > 2 {
		return ErrInvalidInput
	}
	if r.MaxTokens < 0 {
		return ErrInvalidInput
	}
	return nil
}

// IsEmpty returns true if the response has no content.
func (r CompletionResponse) IsEmpty() bool {
	return r.Content == ""
}

// HasUsage returns true if token usage information is available.
func (r CompletionResponse) HasUsage() bool {
	return r.Usage.TotalTokens > 0
}
