package postprocess

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestCollapseDivergence(t *testing.T) {
	// Construct source tree:
	// P (block)
	//   C (identifier)
	cSrc := &treesitter.ASTNode{Type: "block", StartByte: 10, EndByte: 20}
	pSrc := &treesitter.ASTNode{Type: "block", StartByte: 0, EndByte: 100, Children: []*treesitter.ASTNode{cSrc}}
	pSrc.Language = "python"
	cSrc.Parent = pSrc

	// Construct destination tree:
	// Q (block)
	//   R (block)
	//     D (block)
	dDst := &treesitter.ASTNode{Type: "block", StartByte: 60, EndByte: 70}
	rDst := &treesitter.ASTNode{Type: "block", StartByte: 50, EndByte: 150, Children: []*treesitter.ASTNode{dDst}}
	dDst.Parent = rDst
	qDst := &treesitter.ASTNode{Type: "block", StartByte: 0, EndByte: 200, Children: []*treesitter.ASTNode{rDst}}
	qDst.Language = "python"
	rDst.Parent = qDst

	// Mappings:
	// pSrc -> qDst
	// cSrc -> dDst
	// Note that this mapping is depth-inconsistent: dDst.Parent (rDst) != qDst
	ms := engine.NewMapping()
	ms.Add(pSrc, qDst)
	ms.Add(cSrc, dDst)

	// Construct EditScript:
	// P moves to Q
	// C moves with Parent = qDst (manually set, diverging from dDst.Parent = rDst)
	es := actions.NewEditScript()
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     pSrc,
		Parent:   qDst,
		Position: 0,
	})
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     cSrc,
		Parent:   qDst, // Manually set to match P's destination to trigger the divergence
		Position: 0,
	})

	// Run Collapse
	collapsed := Collapse(es, ms, pSrc, qDst)

	// Since we set C's Parent to qDst, it matches dstParent (qDst) in the parent-equality check.
	// Therefore, Collapse thinks they moved to the same parent and collapses them!
	// C's Move action is suppressed, and P's Move action is marked as Subtree: true.
	if collapsed.Size() != 1 {
		t.Fatalf("expected collapsed edit script to have size 1, got %d", collapsed.Size())
	}

	collapsedActions := collapsed.Actions()
	pAction := collapsedActions[0]
	if pAction.Node != pSrc {
		t.Errorf("expected action node to be pSrc, got %v", pAction.Node)
	}
	if !pAction.Subtree {
		t.Errorf("expected action to be a subtree move, but Subtree is false")
	}

	// This demonstrates the divergence: the mapping was depth-inconsistent (dDst.Parent != qDst),
	// but the check passed and collapsed them anyway because the edit script had C's Parent set to Q.
}
