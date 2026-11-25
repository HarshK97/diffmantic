package renderutil

import (
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func buildDeclarationMoveBadges(t *testing.T) MoveBadges {
	t.Helper()
	goRules := rules.Get("go")

	srcContent := "package main\n\nfunc OldFunc() {\n\tprintln(\"old\")\n}\n\nfunc Unchanged() {\n}\n"
	dstContent := "package main\n\nfunc Unchanged() {\n}\n\nfunc OldFunc() {\n\tprintln(\"old\")\n}\n"

	srcLines := strings.Split(srcContent, "\n")
	dstLines := strings.Split(dstContent, "\n")

	srcOffsets := serialize.BuildLineIndex([]byte(srcContent))
	dstOffsets := serialize.BuildLineIndex([]byte(dstContent))

	srcAST, err := treesitter.Parse([]byte(srcContent), "test.go")
	if err != nil {
		t.Fatalf("parse srcContent: %v", err)
	}
	dstAST, err := treesitter.Parse([]byte(dstContent), "test.go")
	if err != nil {
		t.Fatalf("parse dstContent: %v", err)
	}

	srcFunc, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "func OldFunc() {\n\tprintln(\"old\")\n}", "function_declaration")
	if err != nil {
		t.Fatalf("find srcFunc: %v", err)
	}
	dstFunc, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "func OldFunc() {\n\tprintln(\"old\")\n}", "function_declaration")
	if err != nil {
		t.Fatalf("find dstFunc: %v", err)
	}

	actions := []serialize.Action{
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      srcFunc.Type,
				StartByte: srcFunc.StartByte,
				EndByte:   srcFunc.EndByte,
			},
			DestStartByte: &dstFunc.StartByte,
			DestEndByte:   &dstFunc.EndByte,
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

	return BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules)
}

func TestBuildMoveBadges_DeclarationMove(t *testing.T) {
	badges := buildDeclarationMoveBadges(t)
	if badge, ok := badges.SrcLineBadges[2]; !ok || !strings.Contains(badge, "→ L6") {
		t.Errorf("expected SrcLineBadges[2] to contain '→ L6', got %q", badge)
	}
	if badge, ok := badges.DstLineBadges[5]; !ok || !strings.Contains(badge, "↳ L3") {
		t.Errorf("expected DstLineBadges[5] to contain '↳ L3', got %q", badge)
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

	srcAST, err := treesitter.Parse([]byte(srcContent), "test.go")
	if err != nil {
		t.Fatal(err)
	}
	dstAST, err := treesitter.Parse([]byte(dstContent), "test.go")
	if err != nil {
		t.Fatal(err)
	}

	srcStmt, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "absorbed := false", "short_var_declaration")
	if err != nil {
		t.Fatal(err)
	}
	dstStmt, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "absorbed := false", "short_var_declaration")
	if err != nil {
		t.Fatal(err)
	}

	actions := []serialize.Action{
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      srcStmt.Type,
				StartByte: srcStmt.StartByte,
				EndByte:   srcStmt.EndByte,
			},
			DestStartByte: &dstStmt.StartByte,
			DestEndByte:   &dstStmt.EndByte,
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

	badges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules)
	if badge, ok := badges.SrcLineBadges[4]; !ok || !strings.Contains(badge, "→ L4") {
		t.Errorf("expected SrcLineBadges[4] to contain '→ L4', got %q", badge)
	}
	if badge, ok := badges.DstLineBadges[3]; !ok || !strings.Contains(badge, "↳ L5") {
		t.Errorf("expected DstLineBadges[3] to contain '↳ L5', got %q", badge)
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

	srcAST, err := treesitter.ParseWithLanguage([]byte(srcContent), "tsx")
	if err != nil {
		t.Fatal(err)
	}
	dstAST, err := treesitter.ParseWithLanguage([]byte(dstContent), "tsx")
	if err != nil {
		t.Fatal(err)
	}

	srcDiv, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "<div className=\"grid\">\n      <div className=\"card\">\n        <Content />\n      </div>\n    </div>", "jsx_element")
	if err != nil {
		t.Fatal(err)
	}
	dstMain, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "<main className=\"main\">\n      <div className=\"card\">\n        <Content />\n      </div>\n    </main>", "jsx_element")
	if err != nil {
		t.Fatal(err)
	}
	srcOpen, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "<div className=\"grid\">", "jsx_opening_element")
	if err != nil {
		t.Fatal(err)
	}
	dstOpen, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "<main className=\"main\">", "jsx_opening_element")
	if err != nil {
		t.Fatal(err)
	}

	actions := []serialize.Action{
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      srcDiv.Type,
				StartByte: srcDiv.StartByte,
				EndByte:   srcDiv.EndByte,
			},
			DestStartByte: &dstMain.StartByte,
			DestEndByte:   &dstMain.EndByte,
		},
		{
			Action: "delete",
			Node: &serialize.NodeRef{
				Type:      srcOpen.Type,
				StartByte: srcOpen.StartByte,
				EndByte:   srcOpen.EndByte,
			},
		},
		{
			Action: "insert",
			Node: &serialize.NodeRef{
				Type:      dstOpen.Type,
				StartByte: dstOpen.StartByte,
				EndByte:   dstOpen.EndByte,
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

	badges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, tsxRules)

	// Since line 2's opening tag was deleted on source, no '→ L3' should appear on line 2
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

	srcContent := "package main\n\nfunc process() {\n\tif parentCond {\n\t\tfor childCond {\n\t\t\tif grandchildCond {\n\t\t\t}\n\t\t\textractedCall()\n\t\t}\n\t}\n\tanchor()\n}\n"
	dstContent := "package main\n\nfunc process() {\n\tpad01()\n\tpad02()\n\tpad03()\n\tpad04()\n\tpad05()\n\tpad06()\n\tpad07()\n\tif parentCond {\n\t\tfor childCond {\n\t\t\tif grandchildCond {\n\t\t\t}\n\t\t}\n\t}\n\tpad11()\n\tpad12()\n\tpad13()\n\tpad14()\n\tpad15()\n\tpad16()\n\tpad17()\n\tpad18()\n\tpad19()\n\textractedCall()\n\tanchor()\n}\n"

	srcLines := strings.Split(srcContent, "\n")
	dstLines := strings.Split(dstContent, "\n")

	srcOffsets := serialize.BuildLineIndex([]byte(srcContent))
	dstOffsets := serialize.BuildLineIndex([]byte(dstContent))

	srcAST, err := treesitter.Parse([]byte(srcContent), "test.go")
	if err != nil {
		t.Fatal(err)
	}
	dstAST, err := treesitter.Parse([]byte(dstContent), "test.go")
	if err != nil {
		t.Fatal(err)
	}

	pSrc, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "if parentCond {\n\t\tfor childCond {\n\t\t\tif grandchildCond {\n\t\t\t}\n\t\t\textractedCall()\n\t\t}\n\t}", "if_statement")
	if err != nil {
		t.Fatal(err)
	}
	pDst, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "if parentCond {\n\t\tfor childCond {\n\t\t\tif grandchildCond {\n\t\t\t}\n\t\t}\n\t}", "if_statement")
	if err != nil {
		t.Fatal(err)
	}

	c1Src, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "for childCond {\n\t\t\tif grandchildCond {\n\t\t\t}\n\t\t\textractedCall()\n\t\t}", "for_statement")
	if err != nil {
		t.Fatal(err)
	}
	c1Dst, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "for childCond {\n\t\t\tif grandchildCond {\n\t\t\t}\n\t\t}", "for_statement")
	if err != nil {
		t.Fatal(err)
	}

	c2Src, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "if grandchildCond {\n\t\t\t}", "if_statement")
	if err != nil {
		t.Fatal(err)
	}
	c2Dst, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "if grandchildCond {\n\t\t\t}", "if_statement")
	if err != nil {
		t.Fatal(err)
	}

	extSrc, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "extractedCall()", "expression_statement")
	if err != nil {
		t.Fatal(err)
	}
	extDst, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "extractedCall()", "expression_statement")
	if err != nil {
		t.Fatal(err)
	}

	actions := []serialize.Action{
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      pSrc.Type,
				StartByte: pSrc.StartByte,
				EndByte:   pSrc.EndByte,
			},
			DestStartByte:     &pDst.StartByte,
			DestEndByte:       &pDst.EndByte,
			OrigStartByte:     pSrc.StartByte,
			OrigEndByte:       pSrc.EndByte,
			DestOrigStartByte: pDst.StartByte,
			DestOrigEndByte:   pDst.EndByte,
		},
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      c1Src.Type,
				StartByte: c1Src.StartByte,
				EndByte:   c1Src.EndByte,
			},
			DestStartByte:     &c1Dst.StartByte,
			DestEndByte:       &c1Dst.EndByte,
			OrigStartByte:     c1Src.StartByte,
			OrigEndByte:       c1Src.EndByte,
			DestOrigStartByte: c1Dst.StartByte,
			DestOrigEndByte:   c1Dst.EndByte,
		},
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      c2Src.Type,
				StartByte: c2Src.StartByte,
				EndByte:   c2Src.EndByte,
			},
			DestStartByte:     &c2Dst.StartByte,
			DestEndByte:       &c2Dst.EndByte,
			OrigStartByte:     c2Src.StartByte,
			OrigEndByte:       c2Src.EndByte,
			DestOrigStartByte: c2Dst.StartByte,
			DestOrigEndByte:   c2Dst.EndByte,
		},
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      extSrc.Type,
				StartByte: extSrc.StartByte,
				EndByte:   extSrc.EndByte,
			},
			DestStartByte:     &extDst.StartByte,
			DestEndByte:       &extDst.EndByte,
			OrigStartByte:     extSrc.StartByte,
			OrigEndByte:       extSrc.EndByte,
			DestOrigStartByte: extDst.StartByte,
			DestOrigEndByte:   extDst.EndByte,
		},
	}

	pairs := []serialize.LineAlignmentPair{
		{LeftLine: 0, RightLine: 0},
		{LeftLine: 1, RightLine: 1},
		{LeftLine: 2, RightLine: 2},
		{LeftLine: 3, RightLine: -1},
		{LeftLine: 4, RightLine: -1},
		{LeftLine: 5, RightLine: -1},
		{LeftLine: 6, RightLine: -1},
		{LeftLine: 7, RightLine: -1},
		{LeftLine: 8, RightLine: -1},
		{LeftLine: 9, RightLine: -1},
		{LeftLine: -1, RightLine: 3},
		{LeftLine: -1, RightLine: 4},
		{LeftLine: -1, RightLine: 5},
		{LeftLine: -1, RightLine: 6},
		{LeftLine: -1, RightLine: 7},
		{LeftLine: -1, RightLine: 8},
		{LeftLine: -1, RightLine: 9},
		{LeftLine: -1, RightLine: 10},
		{LeftLine: -1, RightLine: 11},
		{LeftLine: -1, RightLine: 12},
		{LeftLine: -1, RightLine: 13},
		{LeftLine: -1, RightLine: 14},
		{LeftLine: -1, RightLine: 15},
		{LeftLine: -1, RightLine: 16},
		{LeftLine: -1, RightLine: 17},
		{LeftLine: -1, RightLine: 18},
		{LeftLine: -1, RightLine: 19},
		{LeftLine: -1, RightLine: 20},
		{LeftLine: -1, RightLine: 21},
		{LeftLine: -1, RightLine: 22},
		{LeftLine: -1, RightLine: 23},
		{LeftLine: -1, RightLine: 24},
		{LeftLine: -1, RightLine: 25},
		{LeftLine: 10, RightLine: 26},
		{LeftLine: 11, RightLine: 27},
	}

	hunks := []Interval{
		{Start: 0, End: 9},
		{Start: 10, End: 15},
		{Start: 16, End: len(pairs) - 1},
	}

	badges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules)

	// Parent move must have directional badges
	if badge, ok := badges.SrcLineBadges[3]; !ok || !strings.Contains(badge, "→ L11") {
		t.Errorf("expected parent move badge on line 3 '→ L11', got %q", badge)
	}
	if badge, ok := badges.DstLineBadges[10]; !ok || !strings.Contains(badge, "↳ L4") {
		t.Errorf("expected parent move badge on dest line 10 '↳ L4', got %q", badge)
	}

	// Co-moving child (for_statement on line 4) must be suppressed
	if badge, exists := badges.SrcLineBadges[4]; exists {
		t.Errorf("expected child for_statement on line 4 to be suppressed, got %q", badge)
	}
	if badge, exists := badges.DstLineBadges[11]; exists {
		t.Errorf("expected child for_statement on dest line 11 to be suppressed, got %q", badge)
	}

	// Co-moving grandchild (if_statement on line 5) must be suppressed
	if badge, exists := badges.SrcLineBadges[5]; exists {
		t.Errorf("expected grandchild if_statement on line 5 to be suppressed, got %q", badge)
	}
	if badge, exists := badges.DstLineBadges[12]; exists {
		t.Errorf("expected grandchild if_statement on dest line 12 to be suppressed, got %q", badge)
	}

	// Extracted statement (line 7 moved to dest line 25 outside parent) must NOT be suppressed
	if badge, ok := badges.SrcLineBadges[7]; !ok || !strings.Contains(badge, "→ L26") {
		t.Errorf("expected extracted statement on line 7 to have '→ L26', got %q", badge)
	}
	if badge, ok := badges.DstLineBadges[25]; !ok || !strings.Contains(badge, "↳ L8") {
		t.Errorf("expected extracted statement on dest line 25 to have '↳ L8', got %q", badge)
	}
}

func TestBuildMoveBadges_AlignmentDistanceThreshold(t *testing.T) {
	goRules := rules.Get("go")

	srcContent := "package main\n\nfunc process() {\n\tif nearCond {\n\t\tdoNear()\n\t}\n\tfarCall()\n\tpad01()\n\tpad02()\n\tpad03()\n\tpad04()\n\tpad05()\n\tpad06()\n\tpad07()\n\tpad08()\n\tpad09()\n\tpad10()\n\tpad11()\n\tpad12()\n\tpad13()\n\tpad14()\n\tpad15()\n\tpad16()\n\tpad17()\n\tpad18()\n\tpad19()\n\tpad20()\n\tpad21()\n\tpad22()\n\tpad23()\n\tpad24()\n\tpad25()\n\tpad26()\n\tpad27()\n\tpad28()\n\tpad29()\n\tpad30()\n}\n"
	dstContent := "package main\n\nfunc process() {\n\tif nearCond {\n\t\tdoNear()\n\t}\n\tpad01()\n\tpad02()\n\tpad03()\n\tpad04()\n\tpad05()\n\tpad06()\n\tpad07()\n\tpad08()\n\tpad09()\n\tpad10()\n\tpad11()\n\tpad12()\n\tpad13()\n\tpad14()\n\tpad15()\n\tpad16()\n\tpad17()\n\tpad18()\n\tpad19()\n\tpad20()\n\tpad21()\n\tpad22()\n\tpad23()\n\tpad24()\n\tpad25()\n\tpad26()\n\tpad27()\n\tpad28()\n\tpad29()\n\tpad30()\n\tfarCall()\n}\n"

	srcLines := strings.Split(srcContent, "\n")
	dstLines := strings.Split(dstContent, "\n")

	srcOffsets := serialize.BuildLineIndex([]byte(srcContent))
	dstOffsets := serialize.BuildLineIndex([]byte(dstContent))

	srcAST, err := treesitter.Parse([]byte(srcContent), "test.go")
	if err != nil {
		t.Fatal(err)
	}
	dstAST, err := treesitter.Parse([]byte(dstContent), "test.go")
	if err != nil {
		t.Fatal(err)
	}

	nearSrc, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "if nearCond {\n\t\tdoNear()\n\t}", "if_statement")
	if err != nil {
		t.Fatal(err)
	}
	nearDst, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "if nearCond {\n\t\tdoNear()\n\t}", "if_statement")
	if err != nil {
		t.Fatal(err)
	}

	farSrc, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "farCall()", "expression_statement")
	if err != nil {
		t.Fatal(err)
	}
	farDst, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "farCall()", "expression_statement")
	if err != nil {
		t.Fatal(err)
	}

	actions := []serialize.Action{
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      nearSrc.Type,
				StartByte: nearSrc.StartByte,
				EndByte:   nearSrc.EndByte,
			},
			DestStartByte:     &nearDst.StartByte,
			DestEndByte:       &nearDst.EndByte,
			OrigStartByte:     nearSrc.StartByte,
			OrigEndByte:       nearSrc.EndByte,
			DestOrigStartByte: nearDst.StartByte,
			DestOrigEndByte:   nearDst.EndByte,
		},
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Type:      farSrc.Type,
				StartByte: farSrc.StartByte,
				EndByte:   farSrc.EndByte,
			},
			DestStartByte:     &farDst.StartByte,
			DestEndByte:       &farDst.EndByte,
			OrigStartByte:     farSrc.StartByte,
			OrigEndByte:       farSrc.EndByte,
			DestOrigStartByte: farDst.StartByte,
			DestOrigEndByte:   farDst.EndByte,
		},
	}

	pairs := make([]serialize.LineAlignmentPair, len(srcLines))
	for i := range pairs {
		pairs[i] = serialize.LineAlignmentPair{LeftLine: i, RightLine: i}
	}
	hunks := []Interval{
		{Start: 0, End: len(pairs) - 1},
	}

	badges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules)

	if badge, exists := badges.SrcLineBadges[3]; exists {
		t.Errorf("expected aligned multi-line if_statement on src line 3 to suppress badge, got %q", badge)
	}
	if badge, exists := badges.DstLineBadges[3]; exists {
		t.Errorf("expected aligned multi-line if_statement on dst line 3 to suppress badge, got %q", badge)
	}
	if badge, ok := badges.SrcLineBadges[6]; !ok || !strings.Contains(badge, "→ L37") {
		t.Errorf("expected distant same-hunk one-liner on src line 6 to have '→ L37', got %q", badge)
	}
	if badge, ok := badges.DstLineBadges[36]; !ok || !strings.Contains(badge, "↳ L7") {
		t.Errorf("expected distant same-hunk one-liner on dst line 36 to have '↳ L7', got %q", badge)
	}
}

func TestBuildMoveBadges_ColorSynchronization(t *testing.T) {
	goRules := rules.Get("go")

	srcContent := "package main\n\nfunc process() {\n\toutlierCall()\n\tif mlCond {\n\t\tmlBody1()\n\t\tmlBody2()\n\t}\n\tgrpSingle()\n\tfor grpCond {\n\t\tgrpBody()\n\t}\n}\n"
	dstContent := "package main\n\nfunc process() {\n\tpad01()\n\tpad02()\n\tpad03()\n\tpad04()\n\tgrpSingle()\n\tfor grpCond {\n\t\tgrpBody()\n\t}\n\tpad05()\n\tpad06()\n\toutlierCall()\n\tif mlCond {\n\t\tmlBody1()\n\t\tmlBody2()\n\t}\n}\n"

	srcLines := strings.Split(srcContent, "\n")
	dstLines := strings.Split(dstContent, "\n")

	srcOffsets := serialize.BuildLineIndex([]byte(srcContent))
	dstOffsets := serialize.BuildLineIndex([]byte(dstContent))

	srcAST, err := treesitter.Parse([]byte(srcContent), "test.go")
	if err != nil {
		t.Fatal(err)
	}
	dstAST, err := treesitter.Parse([]byte(dstContent), "test.go")
	if err != nil {
		t.Fatal(err)
	}

	outlierSrc, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "outlierCall()", "expression_statement")
	if err != nil {
		t.Fatal(err)
	}
	outlierDst, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "outlierCall()", "expression_statement")
	if err != nil {
		t.Fatal(err)
	}

	mlSrc, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "if mlCond {\n\t\tmlBody1()\n\t\tmlBody2()\n\t}", "if_statement")
	if err != nil {
		t.Fatal(err)
	}
	mlDst, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "if mlCond {\n\t\tmlBody1()\n\t\tmlBody2()\n\t}", "if_statement")
	if err != nil {
		t.Fatal(err)
	}

	grpSlSrc, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "grpSingle()", "expression_statement")
	if err != nil {
		t.Fatal(err)
	}
	grpSlDst, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "grpSingle()", "expression_statement")
	if err != nil {
		t.Fatal(err)
	}

	grpMlSrc, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "for grpCond {\n\t\tgrpBody()\n\t}", "for_statement")
	if err != nil {
		t.Fatal(err)
	}
	grpMlDst, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "for grpCond {\n\t\tgrpBody()\n\t}", "for_statement")
	if err != nil {
		t.Fatal(err)
	}

	actions := []serialize.Action{
		{
			Action:         "move",
			MoveColorIndex: 1,
			Node: &serialize.NodeRef{
				Type:      outlierSrc.Type,
				StartByte: outlierSrc.StartByte,
				EndByte:   outlierSrc.EndByte,
			},
			DestStartByte:     &outlierDst.StartByte,
			DestEndByte:       &outlierDst.EndByte,
			OrigStartByte:     outlierSrc.StartByte,
			OrigEndByte:       outlierSrc.EndByte,
			DestOrigStartByte: outlierDst.StartByte,
			DestOrigEndByte:   outlierDst.EndByte,
		},
		{
			Action:         "move",
			MoveColorIndex: 1,
			Node: &serialize.NodeRef{
				Type:      mlSrc.Type,
				StartByte: mlSrc.StartByte,
				EndByte:   mlSrc.EndByte,
			},
			DestStartByte:     &mlDst.StartByte,
			DestEndByte:       &mlDst.EndByte,
			OrigStartByte:     mlSrc.StartByte,
			OrigEndByte:       mlSrc.EndByte,
			DestOrigStartByte: mlDst.StartByte,
			DestOrigEndByte:   mlDst.EndByte,
		},
		{
			Action:         "move",
			GroupID:        "group-ml",
			MoveColorIndex: 1,
			Node: &serialize.NodeRef{
				Type:      grpSlSrc.Type,
				StartByte: grpSlSrc.StartByte,
				EndByte:   grpSlSrc.EndByte,
			},
			DestStartByte:     &grpSlDst.StartByte,
			DestEndByte:       &grpSlDst.EndByte,
			OrigStartByte:     grpSlSrc.StartByte,
			OrigEndByte:       grpSlSrc.EndByte,
			DestOrigStartByte: grpSlDst.StartByte,
			DestOrigEndByte:   grpSlDst.EndByte,
		},
		{
			Action:         "move",
			GroupID:        "group-ml",
			MoveColorIndex: 1,
			Node: &serialize.NodeRef{
				Type:      grpMlSrc.Type,
				StartByte: grpMlSrc.StartByte,
				EndByte:   grpMlSrc.EndByte,
			},
			DestStartByte:     &grpMlDst.StartByte,
			DestEndByte:       &grpMlDst.EndByte,
			OrigStartByte:     grpMlSrc.StartByte,
			OrigEndByte:       grpMlSrc.EndByte,
			DestOrigStartByte: grpMlDst.StartByte,
			DestOrigEndByte:   grpMlDst.EndByte,
		},
	}

	pairs := make([]serialize.LineAlignmentPair, len(srcLines))
	for i := range pairs {
		pairs[i] = serialize.LineAlignmentPair{LeftLine: i, RightLine: -1}
	}
	hunks := []Interval{
		{Start: 0, End: len(pairs) - 1},
	}

	badges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules)

	// Line 3: outlierCall (single-line ungrouped outlier keeps color 1)
	if color, ok := badges.SrcLineBadgeColors[3]; !ok || color != 1 {
		t.Errorf("expected single-line ungrouped outlier src badge color on line 3 to be 1, got %d", color)
	}

	// Line 4: ml if statement (multiline move collapses to color 0)
	if color, ok := badges.SrcLineBadgeColors[4]; !ok || color != 0 {
		t.Errorf("expected multiline move src badge color on line 4 to collapse to 0, got %d", color)
	}

	// Line 8: grpSingle (move in GroupID with MoveColorIndex 1 honors badge color 1)
	if color, ok := badges.SrcLineBadgeColors[8]; !ok || color != 1 {
		t.Errorf("expected move in GroupID on line 8 to honor badge color 1, got %d", color)
	}
}

func TestBuildMoveBadges_ReorderedOutlierColorsAndNoVerbalSuffixes(t *testing.T) {
	goRules := rules.Get("go")

	srcContent := "package main\n\nfunc process() {\n\tif cond3 {\n\t\tbody3()\n\t}\n\tif cond4 {\n\t\tbody4a()\n\t\tbody4b()\n\t}\n}\n"
	dstContent := "package main\n\nfunc process() {\n\tpad01()\n\tpad02()\n\tpad03()\n\tpad04()\n\tpad05()\n\tpad06()\n\tif cond3 {\n\t\tbody3()\n\t}\n\tif cond4 {\n\t\tbody4a()\n\t\tbody4b()\n\t}\n}\n"

	srcLines := strings.Split(srcContent, "\n")
	dstLines := strings.Split(dstContent, "\n")

	srcOffsets := serialize.BuildLineIndex([]byte(srcContent))
	dstOffsets := serialize.BuildLineIndex([]byte(dstContent))

	srcAST, err := treesitter.Parse([]byte(srcContent), "test.go")
	if err != nil {
		t.Fatal(err)
	}
	dstAST, err := treesitter.Parse([]byte(dstContent), "test.go")
	if err != nil {
		t.Fatal(err)
	}

	// 1. 3-line reordered move (outlier <= 3 lines, keeps slot 1 / Magenta)
	reordered3Src, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "if cond3 {\n\t\tbody3()\n\t}", "if_statement")
	if err != nil {
		t.Fatal(err)
	}
	reordered3Dst, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "if cond3 {\n\t\tbody3()\n\t}", "if_statement")
	if err != nil {
		t.Fatal(err)
	}

	// 2. 4-line reordered move (outlier > 3 lines, collapses to slot 0 / Cyan)
	reordered4Src, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "if cond4 {\n\t\tbody4a()\n\t\tbody4b()\n\t}", "if_statement")
	if err != nil {
		t.Fatal(err)
	}
	reordered4Dst, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "if cond4 {\n\t\tbody4a()\n\t\tbody4b()\n\t}", "if_statement")
	if err != nil {
		t.Fatal(err)
	}

	actions := []serialize.Action{
		{
			Action:         "move",
			MoveColorIndex: 1,
			Node: &serialize.NodeRef{
				Type:      reordered3Src.Type,
				StartByte: reordered3Src.StartByte,
				EndByte:   reordered3Src.EndByte,
			},
			DestStartByte:     &reordered3Dst.StartByte,
			DestEndByte:       &reordered3Dst.EndByte,
			OrigStartByte:     reordered3Src.StartByte,
			OrigEndByte:       reordered3Src.EndByte,
			DestOrigStartByte: reordered3Dst.StartByte,
			DestOrigEndByte:   reordered3Dst.EndByte,
		},
		{
			Action:         "move",
			MoveColorIndex: 1,
			Node: &serialize.NodeRef{
				Type:      reordered4Src.Type,
				StartByte: reordered4Src.StartByte,
				EndByte:   reordered4Src.EndByte,
			},
			DestStartByte:     &reordered4Dst.StartByte,
			DestEndByte:       &reordered4Dst.EndByte,
			OrigStartByte:     reordered4Src.StartByte,
			OrigEndByte:       reordered4Src.EndByte,
			DestOrigStartByte: reordered4Dst.StartByte,
			DestOrigEndByte:   reordered4Dst.EndByte,
		},
	}

	pairs := make([]serialize.LineAlignmentPair, len(srcLines))
	for i := range pairs {
		pairs[i] = serialize.LineAlignmentPair{LeftLine: i, RightLine: -1}
	}
	hunks := []Interval{
		{Start: 0, End: len(pairs) - 1},
	}

	badges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules)

	// Line 3: 3-line reordered move keeps slot 1 (Magenta)
	if color, ok := badges.SrcLineBadgeColors[3]; !ok || color != 1 {
		t.Errorf("expected 3-line reordered move badge color on line 3 to be 1 (Magenta), got %d", color)
	}

	// Line 6: 4-line reordered move collapses to slot 0 (Cyan)
	if color, ok := badges.SrcLineBadgeColors[6]; !ok || color != 0 {
		t.Errorf("expected 4-line reordered move badge color on line 6 to collapse to 0 (Cyan), got %d", color)
	}

	// Badges should stay purely numerical without labels like "(hoisted)" or "(sunk)"
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
}

func TestBuildMoveBadges_UsesLargestMoveSide(t *testing.T) {
	goRules := rules.Get("go")

	srcContent := "package main\n\nfunc process() {\n\tif cond {\n\t\tbody()\n\t}\n}\n"
	dstContent := "package main\n\nfunc process() {\n\tpad01()\n\tpad02()\n\tpad03()\n\tpad04()\n\tpad05()\n\tpad06()\n\tpad07()\n\tpad08()\n\tif cond {\n\t\tbody1()\n\t\tbody2()\n\t}\n}\n"

	srcLines := strings.Split(srcContent, "\n")
	dstLines := strings.Split(dstContent, "\n")

	srcOffsets := serialize.BuildLineIndex([]byte(srcContent))
	dstOffsets := serialize.BuildLineIndex([]byte(dstContent))

	srcAST, err := treesitter.Parse([]byte(srcContent), "test.go")
	if err != nil {
		t.Fatal(err)
	}
	dstAST, err := treesitter.Parse([]byte(dstContent), "test.go")
	if err != nil {
		t.Fatal(err)
	}

	srcNode, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "if cond {\n\t\tbody()\n\t}", "if_statement")
	if err != nil {
		t.Fatal(err)
	}
	dstNode, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "if cond {\n\t\tbody1()\n\t\tbody2()\n\t}", "if_statement")
	if err != nil {
		t.Fatal(err)
	}

	actions := []serialize.Action{{
		Action:         "move",
		MoveColorIndex: 1,
		SrcLineCount:   3,
		DstLineCount:   4,
		Node: &serialize.NodeRef{
			Type:      srcNode.Type,
			StartByte: srcNode.StartByte,
			EndByte:   srcNode.EndByte,
		},
		DestStartByte:     &dstNode.StartByte,
		DestEndByte:       &dstNode.EndByte,
		OrigStartByte:     srcNode.StartByte,
		OrigEndByte:       srcNode.EndByte,
		DestOrigStartByte: dstNode.StartByte,
		DestOrigEndByte:   dstNode.EndByte,
	}}

	pairs := make([]serialize.LineAlignmentPair, 0, len(srcLines)+len(dstLines))
	for i := range srcLines {
		pairs = append(pairs, serialize.LineAlignmentPair{LeftLine: i, RightLine: -1})
	}
	for i := range dstLines {
		pairs = append(pairs, serialize.LineAlignmentPair{LeftLine: -1, RightLine: i})
	}

	badges := BuildMoveBadges(actions, []Interval{{Start: 0, End: len(pairs) - 1}}, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules)
	if color := badges.SrcLineBadgeColors[3]; color != 0 {
		t.Errorf("expected source badge on line 3 to use color 0 when destination spans four lines, got %d", color)
	}
	if color := badges.DstLineBadgeColors[11]; color != 0 {
		t.Errorf("expected destination badge on line 11 to use color 0 when destination spans four lines, got %d", color)
	}
}

func TestBuildMoveBadges_ContiguousFlatStatementCollapse(t *testing.T) {
	goRules := rules.Get("go")

	t.Run("ContiguousRunAndMultilineLandmark", func(t *testing.T) {
		srcContent := "package main\n\nfunc process() {\n\tstep1()\n\tstep2()\n\tstep3()\n\tif landmarkCond {\n\t\tlandmarkBody()\n\t}\n\tstep4()\n\tstep5()\n}\n"
		dstContent := "package main\n\nfunc process() {\n\tpad01()\n\tpad02()\n\tpad03()\n\tpad04()\n\tpad05()\n\tpad06()\n\tpad07()\n\tstep1()\n\tstep2()\n\tstep3()\n\tif landmarkCond {\n\t\tlandmarkBody()\n\t}\n\tstep4()\n\tstep5()\n}\n"

		srcLines := strings.Split(srcContent, "\n")
		dstLines := strings.Split(dstContent, "\n")

		srcOffsets := serialize.BuildLineIndex([]byte(srcContent))
		dstOffsets := serialize.BuildLineIndex([]byte(dstContent))

		srcAST, err := treesitter.Parse([]byte(srcContent), "test.go")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse([]byte(dstContent), "test.go")
		if err != nil {
			t.Fatal(err)
		}

		findStmt := func(ast *treesitter.ASTNode, content, text, nType string) *treesitter.ASTNode {
			n, fErr := testutil.FindNodeByText(ast, []byte(content), text, nType)
			if fErr != nil {
				t.Fatalf("failed finding %q: %v", text, fErr)
			}
			return n
		}

		sStep1 := findStmt(srcAST, srcContent, "step1()", "expression_statement")
		dStep1 := findStmt(dstAST, dstContent, "step1()", "expression_statement")
		sStep2 := findStmt(srcAST, srcContent, "step2()", "expression_statement")
		dStep2 := findStmt(dstAST, dstContent, "step2()", "expression_statement")
		sStep3 := findStmt(srcAST, srcContent, "step3()", "expression_statement")
		dStep3 := findStmt(dstAST, dstContent, "step3()", "expression_statement")
		sIf := findStmt(srcAST, srcContent, "if landmarkCond {\n\t\tlandmarkBody()\n\t}", "if_statement")
		dIf := findStmt(dstAST, dstContent, "if landmarkCond {\n\t\tlandmarkBody()\n\t}", "if_statement")
		sStep4 := findStmt(srcAST, srcContent, "step4()", "expression_statement")
		dStep4 := findStmt(dstAST, dstContent, "step4()", "expression_statement")
		sStep5 := findStmt(srcAST, srcContent, "step5()", "expression_statement")
		dStep5 := findStmt(dstAST, dstContent, "step5()", "expression_statement")

		mkAction := func(sNode, dNode *treesitter.ASTNode, grp string) serialize.Action {
			return serialize.Action{
				Action:  "move",
				GroupID: grp,
				Node: &serialize.NodeRef{
					Type:      sNode.Type,
					StartByte: sNode.StartByte,
					EndByte:   sNode.EndByte,
				},
				DestStartByte:     &dNode.StartByte,
				DestEndByte:       &dNode.EndByte,
				OrigStartByte:     sNode.StartByte,
				OrigEndByte:       sNode.EndByte,
				DestOrigStartByte: dNode.StartByte,
				DestOrigEndByte:   dNode.EndByte,
			}
		}

		actions := []serialize.Action{
			mkAction(sStep1, dStep1, "group-1"),
			mkAction(sStep2, dStep2, "group-1"),
			mkAction(sStep3, dStep3, "group-1"),
			mkAction(sIf, dIf, "group-1"),
			mkAction(sStep4, dStep4, "group-1"),
			mkAction(sStep5, dStep5, "group-1"),
		}

		pairs := make([]serialize.LineAlignmentPair, 0, len(srcLines)+len(dstLines))
		for i := range srcLines {
			pairs = append(pairs, serialize.LineAlignmentPair{LeftLine: i, RightLine: -1})
		}
		for i := range dstLines {
			pairs = append(pairs, serialize.LineAlignmentPair{LeftLine: -1, RightLine: i})
		}
		hunks := []Interval{
			{Start: 0, End: len(pairs) - 1},
		}

		badges := BuildMoveBadges(actions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, goRules)

		// First flat run: only the leading statement keeps its badge (line 3 -> dst line 10)
		if badge, ok := badges.SrcLineBadges[3]; !ok || !strings.Contains(badge, "→ L11") {
			t.Errorf("expected line 3 badge '→ L11', got %q", badge)
		}
		if badge, ok := badges.DstLineBadges[10]; !ok || !strings.Contains(badge, "↳ L4") {
			t.Errorf("expected dst line 10 badge '↳ L4', got %q", badge)
		}
		for _, l := range []int{4, 5} {
			if badge, ok := badges.SrcLineBadges[l]; ok {
				t.Errorf("expected line %d badge suppressed, found %q", l, badge)
			}
		}
		for _, l := range []int{11, 12} {
			if badge, ok := badges.DstLineBadges[l]; ok {
				t.Errorf("expected dst line %d badge suppressed, found %q", l, badge)
			}
		}

		// Multiline compound statements stay badged as landmarks (line 6 -> dst line 13)
		if badge, ok := badges.SrcLineBadges[6]; !ok || !strings.Contains(badge, "→ L14") {
			t.Errorf("expected if statement line 6 badge '→ L14', got %q", badge)
		}
		if badge, ok := badges.DstLineBadges[13]; !ok || !strings.Contains(badge, "↳ L7") {
			t.Errorf("expected dst if statement line 13 badge '↳ L7', got %q", badge)
		}

		// Trailing run starts fresh after the compound statement (line 9 -> dst line 16)
		if badge, ok := badges.SrcLineBadges[9]; !ok || !strings.Contains(badge, "→ L17") {
			t.Errorf("expected line 9 badge '→ L17', got %q", badge)
		}
		if badge, ok := badges.DstLineBadges[16]; !ok || !strings.Contains(badge, "↳ L10") {
			t.Errorf("expected dst line 16 badge '↳ L10', got %q", badge)
		}
		if badge, ok := badges.SrcLineBadges[10]; ok {
			t.Errorf("expected line 10 badge suppressed, found %q", badge)
		}
		if badge, ok := badges.DstLineBadges[17]; ok {
			t.Errorf("expected dst line 17 badge suppressed, found %q", badge)
		}
	})

	t.Run("InterveningTriviaAndComments", func(t *testing.T) {
		luaRules := rules.Get("lua")

		srcContent := "a = 1\n-- lua comment line\n\nb = 2\n"
		dstContent := "pad01 = 0\npad02 = 0\npad03 = 0\npad04 = 0\npad05 = 0\npad06 = 0\npad07 = 0\npad08 = 0\npad09 = 0\npad10 = 0\npad11 = 0\npad12 = 0\npad13 = 0\npad14 = 0\npad15 = 0\npad16 = 0\npad17 = 0\npad18 = 0\npad19 = 0\npad20 = 0\na = 1\n--[[ block comment ]]\nb = 2\n"

		srcLines := strings.Split(srcContent, "\n")
		dstLines := strings.Split(dstContent, "\n")

		srcOffsets := serialize.BuildLineIndex([]byte(srcContent))
		dstOffsets := serialize.BuildLineIndex([]byte(dstContent))

		srcAST, err := treesitter.ParseWithLanguage([]byte(srcContent), "lua")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.ParseWithLanguage([]byte(dstContent), "lua")
		if err != nil {
			t.Fatal(err)
		}

		s0Node, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "a = 1", "assignment_statement")
		if err != nil {
			t.Fatal(err)
		}
		d0Node, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "a = 1", "assignment_statement")
		if err != nil {
			t.Fatal(err)
		}

		s1Node, err := testutil.FindNodeByText(srcAST, []byte(srcContent), "b = 2", "assignment_statement")
		if err != nil {
			t.Fatal(err)
		}
		d1Node, err := testutil.FindNodeByText(dstAST, []byte(dstContent), "b = 2", "assignment_statement")
		if err != nil {
			t.Fatal(err)
		}

		triviaActions := []serialize.Action{
			{
				Action:  "move",
				GroupID: "lua-group",
				Node: &serialize.NodeRef{
					Type:      s0Node.Type,
					StartByte: s0Node.StartByte,
					EndByte:   s0Node.EndByte,
				},
				DestStartByte: &d0Node.StartByte,
				DestEndByte:   &d0Node.EndByte,
			},
			{
				Action:  "move",
				GroupID: "lua-group",
				Node: &serialize.NodeRef{
					Type:      s1Node.Type,
					StartByte: s1Node.StartByte,
					EndByte:   s1Node.EndByte,
				},
				DestStartByte: &d1Node.StartByte,
				DestEndByte:   &d1Node.EndByte,
			},
		}

		pairs := make([]serialize.LineAlignmentPair, 0, len(srcLines)+len(dstLines))
		for i := range srcLines {
			pairs = append(pairs, serialize.LineAlignmentPair{LeftLine: i, RightLine: -1})
		}
		for i := range dstLines {
			pairs = append(pairs, serialize.LineAlignmentPair{LeftLine: -1, RightLine: i})
		}
		hunks := []Interval{{Start: 0, End: len(pairs) - 1}}

		badges := BuildMoveBadges(triviaActions, hunks, pairs, srcOffsets, dstOffsets, srcLines, dstLines, luaRules)

		if _, ok := badges.SrcLineBadges[0]; !ok {
			t.Errorf("expected line 0 to keep badge across trivia")
		}
		if badge, ok := badges.SrcLineBadges[3]; ok {
			t.Errorf("expected line 3 badge to be collapsed across Lua comment and blank line, but got %q", badge)
		}
		if badge, ok := badges.DstLineBadges[22]; ok {
			t.Errorf("expected dst line 22 badge to be collapsed across block comment, but got %q", badge)
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

func TestBuildMoveBadges_ArrowPortability(t *testing.T) {
	badges := buildDeclarationMoveBadges(t)
	badge, ok := badges.SrcLineBadges[2]
	if !ok {
		t.Fatalf("expected SrcLineBadges[2] to exist")
	}
	// Minimal terminal fonts and screenshot tools often render Dingbats U+2794 as a missing-glyph box.
	if !strings.Contains(badge, "→") {
		t.Errorf("expected SrcLineBadges[2] to contain portable arrow '→', got %q", badge)
	}
	if strings.Contains(badge, "➔") {
		t.Errorf("expected SrcLineBadges[2] NOT to contain non-portable dingbat '➔', got %q", badge)
	}
}
