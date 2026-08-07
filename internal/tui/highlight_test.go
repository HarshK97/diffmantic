package tui

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/serialize"
)

func TestMergeAllSpansByKind(t *testing.T) {
	srcBytes := []byte("    for cookie in cj:\n")

	moveA := &serialize.Action{
		Action:  "move",
		Node:    &serialize.NodeRef{Type: "for", Label: "for", StartByte: 4, EndByte: 7},
		GroupID: "group-1",
	}
	moveB := &serialize.Action{
		Action:  "move",
		Node:    &serialize.NodeRef{Type: "identifier", Label: "cookie", StartByte: 8, EndByte: 14},
		GroupID: "group-1",
	}
	moveC := &serialize.Action{
		Action:  "move",
		Node:    &serialize.NodeRef{Type: "in", Label: "in", StartByte: 15, EndByte: 17},
		GroupID: "group-1",
	}
	moveD := &serialize.Action{
		Action:  "move",
		Node:    &serialize.NodeRef{Type: "identifier", Label: "cj", StartByte: 18, EndByte: 20},
		GroupID: "group-1",
	}
	deleteContainer := &serialize.Action{
		Action: "delete",
		Node:   &serialize.NodeRef{Type: "for_statement", StartByte: 0, EndByte: 21},
	}

	// Spans are purposefully interleaved with a full-line container delete span.
	hl := &highlights{
		spans: map[int][]span{
			0: {
				{startCol: 0, endCol: 21, kind: kindDelete, totalLen: 21, action: deleteContainer},
				{startCol: 4, endCol: 7, kind: kindMove, totalLen: 3, action: moveA},
				{startCol: 8, endCol: 14, kind: kindMove, totalLen: 6, action: moveB},
				{startCol: 15, endCol: 17, kind: kindMove, totalLen: 2, action: moveC},
				{startCol: 18, endCol: 20, kind: kindMove, totalLen: 2, action: moveD},
			},
		},
		tinted: map[int]actionKind{0: kindDelete},
	}

	mergeAllSpans(hl, srcBytes)

	spans := hl.spans[0]
	var moveSpans []span
	for _, s := range spans {
		if s.kind == kindMove {
			moveSpans = append(moveSpans, s)
		}
	}

	if len(moveSpans) != 1 {
		t.Fatalf("expected 1 merged move span, got %d", len(moveSpans))
	}

	ms := moveSpans[0]
	if ms.startCol != 4 || ms.endCol != 20 {
		t.Errorf("expected merged move span cols 4..20, got %d..%d", ms.startCol, ms.endCol)
	}
}

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
					Parent:    &serialize.NodeRef{Tree: "after", Path: []int{0}},
					OldParent: &serialize.NodeRef{Tree: "before", Path: []int{0}},
					GroupID:   "grp1",
				},
				{
					Action:    "move",
					Node:      &serialize.NodeRef{StartByte: 6, EndByte: 7},
					Parent:    &serialize.NodeRef{Tree: "after", Path: []int{1}},
					OldParent: &serialize.NodeRef{Tree: "before", Path: []int{1}},
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
			srcHL, dstHL := buildHighlights(srcBytes, dstBytes, tt.actions)

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
