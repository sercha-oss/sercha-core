// Package entitydetect contains the LLM-driven entity-detection logic shared
// by the indexing-time entity-extractor stage and any future writer of the
// entity_register cache. The search-time entity-retriever is a pure cache
// reader and does not import this package.
package entitydetect

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// DetectorIdentity is the stable string written to EntitySpan.Detector for
// every span the LLM-backed extractor emits. It also forms part of the cache
// analyzer version so a version bump invalidates cached spans automatically.
//
// v2: added min length / min confidence filters and (type, start, end) dedup.
// v3: dropped confidence from prompt and schema (was theatre — LLM had no
//     calibrated way to assign it). Persisted spans no longer carry the field.
//     v3 also signals the iterative chunked extraction path; v2 cache rows
//     remain readable but won't be looked up under the v3 analyzer version.
const DetectorIdentity = "llm-entity-extractor-v3"

// MinSpanLength rejects spans whose value is too short to plausibly be a
// meaningful entity. Bare currency symbols ("£"), single letters, and stray
// punctuation are all dropped.
const MinSpanLength = 2

// chunkSizeBytes is the target size of each LLM call's text window. ~96K
// characters ≈ 24K tokens — well inside gpt-4o-mini's 128K context window
// even with the prompt overhead (system prompt + categories + extra
// context ≈ 1K tokens). Larger chunks mean fewer round-trips per doc:
// most prose docs fit in 1-2 chunks instead of 8-20, which dominates
// wall-clock since each call is ~1s of network+inference latency.
//
// Chunking is byte-based (not token-based) to keep the splitter
// dependency-free; the LLM tolerates mid-sentence boundaries provided
// the chunker overlaps consecutive windows.
const chunkSizeBytes = 96000

// chunkOverlapBytes is the number of bytes consecutive chunks share.
// Entities straddling a chunk boundary still appear fully in at least one
// chunk; without overlap a name split across two windows would never be
// detected. 1500 bytes scales the overlap with the larger chunk size and
// covers any plausible single-paragraph context (an entity in a
// long sentence might want a couple of lines of context to disambiguate).
const chunkOverlapBytes = 1500

// chunkMaxTokens caps the LLM response per chunk. With 96K-char chunks
// there is more potential output than before; 4096 covers a chunk full of
// hundreds of entities. Bigger than necessary for typical prose but cheap
// because completion tokens we don't use aren't billed.
const chunkMaxTokens = 4096

// systemPrompt instructs the LLM to identify named entities and return them
// as structured JSON. The list of active categories is injected at detection
// time so the prompt reflects the current taxonomy. The optional extra
// context section is appended verbatim — admins use it to bias detection
// toward their domain (e.g. a corpus of insurance case files where
// CL-NNNN-NNNN strings are claim numbers).
const systemPrompt = `You are a named-entity extractor for a privacy-protection system.

Find every occurrence in the text below that fits any of the listed categories.
Be inclusive: if something plausibly matches a category, emit it. A separate
post-processing step filters obvious noise — your job is recall, not precision.
When uncertain, prefer to emit.

For each match return:
  - "type":  the category id (use it verbatim from the list — uppercase, underscores)
  - "value": the exact substring as it appears in the text

Notes:
  - Only use category ids from the list below.
  - Don't invent values: each "value" must be an exact substring of the text.
  - The same value appearing multiple times → one entry per occurrence.
  - Different casings of the same name (e.g. "Pty Ltd" and "PTY LTD") are
    distinct — emit each one you encounter.
  - Empty result is fine if you genuinely find nothing.

Categories (id — what to look for — example):
%s
%s%s
Return ONLY valid JSON matching the schema. No prose, no markdown, no
explanation outside the JSON.`

// formatCategories renders the category list as one inline line per
// category: "id — description — e.g. example1, example2". Empty descriptions
// or examples are omitted (no orphaned dashes) so the prompt stays clean.
//
// Inline beats nested blocks: the previous nested-block format read as a
// rigid checklist to the model and produced empty results on noisy corpora
// where the right answer was obvious to a human. Single-line entries with
// "what to look for" framing reads as orientation rather than gating.
//
// Multiple examples can be supplied in the registry's `example` field by
// separating with `; ` or `|` — both render as a comma-separated list.
func formatCategories(categories []pipeline.EntityTypeMetadata) string {
	var b strings.Builder
	for _, c := range categories {
		b.WriteString("  - ")
		b.WriteString(string(c.ID))
		desc := strings.TrimSpace(c.Description)
		if desc != "" {
			b.WriteString(" — ")
			b.WriteString(desc)
		}
		ex := strings.TrimSpace(c.Example)
		if ex != "" {
			// Allow `;` or `|` to delimit multiple examples in one field
			// without a schema migration. They render as comma-separated
			// quoted strings.
			parts := splitExamples(ex)
			b.WriteString(" — e.g. ")
			for i, p := range parts {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteByte('"')
				b.WriteString(p)
				b.WriteByte('"')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// splitExamples breaks an example string on `;` or `|` and trims each part.
// Empty parts are dropped. Returns the original (trimmed) string in a single-
// element slice when no delimiter is present.
func splitExamples(s string) []string {
	delim := ";"
	if !strings.Contains(s, delim) {
		if strings.Contains(s, "|") {
			delim = "|"
		} else {
			return []string{strings.TrimSpace(s)}
		}
	}
	raw := strings.Split(s, delim)
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return []string{strings.TrimSpace(s)}
	}
	return out
}

// AnalyzerVersion derives an opaque version token from the detector identity,
// the category set (id + description + example, since all three are now sent
// to the LLM and influence its output), and the admin-supplied extra prompt
// context. Any of those changing invalidates cached entries because the
// cache key depends on this string.
//
// Both writer (indexing extractor) and reader (search retriever) call this
// with the same arguments to land on the same cache key.
func AnalyzerVersion(categories []pipeline.EntityTypeMetadata, extraContext string) string {
	h := sha256.New()
	h.Write([]byte(DetectorIdentity))
	h.Write([]byte{0})
	for _, c := range categories {
		h.Write([]byte(c.ID))
		h.Write([]byte{0})
		h.Write([]byte(c.Description))
		h.Write([]byte{0})
		h.Write([]byte(c.Example))
		h.Write([]byte{0})
	}
	h.Write([]byte(extraContext))
	return DetectorIdentity + ":" + hex.EncodeToString(h.Sum(nil))[:12]
}

// ContentSHA256 computes the cache content key for a piece of text.
func ContentSHA256(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// DetectChunked splits text into overlapping windows, calls the LLM once per
// window with the running set of previously-found entities as additional
// guidance, and returns the merged validated set. This is the public entry
// point used by the indexing stage.
//
// extraContext is appended to the system prompt for every chunk — admins use
// it to bias detection toward their domain (e.g. "this corpus contains
// insurance case files; CL-NNNN-NNNN strings are claim numbers").
//
// Why iterative with a running list:
//   - Big docs can't fit in a single LLM call's max output. Chunking keeps
//     each call's response well below MaxTokens.
//   - Showing the model what's already been found in earlier chunks
//     improves consistency: it tends to re-emit names in the casing it has
//     seen, and keeps category assignments stable across the document.
//   - Overlap protects against entities that straddle a chunk boundary —
//     they appear fully in at least one window.
//
// Returned spans are validated against the FULL input text (not per-chunk),
// so offsets are document-relative and dedup is global. Any error during a
// chunk's LLM call is logged-and-skipped: detection is best-effort, an LLM
// hiccup must not lose work already done by other chunks.
func DetectChunked(ctx context.Context, llm driven.LLMService, text string, categories []pipeline.EntityTypeMetadata, extraContext string) ([]pipeline.EntitySpan, error) {
	if text == "" {
		return []pipeline.EntitySpan{}, nil
	}

	docStart := time.Now()
	chunks := splitIntoChunks(text, chunkSizeBytes, chunkOverlapBytes)
	running := make([]pipeline.EntitySpan, 0, 64)
	var totalPromptTokens, totalCompletionTokens int

	for i, chunk := range chunks {
		if err := ctx.Err(); err != nil {
			return Validate(text, running), err
		}
		spans, usage, err := detectChunk(ctx, llm, chunk, categories, extraContext, running)
		totalPromptTokens += usage.PromptTokens
		totalCompletionTokens += usage.CompletionTokens
		if err != nil {
			slog.Warn("entitydetect: chunk failed",
				"chunk_index", i,
				"chunks_total", len(chunks),
				"chunk_bytes", len(chunk),
				"prompt_tokens_so_far", totalPromptTokens,
				"completion_tokens_so_far", totalCompletionTokens,
				"elapsed_ms", time.Since(docStart).Milliseconds(),
				"error", err,
			)
			return Validate(text, running), fmt.Errorf("detect chunk: %w", err)
		}
		running = append(running, spans...)
	}

	// Final validation against the WHOLE document so offsets are
	// document-relative and dedup is global.
	validated := Validate(text, running)
	slog.Info("entitydetect: doc complete",
		"doc_bytes", len(text),
		"chunks", len(chunks),
		"raw_spans", len(running),
		"validated_spans", len(validated),
		"prompt_tokens", totalPromptTokens,
		"completion_tokens", totalCompletionTokens,
		"total_tokens", totalPromptTokens+totalCompletionTokens,
		"elapsed_ms", time.Since(docStart).Milliseconds(),
	)
	return validated, nil
}

// detectChunk performs a single LLM call against one text window. The
// alreadyFound list is rendered into the prompt as a hint — the model is
// told what has already been emitted by earlier chunks so it can stay
// consistent (same casing, same category for the same value).
//
// Offsets in the returned spans are NOT yet document-relative — Validate()
// recomputes them against the full doc later. The function only enforces
// the "value is a substring of THIS chunk" hallucination guard.
func detectChunk(ctx context.Context, llm driven.LLMService, chunk string, categories []pipeline.EntityTypeMetadata, extraContext string, alreadyFound []pipeline.EntitySpan) ([]pipeline.EntitySpan, domain.TokenUsage, error) {
	catBlock := formatCategories(categories)
	contextBlock := ""
	if trimmed := strings.TrimSpace(extraContext); trimmed != "" {
		contextBlock = "\nAdditional context:\n" + trimmed + "\n"
	}
	foundBlock := formatAlreadyFound(alreadyFound)
	prompt := fmt.Sprintf(systemPrompt, catBlock, contextBlock, foundBlock)

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"entities": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"type":  map[string]any{"type": "string"},
						"value": map[string]any{"type": "string"},
					},
					"required":             []string{"type", "value"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"entities"},
		"additionalProperties": false,
	}

	req := domain.NewCompletionRequest(prompt, chunk).
		WithTemperature(0).
		WithMaxTokens(chunkMaxTokens).
		WithResponseSchema(schema)

	callStart := time.Now()
	resp, err := llm.Complete(ctx, req)
	callElapsed := time.Since(callStart)
	if err != nil {
		slog.Warn("entitydetect: llm.Complete failed",
			"chunk_bytes", len(chunk),
			"already_found_count", len(alreadyFound),
			"call_elapsed_ms", callElapsed.Milliseconds(),
			"error", err,
		)
		return nil, domain.TokenUsage{}, fmt.Errorf("llm complete: %w", err)
	}

	var parsed struct {
		Entities []struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"entities"`
	}
	if err := json.Unmarshal([]byte(resp.Content), &parsed); err != nil {
		return nil, resp.Usage, fmt.Errorf("parse response: %w", err)
	}

	out := make([]pipeline.EntitySpan, 0, len(parsed.Entities))
	for _, e := range parsed.Entities {
		// Per-chunk hallucination guard. Validate() repeats the check
		// against the full doc when computing final offsets.
		if !strings.Contains(chunk, e.Value) {
			continue
		}
		out = append(out, pipeline.EntitySpan{
			Type:     pipeline.EntityType(e.Type),
			Value:    e.Value,
			Detector: DetectorIdentity,
		})
	}

	slog.Info("entitydetect: chunk done",
		"chunk_bytes", len(chunk),
		"already_found_count", len(alreadyFound),
		"raw_spans", len(parsed.Entities),
		"survived_validation", len(out),
		"prompt_tokens", resp.Usage.PromptTokens,
		"completion_tokens", resp.Usage.CompletionTokens,
		"call_elapsed_ms", callElapsed.Milliseconds(),
	)
	return out, resp.Usage, nil
}

// formatAlreadyFound renders the running entity list as a compact hint
// block. Empty input returns "" so the prompt collapses cleanly when this
// is the first chunk. Duplicates by (type, value) are removed because the
// purpose is to inform the model, not to flood the prompt with repeats —
// downstream Validate() handles offset-level dedup separately.
func formatAlreadyFound(spans []pipeline.EntitySpan) string {
	if len(spans) == 0 {
		return ""
	}
	seen := make(map[string]struct{}, len(spans))
	var b strings.Builder
	b.WriteString("\nPreviously detected entities (re-emit them when they appear in the text below; you may also discover new ones):\n")
	for _, sp := range spans {
		key := string(sp.Type) + "|" + sp.Value
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		b.WriteString("  - ")
		b.WriteString(string(sp.Type))
		b.WriteString(": ")
		b.WriteString(sp.Value)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	return b.String()
}

// splitIntoChunks splits text into overlapping byte windows of approximately
// size with overlap bytes shared between consecutive chunks. The last chunk
// may be shorter than size. Returns a single-element slice when len(text)
// fits in one window.
//
// Splitting is byte-based and ignores rune/word boundaries — the LLM
// tolerates mid-token cuts because every entity worth catching also appears
// fully inside at least one window thanks to the overlap.
func splitIntoChunks(text string, size, overlap int) []string {
	if size <= 0 {
		size = chunkSizeBytes
	}
	if overlap < 0 || overlap >= size {
		overlap = chunkOverlapBytes
	}
	if len(text) <= size {
		return []string{text}
	}
	step := size - overlap
	chunks := make([]string, 0, (len(text)/step)+1)
	for start := 0; start < len(text); start += step {
		end := start + size
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, text[start:end])
		if end == len(text) {
			break
		}
	}
	return chunks
}

// Validate filters and dedupes a slice of raw spans against the source text:
//
//  1. Drops spans whose trimmed value is shorter than MinSpanLength.
//  2. Drops spans whose value is not a substring of the source text
//     (hallucination guard).
//  3. Recomputes Start/End server-side via strings.Index — the LLM's offsets
//     are unreliable.
//  4. Dedupes by (type, start, end).
//
// The returned slice is sorted by Start ascending.
func Validate(text string, spans []pipeline.EntitySpan) []pipeline.EntitySpan {
	out := make([]pipeline.EntitySpan, 0, len(spans))
	seen := make(map[string]struct{}, len(spans))
	for i := range spans {
		span := spans[i]
		if len(strings.TrimSpace(span.Value)) < MinSpanLength {
			continue
		}
		if !strings.Contains(text, span.Value) {
			continue
		}
		idx := strings.Index(text, span.Value)
		span.Start = idx
		span.End = idx + len(span.Value)

		key := fmt.Sprintf("%s|%d|%d", span.Type, span.Start, span.End)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, span)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}
