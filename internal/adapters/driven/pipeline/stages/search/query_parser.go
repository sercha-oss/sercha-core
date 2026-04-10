package search

import (
	"context"
	"strings"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

const QueryParserStageID = "query-parser"

// QueryParserFactory creates query parser stages.
type QueryParserFactory struct {
	descriptor pipeline.StageDescriptor
}

// NewQueryParserFactory creates a new query parser factory.
func NewQueryParserFactory() *QueryParserFactory {
	return &QueryParserFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          QueryParserStageID,
			Name:        "Query Parser",
			Type:        pipeline.StageTypeParser,
			InputShape:  pipeline.ShapeQuery,
			OutputShape: pipeline.ShapeParsedQuery,
			Cardinality: pipeline.CardinalityOneToOne,
			Version:     "1.0.0",
		},
	}
}

// StageID returns the stage identifier.
func (f *QueryParserFactory) StageID() string {
	return f.descriptor.ID
}

// Descriptor returns the stage descriptor.
func (f *QueryParserFactory) Descriptor() pipeline.StageDescriptor {
	return f.descriptor
}

// Create creates a new query parser stage.
func (f *QueryParserFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	return &QueryParserStage{
		descriptor: f.descriptor,
	}, nil
}

// Validate validates the stage configuration.
func (f *QueryParserFactory) Validate(config pipeline.StageConfig) error {
	return nil
}

// QueryParserStage parses a raw query into structured form.
type QueryParserStage struct {
	descriptor pipeline.StageDescriptor
}

// Descriptor returns the stage descriptor.
func (s *QueryParserStage) Descriptor() pipeline.StageDescriptor {
	return s.descriptor
}

// Process parses the input query.
func (s *QueryParserStage) Process(ctx context.Context, input any) (any, error) {
	searchInput, ok := input.(*pipeline.SearchInput)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected *pipeline.SearchInput"}
	}

	parsed := s.parseQuery(searchInput.Query)
	parsed.SearchFilters = searchInput.Filters

	return parsed, nil
}

// parseQuery extracts terms, phrases, and filters from a query string.
func (s *QueryParserStage) parseQuery(query string) *pipeline.ParsedQuery {
	query = strings.TrimSpace(query)

	parsed := &pipeline.ParsedQuery{
		Original: query,
		Terms:    []string{},
		Phrases:  []string{},
		Filters:  []string{},
	}

	if len(query) == 0 {
		return parsed
	}

	// Extract quoted phrases
	var phrases []string
	inQuote := false
	var currentPhrase strings.Builder
	var remaining strings.Builder

	for _, char := range query {
		if char == '"' {
			if inQuote {
				phrase := strings.TrimSpace(currentPhrase.String())
				if len(phrase) > 0 {
					phrases = append(phrases, phrase)
				}
				currentPhrase.Reset()
			}
			inQuote = !inQuote
		} else if inQuote {
			currentPhrase.WriteRune(char)
		} else {
			remaining.WriteRune(char)
		}
	}

	parsed.Phrases = phrases

	// Extract filter expressions (field:value)
	remainingStr := remaining.String()
	words := strings.Fields(remainingStr)
	var terms []string

	for _, word := range words {
		if strings.Contains(word, ":") {
			parsed.Filters = append(parsed.Filters, word)
		} else {
			terms = append(terms, strings.ToLower(word))
		}
	}

	parsed.Terms = terms

	return parsed
}

// Ensure QueryParserFactory implements StageFactory.
var _ pipelineport.StageFactory = (*QueryParserFactory)(nil)

// Ensure QueryParserStage implements Stage.
var _ pipelineport.Stage = (*QueryParserStage)(nil)
