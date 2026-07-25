package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Vertical scroll
	if msg.Button == tea.MouseButtonWheelUp {
		m.scrollY = clamp(m.scrollY-3, 0, m.maxScrollY())
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelDown {
		m.scrollY = clamp(m.scrollY+3, 0, m.maxScrollY())
		return m, nil
	}

	// Horizontal scroll
	if msg.Button == tea.MouseButtonWheelLeft {
		m.scrollHorizontal(-4)
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelRight {
		m.scrollHorizontal(4)
		return m, nil
	}

	// Left click
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		Y := msg.Y
		X := msg.X

		if Y < titleBarHeight {
			return m, nil
		}

		contentH := m.contentHeight()
		if Y >= titleBarHeight && Y < titleBarHeight+contentH {
			visualRow := Y - titleBarHeight
			vIdx := m.scrollY + visualRow

			if vIdx >= 0 && vIdx < len(m.virtualLines) {
				vl := m.virtualLines[vIdx]

				if vl.foldIdx >= 0 {
					m.toggleFoldAt(vIdx)
					m.updateInspectActions()
					return m, nil
				}

				pw := m.paneWidth()
				gw := m.gutterWidth()

				isLeftGutter := X < gw
				isRightGutter := X >= pw+dividerWidth && X < pw+dividerWidth+gw

				// Click on gutter toggles fold.
				if isLeftGutter || isRightGutter {
					fi := foldAtVirtual(m.virtualLines, m.folds, vIdx)
					if fi >= 0 {
						if isLeftGutter {
							m.activePane = "left"
						} else {
							m.activePane = "right"
						}
						m.toggleFoldAt(vIdx)
						m.updateInspectActions()
						return m, nil
					}
				}

				if X < pw {
					m.activePane = "left"
					m.cursorY = vIdx
				} else if X >= pw+dividerWidth {
					m.activePane = "right"
					m.cursorY = vIdx
				}
				m.clampCursor()
				m.keepCursorInViewport()
				m.updateInspectActions()
			}
		}
	}

	return m, nil
}

func (m *model) scrollHorizontal(delta int) {
	tw := m.textWidth()
	m.scrollXLeft = clamp(m.scrollXLeft+delta, 0, maxScrollX(m.srcLines, tw))
	m.scrollXRight = clamp(m.scrollXRight+delta, 0, maxScrollX(m.dstLines, tw))
}
