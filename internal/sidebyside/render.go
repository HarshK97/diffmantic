package sidebyside

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/HarshK97/diffmantic/internal/color"
	"github.com/HarshK97/diffmantic/internal/inline"
	"github.com/HarshK97/diffmantic/internal/renderutil"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
	"golang.org/x/term"
)

type errWriter struct {
	w   io.Writer
	err error
}

func (ew *errWriter) Write(p []byte) (int, error) {
	if ew.err != nil {
		return 0, ew.err
	}
	var n int
	n, ew.err = ew.w.Write(p)
	return n, ew.err
}

// Render streams the aligned side-by-side diff of src and dst to w.
func Render(
	srcFile, dstFile string,
	srcBytes, dstBytes []byte,
	env *serialize.Envelope,
	opts RenderOptions,
	w io.Writer,
) error {
	if env == nil {
		return nil
	}

	ew := &errWriter{w: w}
	w = ew

	if env.IsBinary {
		if opts.Color {
			_, err := fmt.Fprintf(w, "%sBinary files %s and %s differ%s\n", color.UpdateFg, srcFile, dstFile, color.Reset)
			return err
		}
		_, err := fmt.Fprintf(w, "Binary files %s and %s differ\n", srcFile, dstFile)
		return err
	}

	if len(env.LineAlignment) == 0 {
		return nil
	}

	termWidth := opts.TerminalWidth
	if termWidth <= 0 {
		if wFd, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && wFd > 0 {
			termWidth = wFd
		} else {
			termWidth = 80
		}
	}

	srcLines := strings.Split(string(srcBytes), "\n")
	dstLines := strings.Split(string(dstBytes), "\n")

	srcEndsWithNL := len(srcBytes) > 0 && srcBytes[len(srcBytes)-1] == '\n'
	dstEndsWithNL := len(dstBytes) > 0 && dstBytes[len(dstBytes)-1] == '\n'
	lastSrcLineIdx := -1
	if len(srcBytes) > 0 {
		lastSrcLineIdx = len(srcLines) - 1
	}
	lastDstLineIdx := -1
	if len(dstBytes) > 0 {
		lastDstLineIdx = len(dstLines) - 1
	}

	filteredPairs := env.LineAlignment
	maxLineNum := max(len(srcLines), len(dstLines))
	numWidth := max(3, len(strconv.Itoa(maxLineNum)))

	leftSpansByLine := make(map[int][]serialize.HighlightSpan, len(env.LeftHighlights))
	for _, sp := range env.LeftHighlights {
		leftSpansByLine[sp.Line] = append(leftSpansByLine[sp.Line], sp)
	}

	rightSpansByLine := make(map[int][]serialize.HighlightSpan, len(env.RightHighlights))
	for _, sp := range env.RightHighlights {
		rightSpansByLine[sp.Line] = append(rightSpansByLine[sp.Line], sp)
	}

	isPairChanged := make([]bool, len(filteredPairs))
	for i, pair := range filteredPairs {
		if pair.LeftLine == -1 || pair.RightLine == -1 {
			isPairChanged[i] = true
			continue
		}
		if len(leftSpansByLine[pair.LeftLine]) > 0 || len(rightSpansByLine[pair.RightLine]) > 0 {
			isPairChanged[i] = true
			continue
		}
		srcText := ""
		if pair.LeftLine >= 0 && pair.LeftLine < len(srcLines) {
			srcText = srcLines[pair.LeftLine]
		}
		dstText := ""
		if pair.RightLine >= 0 && pair.RightLine < len(dstLines) {
			dstText = dstLines[pair.RightLine]
		}
		if srcText != dstText {
			isPairChanged[i] = true
		}
	}

	gutterWidth := 0
	if opts.LineNumbers {
		gutterWidth = (numWidth + 1) * 2 // left gutter + right gutter
	}
	separatorWidth := 2 // "  "
	availableWidth := max(20, termWidth-gutterWidth-separatorWidth)
	codeWidth := availableWidth / 2

	if !opts.ForceSideBySide && (termWidth < 80 || codeWidth < 30) {
		inlineOpts := inline.RenderOptions{
			Color:              opts.Color,
			ContextLines:       opts.ContextLines,
			LineNumbers:        opts.LineNumbers,
			DisableAnnotations: opts.DisableAnnotations,
		}
		output := inline.Render(srcFile, dstFile, srcBytes, dstBytes, env, inlineOpts)
		_, err := io.WriteString(w, output)
		return err
	}

	if codeWidth < 10 {
		codeWidth = 10
	}

	changeIntervals := renderutil.BuildChangeIntervals(isPairChanged)
	contextLines := opts.ContextLines
	if contextLines < 0 {
		contextLines = len(filteredPairs)
	}
	hunks := renderutil.MergeHunks(changeIntervals, len(filteredPairs), contextLines)

	srcOffsets := serialize.BuildLineIndex(srcBytes)
	dstOffsets := serialize.BuildLineIndex(dstBytes)

	var r *rules.Rules
	if langName, err := treesitter.DetectLanguageName(srcFile); err == nil {
		r = rules.Get(langName)
	} else if langName, err := treesitter.DetectLanguageName(dstFile); err == nil {
		r = rules.Get(langName)
	}

	var srcLineBadges, dstLineBadges map[int]string
	var srcLineBadgeColors, dstLineBadgeColors map[int]int
	var hunkHeaders map[int]string
	if !opts.DisableAnnotations {
		badges := renderutil.BuildMoveBadges(
			env.Actions,
			hunks,
			filteredPairs,
			srcOffsets,
			dstOffsets,
			srcLines,
			dstLines,
			r,
			true,
		)
		srcLineBadges = badges.SrcLineBadges
		dstLineBadges = badges.DstLineBadges
		srcLineBadgeColors = badges.SrcLineBadgeColors
		dstLineBadgeColors = badges.DstLineBadgeColors
		hunkHeaders = badges.HunkHeaders
	}

	scratch := &RenderScratch{TabWidth: opts.TabWidth}
	sep := "  "

	hasSrc := len(srcBytes) > 0 && srcFile != os.DevNull && srcFile != "/dev/null"
	hasDst := len(dstBytes) > 0 && dstFile != os.DevNull && dstFile != "/dev/null"

	if len(srcBytes) == 0 && !opts.ForceSideBySide {
		return renderWholeFileSingleColumn(dstLines, rightSpansByLine, dstLineBadges, numWidth, termWidth, opts, scratch, color.ActionInsert, w, dstEndsWithNL)
	}
	if len(dstBytes) == 0 && !opts.ForceSideBySide {
		return renderWholeFileSingleColumn(srcLines, leftSpansByLine, srcLineBadges, numWidth, termWidth, opts, scratch, color.ActionDelete, w, srcEndsWithNL)
	}

	for i, h := range hunks {
		if ew.err != nil {
			return ew.err
		}
		layout := ClassifyHunk(h, filteredPairs, isPairChanged, srcLines, dstLines, leftSpansByLine, rightSpansByLine, opts.ForceSideBySide)

		srcStart, srcCount, dstStart, dstCount := renderutil.ComputeHunkRange(h, filteredPairs, hasSrc, hasDst)
		hunkHdrExtra := hunkHeaders[i]
		hunkHeader := renderutil.FormatHunkHeader(srcStart, srcCount, dstStart, dstCount, hunkHdrExtra)
		if opts.Color {
			_, _ = fmt.Fprintf(w, "%s%s%s\n", color.HeaderFg, hunkHeader, color.Reset)
		} else {
			_, _ = fmt.Fprintf(w, "%s\n", hunkHeader)
		}

		if layout == HunkLayoutDualColumn {
			renderSideBySideHunk(w, h, filteredPairs, srcLines, dstLines, srcLineBadges, dstLineBadges, srcLineBadgeColors, dstLineBadgeColors, leftSpansByLine, rightSpansByLine, numWidth, codeWidth, opts, scratch, sep, srcEndsWithNL, dstEndsWithNL, lastSrcLineIdx, lastDstLineIdx)
		} else {
			renderSingleColumnHunk(w, h, filteredPairs, isPairChanged, srcLines, dstLines, srcLineBadges, dstLineBadges, srcLineBadgeColors, dstLineBadgeColors, leftSpansByLine, rightSpansByLine, numWidth, termWidth, opts, scratch, srcEndsWithNL, dstEndsWithNL, lastSrcLineIdx, lastDstLineIdx, layout)
		}
	}

	return ew.err
}

func renderSideBySideHunk(
	w io.Writer,
	h renderutil.Interval,
	filteredPairs []serialize.LineAlignmentPair,
	srcLines, dstLines []string,
	srcLineBadges, dstLineBadges map[int]string,
	srcLineBadgeColors, dstLineBadgeColors map[int]int,
	leftSpansByLine, rightSpansByLine map[int][]serialize.HighlightSpan,
	numWidth, codeWidth int,
	opts RenderOptions,
	scratch *RenderScratch,
	sep string,
	srcEndsWithNL, dstEndsWithNL bool,
	lastSrcLineIdx, lastDstLineIdx int,
) {
	for p := h.Start; p <= h.End; p++ {
		if p < 0 || p >= len(filteredPairs) {
			continue
		}
		pair := filteredPairs[p]

		var leftChunks [][]byte
		if pair.LeftLine >= 0 && pair.LeftLine < len(srcLines) {
			badge := srcLineBadges[pair.LeftLine]
			badgeColor := srcLineBadgeColors[pair.LeftLine]
			lineCtx := renderutil.LineContextAligned
			if pair.RightLine == -1 {
				lineCtx = renderutil.LineContextStandaloneDelete
			}
			leftChunks = scratch.SliceLineToChunks(srcLines[pair.LeftLine], badge, leftSpansByLine[pair.LeftLine], codeWidth, lineCtx, true, opts.Color, badgeColor)
		}

		var rightChunks [][]byte
		if pair.RightLine >= 0 && pair.RightLine < len(dstLines) {
			badge := dstLineBadges[pair.RightLine]
			badgeColor := dstLineBadgeColors[pair.RightLine]
			lineCtx := renderutil.LineContextAligned
			if pair.LeftLine == -1 {
				lineCtx = renderutil.LineContextStandaloneInsert
			}
			rightChunks = scratch.SliceLineToChunks(dstLines[pair.RightLine], badge, rightSpansByLine[pair.RightLine], codeWidth, lineCtx, true, opts.Color, badgeColor)
		}

		numSubRows := max(1, max(len(leftChunks), len(rightChunks)))

		for subRow := range numSubRows {
			if opts.LineNumbers {
				if subRow == 0 && pair.LeftLine >= 0 {
					renderGutter(w, pair.LeftLine, numWidth, opts.Color, pair.RightLine == -1, false)
				} else {
					renderContinuationGutter(w, numWidth, opts.Color)
				}
			}

			if subRow < len(leftChunks) {
				_, _ = w.Write(leftChunks[subRow])
			} else {
				padEmptyColumn(w, codeWidth)
			}

			_, _ = io.WriteString(w, sep)

			if opts.LineNumbers {
				if subRow == 0 && pair.RightLine >= 0 {
					renderGutter(w, pair.RightLine, numWidth, opts.Color, false, pair.LeftLine == -1)
				} else {
					renderContinuationGutter(w, numWidth, opts.Color)
				}
			}

			if subRow < len(rightChunks) {
				_, _ = w.Write(rightChunks[subRow])
			} else {
				padEmptyColumn(w, codeWidth)
			}

			_, _ = io.WriteString(w, "\n")
		}

		if (!srcEndsWithNL && pair.LeftLine == lastSrcLineIdx) || (!dstEndsWithNL && pair.RightLine == lastDstLineIdx) {
			renderEOFNewlineWarning(w, pair.LeftLine == lastSrcLineIdx && !srcEndsWithNL, pair.RightLine == lastDstLineIdx && !dstEndsWithNL, numWidth, codeWidth, opts.LineNumbers, opts.Color, sep)
		}
	}
}

func renderSingleColumnHunk(
	w io.Writer,
	h renderutil.Interval,
	filteredPairs []serialize.LineAlignmentPair,
	isPairChanged []bool,
	srcLines, dstLines []string,
	srcLineBadges, dstLineBadges map[int]string,
	srcLineBadgeColors, dstLineBadgeColors map[int]int,
	leftSpansByLine, rightSpansByLine map[int][]serialize.HighlightSpan,
	numWidth, termWidth int,
	opts RenderOptions,
	scratch *RenderScratch,
	srcEndsWithNL, dstEndsWithNL bool,
	lastSrcLineIdx, lastDstLineIdx int,
	layout HunkLayoutKind,
) {
	gutterWidth := 2 // "+ " or "- "
	if opts.LineNumbers {
		gutterWidth = (numWidth + 1) * 2 // "%*s %*s "
	}
	fullCodeWidth := max(10, termWidth-gutterWidth)

	for p := h.Start; p <= h.End; p++ {
		if p < 0 || p >= len(filteredPairs) {
			continue
		}
		pair := filteredPairs[p]

		var text string
		var badge string
		var badgeColor int
		var spans []serialize.HighlightSpan
		var lineCtx renderutil.LineContext
		var leftLineNum, rightLineNum int
		var action color.ActionKind

		if p < len(isPairChanged) && !isPairChanged[p] {
			leftLineNum = pair.LeftLine
			rightLineNum = pair.RightLine
			if pair.RightLine >= 0 && pair.RightLine < len(dstLines) {
				text = dstLines[pair.RightLine]
			} else if pair.LeftLine >= 0 && pair.LeftLine < len(srcLines) {
				text = srcLines[pair.LeftLine]
			}
			lineCtx = renderutil.LineContextAligned
			action = color.ActionKind(-1)
		} else if pair.LeftLine == -1 && pair.RightLine >= 0 {
			leftLineNum = -1
			rightLineNum = pair.RightLine
			if pair.RightLine < len(dstLines) {
				text = dstLines[pair.RightLine]
			}
			badge = dstLineBadges[pair.RightLine]
			badgeColor = dstLineBadgeColors[pair.RightLine]
			spans = rightSpansByLine[pair.RightLine]
			lineCtx = renderutil.LineContextStandaloneInsert
			action = color.ActionInsert
		} else if pair.LeftLine >= 0 && pair.RightLine == -1 {
			leftLineNum = pair.LeftLine
			rightLineNum = -1
			if pair.LeftLine < len(srcLines) {
				text = srcLines[pair.LeftLine]
			}
			badge = srcLineBadges[pair.LeftLine]
			badgeColor = srcLineBadgeColors[pair.LeftLine]
			spans = leftSpansByLine[pair.LeftLine]
			lineCtx = renderutil.LineContextStandaloneDelete
			action = color.ActionDelete
		} else {
			if layout == HunkLayoutSingleColumnRight {
				leftLineNum = -1
				rightLineNum = pair.RightLine
				if pair.RightLine >= 0 && pair.RightLine < len(dstLines) {
					text = dstLines[pair.RightLine]
					spans = rightSpansByLine[pair.RightLine]
					badge = dstLineBadges[pair.RightLine]
					badgeColor = dstLineBadgeColors[pair.RightLine]
				}
				lineCtx = renderutil.LineContextStandaloneInsert
				action = color.ActionInsert
			} else {
				leftLineNum = pair.LeftLine
				rightLineNum = -1
				if pair.LeftLine >= 0 && pair.LeftLine < len(srcLines) {
					text = srcLines[pair.LeftLine]
					spans = leftSpansByLine[pair.LeftLine]
					badge = srcLineBadges[pair.LeftLine]
					badgeColor = srcLineBadgeColors[pair.LeftLine]
				}
				lineCtx = renderutil.LineContextStandaloneDelete
				action = color.ActionDelete
			}
		}

		chunks := scratch.SliceLineToChunks(text, badge, spans, fullCodeWidth, lineCtx, false, opts.Color, badgeColor)
		if len(chunks) == 0 {
			chunks = [][]byte{nil}
		}

		for subRow, chunk := range chunks {
			if opts.LineNumbers {
				if subRow == 0 {
					renderSingleColumnDoubleGutter(w, leftLineNum, rightLineNum, numWidth, lastSrcLineIdx, lastDstLineIdx, opts.Color, action)
				} else {
					renderSingleColumnContinuationGutter(w, leftLineNum >= 0, rightLineNum >= 0, numWidth, opts.Color)
				}
			} else {
				if subRow == 0 {
					if opts.Color {
						switch action {
						case color.ActionInsert:
							_, _ = fmt.Fprintf(w, "%s%s+%s ", color.Bold, color.InsertFg, color.Reset)
						case color.ActionDelete:
							_, _ = fmt.Fprintf(w, "%s%s-%s ", color.Bold, color.DeleteFg, color.Reset)
						default:
							_, _ = io.WriteString(w, "  ")
						}
					} else {
						switch action {
						case color.ActionInsert:
							_, _ = io.WriteString(w, "+ ")
						case color.ActionDelete:
							_, _ = io.WriteString(w, "- ")
						default:
							_, _ = io.WriteString(w, "  ")
						}
					}
				} else {
					_, _ = io.WriteString(w, "  ")
				}
			}

			if len(chunk) > 0 {
				_, _ = w.Write(chunk)
			}
			_, _ = io.WriteString(w, "\n")
		}

		if (!srcEndsWithNL && pair.LeftLine == lastSrcLineIdx) || (!dstEndsWithNL && pair.RightLine == lastDstLineIdx) {
			warnText := `\ No newline at end of file`
			pad := "  "
			if opts.LineNumbers {
				pad = strings.Repeat(" ", (numWidth+1)*2)
			}
			if opts.Color {
				_, _ = fmt.Fprintf(w, "%s%s%s%s%s\n", pad, color.Italic, color.OverlayFg, warnText, color.Reset)
			} else {
				_, _ = fmt.Fprintf(w, "%s%s\n", pad, warnText)
			}
		}
	}
}

func renderSingleColumnDoubleGutter(
	w io.Writer,
	leftLine, rightLine, numWidth int,
	lastSrcLineIdx, lastDstLineIdx int,
	colorMode bool,
	action color.ActionKind,
) {
	var leftStr string
	if leftLine >= 0 {
		leftStr = strconv.Itoa(leftLine + 1)
	} else if lastSrcLineIdx >= 0 {
		leftStr = ".."
	}
	leftPad := fmt.Sprintf("%*s ", numWidth, leftStr)

	var rightStr string
	if rightLine >= 0 {
		rightStr = strconv.Itoa(rightLine + 1)
	} else if lastDstLineIdx >= 0 {
		rightStr = ".."
	}
	rightPad := fmt.Sprintf("%*s ", numWidth, rightStr)

	if !colorMode {
		_, _ = io.WriteString(w, leftPad)
		_, _ = io.WriteString(w, rightPad)
		return
	}

	switch action {
	case color.ActionDelete:
		_, _ = fmt.Fprintf(w, "%s%s%s%s", color.Bold, color.DeleteFg, leftPad, color.Reset)
		_, _ = fmt.Fprintf(w, "%s%s%s", color.OverlayFg, rightPad, color.Reset)
	case color.ActionInsert:
		_, _ = fmt.Fprintf(w, "%s%s%s", color.OverlayFg, leftPad, color.Reset)
		_, _ = fmt.Fprintf(w, "%s%s%s%s", color.Bold, color.InsertFg, rightPad, color.Reset)
	default:
		_, _ = fmt.Fprintf(w, "%s%s%s", color.OverlayFg, leftPad, color.Reset)
		_, _ = fmt.Fprintf(w, "%s%s%s", color.OverlayFg, rightPad, color.Reset)
	}
}

func renderSingleColumnContinuationGutter(
	w io.Writer,
	hasLeft, hasRight bool,
	numWidth int,
	colorMode bool,
) {
	var leftStr, rightStr string
	if hasLeft {
		leftStr = ".."
	}
	if hasRight {
		rightStr = ".."
	}
	leftPad := fmt.Sprintf("%*s ", numWidth, leftStr)
	rightPad := fmt.Sprintf("%*s ", numWidth, rightStr)

	if !colorMode {
		_, _ = io.WriteString(w, leftPad)
		_, _ = io.WriteString(w, rightPad)
		return
	}

	_, _ = fmt.Fprintf(w, "%s%s%s%s%s%s", color.OverlayFg, leftPad, color.Reset, color.OverlayFg, rightPad, color.Reset)
}

func renderWholeFileSingleColumn(
	lines []string,
	spansByLine map[int][]serialize.HighlightSpan,
	badges map[int]string,
	numWidth, termWidth int,
	opts RenderOptions,
	scratch *RenderScratch,
	action color.ActionKind,
	w io.Writer,
	endsWithNL bool,
) error {
	var hunkHeader string
	if action == color.ActionInsert {
		hunkHeader = renderutil.FormatHunkHeader(0, 0, 1, len(lines), "")
	} else {
		hunkHeader = renderutil.FormatHunkHeader(1, len(lines), 0, 0, "")
	}
	if opts.Color {
		_, _ = fmt.Fprintf(w, "%s%s%s\n", color.HeaderFg, hunkHeader, color.Reset)
	} else {
		_, _ = fmt.Fprintf(w, "%s\n", hunkHeader)
	}

	gutterWidth := 2
	if opts.LineNumbers {
		gutterWidth = numWidth + 1
	}
	fullCodeWidth := max(10, termWidth-gutterWidth)

	var lineCtx renderutil.LineContext
	var colFg string
	var sym string
	if action == color.ActionInsert {
		lineCtx = renderutil.LineContextStandaloneInsert
		colFg = color.InsertFg
		sym = "+"
	} else {
		lineCtx = renderutil.LineContextStandaloneDelete
		colFg = color.DeleteFg
		sym = "-"
	}

	lastLineIdx := len(lines) - 1

	for i, line := range lines {
		badge := badges[i]
		spans := spansByLine[i]
		chunks := scratch.SliceLineToChunks(line, badge, spans, fullCodeWidth, lineCtx, false, opts.Color)
		if len(chunks) == 0 {
			chunks = [][]byte{nil}
		}

		for subRow, chunk := range chunks {
			if opts.LineNumbers {
				if subRow == 0 {
					str := strconv.Itoa(i + 1)
					pad := fmt.Sprintf("%*s ", numWidth, str)
					if opts.Color {
						_, _ = fmt.Fprintf(w, "%s%s%s%s", color.Bold, colFg, pad, color.Reset)
					} else {
						_, _ = io.WriteString(w, pad)
					}
				} else {
					pad := fmt.Sprintf("%*s ", numWidth, "..")
					if opts.Color {
						_, _ = fmt.Fprintf(w, "%s%s%s", color.OverlayFg, pad, color.Reset)
					} else {
						_, _ = io.WriteString(w, pad)
					}
				}
			} else {
				if subRow == 0 {
					if opts.Color {
						_, _ = fmt.Fprintf(w, "%s%s%s%s ", color.Bold, colFg, sym, color.Reset)
					} else {
						_, _ = fmt.Fprintf(w, "%s ", sym)
					}
				} else {
					_, _ = io.WriteString(w, "  ")
				}
			}

			if len(chunk) > 0 {
				_, _ = w.Write(chunk)
			}
			_, _ = io.WriteString(w, "\n")
		}

		if !endsWithNL && i == lastLineIdx {
			warnText := `\ No newline at end of file`
			pad := "  "
			if opts.LineNumbers {
				pad = strings.Repeat(" ", numWidth+1)
			}
			if opts.Color {
				_, _ = fmt.Fprintf(w, "%s%s%s%s%s\n", pad, color.Italic, color.OverlayFg, warnText, color.Reset)
			} else {
				_, _ = fmt.Fprintf(w, "%s%s\n", pad, warnText)
			}
		}
	}

	return nil
}

func padEmptyColumn(w io.Writer, width int) {
	if width <= 0 {
		return
	}
	_, _ = io.WriteString(w, strings.Repeat(" ", width))
}

func renderGutter(w io.Writer, lineNum, numWidth int, colorMode, isDelete, isInsert bool) {
	if lineNum < 0 {
		pad := strings.Repeat(" ", numWidth)
		_, _ = io.WriteString(w, pad+" ")
		return
	}

	str := strconv.Itoa(lineNum + 1)
	pad := fmt.Sprintf("%*s ", numWidth, str)

	if !colorMode {
		_, _ = io.WriteString(w, pad)
		return
	}

	var fg string
	var bold string
	if isDelete {
		fg = color.DeleteFg
		bold = color.Bold
	} else if isInsert {
		fg = color.InsertFg
		bold = color.Bold
	} else {
		fg = color.OverlayFg
	}

	_, _ = fmt.Fprintf(w, "%s%s%s%s", bold, fg, pad, color.Reset)
}

func renderContinuationGutter(w io.Writer, numWidth int, colorMode bool) {
	dots := fmt.Sprintf("%*s ", numWidth, "..")
	if !colorMode {
		_, _ = io.WriteString(w, dots)
		return
	}
	_, _ = fmt.Fprintf(w, "%s%s%s", color.OverlayFg, dots, color.Reset)
}

func renderEOFNewlineWarning(w io.Writer, leftWarn, rightWarn bool, numWidth, codeWidth int, lineNumbers, colorMode bool, sep string) {
	warnText := `\ No newline at end of file`

	if lineNumbers {
		pad := strings.Repeat(" ", numWidth+1)
		_, _ = io.WriteString(w, pad)
	}

	if leftWarn {
		if colorMode {
			_, _ = fmt.Fprintf(w, "%s%s%s%s", color.Italic, color.OverlayFg, warnText, color.Reset)
		} else {
			_, _ = io.WriteString(w, warnText)
		}
		padEmptyColumn(w, max(0, codeWidth-len(warnText)))
	} else {
		padEmptyColumn(w, codeWidth)
	}

	_, _ = io.WriteString(w, sep)

	if lineNumbers {
		pad := strings.Repeat(" ", numWidth+1)
		_, _ = io.WriteString(w, pad)
	}

	if rightWarn {
		if colorMode {
			_, _ = fmt.Fprintf(w, "%s%s%s%s", color.Italic, color.OverlayFg, warnText, color.Reset)
		} else {
			_, _ = io.WriteString(w, warnText)
		}
		padEmptyColumn(w, max(0, codeWidth-len(warnText)))
	} else {
		padEmptyColumn(w, codeWidth)
	}

	_, _ = io.WriteString(w, "\n")
}

// RenderFileBanner prints a header with the file path and change stats.
func RenderFileBanner(path string, insertions, deletions, updates int, colorMode bool, w io.Writer) error {
	var stats string
	if insertions == 0 && deletions == 0 && updates == 0 {
		stats = "[Binary files differ]"
	} else if updates == 0 {
		stats = fmt.Sprintf("[+%d -%d]", insertions, deletions)
	} else {
		stats = fmt.Sprintf("[+%d -%d ~%d]", insertions, deletions, updates)
	}

	if !colorMode {
		_, err := fmt.Fprintf(w, "━━━ %s ━━━ %s ━━━\n", path, stats)
		return err
	}

	bar := color.SurfaceFg + "━━━" + color.Reset
	title := color.Bold + color.HeaderFg + path + color.Reset

	var statStr string
	if insertions == 0 && deletions == 0 && updates == 0 {
		statStr = fmt.Sprintf("[%sBinary files differ%s]", color.UpdateFg, color.Reset)
	} else if updates == 0 {
		statStr = fmt.Sprintf("[%s+%d%s %s-%d%s]",
			color.InsertFg, insertions, color.Reset,
			color.DeleteFg, deletions, color.Reset,
		)
	} else {
		statStr = fmt.Sprintf("[%s+%d%s %s-%d%s %s~%d%s]",
			color.InsertFg, insertions, color.Reset,
			color.DeleteFg, deletions, color.Reset,
			color.UpdateFg, updates, color.Reset,
		)
	}

	_, err := fmt.Fprintf(w, "%s %s %s %s %s\x1b[0m\n", bar, title, bar, statStr, bar)
	return err
}
