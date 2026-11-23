package serialize

import (
	"slices"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/comments"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/postprocess"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
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

func TestBuildHighlightSpans_LuaClosureDelimiterDoesNotMoveIndentation(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`local function processItems(items)
    precedingInit()
    local currTotal = total(items)
    local currDouble = twice(currTotal)
    local currAffinity = score(currDouble)
    succeedingDone(currTotal, currDouble, currAffinity)
end
`)
		dst := []byte(`local function processItems(items)
    precedingInit()
    local currTotal = 0
    local currDouble = 0
    local currAffinity = 0

    step01()
    step02()
    step03()
    step04()
    step05()
    step06()
    step07()
    step08()
    step09()
    step10()
    step11()

    local ensureMetrics = function()
        currTotal = total(items)
        currDouble = twice(currTotal)
        currAffinity = score(currDouble)
    end
    ensureMetrics()
    succeedingDone(currTotal, currDouble, currAffinity)
end
`)
		srcAST, err := treesitter.Parse(src, "main.lua")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "main.lua")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("snippets must parse cleanly: src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}
		part := engine.NewLinePartition(src, dst)
		mr := engine.Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}
		es := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		es = postprocess.Run(es, mr.Mappings, srcAST, dstAST)
		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, srcAST, dstAST, src, dst, EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions: %v", err)
		}

		r := rules.Get("lua")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"processItems", "processItems", "function_declaration",
		)
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			testutil.TextQuery{Text: "(items)", Type: "parameters", Occurrence: 1},
			testutil.TextQuery{Text: "(items)", Type: "parameters", Occurrence: 1},
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"precedingInit()", "precedingInit()", "function_call",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"succeedingDone", "succeedingDone", "function_call",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"currTotal = total(items)", "currTotal = total(items)", "assignment_statement",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"currDouble = twice(currTotal)", "currDouble = twice(currTotal)", "assignment_statement",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"currAffinity = score(currDouble)", "currAffinity = score(currDouble)", "assignment_statement",
		)

		dstClosure, err := testutil.FindNodeByText(dstAST, dst, "function()", "function_definition")
		if err != nil {
			t.Fatalf("ensureMetrics closure not found: %v", err)
		}

		testutil.AssertTagged(t, string(src), toTestSpans(env.LeftHighlights), `local function processItems(items)
    precedingInit()
    local <mov>currTotal = total(items)</mov>
    local <mov>currDouble = twice(currTotal)</mov>
    local <mov>currAffinity = score(currDouble)</mov>
    succeedingDone(currTotal, currDouble, currAffinity)
end
`)

		testutil.AssertTagged(t, string(dst), toTestSpans(env.RightHighlights), `local function processItems(items)
    precedingInit()
    local <ins>currTotal = 0</ins>
    local <ins>currDouble = 0</ins>
    local <ins>currAffinity = 0</ins>

    <ins>step01()</ins>
    <ins>step02()</ins>
    <ins>step03()</ins>
    <ins>step04()</ins>
    <ins>step05()</ins>
    <ins>step06()</ins>
    <ins>step07()</ins>
    <ins>step08()</ins>
    <ins>step09()</ins>
    <ins>step10()</ins>
    <ins>step11()</ins>

    <ins>local ensureMetrics = function()</ins>
<ins>        </ins><mov>currTotal = total(items)</mov>
<ins>        </ins><mov>currDouble = twice(currTotal)</mov>
<ins>        </ins><mov>currAffinity = score(currDouble)</mov>
<ins>    end</ins>
    <ins>ensureMetrics()</ins>
    succeedingDone(currTotal, currDouble, currAffinity)
end
`)
		var closingEnd *treesitter.ASTNode
		for _, n := range dstClosure.Children {
			if r.IsKeyword(n.Type, n.Label) && n.Label == "end" {
				closingEnd = n
			}
		}
		if closingEnd == nil {
			t.Fatal("ensureMetrics closing end not found")
		}
		if !slices.ContainsFunc(env.Actions, func(a Action) bool {
			return a.Action == "insert" && a.Node != nil && a.ASTNode == closingEnd
		}) {
			t.Error("ensureMetrics closing end has no Insert action")
		}
	})
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
		testutil.AssertTagged(t, string(src), toTestSpans(spans), "func f(x <ins>gin.</ins>HandlerFunc) {}\n")
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
		testutil.AssertTagged(t, string(src), toTestSpans(spans), "x := obj<ins>.field</ins>\n")
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
		testutil.AssertTagged(t, string(src), toTestSpans(spans), "<ins>std::</ins>vector<int> v;\n")
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
		testutil.AssertTagged(t, string(src), toTestSpans(spans), "func f(args ...<ins>gin.</ins>HandlerFunc) {}\n")
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
		testutil.AssertTagged(t, string(src), toTestSpans(spans), "foo(a, b<ins>, c</ins>)\n")
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
		testutil.AssertTagged(t, string(src), toTestSpans(spans), "foo(<ins>a,</ins> b)\n")
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
		testutil.AssertTagged(t, string(src), toTestSpans(spans), "foo(a, <del>b,</del> c)\n")
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
		testutil.AssertTagged(t, string(src), toTestSpans(spans), "foo(\n    a,\n    <ins>b,</ins>\n)\n")
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
		testutil.AssertTagged(t, string(src), toTestSpans(spans), "x = [1, 2<ins>, 3</ins>]\n")
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
		testutil.AssertTagged(t, string(leftSrc), toTestSpans(leftSpans), "vals := []T{\n    <mov>{name: \"a\"}</mov><del>,</del>\n    {name: \"b\"},\n}\n")

		rightSpans := BuildHighlightSpans(rightSrc, actions, "right")
		testutil.AssertTagged(t, string(rightSrc), toTestSpans(rightSpans), "vals := []T{\n    {name: \"b\"},\n    <mov>{name: \"a\"}</mov><ins>,</ins>\n}\n")
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
		testutil.AssertTagged(t, string(leftSrc), toTestSpans(leftSpans), "foo(a<del>,</del> <mov>b</mov>)")

		rightSpans := BuildHighlightSpans(rightSrc, actions, "right")
		testutil.AssertTagged(t, string(rightSrc), toTestSpans(rightSpans), "bar(x<ins>,</ins> <mov>b</mov>)")
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
		testutil.AssertTagged(t, string(src), toTestSpans(spans), "block {\n    <ins>foo</ins>,\n}\n")
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
		testutil.AssertTagged(t, string(src), toTestSpans(spans), "foo(<ins>a,</ins> b)")
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

	partitioned := partitionLineSpans(spans, "left", nil, nil)
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

	partitioned := partitionLineSpans(spans, "right", nil, nil)
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
	// Standalone 1-line moves keep slot 1 (Magenta). Grouped moves keep their
	// assigned color so siblings share the same color.
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
	if multiSpan.ColorIndex != 1 {
		t.Errorf("expected multiline move in group to honor ColorIndex 1 (Magenta), got %d", multiSpan.ColorIndex)
	}

	if groupedSingleSpan == nil {
		t.Fatalf("expected grouped single-line move span on line 4")
	}
	if groupedSingleSpan.ColorIndex != 1 {
		t.Errorf("expected single-line move in GroupID to share ColorIndex 1 (Magenta), got %d", groupedSingleSpan.ColorIndex)
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

	// finish() is single-line on both sides, grouped with a multiline if-statement.
	// Both sides keep ColorIndex 0 (<mov>) so the group shares one color.
	testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

func process() {
	<mov>isolatedMove()</mov>
	stationaryA()
	stationaryB()
	<mov>if check() {</mov>
<mov>		doWork()</mov>
<mov>	}</mov>
	<mov>finish()</mov>
}
`)

	testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func process() {
	stationaryA()
	stationaryB()
	<mov>isolatedMove()</mov>
	<ins>if wrapper() {</ins>
		<mov>if check() {</mov>
<mov>			doWork()</mov>
<mov>		}</mov>
		<mov>finish()</mov>
	<ins>}</ins>
}
`)
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
		testutil.AssertTagged(t, string(fileBytes), toTestSpans(spans), "        <ins>.explicit => |lib_name|</ins> try w.print();\n")
	})

	t.Run("RustMatchArmArrow", func(t *testing.T) {
		fileBytes := []byte("    Some(v) => foo();\n")
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
		testutil.AssertTagged(t, string(fileBytes), toTestSpans(spans), "    <ins>Some(v) =></ins> foo();\n")
	})

	t.Run("JavaSwitchRuleArrow", func(t *testing.T) {
		fileBytes := []byte("    case 1 -> foo();\n")
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
		testutil.AssertTagged(t, string(fileBytes), toTestSpans(spans), "    <del>case 1 -></del> foo();\n")
	})
}

func TestBuildHighlightSpans_SyntaxTriviaAbsorptionAndCommentPreservation(t *testing.T) {
	t.Run("TriviaAdjacentCommaAbsorbed", func(t *testing.T) {
		fileBytes := []byte("package main\n\nfunc main() {\n\tfoo(a, b)\n}\n")
		root, err := treesitter.Parse(fileBytes, "main.go")
		if err != nil || root == nil {
			t.Fatal("failed to parse Go source")
		}

		var childA, argListNode *treesitter.ASTNode
		for _, d := range root.Descendants() {
			if d.Type == "argument_list" {
				argListNode = d
				if len(d.Children) > 0 {
					childA = d.Children[0]
				}
				break
			}
		}
		if childA == nil || argListNode == nil {
			t.Fatal("expected to find childA and argListNode")
		}

		actions := []Action{
			{
				Action:  "delete",
				Node:    &NodeRef{Type: childA.Type, StartByte: childA.StartByte, EndByte: childA.EndByte},
				ASTNode: childA,
				Parent:  &NodeRef{Type: argListNode.Type, StartByte: argListNode.StartByte, EndByte: argListNode.EndByte},
			},
		}

		spans := BuildHighlightSpans(fileBytes, actions, "left")
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		// "a," should be absorbed (endCol includes comma)
		trivia := childA.SyntaxTrivia()
		if trivia == nil {
			t.Fatal("expected childA to have syntax trivia")
		}
		expectedEndCol := int(childA.StartCol) + int(trivia.TrailingEnd-childA.StartByte)
		if spans[0].StartCol != int(childA.StartCol) || spans[0].EndCol != expectedEndCol {
			t.Errorf("expected span cols %d..%d, got %d..%d", childA.StartCol, expectedEndCol, spans[0].StartCol, spans[0].EndCol)
		}
	})

	t.Run("TriviaInterveningCommentNotSwallowed", func(t *testing.T) {
		fileBytes := []byte("package main\n\nfunc main() {\n\tfoo(a /* comment */, b)\n}\n")
		root, err := treesitter.Parse(fileBytes, "main.go")
		if err != nil || root == nil {
			t.Fatal("failed to parse Go source")
		}

		var childA, argListNode *treesitter.ASTNode
		for _, d := range root.Descendants() {
			if d.Type == "argument_list" {
				argListNode = d
				if len(d.Children) > 0 {
					childA = d.Children[0]
				}
				break
			}
		}
		if childA == nil || argListNode == nil {
			t.Fatal("expected to find childA and argListNode")
		}

		actions := []Action{
			{
				Action:  "delete",
				Node:    &NodeRef{Type: childA.Type, StartByte: childA.StartByte, EndByte: childA.EndByte},
				ASTNode: childA,
				Parent:  &NodeRef{Type: argListNode.Type, StartByte: argListNode.StartByte, EndByte: argListNode.EndByte},
			},
		}

		spans := BuildHighlightSpans(fileBytes, actions, "left")
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		// Keep the span at childA.EndCol so it doesn't swallow the comment.
		if spans[0].StartCol != int(childA.StartCol) || spans[0].EndCol != int(childA.EndCol) {
			t.Errorf("expected span cols %d..%d (comment preserved untouched), got %d..%d",
				childA.StartCol, childA.EndCol, spans[0].StartCol, spans[0].EndCol)
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

		// intentionally no postprocess.Run: isolates serialization bracket gap merging from move normalization
		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, []byte(oldSrc), []byte(newSrc), EnvelopeOptions{
			IncludeHighlights: true,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		// Whole-snippet tagged assertions: callee and first arg updated, '(' and 'target' silent context,
		// and all other lines (package, func declaration, closing brace) completely silent.
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

func process(target string) {
	<upd>oldHelper</upd>(<upd>"first"</upd>, target)
}
`)

		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func process(target string) {
	<upd>newHelper</upd>(<upd>"second"</upd>, target)
}
`)
	})
}

func toTestSpans(spans []HighlightSpan) []testutil.Span {
	res := make([]testutil.Span, len(spans))
	for i, s := range spans {
		res[i] = testutil.Span{
			Line:       s.Line,
			StartCol:   s.StartCol,
			EndCol:     s.EndCol,
			Action:     s.Action,
			ColorIndex: s.ColorIndex,
		}
	}
	return res
}

func TestResolveMoveColor(t *testing.T) {
	if got := ResolveMoveColor(nil, 0); got != 0 {
		t.Errorf("expected nil action to return color 0, got %d", got)
	}

	act := &Action{
		Action:         "move",
		MoveColorIndex: 1,
	}

	if got := ResolveMoveColor(act, 1); got != 1 {
		t.Errorf("expected single-line ungrouped move to keep color 1, got %d", got)
	}

	if got := ResolveMoveColor(act, 5); got != 0 {
		t.Errorf("expected multi-line move to collapse to 0, got %d", got)
	}

	actInGroup := &Action{
		Action:         "move",
		GroupID:        "group-1",
		MoveColorIndex: 1,
	}
	if got := ResolveMoveColor(actInGroup, 1); got != 1 {
		t.Errorf("expected move in group to honor assigned MoveColorIndex 1, got %d", got)
	}

	if got := ResolveMoveColor(actInGroup, 5); got != 1 {
		t.Errorf("expected multiline move in group to honor assigned MoveColorIndex 1, got %d", got)
	}

	// Standalone outlier moves:
	ungroupedAct := &Action{
		Action:         "move",
		MoveColorIndex: 1,
	}

	// Standalone move with lineCount <= MaxUngroupedHighlightLines (3) keeps slot 1 (Magenta).
	for lines := 1; lines <= MaxUngroupedHighlightLines; lines++ {
		if got := ResolveMoveColor(ungroupedAct, lines); got != 1 {
			t.Errorf("expected ungrouped move with %d lines to retain color 1, got %d", lines, got)
		}
	}

	// Standalone move with lineCount > MaxUngroupedHighlightLines (e.g. 4 lines) collapses to slot 0 (Cyan).
	if got := ResolveMoveColor(ungroupedAct, 4); got != 0 {
		t.Errorf("expected ungrouped move with 4 lines to collapse to 0, got %d", got)
	}
	if got := ResolveMoveColor(ungroupedAct, 10); got != 0 {
		t.Errorf("expected ungrouped move with 10 lines to collapse to 0, got %d", got)
	}
}

func TestContainerDelimiterMoveParity(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		// nodeLen should use the full construct range, not the clipped header.
		insertAct := &Action{
			Action:            "insert",
			OrigStartByte:     100,
			OrigEndByte:       250,
			DestOrigStartByte: 100,
			DestOrigEndByte:   250,
			Node: &NodeRef{
				StartByte: 100,
				EndByte:   130, // clipped to header line
			},
		}
		if got := nodeLen(insertAct, "right"); got != 150 {
			t.Errorf("expected nodeLen on right to return unclipped 150, got %d", got)
		}

		deleteAct := &Action{
			Action:        "delete",
			OrigStartByte: 100,
			OrigEndByte:   250,
			Node: &NodeRef{
				StartByte: 100,
				EndByte:   130, // clipped to header line
			},
		}
		if got := nodeLen(deleteAct, "left"); got != 150 {
			t.Errorf("expected nodeLen on left to return unclipped 150, got %d", got)
		}

		// If a moved block and an inserted container both claim the closing brace, the move wins.
		fileBytes := []byte("func helper() {\n\treturn\n}\n")
		lineIndex := BuildLineIndex(fileBytes)
		moveAct := &Action{
			Action:            "move",
			DestOrigStartByte: 14,
			DestOrigEndByte:   25,
		}
		spans := []internalSpan{
			{startCol: 0, endCol: 1, action: "insert", actRef: insertAct},
			{startCol: 0, endCol: 1, action: "move", actRef: moveAct},
		}
		partitioned := partitionLineSpans(spans, "right", lineIndex, fileBytes)
		if len(partitioned) != 1 || partitioned[0].action != "move" {
			t.Fatalf("expected child block move to beat container insert footer, got: %+v", partitioned)
		}
	})

	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `package main

func Process(condition bool) error {
	if condition {
		step1 := validate()
		step2 := transform(step1)
		step3 := persist(step2)
		notify(step3)
		logAudit(step3)
		return nil
	}
	return nil
}
`
		newSrc := `package main

func Process(condition bool) error {
	if condition {
		return handleCondition()
	}
	return nil
}

func handleCondition() error {
	step1 := validate()
	step2 := transform(step1)
	step3 := persist(step2)
	notify(step3)
	logAudit(step3)
	return nil
}
`
		oldAST, err := treesitter.Parse([]byte(oldSrc), "old.go")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse([]byte(newSrc), "new.go")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		part := engine.NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := engine.Match(oldAST, newAST, []byte(oldSrc), []byte(newSrc), part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)

		// intentionally no postprocess.Run: isolates container footer delimiter move serialization from move normalization
		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, []byte(oldSrc), []byte(newSrc), EnvelopeOptions{
			IncludeHighlights: true,
			IncludeActions:    true,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		// Preserve explicit closing brace span count to guard against touching span fragmentation.
		closingBraceCount := 0
		for _, h := range env.RightHighlights {
			if h.Line == 16 {
				closingBraceCount++
			}
		}
		if closingBraceCount != 1 {
			t.Fatalf("expected exactly 1 highlight span on right line 16, got %d", closingBraceCount)
		}

		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

func Process(condition bool) error {
	<del>if condition</del> <mov>{</mov>
<mov>		step1 := validate()</mov>
<mov>		step2 := transform(step1)</mov>
<mov>		step3 := persist(step2)</mov>
<mov>		notify(step3)</mov>
<mov>		logAudit(step3)</mov>
<mov>		return nil</mov>
<mov>	}</mov>
	return nil
}
`)

		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func Process(condition bool) error {
	<ins>if condition {</ins>
		<ins>return handleCondition()</ins>
	<ins>}</ins>
	return nil
}

<ins>func handleCondition() error</ins> <mov>{</mov>
<mov>	step1 := validate()</mov>
<mov>	step2 := transform(step1)</mov>
<mov>	step3 := persist(step2)</mov>
<mov>	notify(step3)</mov>
<mov>	logAudit(step3)</mov>
<mov>	return nil</mov>
<mov>}</mov>
`)

		// Inlined back: closing brace on line 10 should stay a move instead of taking a delete footer.
		partRev := engine.NewLinePartition([]byte(newSrc), []byte(oldSrc))
		mrRev := engine.Match(newAST, oldAST, []byte(newSrc), []byte(oldSrc), partRev)
		esRev := actions.GenerateEditScript(newAST, oldAST, mrRev.Mappings)

		// intentionally no postprocess.Run: isolates reverse container footer delimiter move serialization from move normalization
		envRev, err := BuildEnvelopeWithOptions(esRev, mrRev.Mappings, newAST, oldAST, []byte(newSrc), []byte(oldSrc), EnvelopeOptions{
			IncludeHighlights: true,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions reverse failed: %v", err)
		}

		revClosingCount := 0
		for _, h := range envRev.RightHighlights {
			if h.Line == 10 {
				revClosingCount++
			}
		}
		if revClosingCount != 1 {
			t.Fatalf("expected exactly 1 highlight span on reverse right line 10, got %d", revClosingCount)
		}

		testutil.AssertTagged(t, newSrc, toTestSpans(envRev.LeftHighlights), `package main

func Process(condition bool) error {
	<del>if condition {</del>
		<del>return handleCondition()</del>
	<del>}</del>
	return nil
}

<del>func handleCondition() error</del> <mov>{</mov>
<mov>	step1 := validate()</mov>
<mov>	step2 := transform(step1)</mov>
<mov>	step3 := persist(step2)</mov>
<mov>	notify(step3)</mov>
<mov>	logAudit(step3)</mov>
<mov>	return nil</mov>
<mov>}</mov>
`)

		testutil.AssertTagged(t, oldSrc, toTestSpans(envRev.RightHighlights), `package main

func Process(condition bool) error {
	<ins>if condition</ins> <mov>{</mov>
<mov>		step1 := validate()</mov>
<mov>		step2 := transform(step1)</mov>
<mov>		step3 := persist(step2)</mov>
<mov>		notify(step3)</mov>
<mov>		logAudit(step3)</mov>
<mov>		return nil</mov>
<mov>	}</mov>
	return nil
}
`)
	})
}

func TestCommentTriviaHighlighting_Unit(t *testing.T) {
	// Verify that DelimiterSpan move actions inherit the anchor's color index.
	srcBytes := []byte("// Doc for relocated function\nfunc Relocated() {}\n")

	act := Action{
		Action:         "move",
		MoveColorIndex: 1, // Slot 1 / Magenta
		Node: &NodeRef{
			StartByte: 30,
			EndByte:   49,
		},
		OrigStartByte: 30,
		OrigEndByte:   49,
	}

	extra := DelimiterSpan{
		StartByte: 0,
		EndByte:   29,
		Action:    "move",
		ActionRef: &act,
	}

	spans := BuildHighlightSpans(srcBytes, []Action{act}, "left", extra)
	var commentSpans []HighlightSpan
	for _, s := range spans {
		if s.Line == 0 {
			commentSpans = append(commentSpans, s)
		}
	}

	if len(commentSpans) == 0 {
		t.Fatalf("expected highlight span on line 0 for comment trivia, got none")
	}
	if commentSpans[0].Action != "move" {
		t.Errorf("expected comment action to be 'move', got %q", commentSpans[0].Action)
	}
	if commentSpans[0].ColorIndex != 1 {
		t.Errorf("expected comment ColorIndex to match anchor (1), got %d", commentSpans[0].ColorIndex)
	}

	// An update action inside the moved comment promotes to move_update.
	updAct := Action{
		Action: "update",
		Node: &NodeRef{
			StartByte: 7,
			EndByte:   16, // "relocated"
		},
		OrigStartByte: 7,
		OrigEndByte:   16,
	}
	spansWithUpd := BuildHighlightSpans(srcBytes, []Action{act, updAct}, "left", extra)
	hasMoveUpdate := false
	for _, s := range spansWithUpd {
		if s.Line == 0 && s.Action == "move_update" {
			hasMoveUpdate = true
			if s.ColorIndex != 1 {
				t.Errorf("expected move_update ColorIndex to match anchor (1), got %d", s.ColorIndex)
			}
		}
	}
	if !hasMoveUpdate {
		t.Errorf("expected comment update inside move to promote to 'move_update', got: %+v", spansWithUpd)
	}

	// Deleted comments shouldn't get marked as moved, even if attached to a moved node.
	delAct := Action{
		Action: "delete",
		Node: &NodeRef{
			StartByte: 0,
			EndByte:   29,
		},
		OrigStartByte: 0,
		OrigEndByte:   29,
	}
	spansWithDel := BuildHighlightSpans(srcBytes, []Action{act, delAct}, "left")
	hasDelete := false
	for _, s := range spansWithDel {
		if s.Line == 0 {
			if s.Action == "move" {
				t.Errorf("expected deleted comment not to take 'move', got: %+v", s)
			}
			if s.Action == "delete" {
				hasDelete = true
			}
		}
	}
	if !hasDelete {
		t.Errorf("expected deleted comment to take 'delete' highlight, got: %+v", spansWithDel)
	}
}

func TestBuildHighlightSpans_BidirectionalMoveColorConsistency(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		leftBytes := []byte("if a {\n\treturn 1\n}\n")
		rightBytes := []byte("if a &&\n\tb {\n\treturn 1\n\treturn 2\n}\n")

		// A moved block that grows from three lines to five.
		act := Action{
			Action:         "move",
			MoveColorIndex: 1,
			SrcLineCount:   3,
			DstLineCount:   5,
			Node: &NodeRef{
				StartByte: 0,
				EndByte:   uint32(len(leftBytes) - 1),
			},
			DestStartByte: ptr(uint32(0)),
			DestEndByte:   ptr(uint32(len(rightBytes) - 1)),
		}

		leftSpans := BuildHighlightSpans(leftBytes, []Action{act}, "left")
		rightSpans := BuildHighlightSpans(rightBytes, []Action{act}, "right")

		if len(leftSpans) == 0 || len(rightSpans) == 0 {
			t.Fatalf("expected highlight spans on both sides, got left=%d right=%d", len(leftSpans), len(rightSpans))
		}

		for _, s := range leftSpans {
			if s.Action == "move" && s.ColorIndex != 0 {
				t.Errorf("expected left span to collapse to ColorIndex 0, got %d", s.ColorIndex)
			}
		}
		for _, s := range rightSpans {
			if s.Action == "move" && s.ColorIndex != 0 {
				t.Errorf("expected right span to collapse to ColorIndex 0, got %d", s.ColorIndex)
			}
		}

		smallAct := Action{
			Action:         "move",
			MoveColorIndex: 1,
			SrcLineCount:   1,
			DstLineCount:   2,
			Node: &NodeRef{
				StartByte: 0,
				EndByte:   uint32(len(leftBytes) - 1),
			},
			DestStartByte: ptr(uint32(0)),
			DestEndByte:   ptr(uint32(len(rightBytes) - 1)),
		}
		leftSmall := BuildHighlightSpans(leftBytes, []Action{smallAct}, "left")
		rightSmall := BuildHighlightSpans(rightBytes, []Action{smallAct}, "right")
		for _, s := range leftSmall {
			if s.Action == "move" && s.ColorIndex != 1 {
				t.Errorf("expected small left span to retain ColorIndex 1, got %d", s.ColorIndex)
			}
		}
		for _, s := range rightSmall {
			if s.Action == "move" && s.ColorIndex != 1 {
				t.Errorf("expected small right span to retain ColorIndex 1, got %d", s.ColorIndex)
			}
		}
	})

	t.Run("FullEngine", func(t *testing.T) {
		r := rules.Get("go")
		if r == nil {
			t.Fatal("rules for go not found")
		}

		oldSrc := `package main

func process() error {
	prepare()
	count++
	if count > limit {
		return flush()
	}
	return nil
}
`
		newSrc := `package main

func process() error {
	prepare()
	if checkCondition() {
		if count > limit {
			return flush(
			)
		}
		count++
		return nil
	}
	return nil
}
`
		oldAST, err := treesitter.Parse([]byte(oldSrc), "old.go")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse([]byte(newSrc), "new.go")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		part := engine.NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := engine.Match(oldAST, newAST, []byte(oldSrc), []byte(newSrc), part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)
		es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, []byte(oldSrc), []byte(newSrc), EnvelopeOptions{
			IncludeHighlights: true,
			IncludeActions:    true,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

func process() error {
	prepare()
	<mov1>count++</mov1>
	<mov>if count > limit {</mov>
<mov>		return flush()</mov>
<mov>	}</mov>
	<mov>return nil</mov>
}
`)

		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func process() error {
	prepare()
	<ins>if checkCondition() {</ins>
		<mov>if count > limit {</mov>
<mov>			return flush(</mov>
<mov>			)</mov>
<mov>		}</mov>
		<mov1>count++</mov1>
		<mov>return nil</mov>
	<ins>}</ins>
	<ins>return nil</ins>
}
`)

		var movedIf *Action
		for i := range env.Actions {
			a := &env.Actions[i]
			if a.Action == "move" && a.SrcLineCount == 3 && a.DstLineCount == 4 {
				movedIf = a
				break
			}
		}
		if movedIf == nil {
			t.Fatal("expected a move that grows from three lines to four")
		}
	})
}

func TestBuildHighlightSpans_PatternWrapperHighlight_FullEngine(t *testing.T) {
	oldSrc := `fn get_style(state: State) -> &'static Style {
    match state {
        State::HunkPlus(_) => &self.plus_style,
        State::CommitMeta => &self.commit_style,
        State::FileMeta => &self.file_style,
        State::HunkHeader => &self.hunk_header_style,
        _ => unreachable!(),
    }
}
`
	newSrc := `fn get_style(state: State) -> &'static Style {
    match state {
        State::HunkPlus(_) => &self.plus_style,
        State::CommitMeta => &self.commit_style,
        State::FileMeta => &self.file_style,
        State::HunkHeader(_, _) => &self.hunk_header_style,
        State::SubmoduleLog => &self.file_style,
        _ => unreachable!(),
    }
}
`
	oldAST, err := treesitter.Parse([]byte(oldSrc), "test.rs")
	if err != nil {
		t.Fatalf("parse oldSrc: %v", err)
	}
	newAST, err := treesitter.Parse([]byte(newSrc), "test.rs")
	if err != nil {
		t.Fatalf("parse newSrc: %v", err)
	}

	part := engine.NewLinePartition([]byte(oldSrc), []byte(newSrc))
	mr := engine.Match(oldAST, newAST, []byte(oldSrc), []byte(newSrc), part)
	es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)
	es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

	env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, []byte(oldSrc), []byte(newSrc), EnvelopeOptions{
		IncludeHighlights: true,
		IncludeActions:    true,
	})
	if err != nil {
		t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
	}

	// State::HunkHeader stays unhighlighted as context; only (_, _) is inserted.
	testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `fn get_style(state: State) -> &'static Style {
    match state {
        State::HunkPlus(_) => &self.plus_style,
        State::CommitMeta => &self.commit_style,
        State::FileMeta => &self.file_style,
        State::HunkHeader<ins>(_, _)</ins> => &self.hunk_header_style,
        <ins>State::SubmoduleLog => &self.file_style,</ins>
        _ => unreachable!(),
    }
}
`)
}

func TestBuildHighlightSpans_PythonPatternWrapperHighlight_FullEngine(t *testing.T) {
	oldSrc := `def get_style(state):
    match state:
        case State.HUNK_PLUS:
            return self.plus_style
        case State.COMMIT_META:
            return self.commit_style
        case State.FILE_META:
            return self.file_style
        case State.HUNK_HEADER:
            return self.hunk_header_style
        case _:
            pass
`
	newSrc := `def get_style(state):
    match state:
        case State.HUNK_PLUS:
            return self.plus_style
        case State.COMMIT_META:
            return self.commit_style
        case State.FILE_META:
            return self.file_style
        case State.HUNK_HEADER(a, b):
            return self.hunk_header_style
        case State.SUBMODULE_LOG:
            return self.file_style
        case _:
            pass
`
	oldAST, err := treesitter.Parse([]byte(oldSrc), "test.py")
	if err != nil {
		t.Fatalf("parse oldSrc: %v", err)
	}
	newAST, err := treesitter.Parse([]byte(newSrc), "test.py")
	if err != nil {
		t.Fatalf("parse newSrc: %v", err)
	}

	part := engine.NewLinePartition([]byte(oldSrc), []byte(newSrc))
	mr := engine.Match(oldAST, newAST, []byte(oldSrc), []byte(newSrc), part)
	es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)
	es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

	env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, []byte(oldSrc), []byte(newSrc), EnvelopeOptions{
		IncludeHighlights: true,
		IncludeActions:    true,
	})
	if err != nil {
		t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
	}

	// Line 9 (0-indexed line 8: `        case State.HUNK_HEADER(a, b):`)
	// State.HUNK_HEADER (cols 13..30) stays unhighlighted as context; only (a, b) is inserted.
	testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `def get_style(state):
    match state:
        case State.HUNK_PLUS:
            return self.plus_style
        case State.COMMIT_META:
            return self.commit_style
        case State.FILE_META:
            return self.file_style
        case State.HUNK_HEADER<ins>(a, b)</ins>:
            return self.hunk_header_style
        <ins>case State.SUBMODULE_LOG:</ins>
<ins>            return self.file_style</ins>
        case _:
            pass
`)
}

func TestBuildHighlightSpans_MoveWithDeletedChildAdjacentToSiblingDelete(t *testing.T) {
	fileBytes := []byte("\tif rErr := getReaderErr(); rErr != nil {\n")
	subtree := true

	// Move short_var_declaration (bytes 4 to 26)
	actMove := Action{
		Action: "move",
		Node: &NodeRef{
			Tree:      "before",
			Type:      "short_var_declaration",
			StartByte: 4,
			EndByte:   26,
		},
		DestNode: &NodeRef{
			Tree:      "after",
			Type:      "assignment_statement",
			StartByte: 2,
			EndByte:   11,
		},
		Subtree: &subtree,
	}

	// Update := to = (bytes 9 to 11)
	actUpdate := Action{
		Action: "update",
		Node: &NodeRef{
			Tree:      "before",
			Type:      "assignment_operator_literal",
			StartByte: 9,
			EndByte:   11,
		},
		DestNode: &NodeRef{
			Tree:      "after",
			Type:      "assignment_operator_literal",
			StartByte: 7,
			EndByte:   8,
		},
	}

	// Delete getReaderErr() (bytes 12 to 26, AST length 14)
	actDelChild := Action{
		Action: "delete",
		Node: &NodeRef{
			Tree:      "before",
			Type:      "call_expression",
			StartByte: 12,
			EndByte:   26,
		},
		Subtree: &subtree,
	}

	// Delete sibling condition (bytes 28 to 39, AST length 78 in real AST)
	actDelSibling := Action{
		Action: "delete",
		Node: &NodeRef{
			Tree:      "before",
			Type:      "binary_expression",
			StartByte: 28,
			EndByte:   39,
		},
		OrigStartByte: 28,
		OrigEndByte:   106, // wider multi-clause AST node length
		Subtree:       &subtree,
	}

	actions := []Action{actMove, actUpdate, actDelChild, actDelSibling}
	spans := BuildHighlightSpans(fileBytes, actions, "left")

	var foundCallDelete bool
	for _, s := range spans {
		if s.StartCol >= 12 && s.EndCol <= 26 {
			if s.Action != "delete" {
				t.Errorf("expected getReaderErr() [cols 12..26] to have action delete, got %s (span [%d..%d])", s.Action, s.StartCol, s.EndCol)
			} else {
				foundCallDelete = true
			}
		}
	}
	if !foundCallDelete {
		t.Errorf("expected delete highlight on getReaderErr() [cols 12..26]")
	}
}

func TestBuildHighlightSpans_MultilineExpressionUnwrapping_FullEngine(t *testing.T) {
	oldSrc := `package main

func getBinaryOperatorLabel(n *ASTNode, r *Rules) string {
	if n == nil {
		return ""
	}
	for _, c := range n.Children {
		if (r != nil && (r.IsOperatorLiteral(c.Type) || r.IsPunctuation(c.Type))) ||
			(r == nil && (rules.IsOperatorLiteral(c.Type) || rules.IsPunctuation(c.Type))) {
			return c.Label
		}
	}
	return ""
}
`
	newSrc := `package main

func getBinaryOperatorLabel(n *ASTNode, r *Rules) string {
	if n == nil {
		return ""
	}
	r = cmp.Or(r, rules.Get(""))
	for _, c := range n.Children {
		if r.IsOperatorLiteral(c.Type) || r.IsPunctuation(c.Type) {
			return c.Label
		}
	}
	return ""
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

	testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

func getBinaryOperatorLabel(n *ASTNode, r *Rules) string {
	if n == nil {
		return ""
	}
	for _, c := range n.Children {
		if <del>(r != nil && (</del>r.IsOperatorLiteral(c.Type) || r.IsPunctuation(c.Type)<del>)) ||</del>
			<del>(r == nil && (rules.IsOperatorLiteral(c.Type) || rules.IsPunctuation(c.Type)))</del> {
			return c.Label
		}
	}
	return ""
}
`)

	testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func getBinaryOperatorLabel(n *ASTNode, r *Rules) string {
	if n == nil {
		return ""
	}
	<ins>r = cmp.Or(r, rules.Get(""))</ins>
	for _, c := range n.Children {
		if r.IsOperatorLiteral(c.Type) || r.IsPunctuation(c.Type) {
			return c.Label
		}
	}
	return ""
}
`)
}

func TestInsertedCommentInMovedBlock_PreservesInsert(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		r := rules.Get("rust")
		if r == nil {
			t.Fatal("rules for rust not found")
		}

		oldSrc := `fn stationary_one() {}
fn stationary_two() {}

fn main() -> std::io::Result<()> {
    let exit_code = run_app()?;
    // when you call process::exit, no destructors are called
    process::exit(exit_code);
}
`
		newSrc := `fn main() -> std::io::Result<()> {
    // Ignore ctrl-c (SIGINT) to avoid leaving an orphaned pager process.
    // See https://github.com/dandavison/delta/issues/681
    ctrlc::set_handler(|| {});
    let exit_code = run_app()?;
    // when you call process::exit, no destructors are called
    process::exit(exit_code);
}

fn stationary_one() {}
fn stationary_two() {}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		oldAST, err := treesitter.Parse(srcBytes, "old.rs")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse(dstBytes, "new.rs")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		if oldAST.HasError || oldAST.ParseErrorCount > 0 {
			t.Fatalf("oldAST has syntax errors: %d", oldAST.ParseErrorCount)
		}
		if newAST.HasError || newAST.ParseErrorCount > 0 {
			t.Fatalf("newAST has syntax errors: %d", newAST.ParseErrorCount)
		}

		srcComments := comments.ExtractComments(oldAST)
		dstComments := comments.ExtractComments(newAST)

		part := engine.NewLinePartition(srcBytes, dstBytes)
		mr := engine.Match(oldAST, newAST, srcBytes, dstBytes, part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)

		var commentLineMappings map[int]int
		if len(srcComments) > 0 || len(dstComments) > 0 {
			cRes := comments.DiffComments(srcComments, dstComments, mr.Mappings)
			if cRes != nil {
				for _, act := range cRes.Actions {
					es.Add(act)
				}
				commentLineMappings = cRes.LineMappings
			}
		}

		es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, srcBytes, dstBytes, EnvelopeOptions{
			IncludeActions:      true,
			IncludeHighlights:   true,
			IncludeAlignment:    true,
			CommentLineMappings: commentLineMappings,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `fn stationary_one() {}
fn stationary_two() {}

<mov>fn main() -> std::io::Result<()> {</mov>
<mov>    let exit_code = run_app()?;</mov>
<mov>    // when you call process::exit, no destructors are called</mov>
<mov>    process::exit(exit_code);</mov>
<mov>}</mov>
`)

		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `<mov>fn main() -> std::io::Result<()> {</mov>
    <ins>// Ignore ctrl-c (SIGINT) to avoid leaving an orphaned pager process.</ins>
    <ins>// See https://github.com/dandavison/delta/issues/681</ins>
    <ins>ctrlc::set_handler(|| {});</ins>
<mov>    let exit_code = run_app()?;</mov>
<mov>    // when you call process::exit, no destructors are called</mov>
<mov>    process::exit(exit_code);</mov>
<mov>}</mov>

fn stationary_one() {}
fn stationary_two() {}
`)
	})
}

func TestBuildHighlightSpans_MultilineStatementInsertCarveout_FullEngine(t *testing.T) {
	t.Run("Rust multiline macro invocation preserves stationary string literal", func(t *testing.T) {
		r := rules.Get("rust")
		if r == nil {
			t.Fatal("rules for rust not found")
		}

		oldSrc := `fn set_terminal_title_to(new_terminal_title: String) {
    let osc_command = "\x1b]0;";
    let osc_end = "\x07";
    let safe_title = bat::sanitize(&new_terminal_title);
    write!(io::stdout(), "{osc_command}{safe_title}{osc_end}").unwrap();
    io::stdout().flush().unwrap();
}
`
		newSrc := `fn set_terminal_title_to(new_terminal_title: String) -> Result<()> {
    let osc_command = "\x1b]0;";
    let osc_end = "\x07";
    let safe_title = bat::sanitize(&new_terminal_title);
    write!(
        io::stdout(),
        "{osc_command}{safe_title}{osc_end}"
    )?;
    io::stdout().flush()?;
    Ok(())
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		oldAST, err := treesitter.Parse(srcBytes, "old.rs")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse(dstBytes, "new.rs")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		if oldAST.HasError || oldAST.ParseErrorCount > 0 {
			t.Fatalf("oldAST has syntax errors: %d", oldAST.ParseErrorCount)
		}
		if newAST.HasError || newAST.ParseErrorCount > 0 {
			t.Fatalf("newAST has syntax errors: %d", newAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(srcBytes, dstBytes)
		mr := engine.Match(oldAST, newAST, srcBytes, dstBytes, part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)
		es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, srcBytes, dstBytes, EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		// Whole-Snippet Verification:
		// 1. Enclosing function maps in place
		testutil.AssertMappedText(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"fn set_terminal_title_to", "fn set_terminal_title_to", "function_item",
		)

		// 2. String literal is mapped in place
		testutil.AssertMappedText(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`"{osc_command}{safe_title}{osc_end}"`, `"{osc_command}{safe_title}{osc_end}"`, "string_literal",
		)

		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `fn set_terminal_title_to(new_terminal_title: String) {
    let osc_command = "\x1b]0;";
    let osc_end = "\x07";
    let safe_title = bat::sanitize(&new_terminal_title);
    write!(io::stdout(), "{osc_command}{safe_title}{osc_end}")<del>.unwrap()</del>;
    <del>io::stdout().flush().unwrap();</del>
}
`)

		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `fn set_terminal_title_to(new_terminal_title: String) <ins>-> Result<()></ins> {
    let osc_command = "\x1b]0;";
    let osc_end = "\x07";
    let safe_title = bat::sanitize(&new_terminal_title);
    write!(
        io::stdout(),
        "{osc_command}{safe_title}{osc_end}"
    )<ins>?;</ins>
    <ins>io::stdout().flush()?;</ins>
    <ins>Ok(())</ins>
}
`)
	})

	t.Run("Go multiline call deletion does not bleed delete over stationary arguments", func(t *testing.T) {
		oldSrc := `package main

func bar() {
	foo(
		"arg1",
		"arg2",
	)
}
`
		newSrc := `package main

func bar() {
	result := foo(
		"arg1",
		"arg2",
	)
	_ = result
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		oldAST, err := treesitter.Parse(srcBytes, "old.go")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse(dstBytes, "new.go")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		if oldAST.HasError || oldAST.ParseErrorCount > 0 {
			t.Fatalf("oldAST has syntax errors: %d", oldAST.ParseErrorCount)
		}
		if newAST.HasError || newAST.ParseErrorCount > 0 {
			t.Fatalf("newAST has syntax errors: %d", newAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(srcBytes, dstBytes)
		mr := engine.Match(oldAST, newAST, srcBytes, dstBytes, part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)
		es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, srcBytes, dstBytes, EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		// Whole-Snippet Verification:
		// 1. Enclosing function maps in place
		testutil.AssertMappedText(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"func bar()", "func bar()", "function_declaration",
		)

		// 2. Stationary arguments map in place
		testutil.AssertMappedText(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`"arg1"`, `"arg1"`, "interpreted_string_literal",
		)

		// 3. Highlight span verification
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

func bar() {
	foo(
		"arg1",
		"arg2",
	)
}
`)

		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func bar() {
	<ins>result :=</ins> foo(
		"arg1",
		"arg2",
	)
	<ins>_ = result</ins>
}
`)
	})

	t.Run("Rust let declaration wrapping match expression does not bleed insert over stationary match arms", func(t *testing.T) {
		oldSrc := `fn main() {
    match shell.as_str() {
        "bash" => println!("{}", completions::BASH_COMPLETION),
        _ => unreachable!("No completion for shell '{shell}' available."),
    }
}
`
		newSrc := `fn main() {
    let completion = match shell.as_str() {
        "bash" => completions::BASH_COMPLETION,
        _ => unreachable!("No completion for shell '{shell}' available."),
    };
    writeln!(io::stdout(), "{completion}")?;
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		oldAST, err := treesitter.Parse(srcBytes, "old.rs")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse(dstBytes, "new.rs")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		if oldAST.HasError || oldAST.ParseErrorCount > 0 {
			t.Fatalf("oldAST has syntax errors: %d", oldAST.ParseErrorCount)
		}
		if newAST.HasError || newAST.ParseErrorCount > 0 {
			t.Fatalf("newAST has syntax errors: %d", newAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(srcBytes, dstBytes)
		mr := engine.Match(oldAST, newAST, srcBytes, dstBytes, part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)
		es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, srcBytes, dstBytes, EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		// Whole-Snippet Verification:
		// 1. Enclosing function maps in place
		testutil.AssertMappedText(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"fn main()", "fn main()", "function_item",
		)

		// 2. Stationary match arm maps in place
		testutil.AssertMappedText(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`_ => unreachable!("No completion for shell '{shell}' available."),`,
			`_ => unreachable!("No completion for shell '{shell}' available."),`,
			"match_arm",
		)

		// 3. Highlight span verification
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `fn main() {
    match shell.as_str() {
        "bash" => <del>println!("{}", completions::BASH_COMPLETION),</del>
        _ => unreachable!("No completion for shell '{shell}' available."),
    }
}
`)

		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `fn main() {
    <ins>let completion =</ins> match shell.as_str() {
        "bash" => <ins>completions::BASH_COMPLETION,</ins>
        _ => unreachable!("No completion for shell '{shell}' available."),
    }<ins>;</ins>
    <ins>writeln!(io::stdout(), "{completion}")?;</ins>
}
`)
	})

	t.Run("C multiline call deletion does not bleed delete over stationary arguments", func(t *testing.T) {
		oldSrc := `void test() {
    foo(
        "arg1",
        "arg2"
    );
}
`
		newSrc := `void test() {
    int res = foo(
        "arg1",
        "arg2"
    );
    return;
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		oldAST, err := treesitter.Parse(srcBytes, "old.c")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse(dstBytes, "new.c")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		if oldAST.HasError || oldAST.ParseErrorCount > 0 {
			t.Fatalf("oldAST has syntax errors: %d", oldAST.ParseErrorCount)
		}
		if newAST.HasError || newAST.ParseErrorCount > 0 {
			t.Fatalf("newAST has syntax errors: %d", newAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(srcBytes, dstBytes)
		mr := engine.Match(oldAST, newAST, srcBytes, dstBytes, part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)
		es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, srcBytes, dstBytes, EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		// Whole-Snippet Verification:
		// 1. Enclosing function maps in place
		testutil.AssertMappedText(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"void test()", "void test()", "function_definition",
		)

		// 2. Stationary arguments map in place
		testutil.AssertMappedText(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`"arg1"`, `"arg1"`, "string_literal",
		)

		// 3. Highlight span verification: stationary arguments on lines 2 and 3 are not deleted
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `void test() {
    foo(
        "arg1",
        "arg2"
    )<del>;</del>
}
`)

		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `void test() {
    <ins>int res = foo(</ins>
<ins>        "arg1",</ins>
<ins>        "arg2"</ins>
<ins>    );</ins>
    <ins>return;</ins>
}
`)
	})

	t.Run("Java multiline call deletion does not bleed delete over stationary arguments", func(t *testing.T) {
		oldSrc := `class Test {
    void run() {
        foo(
            "arg1",
            "arg2"
        );
    }
}
`
		newSrc := `class Test {
    void run() {
        int res = foo(
            "arg1",
            "arg2"
        );
        return;
    }
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		oldAST, err := treesitter.Parse(srcBytes, "Test.java")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse(dstBytes, "Test.java")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		if oldAST.HasError || oldAST.ParseErrorCount > 0 {
			t.Fatalf("oldAST has syntax errors: %d", oldAST.ParseErrorCount)
		}
		if newAST.HasError || newAST.ParseErrorCount > 0 {
			t.Fatalf("newAST has syntax errors: %d", newAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(srcBytes, dstBytes)
		mr := engine.Match(oldAST, newAST, srcBytes, dstBytes, part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)
		es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, srcBytes, dstBytes, EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		// Whole-Snippet Verification:
		// 1. Enclosing class and method map in place
		testutil.AssertMappedText(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"class Test", "class Test", "class_declaration",
		)

		// 2. Stationary arguments map in place
		testutil.AssertMappedText(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`"arg1"`, `"arg1"`, "string_literal",
		)

		// 3. Highlight span verification: stationary arguments on lines 3 and 4 are not deleted
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `class Test {
    void run() {
        foo(
            "arg1",
            "arg2"
        )<del>;</del>
    }
}
`)

		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `class Test {
    void run() {
        <ins>int res = foo(</ins>
<ins>            "arg1",</ins>
<ins>            "arg2"</ins>
<ins>        );</ins>
        <ins>return;</ins>
    }
}
`)
	})

	t.Run("FullEngine/Rust macro token tree does not bleed insert span over stationary children", func(t *testing.T) {
		r := rules.Get("rust")
		if r == nil {
			t.Fatal("rules for rust not found")
		}

		oldSrc := `fn print_trailing() -> Result<()> {
    let cursor_max = 100;
    let cursor = 10;
    write!(
        handle,
        "{}",
        ansi_style.paint(" ".repeat(cursor_max - cursor))
    )?;
    Ok(())
}
`
		newSrc := `fn print_trailing() -> Result<()> {
    let cursor_max = 100;
    let cursor = 10;
    write!(
        handle,
        "{}",
        ansi_style.paint(" ".repeat(cursor_max.saturating_sub(cursor)))
    )?;
    Ok(())
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		oldAST, err := treesitter.Parse(srcBytes, "old.rs")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse(dstBytes, "new.rs")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		if oldAST.HasError || oldAST.ParseErrorCount > 0 {
			t.Fatalf("oldAST has syntax errors: %d", oldAST.ParseErrorCount)
		}
		if newAST.HasError || newAST.ParseErrorCount > 0 {
			t.Fatalf("newAST has syntax errors: %d", newAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(srcBytes, dstBytes)
		mr := engine.Match(oldAST, newAST, srcBytes, dstBytes, part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)
		es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, srcBytes, dstBytes, EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		// Whole-Snippet Verification:
		// 1. Enclosing function declaration maps in place
		testutil.AssertMappedText(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"fn print_trailing", "fn print_trailing", "function_item",
		)

		// 2. Surrounding sequence: let declarations map in place
		testutil.AssertMappedText(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"let cursor_max = 100;", "let cursor_max = 100;", "let_declaration",
		)
		testutil.AssertMappedText(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"let cursor = 10;", "let cursor = 10;", "let_declaration",
		)

		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `fn print_trailing() -> Result<()> {
    let cursor_max = 100;
    let cursor = 10;
    write!(
        handle,
        "{}",
        ansi_style.paint(" ".repeat(cursor_max <del>-</del> cursor))
    )?;
    Ok(())
}
`)

		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `fn print_trailing() -> Result<()> {
    let cursor_max = 100;
    let cursor = 10;
    write!(
        handle,
        "{}",
        ansi_style.paint(" ".repeat(cursor_max<ins>.saturating_sub(</ins>cursor<ins>)</ins>))
    )?;
    Ok(())
}
`)
	})

	t.Run("FullEngine/C initializer list does not bleed insert span over stationary children", func(t *testing.T) {
		oldSrc := `void foo() {
    int item = 10;
    int a = item;
}
`
		newSrc := `void foo() {
    int item = 10;
    int a[] = { item };
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		oldAST, err := treesitter.Parse(srcBytes, "old.c")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse(dstBytes, "new.c")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		if oldAST.HasError || oldAST.ParseErrorCount > 0 {
			t.Fatalf("oldAST has syntax errors: %d", oldAST.ParseErrorCount)
		}
		if newAST.HasError || newAST.ParseErrorCount > 0 {
			t.Fatalf("newAST has syntax errors: %d", newAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(srcBytes, dstBytes)
		mr := engine.Match(oldAST, newAST, srcBytes, dstBytes, part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)
		es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, srcBytes, dstBytes, EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		// Whole-Snippet Verification:
		// 1. Enclosing function declaration maps in place
		testutil.AssertMappedText(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"void foo()", "void foo()", "function_definition",
		)

		// 2. Surrounding sequence: 'int item = 10;' maps in place
		testutil.AssertMappedText(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"int item = 10;", "int item = 10;", "declaration",
		)
		testutil.AssertMappedTextQuery(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			testutil.TextQuery{Text: "item", Type: "identifier", Occurrence: 1},
			testutil.TextQuery{Text: "item", Type: "identifier", Occurrence: 1},
		)
		testutil.AssertMappedTextQuery(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			testutil.TextQuery{Text: "item", Type: "identifier", Occurrence: 2},
			testutil.TextQuery{Text: "item", Type: "identifier", Occurrence: 2},
		)

		// 3. Right highlights on line 2 (0-indexed: int a[] = { item };)
		// The inserted initializer_list delimiters must be carved out so `item` is NOT engulfed in an insert span.
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `void foo() {
    int item = 10;
    int a = item;
}
`)

		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `void foo() {
    int item = 10;
    int a[] = <ins>{</ins> item <ins>}</ins>;
}
`)
	})

	t.Run("FullEngine/Java array initializer does not bleed insert span over stationary children", func(t *testing.T) {
		oldSrc := `class Test {
    void run() {
        int item = 10;
        int a = item;
    }
}
`
		newSrc := `class Test {
    void run() {
        int item = 10;
        int[] a = { item };
    }
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		oldAST, err := treesitter.Parse(srcBytes, "Test.java")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse(dstBytes, "Test.java")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		if oldAST.HasError || oldAST.ParseErrorCount > 0 {
			t.Fatalf("oldAST has syntax errors: %d", oldAST.ParseErrorCount)
		}
		if newAST.HasError || newAST.ParseErrorCount > 0 {
			t.Fatalf("newAST has syntax errors: %d", newAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(srcBytes, dstBytes)
		mr := engine.Match(oldAST, newAST, srcBytes, dstBytes, part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)
		es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, srcBytes, dstBytes, EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		// Whole-Snippet Verification:
		// 1. Enclosing class and method declarations map in place
		testutil.AssertMappedText(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"class Test", "class Test", "class_declaration",
		)

		// 2. Surrounding sequence: 'int item = 10;' maps in place
		testutil.AssertMappedText(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"int item = 10;", "int item = 10;", "local_variable_declaration",
		)
		testutil.AssertMappedTextQuery(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			testutil.TextQuery{Text: "item", Type: "identifier", Occurrence: 1},
			testutil.TextQuery{Text: "item", Type: "identifier", Occurrence: 1},
		)
		testutil.AssertMappedTextQuery(t, oldAST, newAST, srcBytes, dstBytes, mr.Mappings.Src(),
			testutil.TextQuery{Text: "item", Type: "identifier", Occurrence: 2},
			testutil.TextQuery{Text: "item", Type: "identifier", Occurrence: 2},
		)

		// 3. Right highlights on line 3 (0-indexed: int[] a = { item };)
		// The inserted array_initializer delimiters must be carved out so `item` is NOT engulfed in an insert span.
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `class Test {
    void run() {
        int item = 10;
        int a = item;
    }
}
`)

		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `class Test {
    void run() {
        int item = 10;
        int<ins>[]</ins> a = <ins>{</ins> item <ins>}</ins>;
    }
}
`)
	})
}
