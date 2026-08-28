package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Vertical scroll
	if msg.Button == tea.MouseButtonWheelUp {
		m.hoverOpen = false
		m.scrollY = clamp(m.scrollY-3, 0, m.maxScrollY())
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelDown {
		m.hoverOpen = false
		m.scrollY = clamp(m.scrollY+3, 0, m.maxScrollY())
		return m, nil
	}

	// Horizontal scroll
	if msg.Button == tea.MouseButtonWheelLeft {
		m.hoverOpen = false
		m.scrollHorizontal(-4)
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelRight {
		m.hoverOpen = false
		m.scrollHorizontal(4)
		return m, nil
	}

	if msg.Action == tea.MouseActionMotion {
		if m.helpOpen || m.gitTreeOpen || m.gitCommitOpen {
			if m.hoverSource == "mouse" {
				m.hoverOpen = false
			}
			return m, nil
		}
		Y := msg.Y
		X := msg.X
		contentH := m.contentHeight()
		if Y >= titleBarHeight && Y < titleBarHeight+contentH {
			visualRow := Y - titleBarHeight
			vIdx := m.scrollY + visualRow
			if vIdx >= 0 && vIdx < len(m.virtualLines) {
				vl := m.virtualLines[vIdx]
				if vl.foldIdx < 0 {
					pw := m.paneWidth()
					gw := m.gutterWidth()
					var pane string
					var lineIdx int
					var visualCol int
					if X >= gw && X < pw {
						pane = "left"
						lineIdx = vl.leftLine
						visualCol = m.scrollXLeft + (X - gw)
					} else if X >= pw+dividerWidth+gw && X < pw+dividerWidth+pw {
						pane = "right"
						lineIdx = vl.rightLine
						visualCol = m.scrollXRight + (X - (pw + dividerWidth + gw))
					}
					if pane != "" && lineIdx >= 0 {
						actions := actionsAtCoord(&m, pane, lineIdx, visualCol)
						if len(actions) > 0 {
							m.hoverOpen = true
							m.hoverActions = actions
							m.hoverSource = "mouse"
							m.hoverX = X
							m.hoverY = visualRow
							return m, nil
						}
					}
				}
			}
		}
		if m.hoverSource == "mouse" {
			m.hoverOpen = false
		}
		return m, nil
	}

	// Left click
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		m.hoverOpen = false
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
