package serialize

import (
	"maps"
	"reflect"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestAlignLines(t *testing.T) {
	tests := []struct {
		name     string
		srcBytes []byte
		dstBytes []byte
		setup    func() (*actions.EditScript, *engine.Mapping, *treesitter.ASTNode, *treesitter.ASTNode)
		want     []LineAlignmentPair
	}{
		{
			name:     "Identical single-line files",
			srcBytes: []byte("hello"),
			dstBytes: []byte("hello"),
			setup: func() (*actions.EditScript, *engine.Mapping, *treesitter.ASTNode, *treesitter.ASTNode) {
				return nil, nil, nil, nil
			},
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
			},
		},
		{
			name:     "Identical multi-line files",
			srcBytes: []byte("hello\nworld"),
			dstBytes: []byte("hello\nworld"),
			setup: func() (*actions.EditScript, *engine.Mapping, *treesitter.ASTNode, *treesitter.ASTNode) {
				return nil, nil, nil, nil
			},
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: 1, RightLine: 1},
			},
		},
		{
			name:     "Empty old file",
			srcBytes: []byte(""),
			dstBytes: []byte("new\nlines"),
			setup: func() (*actions.EditScript, *engine.Mapping, *treesitter.ASTNode, *treesitter.ASTNode) {
				return nil, nil, nil, nil
			},
			want: []LineAlignmentPair{
				{LeftLine: -1, RightLine: 0},
				{LeftLine: -1, RightLine: 1},
			},
		},
		{
			name:     "Empty new file",
			srcBytes: []byte("old\nlines"),
			dstBytes: []byte(""),
			setup: func() (*actions.EditScript, *engine.Mapping, *treesitter.ASTNode, *treesitter.ASTNode) {
				return nil, nil, nil, nil
			},
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: -1},
				{LeftLine: 1, RightLine: -1},
			},
		},
		{
			name:     "Both empty",
			srcBytes: []byte(""),
			dstBytes: []byte(""),
			setup: func() (*actions.EditScript, *engine.Mapping, *treesitter.ASTNode, *treesitter.ASTNode) {
				return nil, nil, nil, nil
			},
			want: []LineAlignmentPair{
				{LeftLine: -1, RightLine: 0},
			},
		},
		{
			name:     "Deleted lines",
			srcBytes: []byte("line1\nline2\nline3"),
			dstBytes: []byte("line1\nline3"),
			setup: func() (*actions.EditScript, *engine.Mapping, *treesitter.ASTNode, *treesitter.ASTNode) {
				return nil, nil, nil, nil
			},
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
			setup: func() (*actions.EditScript, *engine.Mapping, *treesitter.ASTNode, *treesitter.ASTNode) {
				return nil, nil, nil, nil
			},
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
			setup: func() (*actions.EditScript, *engine.Mapping, *treesitter.ASTNode, *treesitter.ASTNode) {
				// Mock mappings and nodes to simulate overlap on the middle line.
				srcL2 := testutil.NodeAtRC("identifier", "line2", 1, 0)
				srcRoot := testutil.Tree(&treesitter.ASTNode{Type: "module", StartRow: 0, EndRow: 2}, srcL2)

				dstL2 := testutil.NodeAtRC("identifier", "line2_changed", 1, 0)
				dstRoot := testutil.Tree(&treesitter.ASTNode{Type: "module", StartRow: 0, EndRow: 2}, dstL2)

				ms := engine.NewMapping()
				ms.Add(srcL2, dstL2)

				es := actions.NewEditScript()
				es.Add(actions.Action{
					Type:  actions.Update,
					Node:  srcL2,
					Value: "line2_changed",
				})

				return es, ms, srcRoot, dstRoot
			},
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: 1, RightLine: 1},
				{LeftLine: 2, RightLine: 2},
			},
		},
		{
			name:     "Move within same parent block does not flag lines as moved",
			srcBytes: []byte("func foo() {\n  a()\n  b()\n}"),
			dstBytes: []byte("func foo() {\n  b()\n  a()\n}"),
			setup: func() (*actions.EditScript, *engine.Mapping, *treesitter.ASTNode, *treesitter.ASTNode) {
				srcParent := &treesitter.ASTNode{Type: "block", StartRow: 0, EndRow: 3}
				dstParent := &treesitter.ASTNode{Type: "block", StartRow: 0, EndRow: 3}

				srcA := testutil.NodeAtRC("call", "a()", 1, 2)
				srcA.Parent = srcParent

				dstA := testutil.NodeAtRC("call", "a()", 2, 2)
				dstA.Parent = dstParent

				srcParent.Children = []*treesitter.ASTNode{srcA}
				dstParent.Children = []*treesitter.ASTNode{dstA}

				srcRoot := testutil.Tree(srcParent)
				dstRoot := testutil.Tree(dstParent)

				ms := engine.NewMapping()
				ms.Add(srcParent, dstParent)
				ms.Add(srcA, dstA)

				es := actions.NewEditScript()
				es.Add(actions.Action{
					Type: actions.Move,
					Node: srcA,
				})

				return es, ms, srcRoot, dstRoot
			},
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: -1, RightLine: 1},
				{LeftLine: 1, RightLine: 2},
				{LeftLine: 2, RightLine: -1},
				{LeftLine: 3, RightLine: 3},
			},
		},
		{
			name:     "Move across different parent blocks flags lines as moved",
			srcBytes: []byte("func foo() {\n  a()\n}\nfunc bar() {\n}"),
			dstBytes: []byte("func foo() {\n}\nfunc bar() {\n  a()\n}"),
			setup: func() (*actions.EditScript, *engine.Mapping, *treesitter.ASTNode, *treesitter.ASTNode) {
				srcParent1 := &treesitter.ASTNode{Type: "block", StartRow: 0, EndRow: 2}
				srcParent2 := &treesitter.ASTNode{Type: "block", StartRow: 3, EndRow: 4}

				dstParent1 := &treesitter.ASTNode{Type: "block", StartRow: 0, EndRow: 1}
				dstParent2 := &treesitter.ASTNode{Type: "block", StartRow: 2, EndRow: 4}

				srcA := testutil.NodeAtRC("call", "a()", 1, 2)
				srcA.Parent = srcParent1
				srcParent1.Children = []*treesitter.ASTNode{srcA}

				dstA := testutil.NodeAtRC("call", "a()", 3, 2)
				dstA.Parent = dstParent2
				dstParent2.Children = []*treesitter.ASTNode{dstA}

				srcRoot := testutil.Tree(srcParent1, srcParent2)
				dstRoot := testutil.Tree(dstParent1, dstParent2)

				ms := engine.NewMapping()
				ms.Add(srcParent1, dstParent1)
				ms.Add(srcParent2, dstParent2)
				ms.Add(srcA, dstA)

				es := actions.NewEditScript()
				es.Add(actions.Action{
					Type: actions.Move,
					Node: srcA,
				})

				return es, ms, srcRoot, dstRoot
			},
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: 1, RightLine: -1},
				{LeftLine: 2, RightLine: 1},
				{LeftLine: 3, RightLine: 2},
				{LeftLine: -1, RightLine: 3},
				{LeftLine: 4, RightLine: 4},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es, ms, srcRoot, dstRoot := tt.setup()
			got := AlignLines(tt.srcBytes, tt.dstBytes, es, ms, srcRoot, dstRoot)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AlignLines() got = %v, want = %v", got, tt.want)
			}
		})
	}
}

func TestCloseGaps(t *testing.T) {
	tests := []struct {
		name    string
		moved   map[int]bool
		maxLine int
		want    map[int]bool
	}{
		{
			name:    "close single line gap",
			moved:   map[int]bool{1: true, 3: true},
			maxLine: 5,
			want:    map[int]bool{1: true, 2: true, 3: true},
		},
		{
			name:    "close two line gap",
			moved:   map[int]bool{1: true, 4: true},
			maxLine: 6,
			want:    map[int]bool{1: true, 2: true, 3: true, 4: true},
		},
		{
			name:    "do not close gap of 6 lines or more",
			moved:   map[int]bool{1: true, 7: true},
			maxLine: 9,
			want:    map[int]bool{1: true, 7: true},
		},
		{
			name:    "unbounded gap at end",
			moved:   map[int]bool{1: true},
			maxLine: 5,
			want:    map[int]bool{1: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maps.Clone(tt.moved)
			closeGaps(got, tt.maxLine)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("closeGaps() got = %v, want = %v", got, tt.want)
			}
		})
	}
}

func TestCoalesceAlignmentGrid(t *testing.T) {
	srcLines := []string{"/**", " * line 1", " * line 2", " */"}
	dstLines := []string{"/**", " * line 1 modified", " * line 2 modified", " * line 3 extra", " */"}

	grid := []LineAlignmentPair{
		{LeftLine: 0, RightLine: 0},
		{LeftLine: 1, RightLine: -1},
		{LeftLine: -1, RightLine: 1},
		{LeftLine: 2, RightLine: -1},
		{LeftLine: -1, RightLine: 2},
		{LeftLine: -1, RightLine: 3},
		{LeftLine: 3, RightLine: 4},
	}

	got := coalesceAlignmentGrid(grid, srcLines, dstLines, nil, nil)
	want := []LineAlignmentPair{
		{LeftLine: 0, RightLine: 0},
		{LeftLine: 1, RightLine: 1},
		{LeftLine: 2, RightLine: 2},
		{LeftLine: 3, RightLine: 3},
		{LeftLine: -1, RightLine: 4},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("coalesceAlignmentGrid() got = %v, want = %v", got, want)
	}
}

func TestTrivialLineContainerMatching(t *testing.T) {
	srcParent := &treesitter.ASTNode{Type: "block", StartRow: 0, EndRow: 2}
	dstParent := &treesitter.ASTNode{Type: "block", StartRow: 3, EndRow: 5}

	srcChild := &treesitter.ASTNode{Type: "stmt", StartRow: 1, EndRow: 1}
	dstChild := &treesitter.ASTNode{Type: "stmt", StartRow: 4, EndRow: 4}

	srcParent.Children = []*treesitter.ASTNode{srcChild}
	dstParent.Children = []*treesitter.ASTNode{dstChild}
	srcChild.Parent = srcParent
	dstChild.Parent = dstParent

	srcRoot := &treesitter.ASTNode{Type: "root", StartRow: 0, EndRow: 2, Children: []*treesitter.ASTNode{srcParent}}
	dstRoot := &treesitter.ASTNode{Type: "root", StartRow: 0, EndRow: 5, Children: []*treesitter.ASTNode{dstParent}}
	srcParent.Parent = srcRoot
	dstParent.Parent = dstRoot

	ms := engine.NewMapping()

	// Unmapped blocks shouldn't align standalone closing braces.
	if areContainersMatched(srcRoot, dstRoot, 2, 5, ms) {
		t.Errorf("expected areContainersMatched to return false for unmapped blocks")
	}

	ms.Add(srcParent, dstParent)

	// Once mapped, matching braces should align.
	if !areContainersMatched(srcRoot, dstRoot, 2, 5, ms) {
		t.Errorf("expected areContainersMatched to return true for mapped blocks")
	}
}
