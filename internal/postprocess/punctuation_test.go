package postprocess

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/testutil"
)

func TestFilterPunctuation_StrictPunctuationMoveSplit(t *testing.T) {
	// Create T1 (source) and T2 (destination) strict punctuation nodes (e.g. double quote or colon)
	srcPunct := testutil.Leaf("string_delimiter", "\"")
	dstPunct := testutil.Leaf("string_delimiter", "\"")
	dstParent := testutil.Leaf("string_literal", "")

	ms := engine.NewMapping()
	ms.Add(srcPunct, dstPunct)

	es := actions.NewEditScript()
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     srcPunct,
		Parent:   dstParent,
		Position: 0,
	})

	filtered := FilterPunctuation(es, ms)
	if filtered == nil {
		t.Fatal("expected non-nil edit script")
	}

	acts := filtered.Actions()
	if len(acts) != 2 {
		t.Fatalf("expected 2 actions (Delete + Insert), got %d", len(acts))
	}

	if acts[0].Type != actions.Delete || acts[0].Node != srcPunct {
		t.Errorf("expected first action to be Delete of srcPunct, got %+v", acts[0])
	}

	if acts[1].Type != actions.Insert || acts[1].Node != dstPunct {
		t.Errorf("expected second action to be Insert of dstPunct, got %+v", acts[1])
	}
}

func TestFilterPunctuation_SkipSemicolonAndComma(t *testing.T) {
	semiNode := testutil.Leaf("semicolon", ";")
	commaNode := testutil.Leaf("comma", ",")

	ms := engine.NewMapping()
	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Insert, Node: semiNode})
	es.Add(actions.Action{Type: actions.Delete, Node: commaNode})

	filtered := FilterPunctuation(es, ms)
	if filtered.Size() != 0 {
		t.Errorf("expected 0 actions after filtering commas and semicolons, got %d", filtered.Size())
	}
}

func TestFilterPunctuation_NilScript(t *testing.T) {
	if FilterPunctuation(nil, engine.NewMapping()) != nil {
		t.Error("expected nil for nil edit script")
	}
}
