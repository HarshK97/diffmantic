package renderutil

import (
	"fmt"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestExtractDeclarationSignature(t *testing.T) {
	lines := []string{
		"func (h *Header) MarshalXML(e *xml.Encoder, start xml.StartElement) error {",
		"\treturn nil",
		"}",
	}
	sig := ExtractDeclarationSignature(&serialize.NodeRef{Type: "function_declaration"}, lines, 0, 2)
	if sig != "func (h *Header) MarshalXML(e *xml.Encoder, start xml.StartElement) error" {
		t.Errorf("unexpected signature: %q", sig)
	}

	// Test decorator and comment bypassing
	linesWithDecorator := []string{
		"// Header comment",
		"@dataclass",
		"@app.route(\"/api/v1\")",
		"def handle_request(req):",
		"\tpass",
	}
	sigDec := ExtractDeclarationSignature(&serialize.NodeRef{Type: "function_definition"}, linesWithDecorator, 0, 4)
	if sigDec != "def handle_request(req):" {
		t.Errorf("expected decorator bypass, got %q", sigDec)
	}

	longLine := "func VeryLongFunctionNameToTestTruncationBehaviorAcrossBoundaries(withManyArgumentsA string, withManyArgumentsB int) error {"
	longLines := []string{longLine}
	sigLong := ExtractDeclarationSignature(&serialize.NodeRef{Type: "function_declaration"}, longLines, 0, 0)
	if len(sigLong) > 80 {
		t.Errorf("expected signature length <= 80, got %d (%q)", len(sigLong), sigLong)
	}
	if !strings.HasSuffix(sigLong, "...") {
		t.Errorf("expected ellipsis suffix for long signature, got %q", sigLong)
	}

	// Fallback to node label / type
	nodeWithLabel := &serialize.NodeRef{Type: "unknown_type", Label: "CustomLabel"}
	if got := ExtractDeclarationSignature(nodeWithLabel, nil, -1, -1); got != "CustomLabel" {
		t.Errorf("expected label fallback, got %q", got)
	}
}

func TestBuildMoveBadges_Tier2Promotion(t *testing.T) {
	goRules := rules.Get("go")

	srcContent := "package main\n\nfunc OldFunc() {\n\tprintln(\"old\")\n}\n\nfunc Unchanged() {\n}\n"
	dstContent := "package main\n\nfunc Unchanged() {\n}\n\nfunc OldFunc() {\n\tprintln(\"old\")\n}\n"

	srcLines := strings.Split(srcContent, "\n")
	dstLines := strings.Split(dstContent, "\n")

	srcOffsets := serialize.BuildLineIndex([]byte(srcContent))
	dstOffsets := serialize.BuildLineIndex([]byte(dstContent))

	// OldFunc moved from line 2 to line 5
	sStartByte := uint32(srcOffsets[2])
	sEndByte := uint32(srcOffsets[5])
	dStartByte := uint32(dstOffsets[5])
	dEndByte := uint32(len(dstContent))

	actions := []serialize.Action{
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      "function_declaration",
				StartByte: sStartByte,
				EndByte:   sEndByte,
			},
			DestStartByte: &dStartByte,
			DestEndByte:   &dEndByte,
		},
	}

	hunks := []Interval{
		{Start: 0, End: 4}, // Source hunk covering lines 0..4
		{Start: 5, End: 9}, // Destination hunk covering lines 5..9
	}

	pairs := []serialize.LineAlignmentPair{
		{LeftLine: 0, RightLine: 0},
		{LeftLine: 1, RightLine: 1},
		{LeftLine: 2, RightLine: -1},
		{LeftLine: 3, RightLine: -1},
		{LeftLine: 4, RightLine: -1},
		{LeftLine: 5, RightLine: 2},
		{LeftLine: 6, RightLine: 3},
		{LeftLine: -1, RightLine: 5},
		{LeftLine: -1, RightLine: 6},
		{LeftLine: -1, RightLine: 7},
	}

	// 1. promoteDeclarations = true (Inline format: Tier 2 hunk headers, no line badges)
	inlineBadges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules, true)
	if len(inlineBadges.SrcLineBadges) != 0 {
		t.Errorf("inline mode expected 0 SrcLineBadges for declaration move, got %v", inlineBadges.SrcLineBadges)
	}
	if len(inlineBadges.DstLineBadges) != 0 {
		t.Errorf("inline mode expected 0 DstLineBadges for declaration move, got %v", inlineBadges.DstLineBadges)
	}
	if len(inlineBadges.HunkHeaders) == 0 {
		t.Fatalf("inline mode expected HunkHeaders to be populated")
	}
	if !strings.Contains(inlineBadges.HunkHeaders[0], "func OldFunc()") {
		t.Errorf("expected hunk 0 header to mention 'func OldFunc()', got: %q", inlineBadges.HunkHeaders[0])
	}

	// 2. promoteDeclarations = false (Side-by-Side format: Tier 3 line badges, no hunk headers)
	sbsBadges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules, false)
	if len(sbsBadges.HunkHeaders) != 0 {
		t.Errorf("SBS mode expected 0 HunkHeaders, got %v", sbsBadges.HunkHeaders)
	}
	if badge, ok := sbsBadges.SrcLineBadges[2]; !ok || !strings.Contains(badge, "➔ L6") {
		t.Errorf("SBS mode expected SrcLineBadges[2] to contain '➔ L6', got %q", badge)
	}
	if badge, ok := sbsBadges.DstLineBadges[5]; !ok || !strings.Contains(badge, "⤹ L3") {
		t.Errorf("SBS mode expected DstLineBadges[5] to contain '⤹ L3', got %q", badge)
	}
}

func TestBuildMoveBadges_SuppressedWhenOpeningIsDeletedOrInserted(t *testing.T) {
	tsxRules := rules.Get("tsx")

	srcContent := "export function Page() {\n  return (\n    <div className=\"grid\">\n      <div className=\"card\">\n        <Content />\n      </div>\n    </div>\n  )\n}\n"
	dstContent := "export function Page() {\n  return (\n    <main className=\"main\">\n      <div className=\"card\">\n        <Content />\n      </div>\n    </main>\n  )\n}\n"

	srcLines := strings.Split(srcContent, "\n")
	dstLines := strings.Split(dstContent, "\n")

	srcOffsets := serialize.BuildLineIndex([]byte(srcContent))
	dstOffsets := serialize.BuildLineIndex([]byte(dstContent))

	// Move action on outer container from line 2 to line 2 (e.g. div.grid -> main.main)
	sStartByte := uint32(srcOffsets[2])
	sEndByte := uint32(srcOffsets[6])
	dStartByte := uint32(dstOffsets[2])
	dEndByte := uint32(dstOffsets[6])

	// But opening element on line 2 in source was deleted, and opening element on line 2 in dest was inserted.
	actions := []serialize.Action{
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      "jsx_element",
				StartByte: sStartByte,
				EndByte:   sEndByte,
			},
			DestStartByte: &dStartByte,
			DestEndByte:   &dEndByte,
		},
		{
			Action: "delete",
			Node: &serialize.NodeRef{
				Type:      "jsx_opening_element",
				StartByte: sStartByte,
				EndByte:   sStartByte + 22,
			},
		},
		{
			Action: "insert",
			Node: &serialize.NodeRef{
				Type:      "jsx_opening_element",
				StartByte: dStartByte,
				EndByte:   dStartByte + 23,
			},
		},
	}

	hunks := []Interval{
		{Start: 0, End: 8},
	}

	pairs := []serialize.LineAlignmentPair{
		{LeftLine: 0, RightLine: 0},
		{LeftLine: 1, RightLine: 1},
		{LeftLine: 2, RightLine: -1},
		{LeftLine: -1, RightLine: 2},
		{LeftLine: 3, RightLine: 3},
		{LeftLine: 4, RightLine: 4},
		{LeftLine: 5, RightLine: 5},
		{LeftLine: 6, RightLine: 6},
		{LeftLine: 7, RightLine: 7},
	}

	badges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, tsxRules, false)

	// Since line 2's opening tag was deleted on source, no '➔ L3' should appear on line 2
	if badge, exists := badges.SrcLineBadges[2]; exists {
		t.Errorf("expected no SrcLineBadges on line 2 when opening is deleted, got %q", badge)
	}
	// Since line 2's opening tag was inserted on dest, no '⤹ L3' should appear on line 2
	if badge, exists := badges.DstLineBadges[2]; exists {
		t.Errorf("expected no DstLineBadges on line 2 when opening is inserted, got %q", badge)
	}
}

func TestBuildMoveBadges_HierarchicalSuppression(t *testing.T) {
	goRules := rules.Get("go")

	srcLines := make([]string, 15)
	for i := range srcLines {
		srcLines[i] = fmt.Sprintf("src line %d", i)
	}
	dstLines := make([]string, 30)
	for i := range dstLines {
		dstLines[i] = fmt.Sprintf("dst line %d", i)
	}

	srcOffsets := make([]int, len(srcLines)+1)
	for i := range srcLines {
		srcOffsets[i+1] = srcOffsets[i] + len(srcLines[i]) + 1
	}
	dstOffsets := make([]int, len(dstLines)+1)
	for i := range dstLines {
		dstOffsets[i+1] = dstOffsets[i] + len(dstLines[i]) + 1
	}

	// Line 2: parent if_statement (lines 2..6, bytes srcOffsets[2]..srcOffsets[7])
	// moves to destination lines 10..14 (bytes dstOffsets[10]..dstOffsets[15])
	pSrcStart := uint32(srcOffsets[2])
	pSrcEnd := uint32(srcOffsets[7])
	pDstStart := uint32(dstOffsets[10])
	pDstEnd := uint32(dstOffsets[15])

	// Line 3: child for_statement (lines 3..5, bytes srcOffsets[3]..srcOffsets[6])
	// moves to destination lines 11..13 (bytes dstOffsets[11]..dstOffsets[14]) -> strictly inside parent
	c1SrcStart := uint32(srcOffsets[3])
	c1SrcEnd := uint32(srcOffsets[6])
	c1DstStart := uint32(dstOffsets[11])
	c1DstEnd := uint32(dstOffsets[14])

	// Line 4: grandchild if_statement (line 4, bytes srcOffsets[4]..srcOffsets[5])
	// moves to destination line 12 (bytes dstOffsets[12]..dstOffsets[13]) -> strictly inside child & parent
	c2SrcStart := uint32(srcOffsets[4])
	c2SrcEnd := uint32(srcOffsets[5])
	c2DstStart := uint32(dstOffsets[12])
	c2DstEnd := uint32(dstOffsets[13])

	// Line 5: extracted statement (line 5, bytes srcOffsets[5]..srcOffsets[6])
	// was inside parent in source, but moved OUTSIDE parent to destination line 25 (bytes dstOffsets[25]..dstOffsets[26])
	extSrcStart := uint32(srcOffsets[5])
	extSrcEnd := uint32(srcOffsets[6])
	extDstStart := uint32(dstOffsets[25])
	extDstEnd := uint32(dstOffsets[26])

	actions := []serialize.Action{
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      "if_statement",
				StartByte: pSrcStart,
				EndByte:   pSrcEnd,
			},
			DestStartByte:     &pDstStart,
			DestEndByte:       &pDstEnd,
			OrigStartByte:     pSrcStart,
			OrigEndByte:       pSrcEnd,
			DestOrigStartByte: pDstStart,
			DestOrigEndByte:   pDstEnd,
		},
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      "for_statement",
				StartByte: c1SrcStart,
				EndByte:   c1SrcEnd,
			},
			DestStartByte:     &c1DstStart,
			DestEndByte:       &c1DstEnd,
			OrigStartByte:     c1SrcStart,
			OrigEndByte:       c1SrcEnd,
			DestOrigStartByte: c1DstStart,
			DestOrigEndByte:   c1DstEnd,
		},
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      "if_statement",
				StartByte: c2SrcStart,
				EndByte:   c2SrcEnd,
			},
			DestStartByte:     &c2DstStart,
			DestEndByte:       &c2DstEnd,
			OrigStartByte:     c2SrcStart,
			OrigEndByte:       c2SrcEnd,
			DestOrigStartByte: c2DstStart,
			DestOrigEndByte:   c2DstEnd,
		},
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      "expression_statement",
				StartByte: extSrcStart,
				EndByte:   extSrcEnd,
			},
			DestStartByte:     &extDstStart,
			DestEndByte:       &extDstEnd,
			OrigStartByte:     extSrcStart,
			OrigEndByte:       extSrcEnd,
			DestOrigStartByte: extDstStart,
			DestOrigEndByte:   extDstEnd,
		},
	}

	hunks := []Interval{
		{Start: 0, End: 7},   // covers source lines 0..7
		{Start: 8, End: 15},  // covers dest lines 10..15
		{Start: 16, End: 20}, // covers dest lines 24..28
	}

	pairs := []serialize.LineAlignmentPair{
		{LeftLine: 0, RightLine: 0},
		{LeftLine: 1, RightLine: 1},
		{LeftLine: 2, RightLine: -1},
		{LeftLine: 3, RightLine: -1},
		{LeftLine: 4, RightLine: -1},
		{LeftLine: 5, RightLine: -1},
		{LeftLine: 6, RightLine: -1},
		{LeftLine: 7, RightLine: 7},
		{LeftLine: -1, RightLine: 10},
		{LeftLine: -1, RightLine: 11},
		{LeftLine: -1, RightLine: 12},
		{LeftLine: -1, RightLine: 13},
		{LeftLine: -1, RightLine: 14},
		{LeftLine: -1, RightLine: 15},
		{LeftLine: -1, RightLine: 24},
		{LeftLine: -1, RightLine: 25},
		{LeftLine: -1, RightLine: 26},
	}

	badges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules, false)

	// Parent move must have directional badges
	if badge, ok := badges.SrcLineBadges[2]; !ok || !strings.Contains(badge, "➔ L11") {
		t.Errorf("expected parent move badge on line 2 '➔ L11', got %q", badge)
	}
	if badge, ok := badges.DstLineBadges[10]; !ok || !strings.Contains(badge, "⤹ L3") {
		t.Errorf("expected parent move badge on dest line 10 '⤹ L3', got %q", badge)
	}

	// Co-moving child (for_statement on line 3) must be suppressed
	if badge, exists := badges.SrcLineBadges[3]; exists {
		t.Errorf("expected child for_statement on line 3 to be suppressed, got %q", badge)
	}
	if badge, exists := badges.DstLineBadges[11]; exists {
		t.Errorf("expected child for_statement on dest line 11 to be suppressed, got %q", badge)
	}

	// Co-moving grandchild (if_statement on line 4) must be suppressed
	if badge, exists := badges.SrcLineBadges[4]; exists {
		t.Errorf("expected grandchild if_statement on line 4 to be suppressed, got %q", badge)
	}
	if badge, exists := badges.DstLineBadges[12]; exists {
		t.Errorf("expected grandchild if_statement on dest line 12 to be suppressed, got %q", badge)
	}

	// Extracted statement (line 5 moved to dest line 25 outside parent) must NOT be suppressed
	if badge, ok := badges.SrcLineBadges[5]; !ok || !strings.Contains(badge, "➔ L26") {
		t.Errorf("expected extracted statement on line 5 to have '➔ L26', got %q", badge)
	}
	if badge, ok := badges.DstLineBadges[25]; !ok || !strings.Contains(badge, "⤹ L6") {
		t.Errorf("expected extracted statement on dest line 25 to have '⤹ L6', got %q", badge)
	}
}
