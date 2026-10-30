package serialize

import (
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/postprocess"
	"github.com/HarshK97/diffmantic/internal/treesitter"
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
	// Partitioner: inner wins [4,15), container gets [0,4) and [15,18).
	// Same action coalesces → 1 span covering entire range.
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
	if len(leftSpans) != 1 {
		t.Fatalf("expected 1 coalesced delete span (same action merges), got %d: %+v", len(leftSpans), leftSpans)
	}
	if leftSpans[0].StartCol != 0 || leftSpans[0].EndCol != 18 {
		t.Errorf("expected coalesced span [0,18), got [%d,%d)", leftSpans[0].StartCol, leftSpans[0].EndCol)
	}

	// Also test when inner starts at same start col (e.g. 0..3 and 0..18)
	// Partitioner: inner wins [0,3), outer wins [3,18), same action coalesces → 1 span
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
	if len(coalignedSpans) != 1 {
		t.Fatalf("expected 1 coalesced delete span for coaligned start, got %d: %+v", len(coalignedSpans), coalignedSpans)
	}
	if coalignedSpans[0].StartCol != 0 || coalignedSpans[0].EndCol != 18 {
		t.Errorf("expected coalesced span [0,18), got [%d,%d)", coalignedSpans[0].StartCol, coalignedSpans[0].EndCol)
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

func TestAbsorbConnectorDelimiters(t *testing.T) {
	t.Run("absorbs trailing dot in qualified type", func(t *testing.T) {
		src := []byte("func f(x gin.HandlerFunc) {}\n")
		// "gin" is 9..12 inside parent "gin.HandlerFunc" (9..24)
		actions := []Action{
			{
				Action: "insert",
				Node:   &NodeRef{Type: "package_identifier", StartByte: 9, EndByte: 12},
				Parent: &NodeRef{Type: "qualified_type", StartByte: 9, EndByte: 24},
			},
		}
		spans := BuildHighlightSpans(src, actions, "right")
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		if spans[0].StartCol != 9 || spans[0].EndCol != 13 {
			t.Errorf("expected span cols 9..13 covering 'gin.', got %d..%d", spans[0].StartCol, spans[0].EndCol)
		}
	})

	t.Run("absorbs leading dot in member access", func(t *testing.T) {
		src := []byte("x := obj.field\n")
		// "field" is 9..14 inside parent "obj.field" (5..14)
		actions := []Action{
			{
				Action: "insert",
				Node:   &NodeRef{Type: "field_identifier", StartByte: 9, EndByte: 14},
				Parent: &NodeRef{Type: "selector_expression", StartByte: 5, EndByte: 14},
			},
		}
		spans := BuildHighlightSpans(src, actions, "right")
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		if spans[0].StartCol != 8 || spans[0].EndCol != 14 {
			t.Errorf("expected span cols 8..14 covering '.field', got %d..%d", spans[0].StartCol, spans[0].EndCol)
		}
	})

	t.Run("absorbs trailing scope resolution ::", func(t *testing.T) {
		src := []byte("std::vector<int> v;\n")
		// "std" is 0..3 inside parent "std::vector" (0..11)
		actions := []Action{
			{
				Action: "insert",
				Node:   &NodeRef{Type: "namespace_identifier", StartByte: 0, EndByte: 3},
				Parent: &NodeRef{Type: "qualified_identifier", StartByte: 0, EndByte: 11},
			},
		}
		spans := BuildHighlightSpans(src, actions, "right")
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		if spans[0].StartCol != 0 || spans[0].EndCol != 5 {
			t.Errorf("expected span cols 0..5 covering 'std::', got %d..%d", spans[0].StartCol, spans[0].EndCol)
		}
	})

	t.Run("does not absorb variadic ellipsis ...", func(t *testing.T) {
		src := []byte("func f(args ...gin.HandlerFunc) {}\n")
		// "gin" is 15..18 preceded by "..."
		actions := []Action{
			{
				Action: "insert",
				Node:   &NodeRef{Type: "package_identifier", StartByte: 15, EndByte: 18},
				Parent: &NodeRef{Type: "qualified_type", StartByte: 15, EndByte: 30},
			},
		}
		spans := BuildHighlightSpans(src, actions, "right")
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		// Don't absorb leading "...", but still absorb trailing "." -> "gin." (cols 15..19)
		if spans[0].StartCol != 15 || spans[0].EndCol != 19 {
			t.Errorf("expected span cols 15..19 covering 'gin.', got %d..%d", spans[0].StartCol, spans[0].EndCol)
		}
	})

	t.Run("absorbs leading comma for appended argument", func(t *testing.T) {
		src := []byte("foo(a, b, c)\n")
		// "c" is 10..11 inside parent "foo(a, b, c)" argument_list (3..12)
		actions := []Action{
			{
				Action: "insert",
				Node:   &NodeRef{Type: "identifier", StartByte: 10, EndByte: 11},
				Parent: &NodeRef{Type: "argument_list", StartByte: 3, EndByte: 12},
			},
		}
		spans := BuildHighlightSpans(src, actions, "right")
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		if spans[0].StartCol != 8 || spans[0].EndCol != 11 {
			t.Errorf("expected span cols 8..11 covering ', c', got %d..%d", spans[0].StartCol, spans[0].EndCol)
		}
	})

	t.Run("absorbs trailing comma for prepended argument", func(t *testing.T) {
		src := []byte("foo(a, b)\n")
		// "a" is 4..5 inside parent "foo(a, b)" argument_list (3..9)
		actions := []Action{
			{
				Action: "insert",
				Node:   &NodeRef{Type: "identifier", StartByte: 4, EndByte: 5},
				Parent: &NodeRef{Type: "argument_list", StartByte: 3, EndByte: 9},
			},
		}
		spans := BuildHighlightSpans(src, actions, "right")
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		if spans[0].StartCol != 4 || spans[0].EndCol != 6 {
			t.Errorf("expected span cols 4..6 covering 'a,', got %d..%d", spans[0].StartCol, spans[0].EndCol)
		}
	})

	t.Run("absorbs leading comma for deleted intermediate argument", func(t *testing.T) {
		src := []byte("foo(a, b, c)\n")
		// "b" is 7..8 inside parent argument_list (3..12)
		// "b" is followed by ", " at 8..10
		actions := []Action{
			{
				Action: "delete",
				Node:   &NodeRef{Type: "identifier", StartByte: 7, EndByte: 8},
				Parent: &NodeRef{Type: "argument_list", StartByte: 3, EndByte: 12},
			},
		}
		spans := BuildHighlightSpans(src, actions, "left")
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		if spans[0].StartCol != 7 || spans[0].EndCol != 9 {
			t.Errorf("expected span cols 7..9 covering 'b,', got %d..%d", spans[0].StartCol, spans[0].EndCol)
		}
	})

	t.Run("absorbs multiline trailing comma", func(t *testing.T) {
		src := []byte("foo(\n    a,\n    b,\n)\n")
		// Line 0: "foo(\n" (0..5)
		// Line 1: "    a,\n" (5..12)
		// Line 2: "    b,\n" (12..19), "b" is 16..17, "," is 17..18
		actions := []Action{
			{
				Action: "insert",
				Node:   &NodeRef{Type: "identifier", StartByte: 16, EndByte: 17},
				Parent: &NodeRef{Type: "argument_list", StartByte: 3, EndByte: 20},
			},
		}
		spans := BuildHighlightSpans(src, actions, "right")
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		if spans[0].Line != 2 || spans[0].StartCol != 4 || spans[0].EndCol != 6 {
			t.Errorf("expected span line 2 cols 4..6 covering 'b,', got line %d cols %d..%d",
				spans[0].Line, spans[0].StartCol, spans[0].EndCol)
		}
	})

	t.Run("absorbs leading comma in array literal", func(t *testing.T) {
		src := []byte("x = [1, 2, 3]\n")
		// "3" is 11..12 inside array (4..13)
		actions := []Action{
			{
				Action: "insert",
				Node:   &NodeRef{Type: "integer", StartByte: 11, EndByte: 12},
				Parent: &NodeRef{Type: "array", StartByte: 4, EndByte: 13},
			},
		}
		spans := BuildHighlightSpans(src, actions, "right")
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		if spans[0].StartCol != 9 || spans[0].EndCol != 12 {
			t.Errorf("expected span cols 9..12 covering ', 3', got %d..%d", spans[0].StartCol, spans[0].EndCol)
		}
	})

	t.Run("absorbs trailing comma for moved elements on left and right", func(t *testing.T) {
		leftSrc := []byte("vals := []T{\n    {name: \"a\"},\n    {name: \"b\"},\n}\n")
		rightSrc := []byte("vals := []T{\n    {name: \"b\"},\n    {name: \"a\"},\n}\n")
		// Left: {name: "a"}, at line 1, bytes 17..28, comma at 28..29
		// Right: {name: "a"}, at line 2, bytes 34..45, comma at 45..46
		actions := []Action{
			{
				Action:        "move",
				Node:          &NodeRef{Type: "literal_element", StartByte: 17, EndByte: 28},
				OldParent:     &NodeRef{Type: "literal_value", StartByte: 12, EndByte: 49},
				DestStartByte: ptr(uint32(34)),
				DestEndByte:   ptr(uint32(45)),
				Parent:        &NodeRef{Type: "literal_value", StartByte: 12, EndByte: 49},
			},
		}

		leftSpans := BuildHighlightSpans(leftSrc, actions, "left")
		if len(leftSpans) != 2 {
			t.Fatalf("expected 2 left spans (move element + delete comma), got %d: %+v", len(leftSpans), leftSpans)
		}
		if leftSpans[0].Line != 1 || leftSpans[0].StartCol != 4 || leftSpans[0].EndCol != 15 || leftSpans[0].Action != "move" {
			t.Errorf("expected left span 0 covering '{name: \"a\"}' as move, got %+v", leftSpans[0])
		}
		if leftSpans[1].Line != 1 || leftSpans[1].StartCol != 15 || leftSpans[1].EndCol != 16 || leftSpans[1].Action != "delete" {
			t.Errorf("expected left span 1 covering ',' as delete, got %+v", leftSpans[1])
		}

		rightSpans := BuildHighlightSpans(rightSrc, actions, "right")
		if len(rightSpans) != 2 {
			t.Fatalf("expected 2 right spans (move element + insert comma), got %d: %+v", len(rightSpans), rightSpans)
		}
		if rightSpans[0].Line != 2 || rightSpans[0].StartCol != 4 || rightSpans[0].EndCol != 15 || rightSpans[0].Action != "move" {
			t.Errorf("expected right span 0 covering '{name: \"a\"}' as move, got %+v", rightSpans[0])
		}
		if rightSpans[1].Line != 2 || rightSpans[1].StartCol != 15 || rightSpans[1].EndCol != 16 || rightSpans[1].Action != "insert" {
			t.Errorf("expected right span 1 covering ',' as insert, got %+v", rightSpans[1])
		}
	})

	t.Run("absorbs leading comma for moved last element on left and right", func(t *testing.T) {
		leftSrc := []byte("foo(a, b)")
		rightSrc := []byte("bar(x, b)")
		// 'b' is at bytes 7..8, preceded by ", " at 5..7
		actions := []Action{
			{
				Action:        "move",
				Node:          &NodeRef{Type: "identifier", StartByte: 7, EndByte: 8},
				OldParent:     &NodeRef{Type: "argument_list", StartByte: 3, EndByte: 9},
				DestStartByte: ptr(uint32(7)),
				DestEndByte:   ptr(uint32(8)),
				Parent:        &NodeRef{Type: "argument_list", StartByte: 3, EndByte: 9},
			},
		}

		leftSpans := BuildHighlightSpans(leftSrc, actions, "left")
		if len(leftSpans) != 2 {
			t.Fatalf("expected 2 left spans (delete leading comma + move element), got %d: %+v", len(leftSpans), leftSpans)
		}
		if leftSpans[0].Line != 0 || leftSpans[0].StartCol != 5 || leftSpans[0].EndCol != 6 || leftSpans[0].Action != "delete" {
			t.Errorf("expected left span 0 covering ',' as delete, got %+v", leftSpans[0])
		}
		if leftSpans[1].Line != 0 || leftSpans[1].StartCol != 7 || leftSpans[1].EndCol != 8 || leftSpans[1].Action != "move" {
			t.Errorf("expected left span 1 covering 'b' as move, got %+v", leftSpans[1])
		}

		rightSpans := BuildHighlightSpans(rightSrc, actions, "right")
		if len(rightSpans) != 2 {
			t.Fatalf("expected 2 right spans (insert leading comma + move element), got %d: %+v", len(rightSpans), rightSpans)
		}
		if rightSpans[0].Line != 0 || rightSpans[0].StartCol != 5 || rightSpans[0].EndCol != 6 || rightSpans[0].Action != "insert" {
			t.Errorf("expected right span 0 covering ',' as insert, got %+v", rightSpans[0])
		}
		if rightSpans[1].Line != 0 || rightSpans[1].StartCol != 7 || rightSpans[1].EndCol != 8 || rightSpans[1].Action != "move" {
			t.Errorf("expected right span 1 covering 'b' as move, got %+v", rightSpans[1])
		}
	})

	t.Run("does not absorb commas for non-delimited containers", func(t *testing.T) {
		src := []byte("block {\n    foo,\n}\n")
		actions := []Action{
			{
				Action: "insert",
				Node:   &NodeRef{Type: "identifier", StartByte: 12, EndByte: 15},
				Parent: &NodeRef{Type: "block", StartByte: 6, EndByte: 20},
			},
		}
		spans := BuildHighlightSpans(src, actions, "right")
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		if spans[0].StartCol != 4 || spans[0].EndCol != 7 {
			t.Errorf("expected span cols 4..7 covering 'foo', got %d..%d", spans[0].StartCol, spans[0].EndCol)
		}
	})

	t.Run("bounds safety when parent extends beyond file length", func(t *testing.T) {
		src := []byte("foo(a, b)")
		// Synthetic node where Parent.EndByte (100) extends past file length (9)
		actions := []Action{
			{
				Action: "insert",
				Node:   &NodeRef{Type: "identifier", StartByte: 4, EndByte: 5},
				Parent: &NodeRef{Type: "argument_list", StartByte: 3, EndByte: 100},
			},
		}
		spans := BuildHighlightSpans(src, actions, "right")
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		if spans[0].StartCol != 4 || spans[0].EndCol != 6 {
			t.Errorf("expected span cols 4..6 covering 'a,', got %d..%d", spans[0].StartCol, spans[0].EndCol)
		}
	})
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

func TestNestedMoveActionsKeepsOutermost(t *testing.T) {
	src := []byte("0123456789abcdef\n")
	actions := []Action{
		{Action: "move", Node: &NodeRef{Tree: "before", Type: "block", StartByte: 0, EndByte: 16}},
		{Action: "move", Node: &NodeRef{Tree: "before", Type: "identifier", StartByte: 2, EndByte: 5}},
		{Action: "move", Node: &NodeRef{Tree: "before", Type: "identifier", StartByte: 10, EndByte: 12}},
	}
	spans := BuildHighlightSpans(src, actions, "left")
	for _, s := range spans {
		if s.Action == "move" && s.ActionRef != nil && s.ActionRef.Node.StartByte != 0 {
			t.Fatalf("expected only outermost move spans, got nested %+v", s)
		}
	}
	if len(spans) == 0 {
		t.Fatal("expected outermost move span, got none")
	}
}

func TestPartitionDisjointSpans_MoveDeleteOverlap(t *testing.T) {
	fileBytes := []byte("abcdef\n")
	// move covers [0,6) astLen=6, delete covers [0,6) astLen=6
	// Same astLen → tiebreak by priority: move(3) > delete(2) → move wins entire range
	actions := []Action{
		{Action: "move", Node: &NodeRef{Type: "stmt", StartByte: 0, EndByte: 6}},
		{Action: "delete", Node: &NodeRef{Type: "stmt", StartByte: 0, EndByte: 6}},
	}
	spans := BuildHighlightSpans(fileBytes, actions, "left")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span after partition, got %d: %+v", len(spans), spans)
	}
	if spans[0].Action != "move" {
		t.Errorf("expected move to win tiebreak, got %s", spans[0].Action)
	}
}

func TestPartitionDisjointSpans_CoextensiveDuplicates(t *testing.T) {
	fileBytes := []byte("hello world\n")
	// Two identical delete spans on same bytes → inner (same astLen) wins, coalesces to 1
	actions := []Action{
		{Action: "delete", Node: &NodeRef{Type: "a", StartByte: 0, EndByte: 11}},
		{Action: "delete", Node: &NodeRef{Type: "b", StartByte: 0, EndByte: 11}},
	}
	spans := BuildHighlightSpans(fileBytes, actions, "left")
	if len(spans) != 1 {
		t.Fatalf("expected 1 coalesced span for coextensive duplicates, got %d: %+v", len(spans), spans)
	}
}

func TestPartitionDisjointSpans_UpdateInsideMove(t *testing.T) {
	fileBytes := []byte("abcdef\n")
	// An update inside a move promotes to move_update, flanked by the outer move segments.
	// [0,2) move, [2,4) move_update, [4,6) move.
	actions := []Action{
		{Action: "move", Node: &NodeRef{Type: "stmt", StartByte: 0, EndByte: 6}},
		{Action: "update", Node: &NodeRef{Type: "id", StartByte: 2, EndByte: 4}},
	}
	spans := BuildHighlightSpans(fileBytes, actions, "left")
	if len(spans) != 3 {
		t.Fatalf("expected 3 spans (move, move_update, move), got %d: %+v", len(spans), spans)
	}
	// Find the middle span
	for _, s := range spans {
		if s.StartCol == 2 && s.EndCol == 4 {
			if s.Action != "move_update" {
				t.Errorf("expected move_update for update inside move, got %s", s.Action)
			}
			return
		}
	}
	t.Error("expected move_update span at [2,4)")
}

func TestPartitionDisjointSpans_ThreeWayNesting(t *testing.T) {
	fileBytes := []byte("0123456789abcdef\n")
	// Three-way conflict: delete [0,16), move [0,16), and nested update [2,5).
	// Move beats delete on tiebreak; update beats move on AST specificity and promotes to move_update.
	actions := []Action{
		{Action: "delete", Node: &NodeRef{Type: "stmt", StartByte: 0, EndByte: 16}},
		{Action: "move", Node: &NodeRef{Type: "stmt", StartByte: 0, EndByte: 16}},
		{Action: "update", Node: &NodeRef{Type: "id", StartByte: 2, EndByte: 5}},
	}
	spans := BuildHighlightSpans(fileBytes, actions, "left")
	if len(spans) != 3 {
		t.Fatalf("expected 3 disjoint spans, got %d: %+v", len(spans), spans)
	}
	// Verify update segment is promoted to moveUpdate (inside original move)
	for _, s := range spans {
		if s.StartCol == 2 && s.EndCol == 5 {
			if s.Action != "move_update" {
				t.Errorf("expected moveUpdate at [2,5) (inside move), got %s", s.Action)
			}
			return
		}
	}
	t.Error("expected moveUpdate span at [2,5)")
}

func TestPartitionDisjointSpans_NilActionRefFallback(t *testing.T) {
	fileBytes := []byte("abcdef\n")
	// move with ActionRef (astLen=6) vs nil-ActionRef delete (fallback=span width=6)
	// Same astLen → tiebreak: move(3) > delete(2) → move wins
	actions := []Action{
		{Action: "move", Node: &NodeRef{Type: "stmt", StartByte: 0, EndByte: 6}},
		{Action: "delete", Node: nil}, // nil Node → ActionRef will be nil on the span
	}
	spans := BuildHighlightSpans(fileBytes, actions, "left")
	// The nil-Node delete won't produce a span at all (addSpan requires non-nil Node for delete)
	// So only the move span survives
	if len(spans) != 1 {
		t.Fatalf("expected 1 span (nil-Node delete produces nothing), got %d: %+v", len(spans), spans)
	}
	if spans[0].Action != "move" {
		t.Errorf("expected move, got %s", spans[0].Action)
	}
}

func TestPartitionDisjointSpans_DisjointSpansUnchanged(t *testing.T) {
	fileBytes := []byte("hello world\n")
	// Two non-overlapping spans → partitioner should not change them
	actions := []Action{
		{Action: "delete", Node: &NodeRef{Type: "a", StartByte: 0, EndByte: 5}},
		{Action: "insert", Node: &NodeRef{Type: "b", StartByte: 6, EndByte: 11}},
	}
	spans := BuildHighlightSpans(fileBytes, actions, "left")
	if len(spans) != 1 {
		t.Fatalf("expected 1 delete span (insert not on left), got %d: %+v", len(spans), spans)
	}
	if spans[0].StartCol != 0 || spans[0].EndCol != 5 || spans[0].Action != "delete" {
		t.Errorf("expected delete [0,5), got %+v", spans[0])
	}
}

func TestNestedMoveActions_InterveningInsertionPreservesInnerMove(t *testing.T) {
	// Enclosing move k: [0..100] -> [0..100]
	// Intervening insert ins: [20..80]
	// Inner move r: [30..70] -> [30..70]
	// ins sits between k and r on the destination side, so r must not be skipped.
	// We use temporary vars for &uint32 pointers instead of Go 1.26 new(uint32(...)).
	// tree-sitter-go doesn't parse value expressions in new() yet (https://github.com/tree-sitter/tree-sitter-go/pull/199).
	destStartK := uint32(0)
	destEndK := uint32(100)
	destStartR := uint32(30)
	destEndR := uint32(70)
	actions := []Action{
		{
			Action:        "move",
			Node:          &NodeRef{StartByte: 0, EndByte: 100},
			DestStartByte: &destStartK,
			DestEndByte:   &destEndK,
		},
		{
			Action: "insert",
			Node:   &NodeRef{StartByte: 20, EndByte: 80},
		},
		{
			Action:        "move",
			Node:          &NodeRef{StartByte: 30, EndByte: 70},
			DestStartByte: &destStartR,
			DestEndByte:   &destEndR,
		},
	}

	skip := nestedMoveActions(actions)
	if skip[2] {
		t.Errorf("expected inner move at index 2 NOT to be skipped when enclosed in intervening insertion")
	}
}

func TestNestedMoveActions_BreakawayMovePreserved(t *testing.T) {
	// Outer move k: [0..100] -> [500..600]
	// Inner move r: [30..70] -> [200..240] (breaks away to a completely different location)
	destStartK := uint32(500)
	destEndK := uint32(600)
	destStartR := uint32(200)
	destEndR := uint32(240)
	actions := []Action{
		{
			Action:        "move",
			Node:          &NodeRef{StartByte: 0, EndByte: 100},
			DestStartByte: &destStartK,
			DestEndByte:   &destEndK,
		},
		{
			Action:        "move",
			Node:          &NodeRef{StartByte: 30, EndByte: 70},
			DestStartByte: &destStartR,
			DestEndByte:   &destEndR,
		},
	}

	skip := nestedMoveActions(actions)
	if skip[1] {
		t.Errorf("expected breakaway move at index 1 NOT to be skipped")
	}
}

func TestNestedMoveActions_IntactRelocatedBlockSubsumesInnerMove(t *testing.T) {
	// Outer move k: [0..100] -> [500..600]
	// Inner move r: [30..70] -> [530..570] (moves together inside k)
	destStartK := uint32(500)
	destEndK := uint32(600)
	destStartR := uint32(530)
	destEndR := uint32(570)
	actions := []Action{
		{
			Action:        "move",
			Node:          &NodeRef{StartByte: 0, EndByte: 100},
			DestStartByte: &destStartK,
			DestEndByte:   &destEndK,
		},
		{
			Action:        "move",
			Node:          &NodeRef{StartByte: 30, EndByte: 70},
			DestStartByte: &destStartR,
			DestEndByte:   &destEndR,
		},
	}

	skip := nestedMoveActions(actions)
	if !skip[1] {
		t.Errorf("expected inner move at index 1 to be skipped when intact inside outer move")
	}
}

func TestPartitionLineSpans_DifferentMoveColorsNotCoalesced(t *testing.T) {
	// Two adjacent or overlapping move segments with different actRef and colors:
	// Outer move act1 (color 0) and inner move act2 (color 2).
	// partitionLineSpans must not coalesce them into a single color 0 span.
	destStart1 := uint32(500)
	destEnd1 := uint32(600)
	destStart2 := uint32(200)
	destEnd2 := uint32(240)

	act1 := Action{
		Action:         "move",
		MoveColorIndex: 0,
		Node:           &NodeRef{Type: "if_statement", StartByte: 0, EndByte: 50},
		DestStartByte:  &destStart1,
		DestEndByte:    &destEnd1,
	}
	act2 := Action{
		Action:         "move",
		MoveColorIndex: 1,
		Node:           &NodeRef{Type: "short_var_declaration", StartByte: 5, EndByte: 35},
		DestStartByte:  &destStart2,
		DestEndByte:    &destEnd2,
	}

	spans := []internalSpan{
		{startCol: 0, endCol: 50, action: "move", actRef: &act1},
		{startCol: 5, endCol: 35, action: "move", actRef: &act2},
	}

	partitioned := partitionLineSpans(spans, "left", nil, nil, nil)
	foundColor1 := false
	for _, p := range partitioned {
		if p.actRef != nil && p.actRef.MoveColorIndex == 1 {
			foundColor1 = true
			if p.startCol != 5 || p.endCol != 35 {
				t.Errorf("expected color 1 span at [5..35], got [%d..%d]", p.startCol, p.endCol)
			}
		}
	}
	if !foundColor1 {
		t.Errorf("expected inner move with color 1 to survive partitioning without being swallowed by color 0")
	}
}

func TestPartitionLineSpans_MoveUpdateInheritsEnclosingMoveColor(t *testing.T) {
	// Edits inside a moved block should keep the surrounding move's color
	// instead of falling back to default slot 0.
	actMove := Action{
		Action:         "move",
		MoveColorIndex: 1,
		Node:           &NodeRef{Type: "field_expression", StartByte: 0, EndByte: 20},
	}
	actUpdate := Action{
		Action: "update",
		Node:   &NodeRef{Type: "field_identifier", StartByte: 10, EndByte: 15},
	}

	spans := []internalSpan{
		{startCol: 0, endCol: 20, action: "move", actRef: &actMove},
		{startCol: 10, endCol: 15, action: "update", actRef: &actUpdate},
	}

	partitioned := partitionLineSpans(spans, "right", nil, nil, nil)
	if len(partitioned) != 3 {
		t.Fatalf("expected 3 partitioned spans, got %d: %+v", len(partitioned), partitioned)
	}
	if partitioned[1].action != "move_update" {
		t.Errorf("expected span 1 action to be move_update, got %s", partitioned[1].action)
	}
	if partitioned[1].moveColorIndex != 1 {
		t.Errorf("expected span 1 moveColorIndex to be 1, got %d", partitioned[1].moveColorIndex)
	}
}

func TestBuildHighlightSpans_AdaptiveMoveColor(t *testing.T) {
	// Single-line moves keep their slot color (Magenta), while multiline moves
	// collapse to Cyan. If a group mixes both, the whole group takes Cyan so
	// siblings stay the same color.
	fileBytes := []byte("foo(a, b)\nfunc MultiLine() {\n    return 42\n}\nms.Remove(d)\n")

	singleLineAct := Action{
		Action:         "move",
		MoveColorIndex: 1,
		Node:           &NodeRef{Type: "identifier", StartByte: 4, EndByte: 5},
		DestStartByte:  ptr(uint32(100)),
		DestEndByte:    ptr(uint32(101)),
	}

	multiLineAct := Action{
		Action:         "move",
		GroupID:        "group-1",
		MoveColorIndex: 1,
		Node:           &NodeRef{Type: "function_declaration", StartByte: 10, EndByte: 44},
		DestStartByte:  ptr(uint32(200)),
		DestEndByte:    ptr(uint32(300)),
	}

	groupedSingleLineAct := Action{
		Action:         "move",
		GroupID:        "group-1",
		MoveColorIndex: 1,
		Node:           &NodeRef{Type: "expression_statement", StartByte: 45, EndByte: 57},
		DestStartByte:  ptr(uint32(305)),
		DestEndByte:    ptr(uint32(317)),
	}

	actions := []Action{singleLineAct, multiLineAct, groupedSingleLineAct}
	spans := BuildHighlightSpans(fileBytes, actions, "left")

	var singleSpan, multiSpan, groupedSingleSpan *HighlightSpan
	for i := range spans {
		if spans[i].Line == 0 && spans[i].Action == "move" {
			singleSpan = &spans[i]
		}
		if spans[i].Line == 1 && spans[i].Action == "move" {
			multiSpan = &spans[i]
		}
		if spans[i].Line == 4 && spans[i].Action == "move" {
			groupedSingleSpan = &spans[i]
		}
	}

	if singleSpan == nil {
		t.Fatalf("expected move span on line 0")
	}
	if singleSpan.ColorIndex != 1 {
		t.Errorf("expected single-line move to retain ColorIndex 1 (Magenta), got %d", singleSpan.ColorIndex)
	}

	if multiSpan == nil {
		t.Fatalf("expected move span on line 1")
	}
	if multiSpan.ColorIndex != 0 {
		t.Errorf("expected multiline move to have adaptive ColorIndex 0 (Cyan), got %d", multiSpan.ColorIndex)
	}

	if groupedSingleSpan == nil {
		t.Fatalf("expected grouped single-line move span on line 4")
	}
	if groupedSingleSpan.ColorIndex != 0 {
		t.Errorf("expected single-line move in multiline GroupID to share ColorIndex 0 (Cyan), got %d", groupedSingleSpan.ColorIndex)
	}

	t.Run("single line with trailing newline", func(t *testing.T) {
		stmtBytes := []byte("return 42\n")
		stmtAct := Action{
			Action:         "move",
			MoveColorIndex: 1,
			Node:           &NodeRef{Type: "return_statement", StartByte: 0, EndByte: 10},
			DestStartByte:  ptr(uint32(50)),
			DestEndByte:    ptr(uint32(60)),
		}
		stmtSpans := BuildHighlightSpans(stmtBytes, []Action{stmtAct}, "left")
		if len(stmtSpans) == 0 {
			t.Fatalf("expected highlight span")
		}
		if stmtSpans[0].ColorIndex != 1 {
			t.Errorf("expected statement with trailing newline to retain ColorIndex 1, got %d", stmtSpans[0].ColorIndex)
		}
	})
}

func TestBuildHighlightSpans_AdaptiveMoveColor_FullEngine(t *testing.T) {
	// An isolated move takes color slot 0 (Cyan). An adjacent move group
	// containing both a multiline if-statement and a single-line call gets
	// assigned color slot 1 (Magenta). BuildHighlightSpans must collapse the
	// single-line sibling to Cyan so the entire group stays visually unified.
	oldSrc := `package main

func process() {
	isolatedMove()
	stationaryA()
	stationaryB()
	if check() {
		doWork()
	}
	finish()
}
`
	newSrc := `package main

func process() {
	stationaryA()
	stationaryB()
	isolatedMove()
	if wrapper() {
		if check() {
			doWork()
		}
		finish()
	}
}
`
	oldAST, err := treesitter.Parse([]byte(oldSrc), "test.go")
	if err != nil {
		t.Fatalf("parse oldSrc: %v", err)
	}
	newAST, err := treesitter.Parse([]byte(newSrc), "test.go")
	if err != nil {
		t.Fatalf("parse newSrc: %v", err)
	}

	part := engine.NewLinePartition([]byte(oldSrc), []byte(newSrc))
	mr := engine.Match(oldAST, newAST, []byte(oldSrc), []byte(newSrc), part)
	es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)
	es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

	env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, []byte(oldSrc), []byte(newSrc), EnvelopeOptions{
		IncludeActions:    true,
		IncludeHighlights: true,
	})
	if err != nil {
		t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
	}

	// finish() on left line 9 and right line 11 is single-line, but grouped with
	// multiline if check(). Verify both sides collapse finish() to ColorIndex 0.
	for _, h := range env.LeftHighlights {
		if h.Line == 9 && h.Action == "move" && h.ColorIndex != 0 {
			t.Errorf("expected left finish() move highlight to have ColorIndex 0, got %d", h.ColorIndex)
		}
	}
	for _, h := range env.RightHighlights {
		if h.Line == 11 && h.Action == "move" && h.ColorIndex != 0 {
			t.Errorf("expected right finish() move highlight to have ColorIndex 0, got %d", h.ColorIndex)
		}
	}

	// Whole-snippet context line purity: unchanged lines must have zero highlights
	assertContextLinesPure(t, env.LeftHighlights, []int{3, 6, 7, 8, 9})
	assertContextLinesPure(t, env.RightHighlights, []int{5, 6, 7, 8, 9, 10, 11})
}

func TestBuildHighlightSpans_SegmentedContainerClosingDelimiterColor(t *testing.T) {
	// Lines 0 and 2 hold the opening and closing braces.
	srcBytes := []byte("{\n    foo()\n}\n")

	// Block container action whose prologue was adjusted to [0, 2) ("{\n").
	blockAct := Action{
		Action:         "move",
		MoveColorIndex: 1,
		Node:           &NodeRef{Type: "block", StartByte: 0, EndByte: 2},
		DestStartByte:  ptr(uint32(0)),
		DestEndByte:    ptr(uint32(2)),
	}

	actions := []Action{blockAct}
	closingDelimiter := DelimiterSpan{
		StartByte: 12, // '}'
		EndByte:   13,
		Action:    "move",
		ActionRef: &actions[0],
	}

	spans := BuildHighlightSpans(srcBytes, actions, "left", closingDelimiter)

	var openSpan, closeSpan *HighlightSpan
	for i := range spans {
		if spans[i].Line == 0 && spans[i].Action == "move" {
			openSpan = &spans[i]
		}
		if spans[i].Line == 2 && spans[i].Action == "move" {
			closeSpan = &spans[i]
		}
	}

	if openSpan == nil {
		t.Fatalf("expected opening brace move span on line 0")
	}
	if openSpan.ColorIndex != 1 {
		t.Errorf("expected opening delimiter to have ColorIndex 1 (Magenta), got %d", openSpan.ColorIndex)
	}

	if closeSpan == nil {
		t.Fatalf("expected closing delimiter move span on line 2")
	}
	if closeSpan.ColorIndex != 1 {
		t.Errorf("expected closing delimiter to retain matching ColorIndex 1 (Magenta), got %d", closeSpan.ColorIndex)
	}
}

func TestBuildHighlightSpans_MultilineConstructMutesHeaderAndDelimiter(t *testing.T) {
	// A 10-line if statement whose header range was adjusted to line 0 ("if cond {\n")
	src := "if cond {\n" + strings.Repeat("    stmt()\n", 8) + "}\n"
	srcBytes := []byte(src)

	// Header-only node span, but unclipped bounds cover the full construct so it mutes.
	act := Action{
		Action:         "move",
		MoveColorIndex: 1, // initially assigned 1 by interference
		OrigStartByte:  0,
		OrigEndByte:    uint32(len(srcBytes)),
		Node:           &NodeRef{Type: "if_statement", StartByte: 0, EndByte: 9},
		DestStartByte:  ptr(uint32(0)),
		DestEndByte:    ptr(uint32(9)),
	}

	actions := []Action{act}
	closingDelimiter := DelimiterSpan{
		StartByte: uint32(len(srcBytes) - 2), // '}'
		EndByte:   uint32(len(srcBytes) - 1),
		Action:    "move",
		ActionRef: &actions[0],
	}

	spans := BuildHighlightSpans(srcBytes, actions, "left", closingDelimiter)

	var openSpan, closeSpan *HighlightSpan
	for i := range spans {
		if spans[i].Line == 0 && spans[i].Action == "move" {
			openSpan = &spans[i]
		}
		if spans[i].Line == 9 && spans[i].Action == "move" {
			closeSpan = &spans[i]
		}
	}

	if openSpan == nil || closeSpan == nil {
		t.Fatalf("expected both opening and closing move spans, got open=%v, close=%v", openSpan, closeSpan)
	}
	if openSpan.ColorIndex != 0 {
		t.Errorf("expected multiline construct header to mute to ColorIndex 0 (Teal), got %d", openSpan.ColorIndex)
	}
	if closeSpan.ColorIndex != 0 {
		t.Errorf("expected multiline construct closing delimiter to mute to ColorIndex 0 (Teal), got %d", closeSpan.ColorIndex)
	}
}

func TestCaseClauseDelimiterAbsorption(t *testing.T) {
	t.Run("ZigSwitchCaseArrow", func(t *testing.T) {
		fileBytes := []byte("        .explicit => |lib_name| try w.print();\n")
		// .explicit: bytes 8..17
		// =>: bytes 18..20
		// |lib_name|: bytes 21..31
		parent := &NodeRef{
			Type:      "switch_case",
			StartByte: 8,
			EndByte:   46,
		}

		actions := []Action{
			{
				Action: "insert",
				Node:   &NodeRef{Type: "field_expression", StartByte: 8, EndByte: 17},
				Parent: parent,
			},
			{
				Action: "insert",
				Node:   &NodeRef{Type: "payload", StartByte: 21, EndByte: 31},
				Parent: parent,
			},
		}

		spans := BuildHighlightSpans(fileBytes, actions, "right")
		if len(spans) != 1 {
			t.Fatalf("expected 1 merged span for case pattern and payload, got %d: %+v", len(spans), spans)
		}
		if spans[0].StartCol != 8 || spans[0].EndCol != 31 {
			t.Errorf("expected span cols 8..31, got %d..%d", spans[0].StartCol, spans[0].EndCol)
		}
		if spans[0].Action != "insert" {
			t.Errorf("expected action 'insert', got %q", spans[0].Action)
		}
	})

	t.Run("RustMatchArmArrow", func(t *testing.T) {
		fileBytes := []byte("    Some(v) => foo();\n")
		// Some(v): bytes 4..11
		// =>: bytes 12..14
		parent := &NodeRef{
			Type:      "match_arm",
			StartByte: 4,
			EndByte:   22,
		}

		actions := []Action{
			{
				Action: "insert",
				Node:   &NodeRef{Type: "match_pattern", StartByte: 4, EndByte: 11},
				Parent: parent,
			},
		}

		spans := BuildHighlightSpans(fileBytes, actions, "right")
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		// Some(v) => should be absorbed (without trailing space)
		if spans[0].StartCol != 4 || spans[0].EndCol != 14 {
			t.Errorf("expected span cols 4..14 (including '=>'), got %d..%d", spans[0].StartCol, spans[0].EndCol)
		}
	})

	t.Run("JavaSwitchRuleArrow", func(t *testing.T) {
		fileBytes := []byte("    case 1 -> foo();\n")
		// case 1: bytes 4..10
		// ->: bytes 11..13
		parent := &NodeRef{
			Type:      "switch_rule",
			StartByte: 4,
			EndByte:   21,
		}

		actions := []Action{
			{
				Action: "delete",
				Node:   &NodeRef{Type: "switch_label", StartByte: 4, EndByte: 10},
				Parent: parent,
			},
		}

		spans := BuildHighlightSpans(fileBytes, actions, "left")
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		// case 1 -> should be absorbed (without trailing space)
		if spans[0].StartCol != 4 || spans[0].EndCol != 13 {
			t.Errorf("expected span cols 4..13 (including '->'), got %d..%d", spans[0].StartCol, spans[0].EndCol)
		}
	})
}

func TestBuildHighlightSpans_SyntaxTriviaAbsorptionAndCommentPreservation(t *testing.T) {
	t.Run("TriviaAdjacentCommaAbsorbed", func(t *testing.T) {
		fileBytes := []byte("foo(a, b);\n")
		// a: bytes 4..5
		// ,: bytes 5..6
		astIndex := &treesitter.ASTIndex{
			SyntaxTrivia: []treesitter.SyntaxTriviaBlock{
				{
					TrailingStart: 5,
					TrailingEnd:   6,
				},
			},
		}
		astNode := &treesitter.ASTNode{
			ID:        0,
			StartByte: 4,
			EndByte:   5,
			Index:     astIndex,
		}

		actions := []Action{
			{
				Action:  "delete",
				Node:    &NodeRef{Type: "identifier", StartByte: 4, EndByte: 5},
				ASTNode: astNode,
				Parent:  &NodeRef{Type: "argument_list", StartByte: 3, EndByte: 9},
			},
		}

		spans := BuildHighlightSpans(fileBytes, actions, "left")
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		// "a," should be absorbed (endCol at 6, exactly trivia.TrailingEnd)
		if spans[0].StartCol != 4 || spans[0].EndCol != 6 {
			t.Errorf("expected span cols 4..6, got %d..%d", spans[0].StartCol, spans[0].EndCol)
		}
	})

	t.Run("TriviaInterveningCommentNotSwallowed", func(t *testing.T) {
		fileBytes := []byte("foo(a /* comment */, b);\n")
		// a: bytes 4..5
		// /* comment */: bytes 6..19
		// ,: bytes 19..20
		astIndex := &treesitter.ASTIndex{
			SyntaxTrivia: []treesitter.SyntaxTriviaBlock{
				{
					TrailingStart: 19,
					TrailingEnd:   20,
				},
			},
		}
		astNode := &treesitter.ASTNode{
			ID:        0,
			StartByte: 4,
			EndByte:   5,
			Index:     astIndex,
		}

		actions := []Action{
			{
				Action:  "delete",
				Node:    &NodeRef{Type: "identifier", StartByte: 4, EndByte: 5},
				ASTNode: astNode,
				Parent:  &NodeRef{Type: "argument_list", StartByte: 3, EndByte: 23},
			},
		}

		spans := BuildHighlightSpans(fileBytes, actions, "left")
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		// Keep the span at endCol 5 so it doesn't swallow the comment.
		if spans[0].StartCol != 4 || spans[0].EndCol != 5 {
			t.Errorf("expected span cols 4..5 (comment preserved untouched), got %d..%d", spans[0].StartCol, spans[0].EndCol)
		}
	})
}

func TestBuildHighlightSpans_BalancedBracketGapMerging(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		t.Run("DoesNotBridgeOpeningParenWhenInteriorPartiallyPreserved", func(t *testing.T) {
			// "demoteMoveToDelIns(result, ms, a.Node, dstNode)\n"
			//  012345678901234567890123456789012345678901234567
			//  demoteMoveToDelIns: 0..18
			//  (: 18..19
			//  result: 19..25
			//  ms: 27..29
			//  a.Node: 31..37 (preserved)
			//  dstNode: 39..46
			//  ): 46..47
			fileBytes := []byte("demoteMoveToDelIns(result, ms, a.Node, dstNode)\n")
			argListParent := &NodeRef{Type: "argument_list", StartByte: 18, EndByte: 47}
			callParent := &NodeRef{Type: "call_expression", StartByte: 0, EndByte: 47}

			actions := []Action{
				{
					Action: "delete",
					Node:   &NodeRef{Type: "identifier", StartByte: 0, EndByte: 18},
					Parent: callParent,
				},
				{
					Action: "delete",
					Node:   &NodeRef{Type: "identifier", StartByte: 19, EndByte: 25},
					Parent: argListParent,
				},
				{
					Action: "delete",
					Node:   &NodeRef{Type: "identifier", StartByte: 27, EndByte: 29},
					Parent: argListParent,
				},
				{
					Action: "delete",
					Node:   &NodeRef{Type: "identifier", StartByte: 39, EndByte: 46},
					Parent: argListParent,
				},
			}

			spans := BuildHighlightSpans(fileBytes, actions, "left")
			if len(spans) != 3 {
				t.Fatalf("expected 3 separate delete spans (callee, leading args, trailing arg), got %d: %+v", len(spans), spans)
			}
			if spans[0].StartCol != 0 || spans[0].EndCol != 18 {
				t.Errorf("expected callee span [0..18] without '(', got [%d..%d]", spans[0].StartCol, spans[0].EndCol)
			}
			if spans[1].StartCol != 19 || spans[1].EndCol != 30 {
				t.Errorf("expected leading args span [19..30] ('result, ms,'), got [%d..%d]", spans[1].StartCol, spans[1].EndCol)
			}
			if spans[2].StartCol != 37 || spans[2].EndCol != 46 {
				t.Errorf("expected trailing arg span [37..46] (', dstNode') without ')', got [%d..%d]", spans[2].StartCol, spans[2].EndCol)
			}
		})

		t.Run("BridgesOpeningParenAndClosesTrailingParenWhenEntireCallInteriorDeleted", func(t *testing.T) {
			// "foo(a, b)\n"
			//  foo: 0..3, (: 3..4, a: 4..5, b: 7..8, ): 8..9
			fileBytes := []byte("foo(a, b)\n")
			argListParent := &NodeRef{Type: "argument_list", StartByte: 3, EndByte: 9}
			callParent := &NodeRef{Type: "call_expression", StartByte: 0, EndByte: 9}

			actions := []Action{
				{
					Action: "delete",
					Node:   &NodeRef{Type: "identifier", StartByte: 0, EndByte: 3},
					Parent: callParent,
				},
				{
					Action: "delete",
					Node:   &NodeRef{Type: "identifier", StartByte: 4, EndByte: 5},
					Parent: argListParent,
				},
				{
					Action: "delete",
					Node:   &NodeRef{Type: "identifier", StartByte: 7, EndByte: 8},
					Parent: argListParent,
				},
			}

			spans := BuildHighlightSpans(fileBytes, actions, "left")
			if len(spans) != 1 {
				t.Fatalf("expected 1 merged span covering 'foo(a, b)', got %d: %+v", len(spans), spans)
			}
			if spans[0].StartCol != 0 || spans[0].EndCol != 9 {
				t.Errorf("expected balanced span [0..9] covering 'foo(a, b)', got [%d..%d]", spans[0].StartCol, spans[0].EndCol)
			}
		})

		t.Run("DoesNotBridgeUnmatchedClosingBraceInGap", func(t *testing.T) {
			// "return .{ .name = val };\n"
			//  val: 18..21, " }": 21..23, ";": 23..24
			fileBytes := []byte("return .{ .name = val };\n")
			actions := []Action{
				{
					Action: "insert",
					Node:   &NodeRef{Type: "identifier", StartByte: 18, EndByte: 21},
				},
				{
					Action: "insert",
					Node:   &NodeRef{Type: ";", StartByte: 23, EndByte: 24},
				},
			}

			spans := BuildHighlightSpans(fileBytes, actions, "right")
			if len(spans) != 2 {
				t.Fatalf("expected 2 separate insert spans without swallowing '}', got %d: %+v", len(spans), spans)
			}
			if spans[0].StartCol != 18 || spans[0].EndCol != 21 {
				t.Errorf("expected span 0 [18..21], got [%d..%d]", spans[0].StartCol, spans[0].EndCol)
			}
			if spans[1].StartCol != 23 || spans[1].EndCol != 24 {
				t.Errorf("expected span 1 [23..24], got [%d..%d]", spans[1].StartCol, spans[1].EndCol)
			}
		})
	})

	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `package main

func process(target string) {
	oldHelper("first", target)
}
`
		newSrc := `package main

func process(target string) {
	newHelper("second", target)
}
`
		oldAST, err := treesitter.Parse([]byte(oldSrc), "test.go")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse([]byte(newSrc), "test.go")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		part := engine.NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := engine.Match(oldAST, newAST, []byte(oldSrc), []byte(newSrc), part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, []byte(oldSrc), []byte(newSrc), EnvelopeOptions{
			IncludeHighlights: true,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		// On left line 3 (`\toldHelper("first", target)`):
		// oldHelper (1..10) is updated, "first", (11..18) is updated,
		// and '(' at column 10 must NOT be swallowed into an update span.
		var leftLine3 []HighlightSpan
		for _, h := range env.LeftHighlights {
			if h.Line == 3 {
				leftLine3 = append(leftLine3, h)
			}
		}
		if len(leftLine3) != 2 {
			t.Fatalf("expected 2 separate update spans on left line 3, got %d: %+v", len(leftLine3), leftLine3)
		}
		if leftLine3[0].StartCol != 1 || leftLine3[0].EndCol != 10 {
			t.Errorf("expected left callee span [1..10], got [%d..%d]", leftLine3[0].StartCol, leftLine3[0].EndCol)
		}
		if leftLine3[1].StartCol != 11 || leftLine3[1].EndCol != 18 {
			t.Errorf("expected left arg span [11..18], got [%d..%d]", leftLine3[1].StartCol, leftLine3[1].EndCol)
		}

		// On right line 3 (`\tnewHelper("second", target)`):
		// newHelper (1..10) is updated, "second", (11..19) is updated,
		// and '(' at column 10 must remain unhighlighted context.
		var rightLine3 []HighlightSpan
		for _, h := range env.RightHighlights {
			if h.Line == 3 {
				rightLine3 = append(rightLine3, h)
			}
		}
		if len(rightLine3) != 2 {
			t.Fatalf("expected 2 separate update spans on right line 3, got %d: %+v", len(rightLine3), rightLine3)
		}
		if rightLine3[0].StartCol != 1 || rightLine3[0].EndCol != 10 {
			t.Errorf("expected right callee span [1..10], got [%d..%d]", rightLine3[0].StartCol, rightLine3[0].EndCol)
		}
		if rightLine3[1].StartCol != 11 || rightLine3[1].EndCol != 19 {
			t.Errorf("expected right arg span [11..19], got [%d..%d]", rightLine3[1].StartCol, rightLine3[1].EndCol)
		}

		// Whole-snippet context line purity: unchanged lines (0, 1, 2, 4) must have zero highlights
		assertContextLinesPure(t, env.LeftHighlights, []int{3})
		assertContextLinesPure(t, env.RightHighlights, []int{3})
	})
}

// assertContextLinesPure verifies that lines outside of allowedLines have zero highlight spans.
func assertContextLinesPure(t *testing.T, spans []HighlightSpan, allowedLines []int) {
	t.Helper()
	allowed := make(map[int]bool, len(allowedLines))
	for _, l := range allowedLines {
		allowed[l] = true
	}
	for _, s := range spans {
		if !allowed[s.Line] {
			t.Errorf("line %d expected to be pure context (white), but found %s highlight span [%d..%d]",
				s.Line, s.Action, s.StartCol, s.EndCol)
		}
	}
}

func TestResolveMoveColor(t *testing.T) {
	if got := ResolveMoveColor(nil, 0, false); got != 0 {
		t.Errorf("expected nil action to return color 0, got %d", got)
	}

	act := &Action{
		Action:         "move",
		MoveColorIndex: 1,
	}

	if got := ResolveMoveColor(act, 1, false); got != 1 {
		t.Errorf("expected single-line ungrouped move to keep color 1, got %d", got)
	}

	if got := ResolveMoveColor(act, 5, false); got != 0 {
		t.Errorf("expected multi-line move to collapse to 0, got %d", got)
	}

	actInGroup := &Action{
		Action:         "move",
		GroupID:        "group-1",
		MoveColorIndex: 1,
	}
	if got := ResolveMoveColor(actInGroup, 1, true); got != 0 {
		t.Errorf("expected single-line move in multi-line group to collapse to 0, got %d", got)
	}

	if got := ResolveMoveColor(actInGroup, 1, false); got != 1 {
		t.Errorf("expected single-line move in single-line group to keep color 1, got %d", got)
	}

	// Standalone outlier moves:
	ungroupedAct := &Action{
		Action:         "move",
		MoveColorIndex: 1,
	}

	// Standalone move with lineCount <= MaxUngroupedHighlightLines (3) keeps slot 1 (Magenta).
	for lines := 1; lines <= MaxUngroupedHighlightLines; lines++ {
		if got := ResolveMoveColor(ungroupedAct, lines, false); got != 1 {
			t.Errorf("expected ungrouped move with %d lines to retain color 1, got %d", lines, got)
		}
	}

	// Standalone move with lineCount > MaxUngroupedHighlightLines (e.g. 4 lines) collapses to slot 0 (Cyan).
	if got := ResolveMoveColor(ungroupedAct, 4, false); got != 0 {
		t.Errorf("expected ungrouped move with 4 lines to collapse to 0, got %d", got)
	}
	if got := ResolveMoveColor(ungroupedAct, 10, false); got != 0 {
		t.Errorf("expected ungrouped move with 10 lines to collapse to 0, got %d", got)
	}
}
