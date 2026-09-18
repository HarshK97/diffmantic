package sidebyside

import (
	"cmp"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/HarshK97/diffmantic/internal/color"
	"github.com/HarshK97/diffmantic/internal/inline"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
	"golang.org/x/term"
)

type interval struct {
	start int
	end   int
}

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
	lastSrcLineIdx := len(srcLines) - 1
	lastDstLineIdx := len(dstLines) - 1

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

	changeIntervals := buildChangeIntervals(isPairChanged)
	contextLines := opts.ContextLines
	if contextLines < 0 {
		contextLines = len(filteredPairs)
	}
	hunks := mergeHunks(changeIntervals, len(filteredPairs), contextLines)

	srcOffsets := serialize.BuildLineIndex(srcBytes)
	dstOffsets := serialize.BuildLineIndex(dstBytes)

	srcLineBadges, dstLineBadges := buildMoveBadges(env.Actions, srcOffsets, dstOffsets, srcLines, dstLines, hunks, filteredPairs, opts.DisableAnnotations, srcFile, dstFile)

	scratch := &RenderScratch{TabWidth: opts.TabWidth}
	sep := "  "

	prevLayout := HunkLayoutSideBySide
	isFirstSegment := true
	for _, h := range hunks {
		if ew.err != nil {
			return ew.err
		}
		segments := partitionHunkIntoSegments(h, filteredPairs, isPairChanged, srcLines, dstLines, leftSpansByLine, rightSpansByLine, codeWidth, opts.AdaptiveThreshold, opts.ForceSideBySide)

		for _, seg := range segments {
			if ew.err != nil {
				return ew.err
			}
			if !isFirstSegment {
				if prevLayout == HunkLayoutFullWidthInline || seg.layout == HunkLayoutFullWidthInline {
					renderFullWidthHunkSeparator(w, termWidth, opts.Color)
				} else {
					renderHunkSeparator(w, numWidth, codeWidth, opts.LineNumbers, opts.Color)
				}
			}
			isFirstSegment = false
			prevLayout = seg.layout

			segInterval := interval{start: seg.start, end: seg.end}
			if seg.layout == HunkLayoutFullWidthInline {
				renderFullWidthHunk(w, segInterval, filteredPairs, isPairChanged, srcLines, dstLines, srcLineBadges, dstLineBadges, leftSpansByLine, rightSpansByLine, numWidth, termWidth, opts, scratch, srcEndsWithNL, dstEndsWithNL, lastSrcLineIdx, lastDstLineIdx)
			} else {
				renderSideBySideHunk(w, segInterval, filteredPairs, srcLines, dstLines, srcLineBadges, dstLineBadges, leftSpansByLine, rightSpansByLine, numWidth, codeWidth, opts, scratch, sep, srcEndsWithNL, dstEndsWithNL, lastSrcLineIdx, lastDstLineIdx)
			}
		}
	}

	return ew.err
}

func renderSideBySideHunk(
	w io.Writer,
	h interval,
	filteredPairs []serialize.LineAlignmentPair,
	srcLines, dstLines []string,
	srcLineBadges, dstLineBadges map[int]string,
	leftSpansByLine, rightSpansByLine map[int][]serialize.HighlightSpan,
	numWidth, codeWidth int,
	opts RenderOptions,
	scratch *RenderScratch,
	sep string,
	srcEndsWithNL, dstEndsWithNL bool,
	lastSrcLineIdx, lastDstLineIdx int,
) {
	for p := h.start; p <= h.end; p++ {
		if p < 0 || p >= len(filteredPairs) {
			continue
		}
		pair := filteredPairs[p]

		var leftChunks [][]byte
		if pair.LeftLine >= 0 && pair.LeftLine < len(srcLines) {
			badge := srcLineBadges[pair.LeftLine]
			leftChunks = scratch.SliceLineToChunks(srcLines[pair.LeftLine], badge, leftSpansByLine[pair.LeftLine], codeWidth, "left", pair.RightLine == -1, opts.Color)
		}

		var rightChunks [][]byte
		if pair.RightLine >= 0 && pair.RightLine < len(dstLines) {
			badge := dstLineBadges[pair.RightLine]
			rightChunks = scratch.SliceLineToChunks(dstLines[pair.RightLine], badge, rightSpansByLine[pair.RightLine], codeWidth, "right", false, opts.Color)
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

func renderFullWidthHunk(
	w io.Writer,
	h interval,
	filteredPairs []serialize.LineAlignmentPair,
	isPairChanged []bool,
	srcLines, dstLines []string,
	srcLineBadges, dstLineBadges map[int]string,
	leftSpansByLine, rightSpansByLine map[int][]serialize.HighlightSpan,
	numWidth, termWidth int,
	opts RenderOptions,
	scratch *RenderScratch,
	srcEndsWithNL, dstEndsWithNL bool,
	lastSrcLineIdx, lastDstLineIdx int,
) {
	gutterWidth := 2 // "+ " or "- "
	if opts.LineNumbers {
		gutterWidth = (numWidth * 2) + 3 // "%*s %*s  "
	}
	fullCodeWidth := max(10, termWidth-gutterWidth)

	type fullWidthRow struct {
		chunks  [][]byte
		symbol  string
		srcLine int
		dstLine int
		action  color.ActionKind
	}

	p := h.start
	for p <= h.end {
		if p < 0 || p >= len(filteredPairs) {
			p++
			continue
		}
		pair := filteredPairs[p]

		var rows []fullWidthRow
		bEnd := p + 1

		if p < len(isPairChanged) && !isPairChanged[p] {
			// Unchanged context
			text := ""
			if pair.RightLine < len(dstLines) {
				text = dstLines[pair.RightLine]
			} else if pair.LeftLine < len(srcLines) {
				text = srcLines[pair.LeftLine]
			}
			chunks := scratch.SliceLineToChunks(text, "", nil, fullCodeWidth, "right", false, opts.Color)
			rows = append(rows, fullWidthRow{
				chunks:  chunks,
				symbol:  " ",
				srcLine: pair.LeftLine,
				dstLine: pair.RightLine,
				action:  color.ActionKind(-1),
			})
		} else {
			// Group consecutive changes into deletions followed by insertions (2-block diff).
			for bEnd <= h.end && bEnd < len(isPairChanged) && isPairChanged[bEnd] {
				bEnd++
			}

			// 1. All deletions in this sub-block
			for k := p; k < bEnd; k++ {
				kp := filteredPairs[k]
				if kp.LeftLine >= 0 && kp.LeftLine < len(srcLines) {
					badge := srcLineBadges[kp.LeftLine]
					chunks := scratch.SliceLineToChunks(srcLines[kp.LeftLine], badge, leftSpansByLine[kp.LeftLine], fullCodeWidth, "left", true, opts.Color)
					rows = append(rows, fullWidthRow{
						chunks:  chunks,
						symbol:  "-",
						srcLine: kp.LeftLine,
						dstLine: -1,
						action:  color.ActionDelete,
					})
				}
			}

			// 2. All insertions in this sub-block
			for k := p; k < bEnd; k++ {
				kp := filteredPairs[k]
				if kp.RightLine >= 0 && kp.RightLine < len(dstLines) {
					badge := dstLineBadges[kp.RightLine]
					chunks := scratch.SliceLineToChunks(dstLines[kp.RightLine], badge, rightSpansByLine[kp.RightLine], fullCodeWidth, "right", false, opts.Color)
					rows = append(rows, fullWidthRow{
						chunks:  chunks,
						symbol:  "+",
						srcLine: -1,
						dstLine: kp.RightLine,
						action:  color.ActionInsert,
					})
				}
			}
		}

		for _, row := range rows {
			chunks := row.chunks
			if len(chunks) == 0 {
				chunks = [][]byte{nil}
			}

			for subRow, chunk := range chunks {
				if opts.LineNumbers {
					if subRow == 0 {
						renderFullWidthDoubleGutter(w, row.srcLine, row.dstLine, numWidth, opts.Color, row.action)
					} else {
						renderFullWidthContinuationGutter(w, numWidth, opts.Color, row.action)
					}
				} else {
					if subRow == 0 {
						if opts.Color {
							var symFg string
							switch row.action {
							case color.ActionInsert:
								symFg = color.InsertFg
							case color.ActionDelete:
								symFg = color.DeleteFg
							default:
								symFg = color.OverlayFg
							}
							_, _ = fmt.Fprintf(w, "%s%s%s%s ", color.Bold, symFg, row.symbol, color.Reset)
						} else {
							_, _ = fmt.Fprintf(w, "%s ", row.symbol)
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
		}

		for k := p; k < bEnd; k++ {
			kp := filteredPairs[k]
			if (!srcEndsWithNL && kp.LeftLine == lastSrcLineIdx) || (!dstEndsWithNL && kp.RightLine == lastDstLineIdx) {
				renderFullWidthEOFWarning(w, numWidth, opts.LineNumbers, opts.Color)
				break
			}
		}

		p = bEnd
	}
}

func renderFullWidthDoubleGutter(w io.Writer, srcLine, dstLine, numWidth int, colorMode bool, action color.ActionKind) {
	srcStr := ""
	if srcLine >= 0 {
		srcStr = strconv.Itoa(srcLine + 1)
	}
	dstStr := ""
	if dstLine >= 0 {
		dstStr = strconv.Itoa(dstLine + 1)
	}

	srcPad := fmt.Sprintf("%*s", numWidth, srcStr)
	dstPad := fmt.Sprintf("%*s", numWidth, dstStr)

	if !colorMode {
		_, _ = fmt.Fprintf(w, "%s %s  ", srcPad, dstPad)
		return
	}

	sep := "  "
	switch action {
	case color.ActionDelete:
		_, _ = fmt.Fprintf(w, "%s%s%s %s%s%s%s", color.DeleteFg, srcPad, color.Reset, color.OverlayFg, dstPad, color.Reset, sep)
	case color.ActionInsert:
		_, _ = fmt.Fprintf(w, "%s%s%s %s%s%s%s", color.OverlayFg, srcPad, color.Reset, color.InsertFg, dstPad, color.Reset, sep)
	default:
		_, _ = fmt.Fprintf(w, "%s%s%s %s%s%s%s", color.OverlayFg, srcPad, color.Reset, color.OverlayFg, dstPad, color.Reset, sep)
	}
}

func renderFullWidthContinuationGutter(w io.Writer, numWidth int, colorMode bool, action color.ActionKind) {
	var srcDots, dstDots string
	switch action {
	case color.ActionDelete:
		srcDots = fmt.Sprintf("%*s", numWidth, "..")
		dstDots = strings.Repeat(" ", numWidth)
	case color.ActionInsert:
		srcDots = strings.Repeat(" ", numWidth)
		dstDots = fmt.Sprintf("%*s", numWidth, "..")
	default:
		srcDots = fmt.Sprintf("%*s", numWidth, "..")
		dstDots = fmt.Sprintf("%*s", numWidth, "..")
	}

	if !colorMode {
		_, _ = fmt.Fprintf(w, "%s %s  ", srcDots, dstDots)
		return
	}

	sep := "  "
	_, _ = fmt.Fprintf(w, "%s%s%s %s%s%s%s", color.OverlayFg, srcDots, color.Reset, color.OverlayFg, dstDots, color.Reset, sep)
}

func renderFullWidthEOFWarning(w io.Writer, numWidth int, lineNumbers, colorMode bool) {
	warnText := `\ No newline at end of file`
	if lineNumbers {
		pad := strings.Repeat(" ", (numWidth*2)+3)
		_, _ = io.WriteString(w, pad)
	} else {
		_, _ = io.WriteString(w, "  ")
	}
	if colorMode {
		_, _ = fmt.Fprintf(w, "%s%s%s%s\n", color.Italic, color.OverlayFg, warnText, color.Reset)
	} else {
		_, _ = fmt.Fprintf(w, "%s\n", warnText)
	}
}

func renderFullWidthHunkSeparator(w io.Writer, termWidth int, colorMode bool) {
	dotStr := "···"
	if !colorMode {
		_, _ = io.WriteString(w, "─── "+dotStr+" ───\n")
		return
	}
	_, _ = fmt.Fprintf(w, "%s─── %s ───%s\n", color.SurfaceFg, dotStr, color.Reset)
}

func padEmptyColumn(w io.Writer, width int) {
	if width <= 0 {
		return
	}
	buf := make([]byte, width)
	for i := range buf {
		buf[i] = ' '
	}
	_, _ = w.Write(buf)
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
	if isDelete {
		fg = color.DeleteFg
	} else if isInsert {
		fg = color.InsertFg
	} else {
		fg = color.OverlayFg
	}

	_, _ = fmt.Fprintf(w, "%s%s%s", fg, pad, color.Reset)
}

func renderContinuationGutter(w io.Writer, numWidth int, colorMode bool) {
	dots := fmt.Sprintf("%*s ", numWidth, "..")
	if !colorMode {
		_, _ = io.WriteString(w, dots)
		return
	}
	_, _ = fmt.Fprintf(w, "%s%s%s", color.OverlayFg, dots, color.Reset)
}

func renderHunkSeparator(w io.Writer, numWidth, codeWidth int, lineNumbers, colorMode bool) {
	sep := "  "
	var dotStr string
	if colorMode {
		dotStr = color.OverlayFg + "···" + color.Reset

		if lineNumbers {
			pad := strings.Repeat(" ", numWidth+1)
			_, _ = io.WriteString(w, pad)
			_, _ = io.WriteString(w, dotStr)
			padEmptyColumn(w, max(0, codeWidth-3))
			_, _ = io.WriteString(w, sep)
			_, _ = io.WriteString(w, pad)
			_, _ = io.WriteString(w, dotStr)
			padEmptyColumn(w, max(0, codeWidth-3))
			_, _ = io.WriteString(w, "\n")
		} else {
			_, _ = io.WriteString(w, dotStr)
			padEmptyColumn(w, max(0, codeWidth-3))
			_, _ = io.WriteString(w, sep)
			_, _ = io.WriteString(w, dotStr)
			padEmptyColumn(w, max(0, codeWidth-3))
			_, _ = io.WriteString(w, "\n")
		}
	} else {
		dotStr = "···"
		if lineNumbers {
			pad := strings.Repeat(" ", numWidth+1)
			_, _ = io.WriteString(w, pad+dotStr)
			padEmptyColumn(w, max(0, codeWidth-3))
			_, _ = io.WriteString(w, sep+pad+dotStr)
			padEmptyColumn(w, max(0, codeWidth-3))
			_, _ = io.WriteString(w, "\n")
		} else {
			_, _ = io.WriteString(w, dotStr)
			padEmptyColumn(w, max(0, codeWidth-3))
			_, _ = io.WriteString(w, sep+dotStr)
			padEmptyColumn(w, max(0, codeWidth-3))
			_, _ = io.WriteString(w, "\n")
		}
	}
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

func buildChangeIntervals(isPairChanged []bool) []interval {
	var changeIntervals []interval
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
				changeIntervals = append(changeIntervals, interval{start: startIdx, end: i - 1})
				inChange = false
			}
		}
	}
	if inChange {
		changeIntervals = append(changeIntervals, interval{start: startIdx, end: len(isPairChanged) - 1})
	}
	return changeIntervals
}

func mergeHunks(changeIntervals []interval, totalPairs, contextLines int) []interval {
	var hunks []interval
	for _, ci := range changeIntervals {
		hStart := max(0, ci.start-contextLines)
		hEnd := min(totalPairs-1, ci.end+contextLines)

		if len(hunks) > 0 && hStart <= hunks[len(hunks)-1].end+1 {
			hunks[len(hunks)-1].end = hEnd
		} else {
			hunks = append(hunks, interval{start: hStart, end: hEnd})
		}
	}
	return hunks
}

func buildMoveBadges(
	actions []serialize.Action,
	srcOffsets, dstOffsets []int,
	srcLines, dstLines []string,
	hunks []interval,
	filteredPairs []serialize.LineAlignmentPair,
	disabled bool,
	srcFile, dstFile string,
) (map[int]string, map[int]string) {
	srcLineBadges := make(map[int]string)
	dstLineBadges := make(map[int]string)

	if disabled || len(actions) == 0 || len(hunks) == 0 {
		return srcLineBadges, dstLineBadges
	}

	srcLineToHunk := make(map[int]int, len(srcLines))
	dstLineToHunk := make(map[int]int, len(dstLines))
	for hIdx, h := range hunks {
		for p := h.start; p <= h.end; p++ {
			if p >= 0 && p < len(filteredPairs) {
				pair := filteredPairs[p]
				if pair.LeftLine >= 0 {
					srcLineToHunk[pair.LeftLine] = hIdx
				}
				if pair.RightLine >= 0 {
					dstLineToHunk[pair.RightLine] = hIdx
				}
			}
		}
	}

	var r *rules.Rules
	lang, _ := treesitter.DetectLanguage(srcFile)
	if lang == nil {
		lang, _ = treesitter.DetectLanguage(dstFile)
	}
	if lang != nil {
		r = rules.Get(lang.Name)
	}

	type mutatingSpan struct {
		startByte uint32
		endByte   uint32
	}
	var dstMutations []mutatingSpan
	for _, a := range actions {
		if a.Action == "insert" || a.Action == "delete" || a.Action == "update" || a.Action == "move_update" {
			if a.Node != nil && a.Node.Tree == "after" {
				dstMutations = append(dstMutations, mutatingSpan{startByte: a.Node.StartByte, endByte: a.Node.EndByte})
			} else if a.DestStartByte != nil && a.DestEndByte != nil {
				dstMutations = append(dstMutations, mutatingSpan{startByte: *a.DestStartByte, endByte: *a.DestEndByte})
			}
		}
	}
	slices.SortFunc(dstMutations, func(a, b mutatingSpan) int {
		return cmp.Or(
			cmp.Compare(a.startByte, b.startByte),
			cmp.Compare(a.endByte, b.endByte),
		)
	})

	type crossHunkMove struct {
		sStartLine int
		sEndLine   int
		dStartLine int
		dEndLine   int
		sHunk      int
		dHunk      int
		isDecl     bool
		nMut       int
	}

	var crossMoves []crossHunkMove

	for _, a := range actions {
		if a.Action != "move" || a.Node == nil {
			continue
		}

		var dStartByte, dEndByte uint32
		if a.DestStartByte != nil && a.DestEndByte != nil {
			dStartByte = *a.DestStartByte
			dEndByte = *a.DestEndByte
		} else if a.DestNode != nil {
			dStartByte = a.DestNode.StartByte
			dEndByte = a.DestNode.EndByte
		} else {
			continue
		}

		sStartLine, _ := serialize.ByteToLineCol(srcOffsets, a.Node.StartByte)
		sEndLine, _ := serialize.ByteToLineCol(srcOffsets, a.Node.EndByte)
		dStartLine, _ := serialize.ByteToLineCol(dstOffsets, dStartByte)
		dEndLine, _ := serialize.ByteToLineCol(dstOffsets, dEndByte)

		sHunk, hasSHunk := srcLineToHunk[sStartLine]
		if !hasSHunk {
			sHunk = -1
		}
		dHunk, hasDHunk := dstLineToHunk[dStartLine]
		if !hasDHunk {
			dHunk = -1
		}

		// Skip badges if the move stays within the same hunk and is close by.
		lineDist := sStartLine - dStartLine
		if lineDist < 0 {
			lineDist = -lineDist
		}
		if sHunk != -1 && dHunk != -1 && sHunk == dHunk && lineDist < 10 {
			continue
		}

		// Count edits inside the moved node so we know if it was modified.
		nMut := 0
		if dEndByte > dStartByte && len(dstMutations) > 0 {
			idx := sort.Search(len(dstMutations), func(i int) bool {
				return dstMutations[i].startByte >= dStartByte
			})
			for idx < len(dstMutations) && dstMutations[idx].startByte < dEndByte {
				if dstMutations[idx].endByte <= dEndByte {
					nMut++
				}
				idx++
			}
		}

		isDecl := r != nil && r.IsDeclaration(a.Node.Type)
		isBlock := r != nil && r.IsBlock(a.Node.Type)
		isMultiLine := sEndLine > sStartLine || dEndLine > dStartLine
		isStatement := a.Node.Type == "statement" || strings.HasSuffix(a.Node.Type, "_statement") || (r != nil && r.IsCall(a.Node.Type))
		if !isDecl && !isBlock && !isMultiLine && !isStatement {
			continue
		}

		m := crossHunkMove{
			sStartLine: sStartLine,
			sEndLine:   sEndLine,
			dStartLine: dStartLine,
			dEndLine:   dEndLine,
			sHunk:      sHunk,
			dHunk:      dHunk,
			isDecl:     isDecl,
			nMut:       nMut,
		}
		crossMoves = append(crossMoves, m)
	}

	for _, m := range crossMoves {
		// Put move badges on the first line of the moved block.
		if m.sStartLine >= 0 && m.sStartLine < len(srcLines) {
			if _, exists := srcLineBadges[m.sStartLine]; !exists {
				srcLineBadges[m.sStartLine] = fmt.Sprintf(" ➔ L%d", m.dStartLine+1)
			}
		}

		if m.dStartLine >= 0 && m.dStartLine < len(dstLines) {
			if _, exists := dstLineBadges[m.dStartLine]; !exists {
				modStr := ""
				if m.nMut > 0 {
					modStr = ", modified"
				}
				dstLineBadges[m.dStartLine] = fmt.Sprintf(" ⤹ L%d%s", m.sStartLine+1, modStr)
			}
		}
	}

	return srcLineBadges, dstLineBadges
}
