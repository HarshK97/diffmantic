package tui

import (
	"strings"
)

type searchMatch struct {
	virtualRow int    // virtual line index (0-indexed)
	startCol   int    // visual start column position (after tab expansion)
	endCol     int    // visual end column position (exclusive)
	pane       string // "left" or "right"
}

func (m *model) computeSearchMatches() {
	if m.searchQuery == "" {
		m.searchMatches = nil
		m.searchMatchIdx = -1
		return
	}

	q := strings.ToLower(m.searchQuery)
	var matches []searchMatch

	for vIdx, vl := range m.virtualLines {
		if vl.foldIdx >= 0 {
			continue
		}

		if vl.leftLine >= 0 && vl.leftLine < len(m.srcLines) {
			raw := m.srcLines[vl.leftLine]
			expanded, _ := expandLine(raw)
			lowerStr := strings.ToLower(expanded)

			startSearch := 0
			for {
				idx := strings.Index(lowerStr[startSearch:], q)
				if idx == -1 {
					break
				}
				matchStart := startSearch + idx
				matchEnd := matchStart + len(q)

				matches = append(matches, searchMatch{
					virtualRow: vIdx,
					startCol:   matchStart,
					endCol:     matchEnd,
					pane:       "left",
				})
				startSearch = matchStart + len(q)
			}
		}

		if vl.rightLine >= 0 && vl.rightLine < len(m.dstLines) {
			raw := m.dstLines[vl.rightLine]
			expanded, _ := expandLine(raw)
			lowerStr := strings.ToLower(expanded)

			startSearch := 0
			for {
				idx := strings.Index(lowerStr[startSearch:], q)
				if idx == -1 {
					break
				}
				matchStart := startSearch + idx
				matchEnd := matchStart + len(q)

				matches = append(matches, searchMatch{
					virtualRow: vIdx,
					startCol:   matchStart,
					endCol:     matchEnd,
					pane:       "right",
				})
				startSearch = matchStart + len(q)
			}
		}
	}

	m.searchMatches = matches
	m.findNearestSearchMatch()
}

func (m *model) findNearestSearchMatch() {
	if len(m.searchMatches) == 0 {
		m.searchMatchIdx = -1
		return
	}

	for idx, match := range m.searchMatches {
		if match.virtualRow > m.cursorY || (match.virtualRow == m.cursorY && match.pane == m.activePane && match.startCol >= m.cursorX) {
			m.searchMatchIdx = idx
			return
		}
	}
	m.searchMatchIdx = 0
}

func (m *model) jumpToSearchMatch() {
	if m.searchMatchIdx < 0 || m.searchMatchIdx >= len(m.searchMatches) {
		return
	}

	match := m.searchMatches[m.searchMatchIdx]
	m.cursorY = match.virtualRow
	m.cursorX = match.startCol
	m.activePane = match.pane

	m.clampCursor()
	m.keepCursorInViewport()
	m.updateInspectActions()
}

func (m model) searchMatchesFor(virtualRow int, pane string) []searchMatch {
	if len(m.searchMatches) == 0 {
		return nil
	}
	var res []searchMatch
	for _, match := range m.searchMatches {
		if match.virtualRow == virtualRow && match.pane == pane {
			res = append(res, match)
		}
	}
	return res
}
