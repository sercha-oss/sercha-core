// Package chunking provides shared text-splitting helpers for use by both
// indexing and search-side stages.
//
// The package is deliberately narrow: it exposes pure functions that turn
// a raw text body into a flat slice of {heading, body} sections, plus a
// small post-processing pass that folds tiny sections forward. Stage
// implementations layer their own size-based windowing and chunk-record
// construction on top.
package chunking

import "strings"

// MinSectionLength is the body-length threshold below which a section is
// merged forward into the next section by MergeTinySections. Connectors
// commonly emit `# Title\n\nbody` as the first lines of every document;
// without merging, "Title" becomes a chunk of its own — embedding-cheap
// and signal-weak.
const MinSectionLength = 256

// Section is one record produced by SplitSections — a heading line (or "")
// plus the body text up to the next heading.
type Section struct {
	Heading string // e.g. "## Auth flow", or "" for pre-heading prelude
	Body    string
}

// SplitSections walks the text line by line, tracking fenced-code state,
// and produces a flat slice of {heading, body} records. If no headings are
// found outside fenced blocks, returns a single record with an empty
// heading and the entire text as body (signal to fall back to size-based
// chunking).
//
// Markdown ATX headings only — fenced code lines starting with `#` are NOT
// treated as headings. Comments inside Python/Go/shell code blocks routinely
// look like `# something` and would otherwise produce spurious sections.
func SplitSections(text string) []Section {
	lines := strings.Split(text, "\n")
	var sections []Section
	var currentHeading string
	var currentBody strings.Builder
	inFence := false

	flush := func() {
		body := strings.TrimSpace(currentBody.String())
		if currentHeading == "" && body == "" {
			return
		}
		sections = append(sections, Section{Heading: currentHeading, Body: body})
		currentBody.Reset()
	}

	for _, line := range lines {
		// Toggle fenced-code state on lines that start with ``` (allowing
		// optional language suffix). We don't try to be clever about
		// indented/tilde fences — if a corpus needs them, extend here.
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "```") {
			inFence = !inFence
			currentBody.WriteString(line)
			currentBody.WriteByte('\n')
			continue
		}

		if !inFence && IsATXHeading(line) {
			flush()
			currentHeading = strings.TrimSpace(line)
			continue
		}

		currentBody.WriteString(line)
		currentBody.WriteByte('\n')
	}
	flush()

	if len(sections) == 0 {
		return []Section{{Heading: "", Body: strings.TrimSpace(text)}}
	}
	return sections
}

// IsATXHeading reports whether a line is an ATX-style markdown heading
// (`#`..`######` followed by a space and at least one non-space character).
// Lines like `#foo` (no space) or `#` alone are rejected so we don't mistake
// hashtag-style content or empty lines for headings.
func IsATXHeading(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, "#") {
		return false
	}
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 {
		return false
	}
	if level >= len(trimmed) || trimmed[level] != ' ' {
		return false
	}
	return strings.TrimSpace(trimmed[level+1:]) != ""
}

// MergeTinySections folds short sections forward into the next section's
// body. The merge preserves the heading line as the first line of the
// merged body so downstream consumers still see the title text — just
// attached to enough context to be useful.
//
// A section is "tiny" if its body is shorter than threshold.
func MergeTinySections(sections []Section, threshold int) []Section {
	if len(sections) <= 1 {
		return sections
	}

	out := make([]Section, 0, len(sections))
	var pending []Section // sections waiting to be folded into the next

	flushPending := func(target *Section) {
		if len(pending) == 0 {
			return
		}
		var b strings.Builder
		for _, p := range pending {
			if p.Heading != "" {
				b.WriteString(p.Heading)
				b.WriteString("\n\n")
			}
			if p.Body != "" {
				b.WriteString(p.Body)
				b.WriteString("\n\n")
			}
		}
		b.WriteString(target.Body)
		target.Body = strings.TrimSpace(b.String())
		pending = pending[:0]
	}

	for i := range sections {
		sec := sections[i]
		if len(sec.Body) < threshold && i < len(sections)-1 {
			// Not the last section — defer it.
			pending = append(pending, sec)
			continue
		}
		flushPending(&sec)
		out = append(out, sec)
	}

	// Anything still pending means the trailing sections were all tiny.
	// Append them as a final chunk rather than dropping them.
	if len(pending) > 0 {
		var b strings.Builder
		for i, p := range pending {
			if p.Heading != "" {
				b.WriteString(p.Heading)
				b.WriteString("\n\n")
			}
			if p.Body != "" {
				b.WriteString(p.Body)
				if i < len(pending)-1 {
					b.WriteString("\n\n")
				}
			}
		}
		body := strings.TrimSpace(b.String())
		if body != "" {
			out = append(out, Section{Heading: "", Body: body})
		}
	}
	return out
}
