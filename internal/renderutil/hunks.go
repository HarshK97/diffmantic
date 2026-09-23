package renderutil

// Interval represents an inclusive index range [Start, End].
type Interval struct {
	Start int
	End   int
}

// BuildChangeIntervals collapses consecutive changed line pairs into intervals.
func BuildChangeIntervals(isPairChanged []bool) []Interval {
	var changeIntervals []Interval
	inChange := false
	startIdx := 0

	for i, changed := range isPairChanged {
		if changed {
			if !inChange {
				inChange = true
				startIdx = i
			}
		} else {
			if inChange {
				changeIntervals = append(changeIntervals, Interval{Start: startIdx, End: i - 1})
				inChange = false
			}
		}
	}
	if inChange {
		changeIntervals = append(changeIntervals, Interval{Start: startIdx, End: len(isPairChanged) - 1})
	}
	return changeIntervals
}

// MergeHunks expands change intervals by contextLines and merges overlapping hunks.
func MergeHunks(changeIntervals []Interval, totalPairs, contextLines int) []Interval {
	var hunks []Interval
	for _, ci := range changeIntervals {
		hStart := max(0, ci.Start-contextLines)
		hEnd := min(totalPairs-1, ci.End+contextLines)

		if len(hunks) > 0 && hStart <= hunks[len(hunks)-1].End+1 {
			hunks[len(hunks)-1].End = hEnd
		} else {
			hunks = append(hunks, Interval{Start: hStart, End: hEnd})
		}
	}
	return hunks
}
