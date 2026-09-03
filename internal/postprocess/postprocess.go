package postprocess

import (
	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// Run applies postprocessing passes (collapsing subtrees and grouping moves) to the edit script.
func Run(
	es *actions.EditScript,
	ms *engine.Mapping,
	srcRoot, dstRoot *treesitter.ASTNode,
) *actions.EditScript {
	if es == nil {
		return nil
	}
	es = Collapse(es, ms, srcRoot, dstRoot)
	return GroupMoves(es)
}
