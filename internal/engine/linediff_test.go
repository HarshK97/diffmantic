package engine

import (
	"reflect"
	"testing"
)

func TestLineDiff(t *testing.T) {
	tests := []struct {
		name     string
		linesA   []string
		linesB   []string
		expected map[int]int
	}{
		{
			name:     "basic match with insertions and deletions",
			linesA:   []string{"a", "b", "c", "d"},
			linesB:   []string{"a", "x", "c", "y", "d"},
			expected: map[int]int{0: 0, 2: 2, 3: 4},
		},
		{
			name:     "identical files",
			linesA:   []string{"one", "two", "three"},
			linesB:   []string{"one", "two", "three"},
			expected: map[int]int{0: 0, 1: 1, 2: 2},
		},
		{
			name:     "completely different files",
			linesA:   []string{"a", "b"},
			linesB:   []string{"x", "y"},
			expected: map[int]int{},
		},
		{
			name:     "empty inputs",
			linesA:   []string{},
			linesB:   []string{},
			expected: map[int]int{},
		},
		{
			name:     "prefix and suffix trim with middle changes",
			linesA:   []string{"head", "mid_old_1", "mid_old_2", "tail"},
			linesB:   []string{"head", "mid_new_1", "tail"},
			expected: map[int]int{0: 0, 3: 2},
		},
		{
			name: "indent heuristic slides duplicate lines to blank line and block boundaries",
			linesA: []string{
				"        assert(yielded == 3);", // 0
				"        raxStop(&ri);",         // 1
				"    }",                         // 2
				"",                              // 3
				"    raxFree(r);",               // 4
				"}",                             // 5
				"",                              // 6
				"TEST(\"delete\") {",            // 7
			},
			linesB: []string{
				"        assert(yielded == 3);", // 0
				"        raxStop(&ri);",         // 1
				"    }",                         // 2
				"",                              // 3
				"    raxFree(r);",               // 4
				"}",                             // 5
				"",                              // 6
				"TEST(\"insert\") {",            // 7
				"    raxStop(&ri);",             // 8
				"    raxFree(r);",               // 9
				"}",                             // 10
				"",                              // 11
				"TEST(\"delete\") {",            // 12
			},
			expected: map[int]int{
				0: 0,
				1: 1,
				2: 2,
				3: 3,
				4: 4, // Matches first copy (lines 4-5) instead of inserted second copy (lines 9-10)
				5: 5,
				6: 11, // Common blank line before TEST("delete")
				7: 12, // TEST("delete")
			},
		},
		{
			name: "prioritizes substantive code line over isolated closing brace when order inverts",
			linesA: []string{
				"c := engine.createContext(w, req, nil, handlers)", // 0
				"if engine.handlers404 == nil {",                   // 1
				"    http.NotFound(c.Writer, c.Req)",               // 2
				"} else {",                                         // 3
				"    c.Writer.WriteHeader(404)",                    // 4
				"}",                                                // 5
				"",                                                 // 6
				"c.Next()",                                         // 7
				"engine.reuseContext(c)",                           // 8
			},
			linesB: []string{
				"c := engine.createContext(w, req, nil, handlers)", // 0
				"c.Writer.setStatus(404)",                          // 1
				"c.Next()",                                         // 2
				"if !c.Writer.Written() {",                         // 3
				"    c.String(404, \"404 page not found\")",        // 4
				"}",                      // 5
				"engine.reuseContext(c)", // 6
			},
			expected: map[int]int{
				0: 0,
				7: 2, // c.Next() matched despite order inversion with '}'
				8: 6, // engine.reuseContext(c)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LineDiff(tt.linesA, tt.linesB)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("LineDiff() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestComputeLIS(t *testing.T) {
	t.Run("empty candidates", func(t *testing.T) {
		got := computeLIS(nil)
		if len(got) != 0 {
			t.Errorf("expected empty LIS for nil input, got %v", got)
		}
	})

	t.Run("strictly increasing candidates", func(t *testing.T) {
		candidates := []linePair{{i: 0, j: 0}, {i: 1, j: 1}, {i: 2, j: 2}}
		got := computeLIS(candidates)
		if len(got) != 3 {
			t.Fatalf("expected LIS length 3, got %d", len(got))
		}
		for idx, p := range got {
			if p.i != idx || p.j != idx {
				t.Errorf("LIS[%d] = %v, want {%d, %d}", idx, p, idx, idx)
			}
		}
	})

	t.Run("unordered candidates LIS extraction", func(t *testing.T) {
		candidates := []linePair{
			{i: 0, j: 3},
			{i: 1, j: 1},
			{i: 2, j: 4},
			{i: 3, j: 2},
		}
		got := computeLIS(candidates)
		if len(got) != 2 {
			t.Fatalf("expected LIS length 2, got %d", len(got))
		}
		if got[0].j >= got[1].j {
			t.Errorf("expected strictly increasing j values in LIS, got %v", got)
		}
	})
}

func TestLineDiffPatience(t *testing.T) {
	subA := []string{"start", "uniqueA", "middleA", "uniqueB", "end"}
	subB := []string{"start", "uniqueA", "middleB", "uniqueB", "end"}
	matchedA := make(map[int]int)

	lineDiffPatience(subA, subB, 0, 0, matchedA)

	expected := map[int]int{
		0: 0, // start
		1: 1, // uniqueA
		3: 3, // uniqueB
		4: 4, // end
	}

	if !reflect.DeepEqual(matchedA, expected) {
		t.Errorf("lineDiffPatience() = %v, want %v", matchedA, expected)
	}
}

func TestLineDiffLargeMatrixFallback(t *testing.T) {
	// 1001 lines on each side pushes the matrix over 1M cells (1001 x 1001), triggering patience diff.
	n := 1001
	linesA := make([]string, n)
	linesB := make([]string, n)
	for i := 0; i < n; i++ {
		str := string(rune('a'+(i%26))) + "_" + string(rune('A'+((i/26)%26))) + "_" + string(rune('0'+(i%10))) + "_" + string(rune('z'-(i%26))) + "_" + string(rune('Z'-((i/26)%26))) + "_" + string(rune('9'-(i%10))) + "_" + string(rune(i))
		linesA[i] = str
		linesB[i] = str
	}

	got := LineDiff(linesA, linesB)
	if len(got) != n {
		t.Errorf("expected all %d lines to be matched, got %d", n, len(got))
	}
}

func TestLineDiff_DiagonalTieBreaking(t *testing.T) {
	// File A has a block "target" at line 1.
	// File B has two identical "target" blocks: at line 1 (in-place) and at line 15 (inserted).
	linesA := []string{
		"header",
		"target_line_1",
		"target_line_2",
		"footer",
	}
	linesB := []string{
		"header",
		"target_line_1",
		"target_line_2",
		"other_1",
		"other_2",
		"target_line_1",
		"target_line_2",
		"footer",
	}

	got := LineDiff(linesA, linesB)

	// target lines (1, 2) in linesA should match lines (1, 2) in linesB, not (5, 6)
	if got[1] != 1 || got[2] != 2 {
		t.Errorf("expected diagonal tie-breaking to match (1->1, 2->2), got (1->%d, 2->%d)", got[1], got[2])
	}
}

func TestLineDiff_ReindentedSiblingBlocks(t *testing.T) {
	linesA := []string{
		"\t\tif body1 != nil && body2 != nil && m.Src()[body1] == body2 {", // 0
		"\t\t\tm.Add(p1, p2)", // 1
		"\t\t\tif isDecl {",   // 2
		"\t\t\t\treconcileDeclarationSignatures(p1, p2, m, r)", // 3
		"\t\t\t}",                  // 4
		"\t\t\tRecover(p1, p2, m)", // 5
		"\t\t\tcontinue",           // 6
		"\t\t}",                    // 7
		"\t\tif body1 != nil && body2 != nil && !m.Has(body1) {", // 8
		"\t\t\tif c1 != body1 && c2 != body2 {",                  // 9
		"\t\t\t\tif lineDist > 2 {",                              // 10
		"\t\t\t\t\tcontinue",                                     // 11
		"\t\t\t\t}",                                              // 12
		"\t\t\t\tm.Add(p1, p2)",                                  // 13
		"\t\t\t\tRecover(p1, p2, m)",                             // 14
		"\t\t\t\tcontinue",                                       // 15
		"\t\t\t}",                                                // 16
		"\t\t}",                                                  // 17
	}
	linesB := []string{
		"\t\tif body1 != nil && body2 != nil {",                  // 0
		"\t\t\tif m.Src()[body1] == body2 {",                     // 1
		"\t\t\t\tm.Add(p1, p2)",                                  // 2
		"\t\t\t\tif isDecl {",                                    // 3
		"\t\t\t\t\treconcileDeclarationSignatures(p1, p2, m, r)", // 4
		"\t\t\t\t}",                  // 5
		"\t\t\t\tRecover(p1, p2, m)", // 6
		"\t\t\t\tcontinue",           // 7
		"\t\t\t}",                    // 8
		"\t\t\tif !m.Has(body1) {",   // 9
		"\t\t\t\tif c1 != body1 && c2 != body2 {", // 10
		"\t\t\t\t\tif lineDist > 2 {",             // 11
		"\t\t\t\t\t\tcontinue",                    // 12
		"\t\t\t\t\t}",                             // 13
		"\t\t\t\t\tm.Add(p1, p2)",                 // 14
		"\t\t\t\t\tRecover(p1, p2, m)",            // 15
		"\t\t\t\t\tcontinue",                      // 16
		"\t\t\t\t}",                               // 17
		"\t\t\t}",                                 // 18
		"\t\t}",                                   // 19
	}

	got := LineDiff(linesA, linesB)

	// Case 1 lines (1, 5) in A must align with Case 1 lines (2, 6) in B,
	// and Case 2 lines (13, 14) in A must align with Case 2 lines (14, 15) in B,
	// rather than cross-matching Case 2 in A (4 tabs) with Case 1 in B (4 tabs).
	if got[1] != 2 || got[5] != 6 {
		t.Errorf("expected Case 1 re-indented lines (1->2, 5->6), got (1->%d, 5->%d)", got[1], got[5])
	}
	if got[13] != 14 || got[14] != 15 {
		t.Errorf("expected Case 2 re-indented lines (13->14, 14->15), got (13->%d, 14->%d)", got[13], got[14])
	}
}
