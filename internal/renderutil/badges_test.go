package renderutil

import (
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
