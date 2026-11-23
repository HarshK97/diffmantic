package serialize

import (
	"slices"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/postprocess"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestAlignLines(t *testing.T) {
	t.Run("Identical single-line files", func(t *testing.T) {
		src := "hello"
		dst := "hello"
		got := AlignLines([]byte(src), []byte(dst), nil, nil)
		testutil.AssertLinesAligned(t, src, dst, toTestPairs(got),
			testutil.Line("hello"),
			testutil.Line("hello"),
		)
	})

	t.Run("Identical multi-line files", func(t *testing.T) {
		src := "hello\nworld"
		dst := "hello\nworld"
		got := AlignLines([]byte(src), []byte(dst), nil, nil)
		testPairs := toTestPairs(got)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("hello"),
			testutil.Line("hello"),
		)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("world"),
			testutil.Line("world"),
		)
	})

	t.Run("Empty old file", func(t *testing.T) {
		src := ""
		dst := "new\nlines"
		got := AlignLines([]byte(src), []byte(dst), nil, nil)
		testutil.AssertAlignmentGrid(t, src, dst, toTestPairs(got), `
∅ │ new
∅ │ lines
`)
	})

	t.Run("Empty new file", func(t *testing.T) {
		src := "old\nlines"
		dst := ""
		got := AlignLines([]byte(src), []byte(dst), nil, nil)
		testutil.AssertAlignmentGrid(t, src, dst, toTestPairs(got), `
old   │ ∅
lines │ ∅
`)
	})

	t.Run("Both empty", func(t *testing.T) {
		src := ""
		dst := ""
		got := AlignLines([]byte(src), []byte(dst), nil, nil)
		testutil.AssertAlignmentGrid(t, src, dst, toTestPairs(got), `
∅ │ 
`)
	})

	t.Run("Deleted lines", func(t *testing.T) {
		src := "line1\nline2\nline3"
		dst := "line1\nline3"
		got := AlignLines([]byte(src), []byte(dst), nil, nil)
		testPairs := toTestPairs(got)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("line1"),
			testutil.Line("line1"),
		)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("line3"),
			testutil.Line("line3"),
		)
		testutil.AssertLinesNotAligned(t, src, dst, testPairs,
			testutil.Line("line2"),
			testutil.Line("line3"),
		)
	})

	t.Run("Inserted lines", func(t *testing.T) {
		src := "line1\nline3"
		dst := "line1\nline2\nline3"
		got := AlignLines([]byte(src), []byte(dst), nil, nil)
		testPairs := toTestPairs(got)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("line1"),
			testutil.Line("line1"),
		)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("line3"),
			testutil.Line("line3"),
		)
		testutil.AssertLinesNotAligned(t, src, dst, testPairs,
			testutil.Line("line1"),
			testutil.Line("line2"),
		)
		testutil.AssertLinesNotAligned(t, src, dst, testPairs,
			testutil.Line("line3"),
			testutil.Line("line2"),
		)
	})

	t.Run("Single line change in middle", func(t *testing.T) {
		src := "line1\nline2\nline3"
		dst := "line1\nline2_changed\nline3"
		got := AlignLines([]byte(src), []byte(dst), nil, nil)
		testPairs := toTestPairs(got)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("line1"),
			testutil.Line("line1"),
		)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("line2"),
			testutil.Line("line2_changed"),
		)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("line3"),
			testutil.Line("line3"),
		)
	})

	t.Run("Unequal length modified block (more insertions)", func(t *testing.T) {
		src := "header\nold1\nfooter"
		dst := "header\nnew1\nnew2\nfooter"
		got := AlignLines([]byte(src), []byte(dst), nil, nil)
		testPairs := toTestPairs(got)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("header"),
			testutil.Line("header"),
		)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("old1"),
			testutil.Line("new1"),
		)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("footer"),
			testutil.Line("footer"),
		)
		testutil.AssertLinesNotAligned(t, src, dst, testPairs,
			testutil.Line("old1"),
			testutil.Line("new2"),
		)
		testutil.AssertLinesNotAligned(t, src, dst, testPairs,
			testutil.Line("footer"),
			testutil.Line("new2"),
		)
	})

	t.Run("Unequal length modified block (more deletions)", func(t *testing.T) {
		src := "header\nold1\nold2\nfooter"
		dst := "header\nnew1\nfooter"
		got := AlignLines([]byte(src), []byte(dst), nil, nil)
		testPairs := toTestPairs(got)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("header"),
			testutil.Line("header"),
		)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("old1"),
			testutil.Line("new1"),
		)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("footer"),
			testutil.Line("footer"),
		)
		testutil.AssertLinesNotAligned(t, src, dst, testPairs,
			testutil.Line("old2"),
			testutil.Line("new1"),
		)
		testutil.AssertLinesNotAligned(t, src, dst, testPairs,
			testutil.Line("old2"),
			testutil.Line("footer"),
		)
	})

	t.Run("Moved block rendered cleanly via line diff", func(t *testing.T) {
		src := "func foo() {\n  a()\n}\nfunc bar() {\n}"
		dst := "func foo() {\n}\nfunc bar() {\n  a()\n}"
		got := AlignLines([]byte(src), []byte(dst), nil, nil)
		testPairs := toTestPairs(got)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Nth("func foo() {", 1),
			testutil.Nth("func foo() {", 1),
		)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Nth("}", 1),
			testutil.Nth("}", 1),
		)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Nth("func bar() {", 1),
			testutil.Nth("func bar() {", 1),
		)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Nth("}", 2),
			testutil.Nth("}", 2),
		)
		testutil.AssertLinesNotAligned(t, src, dst, testPairs,
			testutil.Line("a()"),
			testutil.Nth("}", 1),
		)
	})

	t.Run("Aligns unchanged line when statement order shifts around it", func(t *testing.T) {
		src := "c := init()\nif err != nil {\n  return\n}\n\nc.Next()\nreuse(c)"
		dst := "c := init()\nsetStatus(404)\nc.Next()\nif done {\n  log()\n}\nreuse(c)"
		got := AlignLines([]byte(src), []byte(dst), nil, nil)
		testPairs := toTestPairs(got)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("c := init()"),
			testutil.Line("c := init()"),
		)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("c.Next()"),
			testutil.Line("c.Next()"),
		)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("reuse(c)"),
			testutil.Line("reuse(c)"),
		)
		testutil.AssertLinesAligned(t, src, dst, testPairs,
			testutil.Line("if err != nil {"),
			testutil.Line("setStatus(404)"),
		)
		testutil.AssertLinesNotAligned(t, src, dst, testPairs,
			testutil.Line("c.Next()"),
			testutil.Line("setStatus(404)"),
		)
		testutil.AssertLinesNotAligned(t, src, dst, testPairs,
			testutil.Line("c.Next()"),
			testutil.Line("if done {"),
		)
	})
}

func TestAlignLinesWithMapping(t *testing.T) {
	// Simulate AST mapping for a statement whose line text was rewritten
	src := "func f() {\n  sh := 1\n  bh := 2\n  return b\n}"
	dst := "func f() {\n  return struct{\n    x int\n  }{1}\n}"

	srcAST, err := treesitter.Parse([]byte(src), "main.go")
	if err != nil {
		t.Fatalf("parse src failed: %v", err)
	}
	dstAST, err := treesitter.Parse([]byte(dst), "main.go")
	if err != nil {
		t.Fatalf("parse dst failed: %v", err)
	}

	parentSrc, err := testutil.FindNodeByText(srcAST, []byte(src), "func f() {", "function_declaration")
	if err != nil {
		t.Fatalf("parentSrc: %v", err)
	}
	parentDst, err := testutil.FindNodeByText(dstAST, []byte(dst), "func f() {", "function_declaration")
	if err != nil {
		t.Fatalf("parentDst: %v", err)
	}
	retSrc, err := testutil.FindNodeByText(srcAST, []byte(src), "return b", "return_statement")
	if err != nil {
		t.Fatalf("retSrc: %v", err)
	}
	retDst, err := testutil.FindNodeByText(dstAST, []byte(dst), "return struct{", "return_statement")
	if err != nil {
		t.Fatalf("retDst: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(parentSrc, parentDst)
	ms.Add(retSrc, retDst)

	got := AlignLines([]byte(src), []byte(dst), ms, nil)
	testPairs := toTestPairs(got)

	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("func f() {"),
		testutil.Line("func f() {"),
	)
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("return b"),
		testutil.Line("return struct{"),
	)
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("}"),
		testutil.Line("}"),
	)
	testutil.AssertLinesNotAligned(t, src, dst, testPairs,
		testutil.Line("sh := 1"),
		testutil.Line("return struct{"),
	)
	testutil.AssertLinesNotAligned(t, src, dst, testPairs,
		testutil.Line("bh := 2"),
		testutil.Line("return struct{"),
	)
}

func TestAlignLinesExtraction(t *testing.T) {
	// Simulate extraction where an outer if statement is removed and inner body moved out
	src := "func f() {\n  if ok {\n    doA()\n    doB()\n  }\n  return\n}"
	dst := "func f() {\n  doA()\n  doB()\n  return\n}"

	srcAST, err := treesitter.Parse([]byte(src), "main.go")
	if err != nil {
		t.Fatalf("parse src failed: %v", err)
	}
	dstAST, err := treesitter.Parse([]byte(dst), "main.go")
	if err != nil {
		t.Fatalf("parse dst failed: %v", err)
	}

	innerA, err := testutil.FindNodeByText(srcAST, []byte(src), "doA()", "expression_statement")
	if err != nil {
		t.Fatalf("innerA: %v", err)
	}
	innerB, err := testutil.FindNodeByText(srcAST, []byte(src), "doB()", "expression_statement")
	if err != nil {
		t.Fatalf("innerB: %v", err)
	}
	destA, err := testutil.FindNodeByText(dstAST, []byte(dst), "doA()", "expression_statement")
	if err != nil {
		t.Fatalf("destA: %v", err)
	}
	destB, err := testutil.FindNodeByText(dstAST, []byte(dst), "doB()", "expression_statement")
	if err != nil {
		t.Fatalf("destB: %v", err)
	}
	parentDst, err := testutil.FindNodeByText(dstAST, []byte(dst), "func f() {", "function_declaration")
	if err != nil {
		t.Fatalf("parentDst: %v", err)
	}

	es := actions.NewEditScript()
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     innerA,
		DestNode: destA,
		Parent:   parentDst,
	})
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     innerB,
		DestNode: destB,
		Parent:   parentDst,
	})

	got := AlignLines([]byte(src), []byte(dst), nil, es)
	testPairs := toTestPairs(got)

	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("func f() {"),
		testutil.Line("func f() {"),
	)
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("doA()"),
		testutil.Line("doA()"),
	)
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("doB()"),
		testutil.Line("doB()"),
	)
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("return"),
		testutil.Line("return"),
	)
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Nth("}", 2),
		testutil.Line("}"),
	)
	testutil.AssertLinesNotAligned(t, src, dst, testPairs,
		testutil.Line("if ok {"),
		testutil.Line("doA()"),
	)
}

func TestAlignLinesSingleStatementBlock(t *testing.T) {
	// Simulate single-statement block where parent block has exact same row range as child call
	src := "if tgt == 'msg' then\n  api.nvim_win_set_cursor(ui.wins.msg)\nelseif tgt == 'pager' then\nend"
	dst := "if tgt == 'msg' then\n  -- comment 1\n  -- comment 2\n  fn.win_execute(ui.wins.msg)\nelseif tgt == 'pager' then\nend"

	srcAST, err := treesitter.Parse([]byte(src), "test.lua")
	if err != nil {
		t.Fatalf("parse src failed: %v", err)
	}
	dstAST, err := treesitter.Parse([]byte(dst), "test.lua")
	if err != nil {
		t.Fatalf("parse dst failed: %v", err)
	}

	callSrc, err := testutil.FindNodeByText(srcAST, []byte(src), "api.nvim_win_set_cursor", "function_call")
	if err != nil {
		t.Fatalf("callSrc: %v", err)
	}
	callDst, err := testutil.FindNodeByText(dstAST, []byte(dst), "fn.win_execute", "function_call")
	if err != nil {
		t.Fatalf("callDst: %v", err)
	}
	blockSrc := callSrc.Parent
	blockDst := callDst.Parent
	if blockSrc == nil || blockDst == nil {
		t.Fatalf("block parents not found: blockSrc=%v, blockDst=%v", blockSrc, blockDst)
	}

	ms := engine.NewMapping()
	ms.Add(blockSrc, blockDst)
	ms.Add(callSrc, callDst)

	got := AlignLines([]byte(src), []byte(dst), ms, nil)
	testPairs := toTestPairs(got)

	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("if tgt == 'msg' then"),
		testutil.Line("if tgt == 'msg' then"),
	)
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("api.nvim_win_set_cursor(ui.wins.msg)"),
		testutil.Line("fn.win_execute(ui.wins.msg)"),
	)
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("elseif tgt == 'pager' then"),
		testutil.Line("elseif tgt == 'pager' then"),
	)
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("end"),
		testutil.Line("end"),
	)
	testutil.AssertLinesNotAligned(t, src, dst, testPairs,
		testutil.Line("api.nvim_win_set_cursor(ui.wins.msg)"),
		testutil.Line("-- comment 1"),
	)
	testutil.AssertLinesNotAligned(t, src, dst, testPairs,
		testutil.Line("api.nvim_win_set_cursor(ui.wins.msg)"),
		testutil.Line("-- comment 2"),
	)
}

func TestAlignLinesContinuationCondition(t *testing.T) {
	src := "if delta.type == 'workspaces' and\n  kong.default_workspace ~= delta.entity.id\nthen\nend"
	dst := "if delta.type == 'workspaces' and\n  delta_entity ~= nil and\n  kong.default_workspace ~= delta_entity.id\nthen\nend"

	srcAST, err := treesitter.Parse([]byte(src), "test.lua")
	if err != nil {
		t.Fatalf("parse src failed: %v", err)
	}
	dstAST, err := treesitter.Parse([]byte(dst), "test.lua")
	if err != nil {
		t.Fatalf("parse dst failed: %v", err)
	}

	condSrc, err := testutil.FindNodeByText(srcAST, []byte(src), "kong.default_workspace", "binary_expression")
	if err != nil {
		t.Fatalf("condSrc: %v", err)
	}
	condDst, err := testutil.FindNodeByText(dstAST, []byte(dst), "kong.default_workspace", "binary_expression")
	if err != nil {
		t.Fatalf("condDst: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(srcAST, dstAST)
	ms.Add(condSrc, condDst)

	got := AlignLines([]byte(src), []byte(dst), ms, nil)
	testPairs := toTestPairs(got)

	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("if delta.type == 'workspaces' and"),
		testutil.Line("if delta.type == 'workspaces' and"),
	)
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("kong.default_workspace ~= delta.entity.id"),
		testutil.Line("kong.default_workspace ~= delta_entity.id"),
	)
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("then"),
		testutil.Line("then"),
	)
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("end"),
		testutil.Line("end"),
	)
	testutil.AssertLinesNotAligned(t, src, dst, testPairs,
		testutil.Line("kong.default_workspace ~= delta.entity.id"),
		testutil.Line("delta_entity ~= nil and"),
	)
}

func TestCalculateSimilarityLinear(t *testing.T) {
	scratch := &alignScratch{
		dp:      make([]int, 0, 4096),
		touched: make([]uint16, 0, 128),
	}

	// 1. Identical strings
	if sim := calculateSimilarity("func foo()", "func foo()", scratch); sim != 1.0 {
		t.Errorf("expected 1.0 for identical strings, got %f", sim)
	}

	// 2. Short string (< 2 bytes)
	if sim := calculateSimilarity("a", "b", scratch); sim != 0.0 {
		t.Errorf("expected 0.0 for single char strings, got %f", sim)
	}

	// 3. Length ratio > 3:1 pruning
	if sim := calculateSimilarity("a", "this is a very long string that exceeds 3x length", scratch); sim != 0.0 {
		t.Errorf("expected 0.0 for ratio-pruned strings, got %f", sim)
	}

	// 4. Moderate similarity (<= 128 bytes)
	s1 := "func CalculateTotal(items []Item) int {"
	s2 := "func CalculateTotalAmount(items []Item) int {"
	sim1 := calculateSimilarity(s1, s2, scratch)
	if sim1 <= 0.5 || sim1 >= 1.0 {
		t.Errorf("expected high similarity between %q and %q, got %f", s1, s2, sim1)
	}

	// Verify rollback cleanly zeroed all touched bigrams
	for i, v := range scratch.bigrams {
		if v != 0 {
			t.Fatalf("dirty scratch.bigrams[%d] = %d after calculateSimilarity", i, v)
		}
	}

	// 5. Extended string (> 128 bytes) - zero heap allocations
	long1 := "func ProcessVeryLongPayloadDataWithManyParametersAndComplexTypes(firstArg TypeA, secondArg TypeB, thirdArg TypeC, fourthArg TypeD, fifthArg TypeE) error {"
	long2 := "func ProcessVeryLongPayloadDataWithManyParametersAndComplexTypes(firstArg TypeA, secondArg TypeB, thirdArg TypeC, fourthArg ModifiedTypeD, fifthArg TypeE) error {"
	sim2 := calculateSimilarity(long1, long2, scratch)
	if sim2 <= 0.8 || sim2 >= 1.0 {
		t.Errorf("expected high similarity between long strings, got %f", sim2)
	}

	for i, v := range scratch.bigrams {
		if v != 0 {
			t.Fatalf("dirty scratch.bigrams[%d] = %d after calculateSimilarity long string", i, v)
		}
	}
}

func TestAlignLinesAsymmetricGap(t *testing.T) {
	// Exercise large asymmetric ratio M=100, N=10 without clipping or panic
	var srcLines, dstLines []string
	srcLines = append(srcLines, "package main", "func LongFunction() {")
	dstLines = append(dstLines, "package main", "func LongFunction() {")

	for i := 0; i < 95; i++ {
		srcLines = append(srcLines, "  deletedLine_"+string(rune('0'+(i%10)))+"()")
	}
	for j := 0; j < 5; j++ {
		dstLines = append(dstLines, "  insertedLine_"+string(rune('0'+(j%10)))+"()")
	}
	srcLines = append(srcLines, "  return", "}")
	dstLines = append(dstLines, "  return", "}")

	srcStr := strings.Join(srcLines, "\n")
	dstStr := strings.Join(dstLines, "\n")

	got := AlignLines([]byte(srcStr), []byte(dstStr), nil, nil)
	testPairs := toTestPairs(got)

	testutil.AssertLinesAligned(t, srcStr, dstStr, testPairs,
		testutil.Line("package main"),
		testutil.Line("package main"),
	)
	testutil.AssertLinesAligned(t, srcStr, dstStr, testPairs,
		testutil.Line("func LongFunction() {"),
		testutil.Line("func LongFunction() {"),
	)
	testutil.AssertLinesAligned(t, srcStr, dstStr, testPairs,
		testutil.Line("return"),
		testutil.Line("return"),
	)
	testutil.AssertLinesAligned(t, srcStr, dstStr, testPairs,
		testutil.Line("}"),
		testutil.Line("}"),
	)
}

func TestAlignLinesBanded(t *testing.T) {
	// Generate large gap (M=100, N=100) with edits in the middle to exercise banded DP
	var srcLines, dstLines []string
	srcLines = append(srcLines, "package main", "func LongFunction() {")
	dstLines = append(dstLines, "package main", "func LongFunction() {")

	for i := 0; i < 80; i++ {
		srcLines = append(srcLines, "  statementA_"+string(rune('0'+(i%10)))+"()")
		if i < 40 {
			dstLines = append(dstLines, "  statementA_"+string(rune('0'+(i%10)))+"()")
		} else {
			dstLines = append(dstLines, "  statementModified_"+string(rune('0'+(i%10)))+"()")
		}
	}
	srcLines = append(srcLines, "  return", "}")
	dstLines = append(dstLines, "  return", "}")

	srcStr := strings.Join(srcLines, "\n")
	dstStr := strings.Join(dstLines, "\n")

	got := AlignLines([]byte(srcStr), []byte(dstStr), nil, nil)
	testPairs := toTestPairs(got)

	testutil.AssertLinesAligned(t, srcStr, dstStr, testPairs,
		testutil.Line("package main"),
		testutil.Line("package main"),
	)
	testutil.AssertLinesAligned(t, srcStr, dstStr, testPairs,
		testutil.Line("func LongFunction() {"),
		testutil.Line("func LongFunction() {"),
	)
	testutil.AssertLinesAligned(t, srcStr, dstStr, testPairs,
		testutil.Line("return"),
		testutil.Line("return"),
	)
	testutil.AssertLinesAligned(t, srcStr, dstStr, testPairs,
		testutil.Line("}"),
		testutil.Line("}"),
	)
}

func TestAlignLines_DataContainerAndASTMappingPrior(t *testing.T) {
	// Old Python dictionary has validations with additionalProperties: False
	// New Python dictionary has validations with additionalProperties: False, followed by inserted upload_validations with identical duplicate additionalProperties: False
	srcStr := "d = {\n  \"validations\": {\n    \"required\": True,\n    \"additionalProperties\": False,\n  },\n  \"assignee\": \"test\",\n}\n"
	dstStr := "d = {\n  \"validations\": {\n    \"required\": True,\n    \"additionalProperties\": False,\n  },\n  \"upload_validations\": {\n    \"required\": True,\n    \"additionalProperties\": False,\n  },\n  \"assignee\": \"test\",\n}\n"

	src := []byte(srcStr)
	dst := []byte(dstStr)

	srcAST, err := treesitter.Parse(src, "test.py")
	if err != nil {
		t.Fatalf("parse src failed: %v", err)
	}
	dstAST, err := treesitter.Parse(dst, "test.py")
	if err != nil {
		t.Fatalf("parse dst failed: %v", err)
	}

	n1, err := testutil.FindNodeByTextQuery(srcAST, src, testutil.TextQuery{Text: "additionalProperties", Type: "pair", Occurrence: 1})
	if err != nil {
		t.Fatalf("n1: %v", err)
	}
	n2, err := testutil.FindNodeByTextQuery(dstAST, dst, testutil.TextQuery{Text: "additionalProperties", Type: "pair", Occurrence: 1})
	if err != nil {
		t.Fatalf("n2: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(n1, n2)

	got := AlignLines(src, dst, ms, nil)
	testPairs := toTestPairs(got)

	// Verify that src line 3 ("additionalProperties": False) aligns with dst line 3 (not dst line 7)
	testutil.AssertLinesAligned(t, srcStr, dstStr, testPairs,
		testutil.Line("\"additionalProperties\": False,"),
		testutil.Nth("\"additionalProperties\": False,", 1),
	)
	testutil.AssertLinesNotAligned(t, srcStr, dstStr, testPairs,
		testutil.Line("\"additionalProperties\": False,"),
		testutil.Nth("\"additionalProperties\": False,", 2),
	)
}

func TestAlignLines_MassiveGap(t *testing.T) {
	// Verify that a massive gap (> 1M cells) does not allocate gigabytes or panic,
	// and accurately splits via sub-anchors.
	var srcLines, dstLines []string
	for i := 0; i < 2000; i++ {
		srcLines = append(srcLines, "deleted_line")
	}
	// Anchor in the middle
	srcLines = append(srcLines, "stable_middle_line")
	dstLines = append(dstLines, "stable_middle_line")
	for i := 0; i < 2000; i++ {
		dstLines = append(dstLines, "inserted_line")
	}

	srcStr := strings.Join(srcLines, "\n")
	dstStr := strings.Join(dstLines, "\n")

	grid := AlignLines([]byte(srcStr), []byte(dstStr), nil, nil)
	testPairs := toTestPairs(grid)

	// Verify the middle anchor was aligned
	testutil.AssertLinesAligned(t, srcStr, dstStr, testPairs,
		testutil.Line("stable_middle_line"),
		testutil.Line("stable_middle_line"),
	)
}

func TestAlignLines_PunctuationLineGuard(t *testing.T) {
	// Deleted if block vs inserted multi-block function with a coincidental `\t}` at line 3.
	srcStr := strings.Join([]string{
		"func Add() {",
		"\tif _, exists := m.src[t1]; !exists {",
		"\t\tm.Pairs = append(m.Pairs, Pair{Src: t1, Dst: t2})",
		"\t}",
		"\tm.src[t1] = t2",
		"}",
	}, "\n")

	dstStr := strings.Join([]string{
		"func Add() {",
		"\tif t1 == nil || t2 == nil {",
		"\t\treturn",
		"\t}",
		"\tif curT2, ok := m.src[t1]; ok && curT2 == t2 {",
		"\t\treturn",
		"\t}",
		"\tm.Pairs = append(m.Pairs, Pair{Src: t1, Dst: t2})",
		"\tm.src[t1] = t2",
		"}",
	}, "\n")

	grid := AlignLines([]byte(srcStr), []byte(dstStr), nil, nil)
	testPairs := toTestPairs(grid)

	testutil.AssertLinesAligned(t, srcStr, dstStr, testPairs,
		testutil.Line("func Add() {"),
		testutil.Line("func Add() {"),
	)
	testutil.AssertLinesAligned(t, srcStr, dstStr, testPairs,
		testutil.Line("m.src[t1] = t2"),
		testutil.Line("m.src[t1] = t2"),
	)
	testutil.AssertLinesAligned(t, srcStr, dstStr, testPairs,
		testutil.Nth("}", 2),
		testutil.Nth("}", 3),
	)
	// Line 3 on left (`\t}`) must not be anchored to Line 3 on right (`\t}`).
	testutil.AssertLinesNotAligned(t, srcStr, dstStr, testPairs,
		testutil.Nth("}", 1),
		testutil.Nth("}", 1),
	)
}

func TestAlignLines_InPlaceClosureMove(t *testing.T) {
	src := `fn test() {
    let a = 1;
    let b = 2;
}
`
	dst := `fn test() {
    model(|| {
        let a = 1;
        let b = 2;
    });
}
`
	oldTree, err := treesitter.Parse([]byte(src), "old.rs")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	newTree, err := treesitter.Parse([]byte(dst), "new.rs")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	var oldFn, newFn, oldA, oldB, newA, newB *treesitter.ASTNode
	for _, d := range oldTree.Descendants() {
		switch d.Type {
		case "function_item":
			oldFn = d
		case "let_declaration":
			if oldA == nil {
				oldA = d
			} else if oldB == nil {
				oldB = d
			}
		}
	}
	for _, d := range newTree.Descendants() {
		switch d.Type {
		case "function_item":
			newFn = d
		case "let_declaration":
			if newA == nil {
				newA = d
			} else if newB == nil {
				newB = d
			}
		}
	}

	if oldFn == nil || newFn == nil || oldA == nil || oldB == nil || newA == nil || newB == nil {
		t.Fatalf("failed to find AST nodes in parsed trees")
	}

	ms := engine.NewMapping()
	ms.Add(oldFn, newFn)
	ms.Add(oldA, newA)
	ms.Add(oldB, newB)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: oldA, DestNode: newA})
	es.Add(actions.Action{Type: actions.Move, Node: oldB, DestNode: newB})

	alignment := AlignLines([]byte(src), []byte(dst), ms, es)
	testPairs := toTestPairs(alignment)

	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("let a = 1;"),
		testutil.Line("let a = 1;"),
	)
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("let b = 2;"),
		testutil.Line("let b = 2;"),
	)
}

func TestAlignLines_AttachedElseClause(t *testing.T) {
	withoutElse := []byte("void f() {\n  if (cond) {\n    a = 1;\n  }\n  after();\n}")
	withElse := []byte("void f() {\n  if (cond) {\n    a = 1;\n  } else {\n    b = 2;\n  }\n  after();\n}")
	chainWithoutElse := []byte("void f() {\n  if (cond1) {\n    a = 1;\n  } else if (cond2) {\n    b = 2;\n  }\n  after();\n}")
	chainWithElse := []byte("void f() {\n  if (cond1) {\n    a = 1;\n  } else if (cond2) {\n    b = 2;\n  } else {\n    c = 3;\n  }\n  after();\n}")

	cases := []struct {
		name              string
		src, dst          []byte
		inPlaceMoveIfStmt bool
		wantLine          int
	}{
		{name: "AddedElse", src: withoutElse, dst: withElse, wantLine: 3},
		{name: "RemovedElse", src: withElse, dst: withoutElse, wantLine: 3},
		{name: "AddedElseOnElseIfChain", src: chainWithoutElse, dst: chainWithElse, wantLine: 5},
		{name: "RemovedElseOnElseIfChain", src: chainWithElse, dst: chainWithoutElse, wantLine: 5},
		{name: "InPlaceMovedIfWithAddedElse", src: withoutElse, dst: withElse, inPlaceMoveIfStmt: true, wantLine: 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t1, err := treesitter.Parse(tc.src, "src.c")
			if err != nil {
				t.Fatalf("parse src: %v", err)
			}
			t2, err := treesitter.Parse(tc.dst, "dst.c")
			if err != nil {
				t.Fatalf("parse dst: %v", err)
			}
			res := engine.Match(t1, t2, tc.src, tc.dst, nil)
			var es *actions.EditScript
			if tc.inPlaceMoveIfStmt {
				es = actions.NewEditScript()
				for _, pair := range res.Mappings.Pairs {
					if pair.Src != nil && pair.Src.Type == "if_statement" {
						es.Add(actions.Action{Type: actions.Move, Node: pair.Src, DestNode: pair.Dst})
						break
					}
				}
			}
			got := AlignLines(tc.src, tc.dst, res.Mappings, es)
			testPairs := toTestPairs(got)

			testutil.AssertLinesAligned(t, string(tc.src), string(tc.dst), testPairs,
				testutil.Line("a = 1;"),
				testutil.Line("a = 1;"),
			)
			testutil.AssertLinesAligned(t, string(tc.src), string(tc.dst), testPairs,
				testutil.Line("after();"),
				testutil.Line("after();"),
			)
			if strings.Contains(string(tc.src), "b = 2;") && strings.Contains(string(tc.dst), "b = 2;") {
				testutil.AssertLinesAligned(t, string(tc.src), string(tc.dst), testPairs,
					testutil.Line("b = 2;"),
					testutil.Line("b = 2;"),
				)
			}
		})
	}
}

func TestAlignLines_ExtractedClosureIntraMoveAlignment(t *testing.T) {
	src := `func run() {
  switch format {
  case "json":
    printJSON()
  case "text":
    printText()
  }
}
`
	dst := `func run() {
  processItem := func() {
    switch format {
    case "json":
      printJSON()
    case "text":
      printText()
    }
  }
  processItem()
}
`

	oldTree, err := treesitter.Parse([]byte(src), "old.go")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	newTree, err := treesitter.Parse([]byte(dst), "new.go")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	part := engine.NewLinePartition([]byte(src), []byte(dst))
	mr := engine.Match(oldTree, newTree, []byte(src), []byte(dst), part)
	es := actions.GenerateEditScript(oldTree, newTree, mr.Mappings)
	postprocess.Run(es, mr.Mappings, oldTree, newTree)

	alignment := AlignLines([]byte(src), []byte(dst), mr.Mappings, es)
	testPairs := toTestPairs(alignment)

	// Verify that the switch header and its cases align across the closure boundary.
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("switch format {"),
		testutil.Line("switch format {"),
	)
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line(`case "json":`),
		testutil.Line(`case "json":`),
	)
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line(`case "text":`),
		testutil.Line(`case "text":`),
	)
}

func TestAlignLines_UnchangedControlFlowAndBracePrecedence(t *testing.T) {
	src := `func test() {
	for _, candidate := range list {
		if candidate.Valid() {
			if a > b {
				doA()
			} else if isBlock(candidate) {
				doB()
			}
		} else {
			if directKeyMatch(candidate) {
				doC()
			}
		}
		break
	}
}
`
	dst := `func test() {
	for _, candidate := range list {
		if candidate.Valid() {
			if a > b || isBlock(candidate) {
				doA()
			}
		} else {
			candCommon := compute()
			if candCommon > currCommon {
				doC()
			}
		}
		break
	}
}
`
	oldTree, err := treesitter.Parse([]byte(src), "old.go")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	newTree, err := treesitter.Parse([]byte(dst), "new.go")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	part := engine.NewLinePartition([]byte(src), []byte(dst))
	mr := engine.Match(oldTree, newTree, []byte(src), []byte(dst), part)
	es := actions.GenerateEditScript(oldTree, newTree, mr.Mappings)
	postprocess.Run(es, mr.Mappings, oldTree, newTree)

	alignment := AlignLines([]byte(src), []byte(dst), mr.Mappings, es)
	testPairs := toTestPairs(alignment)

	// Line 8 on left (`\t\t} else {`) must align with line 6 on right (`\t\t} else {`).
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("} else {"),
		testutil.Line("} else {"),
	)
	// Line 12 on left (`\t\t}`) must align with line 11 on right (`\t\t}`).
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Nth("}", 3),
		testutil.Nth("}", 3),
	)
	// Line 13 on left (`\t\tbreak`) must align with line 12 on right (`\t\tbreak`).
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("break"),
		testutil.Line("break"),
	)
	// Line 14 on left (`\t}`) must align with line 13 on right (`\t}`).
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Nth("}", 4),
		testutil.Nth("}", 4),
	)
}

func TestAlignLines_StationaryStatementPrecedenceOverMove(t *testing.T) {
	src := `func test() {
	primary := collectPrimary()
	text := collectText()
	anchors := mergeAnchors(primary, text)
	inPlaceStatements := collectMappedStatements(fn)
	use(anchors, inPlaceStatements)
}
`
	dst := `func test() {
	inPlaceStatements := collectMappedStatements(fn)
	tier1 := collectTier1()
	anchors := collectStationaryMoveAnchors(inPlaceStatements, tier1)
	use(anchors, inPlaceStatements)
}
`
	oldTree, err := treesitter.Parse([]byte(src), "old.go")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	newTree, err := treesitter.Parse([]byte(dst), "new.go")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	part := engine.NewLinePartition([]byte(src), []byte(dst))
	mr := engine.Match(oldTree, newTree, []byte(src), []byte(dst), part)
	es := actions.GenerateEditScript(oldTree, newTree, mr.Mappings)
	postprocess.Run(es, mr.Mappings, oldTree, newTree)

	alignment := AlignLines([]byte(src), []byte(dst), mr.Mappings, es)
	testPairs := toTestPairs(alignment)

	// Line 3 on left (`\tanchors := mergeAnchors(primary, text)`) must align with
	// line 3 on right (`\tanchors := collectStationaryMoveAnchors(inPlaceStatements, tier1)`).
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("anchors := mergeAnchors(primary, text)"),
		testutil.Line("anchors := collectStationaryMoveAnchors(inPlaceStatements, tier1)"),
	)
	// Line 4 on left (`\tinPlaceStatements := ...`) moved above `anchors :=` on the right,
	// so on the left it cannot align with line 1 on right without crossing `anchors :=`.
	testutil.AssertLinesNotAligned(t, src, dst, testPairs,
		testutil.Line("inPlaceStatements := collectMappedStatements(fn)"),
		testutil.Line("inPlaceStatements := collectMappedStatements(fn)"),
	)
}

func TestStationaryIndex_Crosses(t *testing.T) {
	var nilIdx *stationaryIndex
	if nilIdx.crosses(10, 20) {
		t.Errorf("nil stationaryIndex should never cross")
	}
	emptyIdx := newStationaryIndex(nil)
	if emptyIdx != nil {
		t.Errorf("empty map should return nil stationaryIndex")
	}

	// Single statement: src=10 -> dst=20
	stmts := map[int]int{10: 20}
	idx := newStationaryIndex(stmts)
	if idx == nil {
		t.Fatalf("expected non-nil index")
	}

	singleTests := []struct {
		name string
		s, d int
		want bool
	}{
		{"crosses earlier and lower", 5, 25, true},
		{"crosses later and higher", 15, 15, true},
		{"monotonic before", 5, 15, false},
		{"monotonic after", 15, 25, false},
		{"exact matching coordinate", 10, 20, false},
	}
	for _, tt := range singleTests {
		t.Run("single/"+tt.name, func(t *testing.T) {
			if got := idx.crosses(tt.s, tt.d); got != tt.want {
				t.Errorf("crosses(%d, %d) = %v, want %v", tt.s, tt.d, got, tt.want)
			}
		})
	}

	// Multi-statement scenario: (10, 20), (30, 40), (50, 60)
	multiStmts := map[int]int{
		10: 20,
		30: 40,
		50: 60,
	}
	multiIdx := newStationaryIndex(multiStmts)

	multiTests := []struct {
		name string
		s, d int
		want bool
	}{
		{"inversion below first statement", 20, 15, true},
		{"inversion above second statement", 20, 45, true},
		{"monotonic between statements", 20, 30, false},
		{"before all statements", 5, 10, false},
		{"after all statements", 60, 70, false},
		{"after all statements crossing last", 60, 30, true},
	}
	for _, tt := range multiTests {
		t.Run("multi/"+tt.name, func(t *testing.T) {
			if got := multiIdx.crosses(tt.s, tt.d); got != tt.want {
				t.Errorf("crosses(%d, %d) = %v, want %v", tt.s, tt.d, got, tt.want)
			}
		})
	}
}

func TestAlignLines_StationaryElsePrecedenceOverInsertedClause(t *testing.T) {
	src := `func render() {
	if p < len(isPairChanged) && !isPairChanged[p] {
		text := ""
		if pair.RightLine < len(dstLines) {
			text = dstLines[pair.RightLine]
		}
	} else {
		for bEnd <= h.End {
			bEnd++
		}
	}
}
`
	dst := `func render() {
	if p < len(isPairChanged) && !isPairChanged[p] {
		text := ""
		if pair.RightLine < len(dstLines) {
			text = dstLines[pair.RightLine]
		}
	} else if pair.LeftLine == -1 && pair.RightLine >= 0 {
		leftLineNum = -1
	} else if pair.LeftLine >= 0 && pair.RightLine == -1 {
		rightLineNum = -1
	} else {
		processOther()
	}
}
`
	oldTree, err := treesitter.Parse([]byte(src), "old.go")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	newTree, err := treesitter.Parse([]byte(dst), "new.go")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	part := engine.NewLinePartition([]byte(src), []byte(dst))
	mr := engine.Match(oldTree, newTree, []byte(src), []byte(dst), part)
	es := actions.GenerateEditScript(oldTree, newTree, mr.Mappings)
	postprocess.Run(es, mr.Mappings, oldTree, newTree)

	alignment := AlignLines([]byte(src), []byte(dst), mr.Mappings, es)
	testPairs := toTestPairs(alignment)

	// Line 6 on left (`\t} else {`) must align with line 6 on right (`\t} else if pair.LeftLine == -1 ...`),
	// not with line 10 on right (`\t} else {`).
	testutil.AssertLinesAligned(t, src, dst, testPairs,
		testutil.Line("} else {"),
		testutil.Line("} else if pair.LeftLine == -1 && pair.RightLine >= 0 {"),
	)
	testutil.AssertLinesNotAligned(t, src, dst, testPairs,
		testutil.Line("} else {"),
		testutil.Line("} else {"),
	)
}

func TestCollectMoveRanges_MacroStructuralFilter_Tier4(t *testing.T) {
	src := []byte("package main\n\nfunc main() {\n\tx := 1\n\tfoo()\n\ta := b + c + d\n}\n")
	dst := []byte("package main\n\nfunc other() {\n\t// spacer\n\t// spacer\n\t// spacer\n\t// spacer\n\t// spacer\n\t// spacer\n\tx := 1\n\tfoo()\n\ta := b + c + d\n}\n")

	srcAST, err := treesitter.Parse(src, "main.go")
	if err != nil {
		t.Fatalf("parse src failed: %v", err)
	}
	dstAST, err := treesitter.Parse(dst, "main.go")
	if err != nil {
		t.Fatalf("parse dst failed: %v", err)
	}

	var microSrc, microDst *treesitter.ASTNode
	for _, n := range srcAST.Descendants() {
		if n.Type == "identifier" && n.Label == "x" {
			microSrc = n
			break
		}
	}
	for _, n := range dstAST.Descendants() {
		if n.Type == "identifier" && n.Label == "x" {
			microDst = n
			break
		}
	}

	var stmtSrc, stmtDst *treesitter.ASTNode
	for _, n := range srcAST.Descendants() {
		if n.Type == "expression_statement" {
			stmtSrc = n
			break
		}
	}
	for _, n := range dstAST.Descendants() {
		if n.Type == "expression_statement" {
			stmtDst = n
			break
		}
	}

	var largeExprSrc, largeExprDst *treesitter.ASTNode
	for _, n := range srcAST.Descendants() {
		if n.Type == "binary_expression" && n.Size() >= 4 {
			largeExprSrc = n
			break
		}
	}
	for _, n := range dstAST.Descendants() {
		if n.Type == "binary_expression" && n.Size() >= 4 {
			largeExprDst = n
			break
		}
	}

	if microSrc == nil || microDst == nil || stmtSrc == nil || stmtDst == nil || largeExprSrc == nil || largeExprDst == nil {
		t.Fatalf("nodes not found: microSrc=%v, microDst=%v, stmtSrc=%v, stmtDst=%v, largeExprSrc=%v, largeExprDst=%v",
			microSrc, microDst, stmtSrc, stmtDst, largeExprSrc, largeExprDst)
	}

	ms := engine.NewMapping()
	ms.Add(microSrc, microDst)
	ms.Add(stmtSrc, stmtDst)
	ms.Add(largeExprSrc, largeExprDst)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: microSrc, DestNode: microDst})
	es.Add(actions.Action{Type: actions.Move, Node: stmtSrc, DestNode: stmtDst})
	es.Add(actions.Action{Type: actions.Move, Node: largeExprSrc, DestNode: largeExprDst})

	srcLines := make([]string, 100)
	moves, _ := collectMoveRanges(es, ms, srcLines, nil, nil)

	// Out of the 3 Move actions:
	// microSrc (size 1) must be dropped from line alignment.
	// stmtSrc (statement) and largeExprSrc (size 4) must be admitted.
	if len(moves) != 2 {
		t.Fatalf("expected exactly 2 macro-structural move ranges admitted, got %d", len(moves))
	}

	for _, m := range moves {
		if m.sStart == int(microSrc.StartRow) && m.dStart == int(microDst.StartRow) {
			t.Errorf("micro-move on line %d->%d should be excluded from line alignment moveRange", microSrc.StartRow, microDst.StartRow)
		}
	}
}

func BenchmarkAlignLines(b *testing.B) {
	b.Run("SmallGap", func(b *testing.B) {
		src := []byte("func foo() {\n  a := 1\n  b := 2\n  return a + b\n}")
		dst := []byte("func foo() {\n  a := 10\n  b := 20\n  return a + b\n}")
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			AlignLines(src, dst, nil, nil)
		}
	})

	b.Run("MediumGap_Banded", func(b *testing.B) {
		var srcLines, dstLines []string
		srcLines = append(srcLines, "func BigFunc() {")
		dstLines = append(dstLines, "func BigFunc() {")
		for i := 0; i < 100; i++ {
			srcLines = append(srcLines, "  line_"+string(rune('0'+(i%10)))+"()")
			dstLines = append(dstLines, "  line_mod_"+string(rune('0'+(i%10)))+"()")
		}
		srcLines = append(srcLines, "}")
		dstLines = append(dstLines, "}")
		src := []byte(strings.Join(srcLines, "\n"))
		dst := []byte(strings.Join(dstLines, "\n"))
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			AlignLines(src, dst, nil, nil)
		}
	})

	b.Run("CalculateSimilarity", func(b *testing.B) {
		scratch := &alignScratch{
			dp:      make([]int, 0, 8192),
			touched: make([]uint16, 0, 256),
		}
		s1 := "func CalculateTotalAmount(items []Item, taxRate float64) (float64, error) {"
		s2 := "func CalculateGrossAmount(items []Item, taxRate float64) (float64, error) {"
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			calculateSimilarity(s1, s2, scratch)
		}
	})
}

func TestFilterMonotonicPatienceLIS(t *testing.T) {
	t.Run("EmptyAndSingle", func(t *testing.T) {
		if got := filterMonotonic(nil); got != nil {
			t.Fatalf("got %v, want nil", got)
		}
		want := []anchor{{src: 1, dst: 2}}
		if got := filterMonotonic([]anchor{{src: 1, dst: 2}}); !slices.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("DeduplicationAndDescSorting", func(t *testing.T) {
		input := []anchor{
			{src: 1, dst: 5},
			{src: 1, dst: 5},
			{src: 1, dst: 10},
			{src: 2, dst: 8},
		}
		want := []anchor{{src: 1, dst: 5}, {src: 2, dst: 8}}
		if got := filterMonotonic(input); !slices.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("EqualDestinationCollision", func(t *testing.T) {
		input := []anchor{
			{src: 1, dst: 5},
			{src: 2, dst: 5},
			{src: 3, dst: 10},
		}
		want := []anchor{{src: 1, dst: 5}, {src: 3, dst: 10}}
		if got := filterMonotonic(input); !slices.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("ExhibitB_MonotonicRecovery", func(t *testing.T) {
		input := []anchor{
			{src: 1, dst: 2},
			{src: 2, dst: 1},
			{src: 3, dst: 3},
		}
		res := filterMonotonic(input)
		if len(res) != 2 {
			t.Fatalf("expected len 2, got %d (%v)", len(res), res)
		}
		if res[0].src >= res[1].src || res[0].dst >= res[1].dst {
			t.Fatalf("expected strictly increasing subsequence, got %v", res)
		}
	})

	t.Run("StrictInversion", func(t *testing.T) {
		input := []anchor{
			{src: 1, dst: 3},
			{src: 2, dst: 2},
			{src: 3, dst: 1},
		}
		res := filterMonotonic(input)
		if len(res) != 1 {
			t.Fatalf("expected len 1, got %d (%v)", len(res), res)
		}
	})
}

func TestLineAlignment_StationaryDeclarationOverMovePriority(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `package streamdiff

import (
	"bytes"
	"context"
)

func processStreamJob(ctx context.Context, job *diffJob, cfg StreamConfig) {
	if err := ctx.Err(); err != nil {
		job.err = err
		return
	}

	var buf bytes.Buffer
	if bytes.Equal(job.srcBytes, job.dstBytes) && cfg.Format != "json" {
		job.out = buf.Bytes()
		return
	}

	dr, err := pipeline.Run(job.srcBytes, job.dstBytes, job.hunk.oldPath, job.hunk.newPath, cfg.DiffOpts)
	if err != nil || dr == nil {
		_ = dumpLines(&buf, job.hunk.lines)
		job.out = buf.Bytes()
		return
	}
}
`
		newSrc := `package streamdiff

import (
	"bytes"
	"context"
)

func processStreamJob(ctx context.Context, job *diffJob, cfg StreamConfig) {
	if err := ctx.Err(); err != nil {
		job.err = err
		return
	}

	if bytes.Equal(job.srcBytes, job.dstBytes) && cfg.Format != "json" {
		return
	}

	var buf bytes.Buffer

	dr, err := pipeline.Run(job.srcBytes, job.dstBytes, job.hunk.oldPath, job.hunk.newPath, cfg.DiffOpts)
	if err != nil || dr == nil {
		_ = dumpLines(&buf, job.hunk.lines)
		job.out = buf.Bytes()
		return
	}
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "stream_old.go")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "stream_new.go")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("oldSrc has parse errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("newSrc has parse errors: %d", dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(srcBytes, dstBytes)
		mr := engine.Match(srcAST, dstAST, srcBytes, dstBytes, part)
		es := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		es = postprocess.Run(es, mr.Mappings, srcAST, dstAST)

		// 1. Whole-Snippet Verification: Assert enclosing context maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"func processStreamJob", "func processStreamJob", "function_declaration",
		)

		// 2. Alignment Verification: var buf bytes.Buffer MUST be aligned together as Tier 1 unchanged anchor,
		// and NOT displaced by the moved if-block.
		grid := AlignLines(srcBytes, dstBytes, mr.Mappings, es)
		testPairs := toTestPairs(grid)

		testutil.AssertLinesAligned(t, oldSrc, newSrc, testPairs,
			testutil.Line("var buf bytes.Buffer"),
			testutil.Line("var buf bytes.Buffer"),
		)
		testutil.AssertLinesNotAligned(t, oldSrc, newSrc, testPairs,
			testutil.Line("var buf bytes.Buffer"),
			testutil.Line("if bytes.Equal(job.srcBytes, job.dstBytes) && cfg.Format != \"json\" {"),
		)
	})
}

func toTestPairs(grid []LineAlignmentPair) []testutil.LineAlignmentPair {
	res := make([]testutil.LineAlignmentPair, len(grid))
	for i, p := range grid {
		res[i] = testutil.LineAlignmentPair{
			LeftLine:  p.LeftLine,
			RightLine: p.RightLine,
		}
	}
	return res
}
