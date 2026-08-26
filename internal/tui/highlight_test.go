package tui

import (
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/serialize"
)

func TestBuildHighlightsGroupingAndMerging(t *testing.T) {
	tests := []struct {
		name         string
		src          string
		dst          string
		actions      []serialize.Action
		checkLine    int
		checkPane    string // "src" or "dst"
		expectedSpan span
		expectMerged bool
	}{
		{
			name: "Grouped moves merge across space gaps",
			src:  "    a b c\n",
			dst:  "    a b c\n",
			actions: []serialize.Action{
				{
					Action:  "move",
					Node:    &serialize.NodeRef{StartByte: 4, EndByte: 5},
					GroupID: "grp1",
				},
				{
					Action:  "move",
					Node:    &serialize.NodeRef{StartByte: 6, EndByte: 7},
					GroupID: "grp1",
				},
			},
			checkLine: 0,
			checkPane: "src",
			expectedSpan: span{
				startCol: 4,
				endCol:   7,
				kind:     kindMove,
			},
			expectMerged: true,
		},
		{
			name: "Moves with different GroupIDs and lineages do not merge",
			src:  "    a b c\n",
			dst:  "    a b c\n",
			actions: []serialize.Action{
				{
					Action:    "move",
					Node:      &serialize.NodeRef{StartByte: 4, EndByte: 5},
					Parent:    &serialize.NodeRef{Tree: "after", StartByte: 0, EndByte: 5},
					OldParent: &serialize.NodeRef{Tree: "before", StartByte: 0, EndByte: 5},
					GroupID:   "grp1",
				},
				{
					Action:    "move",
					Node:      &serialize.NodeRef{StartByte: 6, EndByte: 7},
					Parent:    &serialize.NodeRef{Tree: "after", StartByte: 6, EndByte: 10},
					OldParent: &serialize.NodeRef{Tree: "before", StartByte: 6, EndByte: 10},
					GroupID:   "grp2",
				},
			},
			checkLine: 0,
			checkPane: "src",
			expectedSpan: span{
				startCol: 4,
				endCol:   5,
				kind:     kindMove,
			},
			expectMerged: false,
		},
		{
			name: "Gaps with word characters do not merge",
			src:  "    a word b\n",
			dst:  "    a word b\n",
			actions: []serialize.Action{
				{
					Action:  "move",
					Node:    &serialize.NodeRef{StartByte: 4, EndByte: 5},
					GroupID: "grp1",
				},
				{
					Action:  "move",
					Node:    &serialize.NodeRef{StartByte: 11, EndByte: 12},
					GroupID: "grp1",
				},
			},
			checkLine: 0,
			checkPane: "src",
			expectedSpan: span{
				startCol: 4,
				endCol:   5,
				kind:     kindMove,
			},
			expectMerged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcBytes := []byte(tt.src)
			dstBytes := []byte(tt.dst)
			leftSpans := serialize.BuildHighlightSpans(srcBytes, tt.actions, "left")
			rightSpans := serialize.BuildHighlightSpans(dstBytes, tt.actions, "right")
			srcHL, dstHL := buildHighlights(leftSpans, rightSpans)

			var hl *highlights
			if tt.checkPane == "src" {
				hl = srcHL
			} else {
				hl = dstHL
			}

			spans := hl.spans[tt.checkLine]
			if tt.expectMerged {
				if len(spans) != 1 {
					t.Fatalf("expected 1 merged span, got %d", len(spans))
				}
				s := spans[0]
				if s.startCol != tt.expectedSpan.startCol || s.endCol != tt.expectedSpan.endCol {
					t.Errorf("expected cols %d..%d, got %d..%d",
						tt.expectedSpan.startCol, tt.expectedSpan.endCol, s.startCol, s.endCol)
				}
			} else {
				if len(spans) < 2 {
					t.Fatalf("expected at least 2 unmerged spans, got %d", len(spans))
				}
			}
		})
	}
}

func TestInnerInsertOverridingContainerMove(t *testing.T) {
	// Destination code:
	// Line 0: if !c.Writer.Written() {
	// Line 1:     c.String(404, "404 page not found")
	// Line 2: }
	//
	// Move action covers lines 0..2 (bytes 0..80, astLen=80)
	// Insert action covers line 1 (bytes 25..65, astLen=40)
	destBytes := []byte("if !c.Writer.Written() {\n    c.String(404, \"404 page not found\")\n}\n")

	destStartMove := uint32(0)
	destEndMove := uint32(len(destBytes))

	destStartInsert := uint32(25)
	destEndInsert := uint32(65)

	actions := []serialize.Action{
		{
			Action:        "move",
			DestStartByte: &destStartMove,
			DestEndByte:   &destEndMove,
		},
		{
			Action: "insert",
			Node: &serialize.NodeRef{
				Tree:      "after",
				StartByte: destStartInsert,
				EndByte:   destEndInsert,
			},
		},
	}

	rightSpans := serialize.BuildHighlightSpans(destBytes, actions, "right")
	_, dstHL := buildHighlights(nil, rightSpans)

	// Verify line tint on line 1 is kindInsert (not kindMove)
	if kind, ok := dstHL.tinted[1]; !ok || kind != kindInsert {
		t.Fatalf("expected line 1 tint to be kindInsert (%v), got %v", kindInsert, kind)
	}

	// Verify character rendering via renderStyledLine on model
	m := model{}
	rendered := m.renderStyledLine(
		"    c.String(404, \"404 page not found\")",
		dstHL.spans[1],
		nil,
		nil,
		0,
		80,
		-1,
		"right",
		1,
	)

	if rendered == "" {
		t.Fatal("expected non-empty rendered line")
	}
	// Verify that the inner inserted content gets the insert background tint
	insertBgSample := defaultTheme.Styles.HlInsert.Render("c")
	if !strings.Contains(rendered, insertBgSample) {
		t.Errorf("expected rendered line to contain insert-styled characters, got %q", rendered)
	}
}

func TestLeftPaneInnerMoveOverridingContainerDelete(t *testing.T) {
	// Source code:
	// Line 0: } else {
	// Line 1:     c.Writer.WriteHeader(404)
	// Line 2: }
	//
	// Delete block covers lines 0..2 (bytes 0..50, astLen=50)
	// Move expression_statement covers line 1 (bytes 13..38, astLen=25)
	// Update field_identifier covers WriteHeader on line 1 (bytes 22..33, astLen=11)
	srcBytes := []byte("} else {\n    c.Writer.WriteHeader(404)\n}\n")

	actions := []serialize.Action{
		{
			Action: "delete",
			Node: &serialize.NodeRef{
				Tree:      "before",
				StartByte: 0,
				EndByte:   50,
			},
		},
		{
			Action: "move",
			Node: &serialize.NodeRef{
				Tree:      "before",
				StartByte: 13,
				EndByte:   38,
			},
		},
		{
			Action: "update",
			Node: &serialize.NodeRef{
				Tree:      "before",
				StartByte: 22,
				EndByte:   33,
			},
			OldValue: "WriteHeader",
			NewValue: "setStatus",
		},
	}

	leftSpans := serialize.BuildHighlightSpans(srcBytes, actions, "left")
	srcHL, _ := buildHighlights(leftSpans, nil)

	// Verify line tint on line 1 is kindMove or kindUpdate (not kindDelete)
	if kind, ok := srcHL.tinted[1]; !ok || kind == kindDelete {
		t.Fatalf("expected line 1 tint on left pane not to be kindDelete, got %v", kind)
	}

	m := model{}
	rendered := m.renderStyledLine(
		"    c.Writer.WriteHeader(404)",
		srcHL.spans[1],
		nil,
		nil,
		0,
		80,
		-1,
		"left",
		1,
	)
	if rendered == "" {
		t.Fatal("expected non-empty rendered line")
	}
	// Inner move should override container delete
	moveBgSample := defaultTheme.Styles.HlMove.Render("c")
	if !strings.Contains(rendered, moveBgSample) {
		t.Errorf("expected rendered line to contain move-styled characters overriding delete, got %q", rendered)
	}
}

func TestMoveInsideInsertedContainerHighlight(t *testing.T) {
	// Source code:
	// Line 0: u.SetName("alice")
	//
	// Outer insert covers entire line (bytes 0..18, astLen=18)
	// Inner move covers u.SetName (bytes 0..9, astLen=9)
	dstBytes := []byte("u.SetName(\"alice\")\n")

	actions := []serialize.Action{
		{
			Action: "insert",
			DestNode: &serialize.NodeRef{
				Tree:      "after",
				StartByte: 0,
				EndByte:   18,
			},
		},
		{
			Action: "move",
			DestNode: &serialize.NodeRef{
				Tree:      "after",
				StartByte: 0,
				EndByte:   9,
			},
		},
	}

	rightSpans := serialize.BuildHighlightSpans(dstBytes, actions, "right")
	_, dstHL := buildHighlights(nil, rightSpans)

	m := model{}
	rendered := m.renderStyledLine(
		"u.SetName(\"alice\")",
		dstHL.spans[0],
		nil,
		nil,
		0,
		80,
		-1,
		"right",
		0,
	)

	if rendered == "" {
		t.Fatal("expected non-empty rendered line")
	}
	// Inner move should override outer insert for the CallExpr span
	moveBgSample := defaultTheme.Styles.HlMove.Render("u")
	if !strings.Contains(rendered, moveBgSample) {
		t.Errorf("expected rendered line to contain inner move-styled characters, got %q", rendered)
	}
}
