package testutil

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Span is a visual column byte range to highlight on a 0-based line.
// It is intentionally a self-contained data struct so internal/testutil does not
// import internal/serialize, internal/engine, or internal/pipeline.
type Span struct {
	Line       int    // 0-based line index matching Diffmantic HighlightSpan.Line
	StartCol   int    // 0-based byte offset within line
	EndCol     int    // 0-based byte offset within line
	Action     string // "delete", "insert", "update", "move", "move_update"
	ColorIndex int    // 0: default (<mov>/<mUpd>), 1: (<mov1>/<mUpd1>), 2: (<mov2>/<mUpd2>)
}

// Matches tag vocabulary tokens that must be escaped if found literally in source text.
// Matches either literal \<tag or literal <tag (or closing \</tag or </tag).
var tagVocabEscRegex = regexp.MustCompile(`(\\?)(</?(?:del|ins|upd|mov\d*|mUpd\d*)(?:[\s>]|$))`)

func escapeTagVocabulary(text string) string {
	return tagVocabEscRegex.ReplaceAllStringFunc(text, func(m string) string {
		if strings.HasPrefix(m, `\`) {
			// Literal backslash before tag vocabulary: double the backslash
			return `\` + m
		}
		// Literal tag vocabulary token: prefix with backslash
		return `\` + m
	})
}

func actionTagName(action string, colorIndex int) string {
	act := strings.ToLower(action)
	switch act {
	case "delete":
		return "del"
	case "insert":
		return "ins"
	case "update":
		return "upd"
	case "move":
		if colorIndex == 0 {
			return "mov"
		}
		return fmt.Sprintf("mov%d", colorIndex)
	case "move_update":
		if colorIndex == 0 {
			return "mUpd"
		}
		return fmt.Sprintf("mUpd%d", colorIndex)
	default:
		return act
	}
}

// FormatTagged renders the source string with highlight spans converted into inline semantic tags
// (<del>, <ins>, <upd>, <mov>, <mUpd>), while leaving unchanged context completely silent.
//
// Rules enforced:
//  1. Overlap: Spans on the same line must be strictly disjoint. Overlapping spans trigger a fatal error.
//  2. Merge: Adjacent spans on the same line are merged if and only if both Action and ColorIndex match.
//  3. Escaping: Literal source occurrences of tag vocabulary tokens (e.g. HTML/JSX <del>, <ins>) are
//     escaped as \<del>, \<ins>, etc.
func FormatTagged(t testing.TB, src string, spans []Span) string {
	t.Helper()

	lines := strings.Split(src, "\n")
	spansByLine := make(map[int][]Span)

	for _, s := range spans {
		if s.Line < 0 || s.Line >= len(lines) {
			t.Fatalf("testutil.FormatTagged: span line %d is out of range [0..%d)", s.Line, len(lines))
		}
		lineText := lines[s.Line]
		if s.StartCol < 0 || s.EndCol > len(lineText) || s.StartCol > s.EndCol {
			t.Fatalf("testutil.FormatTagged: span [%d..%d] on line %d is out of bounds [0..%d] (line text: %q)",
				s.StartCol, s.EndCol, s.Line, len(lineText), lineText)
		}
		spansByLine[s.Line] = append(spansByLine[s.Line], s)
	}

	outLines := make([]string, len(lines))

	for lineIdx, lineText := range lines {
		lineSpans := spansByLine[lineIdx]

		if len(lineSpans) == 0 {
			outLines[lineIdx] = escapeTagVocabulary(lineText)
			continue
		}

		// Sort spans by StartCol ascending, then EndCol ascending
		slices.SortFunc(lineSpans, func(a, b Span) int {
			return cmp.Or(
				cmp.Compare(a.StartCol, b.StartCol),
				cmp.Compare(a.EndCol, b.EndCol),
				cmp.Compare(a.Action, b.Action),
				cmp.Compare(a.ColorIndex, b.ColorIndex),
			)
		})

		// Validate disjointness and merge adjacent identical spans
		var merged []Span
		curr := lineSpans[0]

		for i := 1; i < len(lineSpans); i++ {
			next := lineSpans[i]
			if next.StartCol < curr.EndCol {
				t.Fatalf("testutil.FormatTagged: overlapping spans on line %d: [%d..%d %s:%d] and [%d..%d %s:%d] on %q",
					lineIdx, curr.StartCol, curr.EndCol, curr.Action, curr.ColorIndex,
					next.StartCol, next.EndCol, next.Action, next.ColorIndex, lineText)
			}
			if next.StartCol == curr.EndCol && next.Action == curr.Action && next.ColorIndex == curr.ColorIndex {
				// Merge adjacent identical spans
				curr.EndCol = next.EndCol
			} else {
				merged = append(merged, curr)
				curr = next
			}
		}
		merged = append(merged, curr)

		// Render the line
		var sb strings.Builder
		col := 0

		for _, sp := range merged {
			if sp.StartCol > col {
				sb.WriteString(escapeTagVocabulary(lineText[col:sp.StartCol]))
			}
			tag := actionTagName(sp.Action, sp.ColorIndex)
			sb.WriteString("<")
			sb.WriteString(tag)
			sb.WriteString(">")
			sb.WriteString(escapeTagVocabulary(lineText[sp.StartCol:sp.EndCol]))
			sb.WriteString("</")
			sb.WriteString(tag)
			sb.WriteString(">")
			col = sp.EndCol
		}

		if col < len(lineText) {
			sb.WriteString(escapeTagVocabulary(lineText[col:]))
		}

		outLines[lineIdx] = sb.String()
	}

	return strings.Join(outLines, "\n")
}

// DiffStrings produces a readable unified line diff between want and got.
func DiffStrings(want, got string) string {
	if want == got {
		return ""
	}
	wLines := strings.Split(want, "\n")
	gLines := strings.Split(got, "\n")

	var sb strings.Builder
	maxLen := max(len(wLines), len(gLines))

	for i := 0; i < maxLen; i++ {
		w := ""
		if i < len(wLines) {
			w = wLines[i]
		}
		g := ""
		if i < len(gLines) {
			g = gLines[i]
		}

		if w == g {
			fmt.Fprintf(&sb, "  %s\n", w)
		} else {
			if i < len(wLines) {
				fmt.Fprintf(&sb, "- %s\n", w)
			}
			if i < len(gLines) {
				fmt.Fprintf(&sb, "+ %s\n", g)
			}
		}
	}
	return sb.String()
}

// AssertTagged verifies that applying the provided highlight spans to src matches expected.
// If there is any discrepancy, it reports a line-by-line diff.
func AssertTagged(t testing.TB, src string, spans []Span, expected string) {
	t.Helper()
	actual := FormatTagged(t, src, spans)
	if expected != actual {
		diff := DiffStrings(expected, actual)
		t.Fatalf("testutil.AssertTagged mismatch (-want +got):\n%s", diff)
	}
}
