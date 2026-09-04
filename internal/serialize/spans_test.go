package serialize

import (
	"testing"
)

func TestBuildHighlightSpansGapMerging(t *testing.T) {
	fileBytes := []byte("    for cookie in cj:\n")
	actions := []Action{
		{
			Action:  "move",
			Node:    &NodeRef{Type: "for", Label: "for", StartByte: 4, EndByte: 7},
			GroupID: "group-1",
		},
		{
			Action:  "move",
			Node:    &NodeRef{Type: "identifier", Label: "cookie", StartByte: 8, EndByte: 14},
			GroupID: "group-1",
		},
		{
			Action:  "move",
			Node:    &NodeRef{Type: "in", Label: "in", StartByte: 15, EndByte: 17},
			GroupID: "group-1",
		},
		{
			Action:  "move",
			Node:    &NodeRef{Type: "identifier", Label: "cj", StartByte: 18, EndByte: 20},
			GroupID: "group-1",
		},
	}

	leftSpans := BuildHighlightSpans(fileBytes, actions, "left")
	if len(leftSpans) != 1 {
		t.Fatalf("expected 1 merged highlight span, got %d", len(leftSpans))
	}

	span := leftSpans[0]
	if span.Line != 0 || span.StartCol != 4 || span.EndCol != 20 || span.Action != "move" {
		t.Errorf("expected merged span line=0 cols 4..20 action='move', got line=%d cols %d..%d action=%s",
			span.Line, span.StartCol, span.EndCol, span.Action)
	}
}

func TestBuildHighlightSpansInterleavedActions(t *testing.T) {
	fileBytes := []byte("    for cookie in cj:\n")
	actions := []Action{
		{
			Action: "delete",
			Node:   &NodeRef{Type: "for_statement", StartByte: 0, EndByte: 21},
		},
		{
			Action:  "move",
			Node:    &NodeRef{Type: "for", Label: "for", StartByte: 4, EndByte: 7},
			GroupID: "group-1",
		},
		{
			Action:  "move",
			Node:    &NodeRef{Type: "identifier", Label: "cookie", StartByte: 8, EndByte: 14},
			GroupID: "group-1",
		},
		{
			Action:  "move",
			Node:    &NodeRef{Type: "in", Label: "in", StartByte: 15, EndByte: 17},
			GroupID: "group-1",
		},
		{
			Action:  "move",
			Node:    &NodeRef{Type: "identifier", Label: "cj", StartByte: 18, EndByte: 20},
			GroupID: "group-1",
		},
	}

	leftSpans := BuildHighlightSpans(fileBytes, actions, "left")
	var moveSpans []HighlightSpan
	for _, s := range leftSpans {
		if s.Action == "move" {
			moveSpans = append(moveSpans, s)
		}
	}

	if len(moveSpans) != 1 {
		t.Fatalf("expected 1 merged move span, got %d", len(moveSpans))
	}
	if moveSpans[0].StartCol != 4 || moveSpans[0].EndCol != 20 {
		t.Errorf("expected merged move span cols 4..20, got %d..%d", moveSpans[0].StartCol, moveSpans[0].EndCol)
	}
}

func TestBuildHighlightSpansInnerSpanPreservation(t *testing.T) {
	fileBytes := []byte("def hello_world():\n")
	// Container delete covers 0..18 (astLen=18)
	// Inner delete covers 4..15 (astLen=11)
	actions := []Action{
		{
			Action: "delete",
			Node:   &NodeRef{Type: "function_definition", StartByte: 0, EndByte: 18},
		},
		{
			Action: "delete",
			Node:   &NodeRef{Type: "identifier", StartByte: 4, EndByte: 15},
		},
	}

	leftSpans := BuildHighlightSpans(fileBytes, actions, "left")
	if len(leftSpans) != 2 {
		t.Fatalf("expected 2 delete spans (outer and inner preserved), got %d", len(leftSpans))
	}

	// Also test when inner starts at same start col (e.g. 0..10 and 0..18)
	actionsCoaligned := []Action{
		{
			Action: "delete",
			Node:   &NodeRef{Type: "def", StartByte: 0, EndByte: 3},
		},
		{
			Action: "delete",
			Node:   &NodeRef{Type: "function_definition", StartByte: 0, EndByte: 18},
		},
	}

	coalignedSpans := BuildHighlightSpans(fileBytes, actionsCoaligned, "left")
	if len(coalignedSpans) != 2 {
		t.Fatalf("expected 2 delete spans for coaligned start (inner 0..3 and outer 0..18), got %d", len(coalignedSpans))
	}
}

func TestBuildHighlightSpansRightSideMoveNodeLen(t *testing.T) {
	destBytes := []byte("    c.String(404, \"not found\")\n")
	destStart := uint32(4)
	destEnd := uint32(30)
	actions := []Action{
		{
			Action:        "move",
			DestStartByte: &destStart,
			DestEndByte:   &destEnd,
		},
	}

	rightSpans := BuildHighlightSpans(destBytes, actions, "right")
	if len(rightSpans) != 1 {
		t.Fatalf("expected 1 move span on right pane, got %d", len(rightSpans))
	}
	if rightSpans[0].ActionRef == nil {
		t.Fatal("expected non-nil ActionRef")
	}
	nl := nodeLen(rightSpans[0].ActionRef, "right")
	if nl != 26 {
		t.Errorf("expected nodeLen=26 on right pane, got %d", nl)
	}
}

func TestBuildHighlightSpansWithDelimiterSpan(t *testing.T) {
	fileBytes := []byte("func foo() {\n    return 42\n}\n")
	act := Action{
		Action: "delete",
		Node:   &NodeRef{Type: "function_declaration", StartByte: 0, EndByte: 12}, // "func foo() {\n"
	}
	// Add closing delimiter '}' on line 2 (bytes 27..28)
	delims := []DelimiterSpan{
		{
			StartByte: 27,
			EndByte:   28,
			Action:    "delete",
			ActionRef: &act,
		},
	}

	leftSpans := BuildHighlightSpans(fileBytes, []Action{act}, "left", delims...)
	if len(leftSpans) != 2 {
		t.Fatalf("expected 2 delete spans (header and closing delimiter), got %d: %+v", len(leftSpans), leftSpans)
	}

	if leftSpans[0].Line != 0 || leftSpans[0].Action != "delete" {
		t.Errorf("expected header span on line 0, got %+v", leftSpans[0])
	}
	if leftSpans[1].Line != 2 || leftSpans[1].StartCol != 0 || leftSpans[1].EndCol != 1 || leftSpans[1].Action != "delete" {
		t.Errorf("expected delimiter span on line 2 cols 0..1 action='delete', got %+v", leftSpans[1])
	}
}
