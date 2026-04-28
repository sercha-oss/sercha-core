package indexing

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/pipeline/stages/textfilter"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
)

func TestChunkerFactory_Create(t *testing.T) {
	factory := NewChunkerFactory()

	if factory.StageID() != ChunkerStageID {
		t.Errorf("expected stage ID %s, got %s", ChunkerStageID, factory.StageID())
	}

	desc := factory.Descriptor()
	if desc.Type != pipeline.StageTypeTransformer {
		t.Errorf("expected type Transformer, got %s", desc.Type)
	}
	if desc.InputShape != pipeline.ShapeContent {
		t.Errorf("expected input shape Content, got %s", desc.InputShape)
	}
	if desc.OutputShape != pipeline.ShapeChunk {
		t.Errorf("expected output shape Chunk, got %s", desc.OutputShape)
	}

	// Create with default config
	config := pipeline.StageConfig{StageID: ChunkerStageID, Enabled: true}
	stage, err := factory.Create(config, nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if stage == nil {
		t.Error("expected stage to be created")
	}
}

func TestChunkerStage_Process(t *testing.T) {
	factory := NewChunkerFactory()
	config := pipeline.StageConfig{
		StageID:    ChunkerStageID,
		Enabled:    true,
		Parameters: map[string]any{"chunk_size": float64(100), "chunk_overlap": float64(20)},
	}
	stage, _ := factory.Create(config, nil)

	tests := []struct {
		name        string
		input       *pipeline.IndexingInput
		minChunks   int
		expectError bool
	}{
		{
			name: "short content single chunk",
			input: &pipeline.IndexingInput{
				DocumentID: "doc-1",
				Content:    "This is a short piece of content.",
			},
			minChunks: 1,
		},
		{
			name: "long content multiple chunks",
			input: &pipeline.IndexingInput{
				DocumentID: "doc-2",
				Content:    strings.Repeat("This is a test sentence that will be chunked. ", 20),
			},
			minChunks: 2,
		},
		{
			name: "empty content",
			input: &pipeline.IndexingInput{
				DocumentID: "doc-3",
				Content:    "",
			},
			minChunks: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := stage.Process(context.Background(), tt.input)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err == nil {
				chunks, ok := result.([]*pipeline.Chunk)
				if !ok {
					t.Fatal("expected result to be []*pipeline.Chunk")
				}
				if len(chunks) < tt.minChunks {
					t.Errorf("expected at least %d chunks, got %d", tt.minChunks, len(chunks))
				}

				// Verify chunk properties
				for i, chunk := range chunks {
					if chunk.DocumentID != tt.input.DocumentID {
						t.Errorf("chunk %d: expected document ID %s, got %s", i, tt.input.DocumentID, chunk.DocumentID)
					}
					if chunk.Position != i {
						t.Errorf("chunk %d: expected position %d, got %d", i, i, chunk.Position)
					}
					if chunk.ID == "" {
						t.Errorf("chunk %d: expected non-empty ID", i)
					}
				}
			}
		})
	}
}

func TestChunkerStage_FiltersNonTextChunks(t *testing.T) {
	factory := NewChunkerFactory()
	config := pipeline.StageConfig{
		StageID:    ChunkerStageID,
		Enabled:    true,
		Parameters: map[string]any{"chunk_size": float64(100), "chunk_overlap": float64(10)},
	}
	stage, _ := factory.Create(config, nil)

	// Simulate a document that starts with valid markdown then has a base64 blob.
	// With chunk_size=100, the text portion and base64 portion will be in separate chunks.
	textPart := strings.Repeat("This is valid markdown content with spaces and normal text. ", 3)
	base64Part := strings.Repeat("eJztWG1vGjkQivWfmolXvJyJ1V8IzS9Sy9tokJ00qURMl4DTrz2nu0loY", 3)

	input := &pipeline.IndexingInput{
		DocumentID: "doc-mixed",
		SourceID:   "src-1",
		Content:    textPart + base64Part,
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	chunks, ok := result.([]*pipeline.Chunk)
	if !ok {
		t.Fatal("expected []*pipeline.Chunk")
	}

	// All returned chunks should be text content, not base64
	for i, chunk := range chunks {
		if textfilter.IsLikelyNonText(chunk.Content) {
			t.Errorf("chunk %d should have been filtered (non-text content): %q", i, chunk.Content[:min(60, len(chunk.Content))])
		}
	}

	// We should have fewer chunks than if no filtering occurred
	if len(chunks) == 0 {
		t.Error("expected at least one text chunk to survive filtering")
	}
}

func TestChunkerStage_AllNonTextContent(t *testing.T) {
	factory := NewChunkerFactory()
	config := pipeline.StageConfig{
		StageID:    ChunkerStageID,
		Enabled:    true,
		Parameters: map[string]any{"chunk_size": float64(100), "chunk_overlap": float64(10)},
	}
	stage, _ := factory.Create(config, nil)

	// Document where ALL content is base64 — every chunk window will be non-text.
	// Previously this would panic with index out of bounds on chunks[len(chunks)-1]
	// because no chunks were appended but the overlap logic still accessed the slice.
	base64Only := strings.Repeat("eJztWG1vGjkQivWfmolXvJyJ1V8IzS9Sy9tokJ00qURMl4DTrz2nu0loY", 5)

	input := &pipeline.IndexingInput{
		DocumentID: "doc-all-base64",
		SourceID:   "src-1",
		Content:    base64Only,
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process() should not panic or error, got: %v", err)
	}

	chunks, ok := result.([]*pipeline.Chunk)
	if !ok {
		t.Fatal("expected []*pipeline.Chunk")
	}

	// All content is non-text, so no chunks should survive
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for all-base64 content, got %d", len(chunks))
	}
}

func TestChunkerStage_NoInfiniteLoopOnMixedContent(t *testing.T) {
	factory := NewChunkerFactory()
	config := pipeline.StageConfig{
		StageID:    ChunkerStageID,
		Enabled:    true,
		Parameters: map[string]any{"chunk_size": float64(1024), "chunk_overlap": float64(100)},
	}
	stage, _ := factory.Create(config, nil)

	// Simulate a real .api.mdx file: normal frontmatter, then a large base64 blob
	// with a space early in the chunk window (triggers word-boundary adjustment).
	// Before the fix, this caused an infinite loop because:
	// 1. Text chunks are appended (last StartOffset far behind)
	// 2. Base64 chunk is skipped by isLikelyNonText
	// 3. Word boundary sets end close to offset
	// 4. offset = end - overlap regresses backward
	// 5. Guard only checks last appended chunk, doesn't catch the regression
	normalText := strings.Repeat("This is normal markdown documentation with proper spacing and words. ", 40) // ~2800 chars
	// Base64-like content with a space near the start (triggers word-boundary + overlap regression)
	base64Block := "api: " + strings.Repeat("eJztWG1vGjkQivWfmolXvJyJ1V8IzS9Sy9tokJ00qURMl4DTrz2nu0loY", 40) // ~2400 chars
	trailingText := "\n" + strings.Repeat("More normal text after the base64 block for testing. ", 20)

	input := &pipeline.IndexingInput{
		DocumentID: "doc-api-mdx",
		SourceID:   "src-docs",
		Content:    normalText + base64Block + trailingText,
	}

	type processResult struct {
		output any
		err    error
	}
	done := make(chan processResult, 1)
	go func() {
		out, err := stage.Process(context.Background(), input)
		done <- processResult{out, err}
	}()

	select {
	case res := <-done:
		if res.err != nil {
			t.Fatalf("Process() error = %v", res.err)
		}
		chunks := res.output.([]*pipeline.Chunk)
		if len(chunks) == 0 {
			t.Error("expected at least one chunk from the normal text portions")
		}
		for i, chunk := range chunks {
			if textfilter.IsLikelyNonText(chunk.Content) {
				t.Errorf("chunk %d contains non-text content that should have been filtered", i)
			}
		}
		t.Logf("completed with %d chunks (no infinite loop)", len(chunks))
	case <-time.After(3 * time.Second):
		t.Fatal("chunker stuck in infinite loop — did not complete within 3 seconds")
	}
}

// Headings emitted by the HTML/PDF/Notion normalisers split the document
// into one chunk per section, with the section heading prepended to each
// chunk's content so the embedder has topical context.
func TestChunkerStage_SplitsOnATXHeadings(t *testing.T) {
	factory := NewChunkerFactory()
	stage, _ := factory.Create(pipeline.StageConfig{
		StageID: ChunkerStageID,
		Enabled: true,
		// Big chunk size — sections fit in one window each.
		Parameters: map[string]any{"chunk_size": float64(4096), "chunk_overlap": float64(100)},
	}, nil)

	body1 := strings.Repeat("Auth flow body content. ", 30) // ~700 chars > MinSectionLength
	body2 := strings.Repeat("OAuth specifics body. ", 30)
	body3 := strings.Repeat("Token refresh body content. ", 30)

	input := &pipeline.IndexingInput{
		DocumentID: "doc-sections",
		SourceID:   "src-1",
		Content: "## Auth flow\n\n" + body1 +
			"\n\n## OAuth\n\n" + body2 +
			"\n\n## Token refresh\n\n" + body3,
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	chunks := result.([]*pipeline.Chunk)

	if len(chunks) != 3 {
		t.Fatalf("want 3 chunks (one per section), got %d", len(chunks))
	}

	wants := []string{
		"## Auth flow\n\nAuth flow body content.",
		"## OAuth\n\nOAuth specifics body.",
		"## Token refresh\n\nToken refresh body content.",
	}
	for i, want := range wants {
		if !strings.HasPrefix(chunks[i].Content, want) {
			t.Errorf("chunk %d: want prefix %q, got %q", i, want, chunks[i].Content[:min(80, len(chunks[i].Content))])
		}
	}

	for i, c := range chunks {
		if c.Position != i {
			t.Errorf("chunk %d: position = %d, want %d", i, c.Position, i)
		}
	}
}

// A section longer than chunk_size gets sub-chunked, and every sub-chunk
// keeps the section heading prepended so the embedder doesn't lose context.
func TestChunkerStage_LongSectionGetsSubChunkedWithHeadingPrepended(t *testing.T) {
	factory := NewChunkerFactory()
	stage, _ := factory.Create(pipeline.StageConfig{
		StageID:    ChunkerStageID,
		Enabled:    true,
		Parameters: map[string]any{"chunk_size": float64(300), "chunk_overlap": float64(30)},
	}, nil)

	longBody := strings.Repeat("Authentication flow detailed content with words. ", 50) // ~2400 chars
	input := &pipeline.IndexingInput{
		DocumentID: "doc-long-section",
		SourceID:   "src-1",
		Content:    "## Auth flow\n\n" + longBody,
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	chunks := result.([]*pipeline.Chunk)

	if len(chunks) < 2 {
		t.Fatalf("want at least 2 sub-chunks for a long section, got %d", len(chunks))
	}
	for i, c := range chunks {
		if !strings.HasPrefix(c.Content, "## Auth flow\n\n") {
			t.Errorf("sub-chunk %d missing heading prefix:\n%s", i, c.Content[:min(80, len(c.Content))])
		}
	}
}

// Tiny sections (e.g. the connector-emitted `# Title\n\nbody` prelude) get
// folded forward into the next section so we don't end up with a 10-char
// chunk that's just the title.
func TestChunkerStage_TinySectionsMergeForward(t *testing.T) {
	factory := NewChunkerFactory()
	stage, _ := factory.Create(pipeline.StageConfig{
		StageID:    ChunkerStageID,
		Enabled:    true,
		Parameters: map[string]any{"chunk_size": float64(4096), "chunk_overlap": float64(100)},
	}, nil)

	bigBody := strings.Repeat("body content for the auth section. ", 30) // > MinSectionLength
	input := &pipeline.IndexingInput{
		DocumentID: "doc-with-title",
		SourceID:   "src-1",
		// `# Title` is a tiny section (no body); should fold into Auth flow.
		Content: "# Title\n\n## Auth flow\n\n" + bigBody,
	}

	result, err := stage.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	chunks := result.([]*pipeline.Chunk)

	if len(chunks) != 1 {
		t.Fatalf("want 1 chunk after merging, got %d:\n%v", len(chunks), chunkContents(chunks))
	}
	if !strings.Contains(chunks[0].Content, "# Title") {
		t.Errorf("merged chunk missing title:\n%s", chunks[0].Content[:min(120, len(chunks[0].Content))])
	}
	if !strings.Contains(chunks[0].Content, "Auth flow") {
		t.Errorf("merged chunk missing Auth flow heading:\n%s", chunks[0].Content[:min(120, len(chunks[0].Content))])
	}
	if !strings.Contains(chunks[0].Content, "body content for the auth section") {
		t.Errorf("merged chunk missing body:\n%s", chunks[0].Content[:min(120, len(chunks[0].Content))])
	}
}

// Inside a fenced code block, lines starting with `#` are comments (Python,
// shell, CSS selectors, Go cgo directives) — not headings. The chunker must
// not split on them or it will produce nonsense sections from code samples.
func TestChunkerStage_DoesNotSplitInsideCodeFences(t *testing.T) {
	bigBody := strings.Repeat("real prose talking about the example. ", 20)
	codeBlock := "```python\n" +
		"# this is a Python comment, not an H1\n" +
		"# def foo():\n" +
		"#   return 1\n" +
		"```\n"

	// Newline before the fence so it opens on its own line — CommonMark
	// requires this, and every normaliser/connector that emits fences
	// (Notion, GitHub) follows that convention.
	sections := splitSections("## Real heading\n\n" + bigBody + "\n" + codeBlock + bigBody)

	// Exactly one section — `## Real heading`. The `#` lines inside the
	// fence must not have spawned additional sections.
	if len(sections) != 1 {
		t.Fatalf("want 1 section, got %d:\n%v", len(sections), sectionHeadings(sections))
	}
	if sections[0].heading != "## Real heading" {
		t.Errorf("section heading = %q, want %q", sections[0].heading, "## Real heading")
	}
	// The Python comments must end up in the section body (not stripped
	// or spawning new sections).
	if !strings.Contains(sections[0].body, "# this is a Python comment, not an H1") {
		t.Error("python comment body got dropped")
	}
}

func TestChunkerStage_InvalidInput(t *testing.T) {
	factory := NewChunkerFactory()
	stage, _ := factory.Create(pipeline.StageConfig{}, nil)

	// Test with invalid input type
	_, err := stage.Process(context.Background(), "invalid input")
	if err == nil {
		t.Error("expected error for invalid input type")
	}
}

func chunkContents(chunks []*pipeline.Chunk) []string {
	out := make([]string, len(chunks))
	for i, c := range chunks {
		end := len(c.Content)
		if end > 80 {
			end = 80
		}
		out[i] = c.Content[:end]
	}
	return out
}

func sectionHeadings(sections []section) []string {
	out := make([]string, len(sections))
	for i, s := range sections {
		out[i] = s.heading
	}
	return out
}
