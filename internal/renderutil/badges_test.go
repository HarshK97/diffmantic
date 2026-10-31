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

	// 1. promoteDeclarations = true (Unified format: Tier 2 hunk headers, no line badges)
	unifiedBadges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules, true)
	if len(unifiedBadges.SrcLineBadges) != 0 {
		t.Errorf("unified mode expected 0 SrcLineBadges for declaration move, got %v", unifiedBadges.SrcLineBadges)
	}
	if len(unifiedBadges.DstLineBadges) != 0 {
		t.Errorf("unified mode expected 0 DstLineBadges for declaration move, got %v", unifiedBadges.DstLineBadges)
	}
	if len(unifiedBadges.HunkHeaders) == 0 {
		t.Fatalf("unified mode expected HunkHeaders to be populated")
	}
	if !strings.Contains(unifiedBadges.HunkHeaders[0], "func OldFunc()") {
		t.Errorf("expected hunk 0 header to mention 'func OldFunc()', got: %q", unifiedBadges.HunkHeaders[0])
	}

	// 2. promoteDeclarations = false (Side-by-Side format: Tier 3 line badges, no hunk headers)
	sbsBadges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules, false)
	if len(sbsBadges.HunkHeaders) != 0 {
		t.Errorf("SBS mode expected 0 HunkHeaders, got %v", sbsBadges.HunkHeaders)
	}
	if badge, ok := sbsBadges.SrcLineBadges[2]; !ok || !strings.Contains(badge, "➔ L6") {
		t.Errorf("SBS mode expected SrcLineBadges[2] to contain '➔ L6', got %q", badge)
	}
	if badge, ok := sbsBadges.DstLineBadges[5]; !ok || !strings.Contains(badge, "↳ L3") {
		t.Errorf("SBS mode expected DstLineBadges[5] to contain '↳ L3', got %q", badge)
	}
}

func TestBuildMoveBadges_SingleLineStatementMove(t *testing.T) {
	goRules := rules.Get("go")

	srcContent := "package main\n\nfunc Foo() {\n\tif true {\n\t\tabsorbed := false\n\t}\n}\n"
	dstContent := "package main\n\nfunc Foo() {\n\tabsorbed := false\n\t// inserted big block\n\tprintln(\"1\")\n\tprintln(\"2\")\n\tif true {\n\t}\n}\n"

	srcLines := strings.Split(srcContent, "\n")
	dstLines := strings.Split(dstContent, "\n")

	srcOffsets := serialize.BuildLineIndex([]byte(srcContent))
	dstOffsets := serialize.BuildLineIndex([]byte(dstContent))

	sStartByte := uint32(strings.Index(srcContent, "absorbed := false"))
	sEndByte := sStartByte + uint32(len("absorbed := false"))
	dStartByte := uint32(strings.Index(dstContent, "absorbed := false"))
	dEndByte := dStartByte + uint32(len("absorbed := false"))

	actions := []serialize.Action{
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      "short_var_declaration",
				StartByte: sStartByte,
				EndByte:   sEndByte,
			},
			DestStartByte: &dStartByte,
			DestEndByte:   &dEndByte,
		},
	}

	hunks := []Interval{
		{Start: 0, End: 5},
		{Start: 6, End: 12},
	}

	pairs := []serialize.LineAlignmentPair{
		{LeftLine: 0, RightLine: 0},
		{LeftLine: 1, RightLine: 1},
		{LeftLine: 2, RightLine: 2},
		{LeftLine: -1, RightLine: 3}, // dst line 3: absorbed := false
		{LeftLine: -1, RightLine: 4},
		{LeftLine: -1, RightLine: 5},
		{LeftLine: -1, RightLine: 6},
		{LeftLine: 3, RightLine: 7},
		{LeftLine: 4, RightLine: -1}, // src line 4: absorbed := false
		{LeftLine: 5, RightLine: 8},
		{LeftLine: 6, RightLine: 9},
	}

	// Single-line statement moves get line badges in both unified and SBS mode, not hunk headers.
	unifiedBadges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules, true)
	if badge, ok := unifiedBadges.SrcLineBadges[4]; !ok || !strings.Contains(badge, "➔ L4") {
		t.Errorf("unified mode expected SrcLineBadges[4] to contain '➔ L4', got %q", badge)
	}
	if badge, ok := unifiedBadges.DstLineBadges[3]; !ok || !strings.Contains(badge, "↳ L5") {
		t.Errorf("unified mode expected DstLineBadges[3] to contain '↳ L5', got %q", badge)
	}

	sbsBadges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules, false)
	if badge, ok := sbsBadges.SrcLineBadges[4]; !ok || !strings.Contains(badge, "➔ L4") {
		t.Errorf("SBS mode expected SrcLineBadges[4] to contain '➔ L4', got %q", badge)
	}
	if badge, ok := sbsBadges.DstLineBadges[3]; !ok || !strings.Contains(badge, "↳ L5") {
		t.Errorf("SBS mode expected DstLineBadges[3] to contain '↳ L5', got %q", badge)
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
	// Since line 2's opening tag was inserted on dest, no '↳ L3' should appear on line 2
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
	if badge, ok := badges.DstLineBadges[10]; !ok || !strings.Contains(badge, "↳ L3") {
		t.Errorf("expected parent move badge on dest line 10 '↳ L3', got %q", badge)
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
	if badge, ok := badges.DstLineBadges[25]; !ok || !strings.Contains(badge, "↳ L6") {
		t.Errorf("expected extracted statement on dest line 25 to have '↳ L6', got %q", badge)
	}
}

func TestBuildMoveBadges_AlignmentDistanceThreshold(t *testing.T) {
	goRules := rules.Get("go")

	srcLines := make([]string, 40)
	for i := range srcLines {
		srcLines[i] = fmt.Sprintf("src line %d", i)
	}
	dstLines := make([]string, 40)
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

	// 1. Aligned on the same row (distance 0 <= 15), so suppress the badge.
	nearSrcStart := uint32(srcOffsets[2])
	nearSrcEnd := uint32(srcOffsets[5])
	nearDstStart := uint32(dstOffsets[2])
	nearDstEnd := uint32(dstOffsets[5])

	// 2. Same hunk, but moves 24 alignment rows away (> 15 threshold), so keep the badge.
	farSrcStart := uint32(srcOffsets[6])
	farSrcEnd := uint32(srcOffsets[7] - 1)
	farDstStart := uint32(dstOffsets[30])
	farDstEnd := uint32(dstOffsets[31] - 1)

	actions := []serialize.Action{
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      "if_statement",
				StartByte: nearSrcStart,
				EndByte:   nearSrcEnd,
			},
			DestStartByte:     &nearDstStart,
			DestEndByte:       &nearDstEnd,
			OrigStartByte:     nearSrcStart,
			OrigEndByte:       nearSrcEnd,
			DestOrigStartByte: nearDstStart,
			DestOrigEndByte:   nearDstEnd,
		},
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      "expression_statement",
				StartByte: farSrcStart,
				EndByte:   farSrcEnd,
			},
			DestStartByte:     &farDstStart,
			DestEndByte:       &farDstEnd,
			OrigStartByte:     farSrcStart,
			OrigEndByte:       farSrcEnd,
			DestOrigStartByte: farDstStart,
			DestOrigEndByte:   farDstEnd,
		},
	}

	pairs := make([]serialize.LineAlignmentPair, 40)
	for i := range pairs {
		pairs[i] = serialize.LineAlignmentPair{LeftLine: i, RightLine: i}
	}
	hunks := []Interval{
		{Start: 0, End: 39},
	}

	badges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules, false)

	if badge, exists := badges.SrcLineBadges[2]; exists {
		t.Errorf("expected aligned multi-line if_statement on src line 2 to suppress badge, got %q", badge)
	}
	if badge, exists := badges.DstLineBadges[2]; exists {
		t.Errorf("expected aligned multi-line if_statement on dst line 2 to suppress badge, got %q", badge)
	}
	if badge, ok := badges.SrcLineBadges[6]; !ok || !strings.Contains(badge, "➔ L31") {
		t.Errorf("expected distant same-hunk one-liner on src line 6 (alignDist=24) to have '➔ L31', got %q", badge)
	}
	if badge, ok := badges.DstLineBadges[30]; !ok || !strings.Contains(badge, "↳ L7") {
		t.Errorf("expected distant same-hunk one-liner on dst line 30 (alignDist=24) to have '↳ L7', got %q", badge)
	}
}

func TestBuildMoveBadges_ColorSynchronization(t *testing.T) {
	goRules := rules.Get("go")

	srcLines := make([]string, 30)
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

	// 1. Single-line move with MoveColorIndex: 1 (outlier)
	outlierSrcStart := uint32(srcOffsets[2])
	outlierSrcEnd := uint32(srcOffsets[3] - 1)
	outlierDstStart := uint32(dstOffsets[20])
	outlierDstEnd := uint32(dstOffsets[21] - 1)

	// 2. Multiline move with MoveColorIndex: 1
	mlSrcStart := uint32(srcOffsets[5])
	mlSrcEnd := uint32(srcOffsets[8])
	mlDstStart := uint32(dstOffsets[23])
	mlDstEnd := uint32(dstOffsets[26])

	// 3. Single-line move that belongs to a multiline group "group-ml"
	grpSlSrcStart := uint32(srcOffsets[10])
	grpSlSrcEnd := uint32(srcOffsets[11] - 1)
	grpSlDstStart := uint32(dstOffsets[15])
	grpSlDstEnd := uint32(dstOffsets[16] - 1)

	// Co-member of "group-ml" that is multiline
	grpMlSrcStart := uint32(srcOffsets[12])
	grpMlSrcEnd := uint32(srcOffsets[14])
	grpMlDstStart := uint32(dstOffsets[17])
	grpMlDstEnd := uint32(dstOffsets[19])

	actions := []serialize.Action{
		{
			Action:         "move",
			MoveColorIndex: 1,
			Node: &serialize.NodeRef{
				Type:      "expression_statement",
				StartByte: outlierSrcStart,
				EndByte:   outlierSrcEnd,
			},
			DestStartByte:     &outlierDstStart,
			DestEndByte:       &outlierDstEnd,
			OrigStartByte:     outlierSrcStart,
			OrigEndByte:       outlierSrcEnd,
			DestOrigStartByte: outlierDstStart,
			DestOrigEndByte:   outlierDstEnd,
		},
		{
			Action:         "move",
			MoveColorIndex: 1,
			Node: &serialize.NodeRef{
				Type:      "if_statement",
				StartByte: mlSrcStart,
				EndByte:   mlSrcEnd,
			},
			DestStartByte:     &mlDstStart,
			DestEndByte:       &mlDstEnd,
			OrigStartByte:     mlSrcStart,
			OrigEndByte:       mlSrcEnd,
			DestOrigStartByte: mlDstStart,
			DestOrigEndByte:   mlDstEnd,
		},
		{
			Action:         "move",
			GroupID:        "group-ml",
			MoveColorIndex: 1,
			Node: &serialize.NodeRef{
				Type:      "expression_statement",
				StartByte: grpSlSrcStart,
				EndByte:   grpSlSrcEnd,
			},
			DestStartByte:     &grpSlDstStart,
			DestEndByte:       &grpSlDstEnd,
			OrigStartByte:     grpSlSrcStart,
			OrigEndByte:       grpSlSrcEnd,
			DestOrigStartByte: grpSlDstStart,
			DestOrigEndByte:   grpSlDstEnd,
		},
		{
			Action:         "move",
			GroupID:        "group-ml",
			MoveColorIndex: 1,
			Node: &serialize.NodeRef{
				Type:      "for_statement",
				StartByte: grpMlSrcStart,
				EndByte:   grpMlSrcEnd,
			},
			DestStartByte:     &grpMlDstStart,
			DestEndByte:       &grpMlDstEnd,
			OrigStartByte:     grpMlSrcStart,
			OrigEndByte:       grpMlSrcEnd,
			DestOrigStartByte: grpMlDstStart,
			DestOrigEndByte:   grpMlDstEnd,
		},
	}

	hunks := []Interval{
		{Start: 0, End: 29},
	}
	pairs := make([]serialize.LineAlignmentPair, 30)
	for i := range pairs {
		pairs[i] = serialize.LineAlignmentPair{LeftLine: i, RightLine: -1}
	}

	badges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules, false)

	if color, ok := badges.SrcLineBadgeColors[2]; !ok || color != 1 {
		t.Errorf("expected single-line ungrouped outlier src badge color to be 1, got %d", color)
	}

	if color, ok := badges.SrcLineBadgeColors[5]; !ok || color != 0 {
		t.Errorf("expected multiline move src badge color to collapse to 0, got %d", color)
	}

	if color, ok := badges.SrcLineBadgeColors[10]; !ok || color != 0 {
		t.Errorf("expected single-line move in multiline group to collapse src badge color to 0, got %d", color)
	}
}

func TestBuildMoveBadges_ReorderedOutlierColorsAndNoVerbalSuffixes(t *testing.T) {
	goRules := rules.Get("go")

	srcLines := make([]string, 30)
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

	// 1. 3-line reordered move (outlier <= 3 lines, keeps slot 1 / Magenta)
	reordered3SrcStart := uint32(srcOffsets[2])
	reordered3SrcEnd := uint32(srcOffsets[5] - 1) // lines 2, 3, 4 (3 lines)
	reordered3DstStart := uint32(dstOffsets[20])
	reordered3DstEnd := uint32(dstOffsets[23] - 1)

	// 2. 4-line reordered move (outlier > 3 lines, collapses to slot 0 / Cyan)
	reordered4SrcStart := uint32(srcOffsets[7])
	reordered4SrcEnd := uint32(srcOffsets[11] - 1) // lines 7, 8, 9, 10 (4 lines)
	reordered4DstStart := uint32(dstOffsets[24])
	reordered4DstEnd := uint32(dstOffsets[28] - 1)

	actions := []serialize.Action{
		{
			Action:         "move",
			MoveColorIndex: 1,
			Node: &serialize.NodeRef{
				Type:      "if_statement",
				StartByte: reordered3SrcStart,
				EndByte:   reordered3SrcEnd,
			},
			DestStartByte:     &reordered3DstStart,
			DestEndByte:       &reordered3DstEnd,
			OrigStartByte:     reordered3SrcStart,
			OrigEndByte:       reordered3SrcEnd,
			DestOrigStartByte: reordered3DstStart,
			DestOrigEndByte:   reordered3DstEnd,
		},
		{
			Action:         "move",
			MoveColorIndex: 1,
			Node: &serialize.NodeRef{
				Type:      "if_statement",
				StartByte: reordered4SrcStart,
				EndByte:   reordered4SrcEnd,
			},
			DestStartByte:     &reordered4DstStart,
			DestEndByte:       &reordered4DstEnd,
			OrigStartByte:     reordered4SrcStart,
			OrigEndByte:       reordered4SrcEnd,
			DestOrigStartByte: reordered4DstStart,
			DestOrigEndByte:   reordered4DstEnd,
		},
	}

	hunks := []Interval{
		{Start: 0, End: 29},
	}
	pairs := make([]serialize.LineAlignmentPair, 30)
	for i := range pairs {
		pairs[i] = serialize.LineAlignmentPair{LeftLine: i, RightLine: -1}
	}

	badges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules, false)

	// 1. 3-line reordered move keeps slot 1 (Magenta)
	if color, ok := badges.SrcLineBadgeColors[2]; !ok || color != 1 {
		t.Errorf("expected 3-line reordered move badge color to be 1 (Magenta), got %d", color)
	}

	// 2. 4-line reordered move collapses to slot 0 (Cyan)
	if color, ok := badges.SrcLineBadgeColors[7]; !ok || color != 0 {
		t.Errorf("expected 4-line reordered move badge color to collapse to 0 (Cyan), got %d", color)
	}

	// 3. Badges should stay purely numerical without labels like "(hoisted)" or "(sunk)"
	for line, badge := range badges.SrcLineBadges {
		if strings.Contains(badge, "hoisted") || strings.Contains(badge, "sunk") {
			t.Errorf("badge on line %d contains unexpected label: %q", line, badge)
		}
	}
	for line, badge := range badges.DstLineBadges {
		if strings.Contains(badge, "hoisted") || strings.Contains(badge, "sunk") {
			t.Errorf("dst badge on line %d contains unexpected label: %q", line, badge)
		}
	}
	for hIdx, header := range badges.HunkHeaders {
		if strings.Contains(header, "hoisted") || strings.Contains(header, "sunk") {
			t.Errorf("hunk header %d contains unexpected label: %q", hIdx, header)
		}
	}
}

func TestBuildMoveBadges_ContiguousFlatStatementCollapse(t *testing.T) {
	goRules := rules.Get("go")

	srcLines := make([]string, 40)
	for i := range 40 {
		srcLines[i] = fmt.Sprintf("src line %d", i)
	}
	dstLines := make([]string, 50)
	for i := range 50 {
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

	pairs := make([]serialize.LineAlignmentPair, 0, 90)
	for i := range 40 {
		pairs = append(pairs, serialize.LineAlignmentPair{LeftLine: i, RightLine: -1})
	}
	for i := range 50 {
		pairs = append(pairs, serialize.LineAlignmentPair{LeftLine: -1, RightLine: i})
	}
	hunks := []Interval{
		{Start: 0, End: len(pairs) - 1},
	}

	mkAction := func(sType string, sL1, sL2, dL1, dL2 int, grp string) serialize.Action {
		sStart := uint32(srcOffsets[sL1])
		sEnd := uint32(srcOffsets[sL2+1] - 1)
		dStart := uint32(dstOffsets[dL1])
		dEnd := uint32(dstOffsets[dL2+1] - 1)
		return serialize.Action{
			Action:  "move",
			GroupID: grp,
			Node: &serialize.NodeRef{
				Type:      sType,
				StartByte: sStart,
				EndByte:   sEnd,
			},
			DestStartByte:     &dStart,
			DestEndByte:       &dEnd,
			OrigStartByte:     sStart,
			OrigEndByte:       sEnd,
			DestOrigStartByte: dStart,
			DestOrigEndByte:   dEnd,
		}
	}

	t.Run("ContiguousRunAndMultilineLandmark", func(t *testing.T) {
		actions := []serialize.Action{
			mkAction("expression_statement", 2, 2, 20, 20, "group-1"),
			mkAction("expression_statement", 3, 3, 21, 21, "group-1"),
			mkAction("expression_statement", 4, 4, 22, 22, "group-1"),
			mkAction("if_statement", 5, 7, 23, 25, "group-1"),
			mkAction("expression_statement", 8, 8, 26, 26, "group-1"),
			mkAction("expression_statement", 9, 9, 27, 27, "group-1"),
		}

		badges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules, false)

		// First flat run: only the leading statement keeps its badge.
		if badge, ok := badges.SrcLineBadges[2]; !ok || !strings.Contains(badge, "➔ L21") {
			t.Errorf("expected line 2 badge '➔ L21', got %q", badge)
		}
		if badge, ok := badges.DstLineBadges[20]; !ok || !strings.Contains(badge, "↳ L3") {
			t.Errorf("expected dst line 20 badge '↳ L3', got %q", badge)
		}
		for _, l := range []int{3, 4} {
			if badge, ok := badges.SrcLineBadges[l]; ok {
				t.Errorf("expected line %d badge suppressed, found %q", l, badge)
			}
		}
		for _, l := range []int{21, 22} {
			if badge, ok := badges.DstLineBadges[l]; ok {
				t.Errorf("expected dst line %d badge suppressed, found %q", l, badge)
			}
		}

		// Multiline compound statements stay badged as landmarks.
		if badge, ok := badges.SrcLineBadges[5]; !ok || !strings.Contains(badge, "➔ L24") {
			t.Errorf("expected if statement line 5 badge '➔ L24', got %q", badge)
		}
		if badge, ok := badges.DstLineBadges[23]; !ok || !strings.Contains(badge, "↳ L6") {
			t.Errorf("expected dst if statement line 23 badge '↳ L6', got %q", badge)
		}

		// Trailing run starts fresh after the compound statement.
		if badge, ok := badges.SrcLineBadges[8]; !ok || !strings.Contains(badge, "➔ L27") {
			t.Errorf("expected line 8 badge '➔ L27', got %q", badge)
		}
		if badge, ok := badges.DstLineBadges[26]; !ok || !strings.Contains(badge, "↳ L9") {
			t.Errorf("expected dst line 26 badge '↳ L9', got %q", badge)
		}
		if badge, ok := badges.SrcLineBadges[9]; ok {
			t.Errorf("expected line 9 badge suppressed, found %q", badge)
		}
		if badge, ok := badges.DstLineBadges[27]; ok {
			t.Errorf("expected dst line 27 badge suppressed, found %q", badge)
		}
	})

	t.Run("InterveningTriviaAndComments", func(t *testing.T) {
		triviaSrc := make([]string, 10)
		triviaDst := make([]string, 20)
		triviaSrc[0] = "a = 1"
		triviaSrc[1] = "-- lua comment line"
		triviaSrc[2] = ""
		triviaSrc[3] = "b = 2"

		triviaDst[10] = "a = 1"
		triviaDst[11] = "/* block comment */"
		triviaDst[12] = "b = 2"

		tSrcOffsets := make([]int, len(triviaSrc)+1)
		for i := range triviaSrc {
			tSrcOffsets[i+1] = tSrcOffsets[i] + len(triviaSrc[i]) + 1
		}
		tDstOffsets := make([]int, len(triviaDst)+1)
		for i := range triviaDst {
			tDstOffsets[i+1] = tDstOffsets[i] + len(triviaDst[i]) + 1
		}

		tPairs := make([]serialize.LineAlignmentPair, 0, 40)
		for i := range 20 {
			tPairs = append(tPairs, serialize.LineAlignmentPair{LeftLine: i, RightLine: -1})
		}
		for i := range 20 {
			tPairs = append(tPairs, serialize.LineAlignmentPair{LeftLine: -1, RightLine: i})
		}
		tHunks := []Interval{{Start: 0, End: len(tPairs) - 1}}

		s0Start := uint32(tSrcOffsets[0])
		s0End := uint32(tSrcOffsets[1] - 1)
		d0Start := uint32(tDstOffsets[10])
		d0End := uint32(tDstOffsets[11] - 1)

		s1Start := uint32(tSrcOffsets[3])
		s1End := uint32(tSrcOffsets[4] - 1)
		d1Start := uint32(tDstOffsets[12])
		d1End := uint32(tDstOffsets[13] - 1)

		triviaActions := []serialize.Action{
			{
				Action:  "move",
				GroupID: "lua-group",
				Node: &serialize.NodeRef{
					Type:      "assignment_statement",
					StartByte: s0Start,
					EndByte:   s0End,
				},
				DestStartByte: &d0Start,
				DestEndByte:   &d0End,
			},
			{
				Action:  "move",
				GroupID: "lua-group",
				Node: &serialize.NodeRef{
					Type:      "assignment_statement",
					StartByte: s1Start,
					EndByte:   s1End,
				},
				DestStartByte: &d1Start,
				DestEndByte:   &d1End,
			},
		}

		luaRules := rules.Get("lua")
		badges := BuildMoveBadges(triviaActions, tHunks, tPairs, tSrcOffsets, tDstOffsets, triviaSrc, triviaDst, luaRules, false)

		if _, ok := badges.SrcLineBadges[0]; !ok {
			t.Errorf("expected line 0 to keep badge across trivia")
		}
		if badge, ok := badges.SrcLineBadges[3]; ok {
			t.Errorf("expected line 3 badge to be collapsed across Lua comment and blank line, but got %q", badge)
		}
		if badge, ok := badges.DstLineBadges[12]; ok {
			t.Errorf("expected dst line 12 badge to be collapsed across block comment, but got %q", badge)
		}
	})

	t.Run("TriviaBoundsSafety", func(t *testing.T) {
		lines := []string{"// comment", "code"}
		if isLinesTriviaOnly(lines, -1, 0) {
			t.Errorf("expected negative startLine to return false")
		}
		if isLinesTriviaOnly(lines, 0, 5) {
			t.Errorf("expected out-of-bounds endLine to return false")
		}
		if !isLinesTriviaOnly(lines, 0, 0) {
			t.Errorf("expected line 0 comment to be trivia")
		}
		if isLinesTriviaOnly(lines, 1, 1) {
			t.Errorf("expected line 1 code not to be trivia")
		}
	})
}
