package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestLinePartition(t *testing.T) {
	t.Run("Identical lines classification and CanMatch", func(t *testing.T) {
		srcA := []byte("func main() {\n\tx := 1\n}\n")
		srcB := []byte("func main() {\n\tx := 1\n}\n")

		part := NewLinePartition(srcA, srcB)

		n1 := &treesitter.ASTNode{StartRow: 0, EndRow: 2}
		n2 := &treesitter.ASTNode{StartRow: 0, EndRow: 2}

		if !part.CanMatch(n1, n2) {
			t.Errorf("expected identical subtrees on matching lines to be allowed")
		}
	})

	t.Run("Forbids cross-matching between Group 1 (non-edited) and Group 2 (edited)", func(t *testing.T) {
		srcA := []byte("func main() {\n\t// untouched\n\tdeletedLine()\n}\n")
		srcB := []byte("func main() {\n\t// untouched\n\tinsertedLine()\n}\n")

		part := NewLinePartition(srcA, srcB)

		n1 := &treesitter.ASTNode{StartRow: 1, EndRow: 1}
		n2 := &treesitter.ASTNode{StartRow: 2, EndRow: 2}

		if part.CanMatch(n1, n2) {
			t.Errorf("expected CanMatch to return false between Group 1 (non-edited) and Group 2 (edited) lines")
		}
	})

	t.Run("Allows matching between Group 2 (edited) and Group 2 (edited) nodes", func(t *testing.T) {
		srcA := []byte("func oldFunc() {\n\tdeletedCode()\n}\n")
		srcB := []byte("func newFunc() {\n\tinsertedCode()\n}\n")

		part := NewLinePartition(srcA, srcB)

		n1 := &treesitter.ASTNode{StartRow: 1, EndRow: 1}
		n2 := &treesitter.ASTNode{StartRow: 1, EndRow: 1}

		if !part.CanMatch(n1, n2) {
			t.Errorf("expected CanMatch to return true between two Group 2 (edited) nodes")
		}
	})

	t.Run("Forbids Group 1 matching on non-corresponding line offsets", func(t *testing.T) {
		srcA := []byte("lineA0\nlineA1\nlineA2\n")
		srcB := []byte("header\nlineA0\nlineA1\nlineA2\n")

		part := NewLinePartition(srcA, srcB)

		n1 := &treesitter.ASTNode{StartRow: 0, EndRow: 0}
		n2Correct := &treesitter.ASTNode{StartRow: 1, EndRow: 1}
		n2Wrong := &treesitter.ASTNode{StartRow: 2, EndRow: 2}

		if !part.CanMatch(n1, n2Correct) {
			t.Errorf("expected CanMatch to return true for mapped line offsets")
		}
		if part.CanMatch(n1, n2Wrong) {
			t.Errorf("expected CanMatch to return false for non-corresponding Group 1 lines")
		}
	})
}
