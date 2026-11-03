package renderutil

import (
	"github.com/HarshK97/diffmantic/internal/serialize"
)

// IsPairChanged reports whether an alignment pair contains visible changes.
func IsPairChanged(
	pair serialize.LineAlignmentPair,
	leftSpans []serialize.HighlightSpan,
	rightSpans []serialize.HighlightSpan,
	sText, dText string,
	showStyleChanges bool,
	isEOFChange bool,
	significantWhitespace bool,
) bool {
	if pair.LeftLine == -1 || pair.RightLine == -1 {
		return true
	}
	if len(leftSpans) > 0 || len(rightSpans) > 0 {
		return true
	}
	if sText == dText {
		return isEOFChange
	}
	if showStyleChanges {
		return true
	}
	return isEOFChange || !serialize.EqualIgnoringStyle(sText, dText, significantWhitespace)
}
