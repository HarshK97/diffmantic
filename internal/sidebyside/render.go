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
			false, // promoteDeclarations = false for SBS
		)
		srcLineBadges = badges.SrcLineBadges
		dstLineBadges = badges.DstLineBadges
	}

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

			segInterval := renderutil.Interval{Start: seg.start, End: seg.end}
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
	h renderutil.Interval,
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
	for p := h.Start; p <= h.End; p++ {
		if p < 0 || p >= len(filteredPairs) {
			continue
		}
		pair := filteredPairs[p]

		var leftChunks [][]byte
		if pair.LeftLine >= 0 && pair.LeftLine < len(srcLines) {
			badge := srcLineBadges[pair.LeftLine]
			lineCtx := renderutil.LineContextAligned
			if pair.RightLine == -1 {
				lineCtx = renderutil.LineContextStandaloneDelete
			}
			leftChunks = scratch.SliceLineToChunks(srcLines[pair.LeftLine], badge, leftSpansByLine[pair.LeftLine], codeWidth, lineCtx, true, opts.Color)
		}

		var rightChunks [][]byte
		if pair.RightLine >= 0 && pair.RightLine < len(dstLines) {
			badge := dstLineBadges[pair.RightLine]
			lineCtx := renderutil.LineContextAligned
			if pair.LeftLine == -1 {
				lineCtx = renderutil.LineContextStandaloneInsert
			}
			rightChunks = scratch.SliceLineToChunks(dstLines[pair.RightLine], badge, rightSpansByLine[pair.RightLine], codeWidth, lineCtx, true, opts.Color)
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
	h renderutil.Interval,
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

	p := h.Start
	for p <= h.End {
		if p < 0 || p >= len(filteredPairs) {
			p++
			continue
		}
		pair := filteredPairs[p]

		var rows []fullWidthRow
		bEnd := p + 1

		if p < len(isPairChanged) && !isPairChanged[p] {
			text := ""
			if pair.RightLine < len(dstLines) {
				text = dstLines[pair.RightLine]
			} else if pair.LeftLine < len(srcLines) {
				text = srcLines[pair.LeftLine]
			}
			chunks := scratch.SliceLineToChunks(text, "", nil, fullCodeWidth, renderutil.LineContextAligned, false, opts.Color)
			rows = append(rows, fullWidthRow{
				chunks:  chunks,
				symbol:  " ",
				srcLine: pair.LeftLine,
				dstLine: pair.RightLine,
				action:  color.ActionKind(-1),
			})
		} else {
			// Group consecutive changes into deletions followed by insertions (2-block diff).
			for bEnd <= h.End && bEnd < len(isPairChanged) && isPairChanged[bEnd] {
				bEnd++
			}

			// 1. All deletions in this sub-block
			for k := p; k < bEnd; k++ {
				kp := filteredPairs[k]
				if kp.LeftLine >= 0 && kp.LeftLine < len(srcLines) {
					badge := srcLineBadges[kp.LeftLine]
					lineCtx := renderutil.LineContextAligned
					if kp.RightLine == -1 {
						lineCtx = renderutil.LineContextStandaloneDelete
					}
					chunks := scratch.SliceLineToChunks(srcLines[kp.LeftLine], badge, leftSpansByLine[kp.LeftLine], fullCodeWidth, lineCtx, false, opts.Color)
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
					lineCtx := renderutil.LineContextAligned
					if kp.LeftLine == -1 {
						lineCtx = renderutil.LineContextStandaloneInsert
					}
					chunks := scratch.SliceLineToChunks(dstLines[kp.RightLine], badge, rightSpansByLine[kp.RightLine], fullCodeWidth, lineCtx, false, opts.Color)
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
