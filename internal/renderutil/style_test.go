package renderutil

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/serialize"
)

func TestIsPairChanged(t *testing.T) {
	pair := serialize.LineAlignmentPair{LeftLine: 0, RightLine: 0}
	unpairedLeft := serialize.LineAlignmentPair{LeftLine: 0, RightLine: -1}
	unpairedRight := serialize.LineAlignmentPair{LeftLine: -1, RightLine: 0}

	span := serialize.HighlightSpan{StartCol: 0, EndCol: 4}

	// Unpaired lines are always treated as changes.
	if !IsPairChanged(unpairedLeft, nil, nil, "a", "", false, false, false) {
		t.Errorf("expected unpaired left line to be changed")
	}
	if !IsPairChanged(unpairedRight, nil, nil, "", "b", false, false, false) {
		t.Errorf("expected unpaired right line to be changed")
	}

	// Lines with active highlight spans are always treated as changes.
	if !IsPairChanged(pair, []serialize.HighlightSpan{span}, nil, "text", "text", false, false, false) {
		t.Errorf("expected line with left spans to be changed")
	}
	if !IsPairChanged(pair, nil, []serialize.HighlightSpan{span}, "text", "text", false, false, false) {
		t.Errorf("expected line with right spans to be changed")
	}

	// Exact text matches only change if there's an EOF newline shift.
	if IsPairChanged(pair, nil, nil, "exact", "exact", false, false, false) {
		t.Errorf("expected exact match without EOF change to be unchanged")
	}
	if !IsPairChanged(pair, nil, nil, "exact", "exact", false, true, false) {
		t.Errorf("expected exact match with EOF change to be changed")
	}

	// Style-only changes depend on showStyleChanges and significantWhitespace.
	sText := "    x = 1"
	dText := "  x = 1"

	// With insignificant whitespace (e.g. Go, Rust):
	// By default (showStyleChanges=false), should be unchanged
	if IsPairChanged(pair, nil, nil, sText, dText, false, false, false) {
		t.Errorf("expected style change to be unchanged when showStyleChanges=false and significantWhitespace=false")
	}
	// When showStyleChanges=true, should be changed
	if !IsPairChanged(pair, nil, nil, sText, dText, true, false, false) {
		t.Errorf("expected style change to be changed when showStyleChanges=true")
	}

	// With significant whitespace (e.g. Python):
	// Indentation change is always changed even when showStyleChanges=false
	if !IsPairChanged(pair, nil, nil, sText, dText, false, false, true) {
		t.Errorf("expected indentation change to be changed when significantWhitespace=true")
	}

	// Interior whitespace formatting:
	sInterior := "val x = 1"
	dInterior := "val    x = 1"
	if IsPairChanged(pair, nil, nil, sInterior, dInterior, false, false, true) {
		t.Errorf("expected interior whitespace difference to be unchanged when showStyleChanges=false")
	}
	if !IsPairChanged(pair, nil, nil, sInterior, dInterior, true, false, true) {
		t.Errorf("expected interior whitespace difference to be changed when showStyleChanges=true")
	}
}

func TestIsPairChanged_TwoPlane(t *testing.T) {
	pair := serialize.LineAlignmentPair{LeftLine: 0, RightLine: 0}
	sText := "    x = 1"
	dText := "  x = 1"

	// When the AST is identical, hide re-indentation by default unless asked.
	if IsPairChanged(pair, nil, nil, sText, dText, false, false, false) {
		t.Errorf("AST plane: expected formatting change to be suppressed by default")
	}
	if !IsPairChanged(pair, nil, nil, sText, dText, true, false, false) {
		t.Errorf("AST plane: expected formatting change to be revealed with showStyleChanges=true")
	}

	// In line-diff fallback, keep indentation changes so we don't silently hide Python dedents.
	if !IsPairChanged(pair, nil, nil, sText, dText, false, false, true) {
		t.Errorf("Fallback plane: expected indentation change to be marked changed")
	}
}
