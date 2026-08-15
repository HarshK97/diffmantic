package tui

import (
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
