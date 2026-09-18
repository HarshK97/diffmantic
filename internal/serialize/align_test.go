package serialize

import (
	"reflect"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestAlignLines(t *testing.T) {
	tests := []struct {
		name     string
		srcBytes []byte
		dstBytes []byte
		want     []LineAlignmentPair
	}{
		{
			name:     "Identical single-line files",
			srcBytes: []byte("hello"),
			dstBytes: []byte("hello"),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
			},
		},
		{
			name:     "Identical multi-line files",
			srcBytes: []byte("hello\nworld"),
			dstBytes: []byte("hello\nworld"),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: 1, RightLine: 1},
			},
		},
		{
			name:     "Empty old file",
			srcBytes: []byte(""),
			dstBytes: []byte("new\nlines"),
			want: []LineAlignmentPair{
				{LeftLine: -1, RightLine: 0},
				{LeftLine: -1, RightLine: 1},
			},
		},
		{
			name:     "Empty new file",
			srcBytes: []byte("old\nlines"),
			dstBytes: []byte(""),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: -1},
				{LeftLine: 1, RightLine: -1},
			},
		},
		{
			name:     "Both empty",
			srcBytes: []byte(""),
			dstBytes: []byte(""),
			want: []LineAlignmentPair{
				{LeftLine: -1, RightLine: 0},
			},
		},
		{
			name:     "Deleted lines",
			srcBytes: []byte("line1\nline2\nline3"),
			dstBytes: []byte("line1\nline3"),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: 1, RightLine: -1},
				{LeftLine: 2, RightLine: 1},
			},
		},
		{
			name:     "Inserted lines",
			srcBytes: []byte("line1\nline3"),
			dstBytes: []byte("line1\nline2\nline3"),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: -1, RightLine: 1},
				{LeftLine: 1, RightLine: 2},
			},
		},
		{
			name:     "Single line change in middle",
			srcBytes: []byte("line1\nline2\nline3"),
			dstBytes: []byte("line1\nline2_changed\nline3"),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: 1, RightLine: 1},
				{LeftLine: 2, RightLine: 2},
			},
		},
		{
			name:     "Unequal length modified block (more insertions)",
			srcBytes: []byte("header\nold1\nfooter"),
			dstBytes: []byte("header\nnew1\nnew2\nfooter"),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: 1, RightLine: 1},
				{LeftLine: -1, RightLine: 2},
				{LeftLine: 2, RightLine: 3},
			},
		},
		{
			name:     "Unequal length modified block (more deletions)",
			srcBytes: []byte("header\nold1\nold2\nfooter"),
			dstBytes: []byte("header\nnew1\nfooter"),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: 1, RightLine: 1},
				{LeftLine: 2, RightLine: -1},
				{LeftLine: 3, RightLine: 2},
			},
		},
		{
			name:     "Moved block rendered cleanly via line diff",
			srcBytes: []byte("func foo() {\n  a()\n}\nfunc bar() {\n}"),
			dstBytes: []byte("func foo() {\n}\nfunc bar() {\n  a()\n}"),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: 1, RightLine: -1},
				{LeftLine: 2, RightLine: 1},
				{LeftLine: 3, RightLine: 2},
				{LeftLine: -1, RightLine: 3},
				{LeftLine: 4, RightLine: 4},
			},
		},
		{
			name:     "Aligns unchanged line when statement order shifts around it",
			srcBytes: []byte("c := init()\nif err != nil {\n  return\n}\n\nc.Next()\nreuse(c)"),
			dstBytes: []byte("c := init()\nsetStatus(404)\nc.Next()\nif done {\n  log()\n}\nreuse(c)"),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},  // c := init()
				{LeftLine: 1, RightLine: 1},  // if err != nil { <-> setStatus(404)
				{LeftLine: 2, RightLine: -1}, // return <-> filler
				{LeftLine: 3, RightLine: -1}, // } <-> filler
				{LeftLine: 4, RightLine: -1}, // "" <-> filler
				{LeftLine: 5, RightLine: 2},  // c.Next() <-> c.Next() (ALIGNED!)
				{LeftLine: -1, RightLine: 3}, // filler <-> if done {
				{LeftLine: -1, RightLine: 4}, // filler <-> log()
				{LeftLine: -1, RightLine: 5}, // filler <-> }
				{LeftLine: 6, RightLine: 6},  // reuse(c) <-> reuse(c)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AlignLines(tt.srcBytes, tt.dstBytes, nil, nil)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AlignLines() got = %v, want = %v", got, tt.want)
			}
		})
	}
}

func TestAlignLinesWithMapping(t *testing.T) {
	// Simulate AST mapping for a statement whose line text was rewritten
	src := []byte("func f() {\n  sh := 1\n  bh := 2\n  return b\n}")
	dst := []byte("func f() {\n  return struct{\n    x int\n  }{1}\n}")

	ms := engine.NewMapping()
	parentSrc := &treesitter.ASTNode{StartRow: 0, EndRow: 4}
	parentDst := &treesitter.ASTNode{StartRow: 0, EndRow: 4}

	retSrc := &treesitter.ASTNode{
		Type:     "return_statement",
		StartRow: 3,
		EndRow:   3,
		Parent:   parentSrc,
	}
	retDst := &treesitter.ASTNode{
		Type:     "return_statement",
		StartRow: 1,
		EndRow:   3,
		Parent:   parentDst,
	}
	ms.Add(parentSrc, parentDst)
	ms.Add(retSrc, retDst)

	got := AlignLines(src, dst, ms, nil)
	want := []LineAlignmentPair{
		{LeftLine: 0, RightLine: 0},  // func f() {
		{LeftLine: 1, RightLine: -1}, // sh := 1
		{LeftLine: 2, RightLine: -1}, // bh := 2
		{LeftLine: 3, RightLine: 1},  // return b <-> return struct{ (ALIGNED!)
		{LeftLine: -1, RightLine: 2}, // filler <-> x int
		{LeftLine: -1, RightLine: 3}, // filler <-> }{1}
		{LeftLine: 4, RightLine: 4},  // }
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("AlignLines() got = %v, want = %v", got, want)
	}
}

func TestAlignLinesExtraction(t *testing.T) {
	// Simulate extraction where an outer if statement is removed and inner body moved out
	src := []byte("func f() {\n  if ok {\n    doA()\n    doB()\n  }\n  return\n}")
	dst := []byte("func f() {\n  doA()\n  doB()\n  return\n}")

	es := actions.NewEditScript()
	innerA := &treesitter.ASTNode{StartRow: 2, EndRow: 2}
	destA := &treesitter.ASTNode{StartRow: 1, EndRow: 1}
	innerB := &treesitter.ASTNode{StartRow: 3, EndRow: 3}
	destB := &treesitter.ASTNode{StartRow: 2, EndRow: 2}
	parentDst := &treesitter.ASTNode{StartRow: 0, EndRow: 4}

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

	got := AlignLines(src, dst, nil, es)
	want := []LineAlignmentPair{
		{LeftLine: 0, RightLine: 0},  // func f() {
		{LeftLine: 1, RightLine: -1}, // if ok { (deleted container)
		{LeftLine: 2, RightLine: -1}, //   doA()
		{LeftLine: 3, RightLine: -1}, //   doB()
		{LeftLine: 4, RightLine: -1}, // }
		{LeftLine: -1, RightLine: 1}, // doA() (extracted container)
		{LeftLine: -1, RightLine: 2}, // doB()
		{LeftLine: 5, RightLine: 3},  // return
		{LeftLine: 6, RightLine: 4},  // }
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("AlignLines() for extraction got = %v, want = %v", got, want)
	}
}

func TestAlignLinesSingleStatementBlock(t *testing.T) {
	// Simulate single-statement block where parent block has exact same row range as child call
	src := []byte("elseif tgt == 'msg' then\n  api.nvim_win_set_cursor(ui.wins.msg)\nelseif tgt == 'pager' then")
	dst := []byte("elseif tgt == 'msg' then\n  -- comment 1\n  -- comment 2\n  fn.win_execute(ui.wins.msg)\nelseif tgt == 'pager' then")

	ms := engine.NewMapping()
	blockSrc := &treesitter.ASTNode{Type: "block", StartRow: 1, EndRow: 1}
	blockDst := &treesitter.ASTNode{Type: "block", StartRow: 3, EndRow: 3}
	callSrc := &treesitter.ASTNode{Type: "function_call", StartRow: 1, EndRow: 1, Parent: blockSrc}
	callDst := &treesitter.ASTNode{Type: "function_call", StartRow: 3, EndRow: 3, Parent: blockDst}

	ms.Add(blockSrc, blockDst)
	ms.Add(callSrc, callDst)

	got := AlignLines(src, dst, ms, nil)
	want := []LineAlignmentPair{
		{LeftLine: 0, RightLine: 0},  // elseif tgt == 'msg' then
		{LeftLine: -1, RightLine: 1}, // filler <-> -- comment 1
		{LeftLine: -1, RightLine: 2}, // filler <-> -- comment 2
		{LeftLine: 1, RightLine: 3},  // api.nvim_win_set_cursor <-> fn.win_execute (ALIGNED!)
		{LeftLine: 2, RightLine: 4},  // elseif tgt == 'pager' then
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("AlignLines() got = %v, want = %v", got, want)
	}
}

func TestAlignLinesContinuationCondition(t *testing.T) {
	src := []byte("if delta.type == 'workspaces' and\n  kong.default_workspace ~= delta.entity.id\nthen")
	dst := []byte("if delta.type == 'workspaces' and\n  delta_entity ~= nil and\n  kong.default_workspace ~= delta_entity.id\nthen")

	ms := engine.NewMapping()
	rootSrc := &treesitter.ASTNode{Type: "source_file", StartRow: 0, EndRow: 2}
	rootDst := &treesitter.ASTNode{Type: "source_file", StartRow: 0, EndRow: 3}
	parentSrc := &treesitter.ASTNode{Type: "binary_expression", StartRow: 0, EndRow: 1, Parent: rootSrc}
	parentDst := &treesitter.ASTNode{Type: "binary_expression", StartRow: 0, EndRow: 2, Parent: rootDst}
	condSrc := &treesitter.ASTNode{Type: "binary_expression", StartRow: 1, EndRow: 1, StartCol: 2, EndCol: 44, Parent: parentSrc}
	condDst := &treesitter.ASTNode{Type: "binary_expression", StartRow: 2, EndRow: 2, StartCol: 2, EndCol: 44, Parent: parentDst}

	ms.Add(rootSrc, rootDst)
	ms.Add(condSrc, condDst)

	got := AlignLines(src, dst, ms, nil)
	want := []LineAlignmentPair{
		{LeftLine: 0, RightLine: 0},  // if delta.type == 'workspaces' and
		{LeftLine: -1, RightLine: 1}, // filler <-> delta_entity ~= nil and
		{LeftLine: 1, RightLine: 2},  // kong.default_workspace ~= ... (ALIGNED!)
		{LeftLine: 2, RightLine: 3},  // then
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("AlignLines() got = %v, want = %v", got, want)
	}
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

	srcBytes := []byte(strings.Join(srcLines, "\n"))
	dstBytes := []byte(strings.Join(dstLines, "\n"))

	got := AlignLines(srcBytes, dstBytes, nil, nil)
	if len(got) == 0 {
		t.Fatalf("expected non-empty alignment grid")
	}
	if got[0].LeftLine != 0 || got[0].RightLine != 0 {
		t.Errorf("first line mismatch: %+v", got[0])
	}
	last := got[len(got)-1]
	if last.LeftLine != len(srcLines)-1 || last.RightLine != len(dstLines)-1 {
		t.Errorf("last line mismatch: %+v", last)
	}
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

	srcBytes := []byte(strings.Join(srcLines, "\n"))
	dstBytes := []byte(strings.Join(dstLines, "\n"))

	got := AlignLines(srcBytes, dstBytes, nil, nil)
	if len(got) == 0 {
		t.Fatalf("expected non-empty alignment grid")
	}

	// Verify top and bottom lines align
	if got[0].LeftLine != 0 || got[0].RightLine != 0 {
		t.Errorf("first line mismatch: %+v", got[0])
	}
	last := got[len(got)-1]
	if last.LeftLine != len(srcLines)-1 || last.RightLine != len(dstLines)-1 {
		t.Errorf("last line mismatch: %+v (expected LeftLine=%d, RightLine=%d)", last, len(srcLines)-1, len(dstLines)-1)
	}
}

func TestAlignLines_DataContainerAndASTMappingPrior(t *testing.T) {
	// Old JSON has validations with additionalProperties: false
	// New JSON has validations with additionalProperties: false, followed by inserted upload_validations with identical duplicate additionalProperties: false
	src := []byte(`{
  "validations": {
    "required": true,
    "additionalProperties": false
  },
  "assignee": "test"
}`)
	dst := []byte(`{
  "validations": {
    "required": true,
    "additionalProperties": false
  },
  "upload_validations": {
    "required": true,
    "additionalProperties": false
  },
  "assignee": "test"
}`)

	// Construct mock Mapping pairing the first additionalProperties (src line 3) with dst line 3
	ms := engine.NewMapping()
	n1 := &treesitter.ASTNode{Type: "pair", Label: "additionalProperties", StartRow: 3, EndRow: 3, Language: "json"}
	n2 := &treesitter.ASTNode{Type: "pair", Label: "additionalProperties", StartRow: 3, EndRow: 3, Language: "json"}
	ms.Add(n1, n2)

	got := AlignLines(src, dst, ms, nil)

	// Verify that src line 3 ("additionalProperties": false) aligns with dst line 3 (not dst line 7)
	foundMappedPair := false
	for _, p := range got {
		if p.LeftLine == 3 && p.RightLine == 3 {
			foundMappedPair = true
			break
		}
	}
	if !foundMappedPair {
		t.Errorf("expected src line 3 to align with dst line 3 via AST mapping, got alignment grid: %+v", got)
	}
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

	srcBytes := []byte(strings.Join(srcLines, "\n"))
	dstBytes := []byte(strings.Join(dstLines, "\n"))

	grid := AlignLines(srcBytes, dstBytes, nil, nil)
	if len(grid) == 0 {
		t.Fatal("expected non-empty alignment grid")
	}

	// Verify the middle anchor was aligned
	foundMiddle := false
	for _, pair := range grid {
		if pair.LeftLine == 2000 && pair.RightLine == 0 {
			foundMiddle = true
			break
		}
	}
	if !foundMiddle {
		t.Errorf("expected middle anchor (2000, 0) to align in grid")
	}
}

func TestAlignLines_PunctuationLineGuard(t *testing.T) {
	// Deleted if block vs inserted multi-block function with a coincidental `\t}` at line 3.
	src := []byte(strings.Join([]string{
		"func Add() {",
		"\tif _, exists := m.src[t1]; !exists {",
		"\t\tm.Pairs = append(m.Pairs, Pair{Src: t1, Dst: t2})",
		"\t}",
		"\tm.src[t1] = t2",
		"}",
	}, "\n"))

	dst := []byte(strings.Join([]string{
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
	}, "\n"))

	grid := AlignLines(src, dst, nil, nil)
	// Line 3 on left (`\t}`) must not be anchored to Line 3 on right (`\t}`).
	for _, pair := range grid {
		if pair.LeftLine == 3 && pair.RightLine == 3 {
			t.Errorf("unexpected false alignment on unmapped punctuation line 3: got LeftLine=3, RightLine=3")
		}
		if pair.LeftLine == 4 && pair.RightLine != 8 {
			t.Errorf("expected common tail m.src[t1] = t2 (left 4) to align with right 8, got %d", pair.RightLine)
		}
	}
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

	alignedMap := make(map[int]int)
	for _, p := range alignment {
		if p.LeftLine != -1 {
			alignedMap[p.LeftLine] = p.RightLine
		}
	}

	if alignedMap[1] != 2 {
		t.Errorf("expected left line 1 (let a = 1) to align with right line 2, got %d (grid: %+v)", alignedMap[1], alignment)
	}
	if alignedMap[2] != 3 {
		t.Errorf("expected left line 2 (let b = 2) to align with right line 3, got %d (grid: %+v)", alignedMap[2], alignment)
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
