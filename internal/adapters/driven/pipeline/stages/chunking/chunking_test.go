package chunking

import (
	"strings"
	"testing"
)

func TestIsATXHeading(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"# Heading", true},
		{"## Sub", true},
		{"### Sub-sub", true},
		{"###### Six", true},
		{"####### Seven (too deep)", false},
		{"#NoSpace", false},
		{"#", false},
		{"# ", false}, // empty heading text
		{"  # Indented", true},
		{"\t# Tab-indented", true},
		{"text # not a heading", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			if got := IsATXHeading(tc.line); got != tc.want {
				t.Errorf("IsATXHeading(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

func TestSplitSections_NoHeadings(t *testing.T) {
	text := "just some plain prose with no markdown structure."
	sections := SplitSections(text)
	if len(sections) != 1 {
		t.Fatalf("want 1 fallback section, got %d", len(sections))
	}
	if sections[0].Heading != "" {
		t.Errorf("fallback heading = %q, want empty", sections[0].Heading)
	}
	if sections[0].Body != text {
		t.Errorf("fallback body = %q, want %q", sections[0].Body, text)
	}
}

func TestSplitSections_HeadingHierarchy(t *testing.T) {
	text := `# Title

intro paragraph

## Section A

body of A

### Section A.1

deeper body
`
	sections := SplitSections(text)
	if len(sections) != 3 {
		t.Fatalf("want 3 sections, got %d: %+v", len(sections), sections)
	}
	wantHeadings := []string{"# Title", "## Section A", "### Section A.1"}
	for i, want := range wantHeadings {
		if sections[i].Heading != want {
			t.Errorf("section[%d].Heading = %q, want %q", i, sections[i].Heading, want)
		}
	}
	if sections[0].Body != "intro paragraph" {
		t.Errorf("section[0].Body = %q", sections[0].Body)
	}
}

// Inside a fenced code block, lines starting with `#` are comments (Python,
// shell, CSS selectors, Go cgo directives) — not headings. SplitSections
// must not split on them or it will produce nonsense sections from code
// samples.
func TestSplitSections_DoesNotSplitInsideCodeFences(t *testing.T) {
	bigBody := strings.Repeat("real prose talking about the example. ", 20)
	codeBlock := "```python\n" +
		"# this is a Python comment, not an H1\n" +
		"# def foo():\n" +
		"#   return 1\n" +
		"```\n"

	// Newline before the fence so it opens on its own line — CommonMark
	// requires this, and every normaliser/connector that emits fences
	// (Notion, GitHub) follows that convention.
	sections := SplitSections("## Real heading\n\n" + bigBody + "\n" + codeBlock + bigBody)

	if len(sections) != 1 {
		t.Fatalf("want 1 section, got %d", len(sections))
	}
	if sections[0].Heading != "## Real heading" {
		t.Errorf("section heading = %q, want %q", sections[0].Heading, "## Real heading")
	}
	// The Python comments must end up in the section body (not stripped
	// or spawning new sections).
	if !strings.Contains(sections[0].Body, "# this is a Python comment, not an H1") {
		t.Error("python comment body got dropped")
	}
}

func TestMergeTinySections_FoldsTitleForward(t *testing.T) {
	// "# Title" with empty body is exactly what every connector emits as
	// the prelude — without merging, it ends up as a tiny chunk.
	sections := []Section{
		{Heading: "# Title", Body: ""},
		{Heading: "## Real section", Body: strings.Repeat("real content ", 50)}, // > MinSectionLength
	}
	merged := MergeTinySections(sections, MinSectionLength)
	if len(merged) != 1 {
		t.Fatalf("want 1 merged section, got %d", len(merged))
	}
	if !strings.Contains(merged[0].Body, "# Title") {
		t.Error("title heading lost during merge")
	}
	if !strings.Contains(merged[0].Body, "real content") {
		t.Error("real body lost during merge")
	}
}

func TestMergeTinySections_LeavesLargeSectionsAlone(t *testing.T) {
	body := strings.Repeat("plenty of body text here. ", 30) // > 256
	sections := []Section{
		{Heading: "## A", Body: body},
		{Heading: "## B", Body: body},
	}
	merged := MergeTinySections(sections, MinSectionLength)
	if len(merged) != 2 {
		t.Errorf("want 2 sections preserved, got %d", len(merged))
	}
}

func TestMergeTinySections_TrailingTinySectionsAppendAtEnd(t *testing.T) {
	bigBody := strings.Repeat("plenty of body text here. ", 30)
	sections := []Section{
		{Heading: "## A", Body: bigBody},
		{Heading: "## B", Body: "tiny"},
		{Heading: "## C", Body: "also tiny"},
	}
	merged := MergeTinySections(sections, MinSectionLength)
	// Trailing tiny sections fold into a final synthetic section rather
	// than being dropped.
	if len(merged) != 2 {
		t.Fatalf("want 2 final sections, got %d", len(merged))
	}
	if !strings.Contains(merged[1].Body, "tiny") || !strings.Contains(merged[1].Body, "also tiny") {
		t.Errorf("trailing tinies lost: %+v", merged[1])
	}
}

func TestMergeTinySections_SingleSectionPassesThrough(t *testing.T) {
	sections := []Section{{Heading: "# only", Body: "tiny"}}
	merged := MergeTinySections(sections, MinSectionLength)
	if len(merged) != 1 {
		t.Errorf("single section should pass through unchanged, got %d", len(merged))
	}
}
