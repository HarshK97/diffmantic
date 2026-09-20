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
		destStart := uint32(34)
		destEnd := uint32(45)
		actions := []Action{
			{
				Action:        "move",
				Node:          &NodeRef{Type: "literal_element", StartByte: 17, EndByte: 28},
				OldParent:     &NodeRef{Type: "literal_value", StartByte: 12, EndByte: 49},
				DestStartByte: &destStart,
				DestEndByte:   &destEnd,
				Parent:        &NodeRef{Type: "literal_value", StartByte: 12, EndByte: 49},
			},
		}

		leftSpans := BuildHighlightSpans(leftSrc, actions, "left")
		if len(leftSpans) != 1 {
			t.Fatalf("expected 1 left span, got %d", len(leftSpans))
		}
		if leftSpans[0].Line != 1 || leftSpans[0].StartCol != 4 || leftSpans[0].EndCol != 16 {
			t.Errorf("expected left span line 1 cols 4..16 covering '{name: \"a\"},', got line %d cols %d..%d",
				leftSpans[0].Line, leftSpans[0].StartCol, leftSpans[0].EndCol)
		}

		rightSpans := BuildHighlightSpans(rightSrc, actions, "right")
		if len(rightSpans) != 1 {
			t.Fatalf("expected 1 right span, got %d", len(rightSpans))
		}
		if rightSpans[0].Line != 2 || rightSpans[0].StartCol != 4 || rightSpans[0].EndCol != 16 {
			t.Errorf("expected right span line 2 cols 4..16 covering '{name: \"a\"},', got line %d cols %d..%d",
				rightSpans[0].Line, rightSpans[0].StartCol, rightSpans[0].EndCol)
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
