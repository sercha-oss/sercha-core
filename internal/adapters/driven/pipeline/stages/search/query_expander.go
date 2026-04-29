package search

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

const QueryExpanderStageID = "query-expander"

// minTermsForExpansion is the smallest term count that benefits from LLM
// query expansion. Short keyword queries ("annual report 2021") are
// already specific; paraphrases waste an LLM round trip and produce
// variants that retrieve roughly the same set anyway.
const minTermsForExpansion = 5

// queryExpansionSystemPrompt is the LLM system prompt for generating query variants.
const queryExpansionSystemPrompt = `You are a search query expansion assistant. Given a user's search query, generate 2-3 alternative search phrases that:
1. Use synonyms or related terminology
2. Rephrase the question in different ways
3. Focus on different aspects of the topic
4. Maintain the original search intent

Return ONLY a JSON array of strings. Do not include the original query.

Example:
Query: "kubernetes deployment strategies"
Response: ["k8s rollout methods", "container orchestration deployment patterns", "kubernetes blue-green canary deployment"]`

// QueryExpanderFactory creates query expander stages.
type QueryExpanderFactory struct {
	descriptor pipeline.StageDescriptor
}

// NewQueryExpanderFactory creates a new query expander factory.
func NewQueryExpanderFactory() *QueryExpanderFactory {
	return &QueryExpanderFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          QueryExpanderStageID,
			Name:        "Query Expander",
			Type:        pipeline.StageTypeExpander,
			InputShape:  pipeline.ShapeParsedQuery,
			OutputShape: pipeline.ShapeQuerySet,
			Cardinality: pipeline.CardinalityOneToMany,
			Capabilities: []pipeline.CapabilityRequirement{
				{Type: pipeline.CapabilityLLM, Mode: pipeline.CapabilityOptional},
			},
			Version: "1.0.0",
		},
	}
}

func (f *QueryExpanderFactory) StageID() string                            { return f.descriptor.ID }
func (f *QueryExpanderFactory) Descriptor() pipeline.StageDescriptor       { return f.descriptor }
func (f *QueryExpanderFactory) Validate(config pipeline.StageConfig) error { return nil }

func (f *QueryExpanderFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	// LLM is optional - if not available, we'll gracefully degrade
	var llmService driven.LLMService
	if llmInst, ok := capabilities.Get(pipeline.CapabilityLLM); ok {
		if llm, ok := llmInst.Instance.(driven.LLMService); ok {
			llmService = llm
		}
	}

	maxVariants := 3
	if v, ok := config.Parameters["max_variants"].(float64); ok {
		maxVariants = int(v)
	}

	return &QueryExpanderStage{
		descriptor:  f.descriptor,
		llm:         llmService,
		maxVariants: maxVariants,
	}, nil
}

// QueryExpanderStage expands a single query into multiple query variants using LLM.
// If LLM is unavailable, it gracefully degrades by returning just the original query.
type QueryExpanderStage struct {
	descriptor  pipeline.StageDescriptor
	llm         driven.LLMService
	maxVariants int
}

func (s *QueryExpanderStage) Descriptor() pipeline.StageDescriptor { return s.descriptor }

func (s *QueryExpanderStage) Process(ctx context.Context, input any) (any, error) {
	parsed, ok := input.(*pipeline.ParsedQuery)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected *pipeline.ParsedQuery"}
	}

	// Always start with the original query as the first variant
	variants := []*pipeline.ParsedQuery{parsed}

	// Skip expansion when the user's intent is unambiguous:
	//   - Short keyword queries (≤4 terms) are already specific. Paraphrases
	//     drag in adjacent terminology, dilute precision, and add a 1-3s
	//     LLM round-trip for no measurable recall gain.
	//   - Quoted phrase queries are explicit "find this exact text" intent.
	//     Paraphrasing them defeats the purpose.
	if s.shouldSkipExpansion(parsed) {
		slog.Debug("search.query_expander skipped",
			"phase", "expand_query",
			"reason", s.skipReason(parsed),
			"term_count", len(parsed.Terms),
			"phrase_count", len(parsed.Phrases),
		)
		return variants, nil
	}

	// If LLM is available, generate additional variants
	if s.llm != nil {
		expansions, err := s.generateVariants(ctx, parsed.Original)
		if err != nil {
			// Graceful degradation - return original query only
			return variants, nil
		}

		// Create ParsedQuery instances for each expansion
		for _, expansion := range expansions {
			variants = append(variants, &pipeline.ParsedQuery{
				Original:      expansion,
				Terms:         s.tokenize(expansion),
				Phrases:       []string{},
				SearchFilters: parsed.SearchFilters,
			})
		}
	}

	return variants, nil
}

// shouldSkipExpansion decides when LLM-based query expansion adds no value.
// Returns true for short keyword queries and any query containing quoted
// phrases (the user has expressed exact-text intent).
func (s *QueryExpanderStage) shouldSkipExpansion(parsed *pipeline.ParsedQuery) bool {
	if len(parsed.Phrases) > 0 {
		return true
	}
	if len(parsed.Terms) < minTermsForExpansion {
		return true
	}
	return false
}

// skipReason annotates the skip log line so future tuning can tell which
// branch fired without re-deriving it from the term/phrase counts.
func (s *QueryExpanderStage) skipReason(parsed *pipeline.ParsedQuery) string {
	if len(parsed.Phrases) > 0 {
		return "quoted_phrase_present"
	}
	return "term_count_below_minimum"
}

// generateVariants uses the LLM to generate query expansion variants.
func (s *QueryExpanderStage) generateVariants(ctx context.Context, query string) ([]string, error) {
	// Create JSON schema for structured output
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"queries": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
				"minItems": 2,
				"maxItems": s.maxVariants,
			},
		},
		"required":             []string{"queries"},
		"additionalProperties": false,
	}

	req := domain.NewCompletionRequest(queryExpansionSystemPrompt, query).
		WithTemperature(0.7).
		WithMaxTokens(200).
		WithResponseSchema(schema)

	resp, err := s.llm.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse the JSON response
	var result struct {
		Queries []string `json:"queries"`
	}
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		return nil, err
	}

	// Limit to maxVariants
	if len(result.Queries) > s.maxVariants {
		result.Queries = result.Queries[:s.maxVariants]
	}

	return result.Queries, nil
}

// tokenize is a simple tokenizer for breaking query text into terms.
func (s *QueryExpanderStage) tokenize(text string) []string {
	// Simple whitespace tokenization (matches query_parser behavior)
	words := strings.Fields(text)
	terms := make([]string, 0, len(words))
	for _, word := range words {
		// Remove common punctuation and convert to lowercase
		word = strings.ToLower(strings.Trim(word, ".,!?;:"))
		if word != "" {
			terms = append(terms, word)
		}
	}
	return terms
}

// Interface assertions
var (
	_ pipelineport.StageFactory = (*QueryExpanderFactory)(nil)
	_ pipelineport.Stage        = (*QueryExpanderStage)(nil)
)
