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
			moved:   map[int]bool{0: true, 7: true},
			maxLine: 8,
			want:    map[int]bool{0: true, 7: true},
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
