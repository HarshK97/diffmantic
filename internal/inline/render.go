// Package inline renders AST-aware inline diffs.
package inline

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/HarshK97/diffmantic/internal/color"
	"github.com/HarshK97/diffmantic/internal/renderutil"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

type lineKind int

const (
	kindContext lineKind = iota
	kindDelete
	kindInsert
)

type hunkLine struct {
	kind           lineKind
	srcLineIdx     int
	dstLineIdx     int
	hasCounterpart bool
	text           string
}

// Render formats the diff envelope as an inline diff with AST highlights and move markers.
func Render(srcFile, dstFile string, srcBytes, dstBytes []byte, env *serialize.Envelope, opts RenderOptions) string {
	if env == nil {
		return ""
	}

	if env.IsBinary {
		if opts.Color {
			return fmt.Sprintf("%sBinary files %s and %s differ%s\n", color.UpdateFg, srcFile, dstFile, color.Reset)
		}
		return fmt.Sprintf("Binary files %s and %s differ\n", srcFile, dstFile)
	}

	if len(env.LineAlignment) == 0 {
		return ""
	}

	contextLines := opts.ContextLines
	if contextLines < 0 {
		contextLines = 3
	}

	srcLines := strings.Split(string(srcBytes), "\n")
	dstLines := strings.Split(string(dstBytes), "\n")

	srcEndsWithNL := len(srcBytes) > 0 && srcBytes[len(srcBytes)-1] == '\n'
	dstEndsWithNL := len(dstBytes) > 0 && dstBytes[len(dstBytes)-1] == '\n'

	lastSrcLineIdx := len(srcLines) - 1
	if srcEndsWithNL && lastSrcLineIdx >= 0 && srcLines[lastSrcLineIdx] == "" {
		lastSrcLineIdx--
	}
	lastDstLineIdx := len(dstLines) - 1
	if dstEndsWithNL && lastDstLineIdx >= 0 && dstLines[lastDstLineIdx] == "" {
		lastDstLineIdx--
	}

	srcEOFLine := -1
	if srcEndsWithNL && len(srcLines) > 0 && srcLines[len(srcLines)-1] == "" {
		srcEOFLine = len(srcLines) - 1
	}
	dstEOFLine := -1
	if dstEndsWithNL && len(dstLines) > 0 && dstLines[len(dstLines)-1] == "" {
		dstEOFLine = len(dstLines) - 1
	}

	// Strip trailing empty element added by strings.Split on trailing newlines.
	filteredPairs := make([]serialize.LineAlignmentPair, 0, len(env.LineAlignment))
	for _, pair := range env.LineAlignment {
		if srcEOFLine != -1 && pair.LeftLine == srcEOFLine && (pair.RightLine == dstEOFLine || pair.RightLine == -1) {
			continue
		}
		if dstEOFLine != -1 && pair.RightLine == dstEOFLine && (pair.LeftLine == srcEOFLine || pair.LeftLine == -1) {
			continue
		}
		if srcEOFLine != -1 && pair.LeftLine == srcEOFLine && pair.RightLine != -1 {
			pair.LeftLine = -1
		}
		if dstEOFLine != -1 && pair.RightLine == dstEOFLine && pair.LeftLine != -1 {
			pair.RightLine = -1
		}
		filteredPairs = append(filteredPairs, pair)
	}

	leftSpansByLine := make(map[int][]serialize.HighlightSpan, len(env.LeftHighlights))
	for _, sp := range env.LeftHighlights {
		leftSpansByLine[sp.Line] = append(leftSpansByLine[sp.Line], sp)
	}

	rightSpansByLine := make(map[int][]serialize.HighlightSpan, len(env.RightHighlights))
	for _, sp := range env.RightHighlights {
		rightSpansByLine[sp.Line] = append(rightSpansByLine[sp.Line], sp)
	}

	if len(env.Actions) == 0 && len(leftSpansByLine) == 0 && len(rightSpansByLine) == 0 && srcEndsWithNL == dstEndsWithNL && bytes.Equal(srcBytes, dstBytes) {
		return ""
	}

	isPairChanged := make([]bool, len(filteredPairs))
	hasAnyChange := false

	for i, pair := range filteredPairs {
		if pair.LeftLine == -1 || pair.RightLine == -1 {
			isPairChanged[i] = true
			hasAnyChange = true
		} else if len(leftSpansByLine[pair.LeftLine]) > 0 || len(rightSpansByLine[pair.RightLine]) > 0 {
			isPairChanged[i] = true
			hasAnyChange = true
		} else {
			sText := ""
			if pair.LeftLine >= 0 && pair.LeftLine < len(srcLines) {
				sText = srcLines[pair.LeftLine]
			}
			dText := ""
			if pair.RightLine >= 0 && pair.RightLine < len(dstLines) {
				dText = dstLines[pair.RightLine]
			}

			isEOFLine := (pair.LeftLine == lastSrcLineIdx && pair.RightLine == lastDstLineIdx)
			if sText != dText || (isEOFLine && srcEndsWithNL != dstEndsWithNL) {
				isPairChanged[i] = true
				hasAnyChange = true
			}
		}
	}

	if !hasAnyChange && srcEndsWithNL == dstEndsWithNL {
		return ""
	}

	changeIntervals := renderutil.BuildChangeIntervals(isPairChanged)
	if len(changeIntervals) == 0 {
		return ""
	}

	hunks := renderutil.MergeHunks(changeIntervals, len(filteredPairs), contextLines)

	srcOffsets := serialize.BuildLineIndex(srcBytes)
	dstOffsets := serialize.BuildLineIndex(dstBytes)

	// Need language rules to tell declarations apart from statements for badge placement.
	var r *rules.Rules
	if langName, err := treesitter.DetectLanguageName(srcFile); err == nil {
		r = rules.Get(langName)
	} else if langName, err := treesitter.DetectLanguageName(dstFile); err == nil {
		r = rules.Get(langName)
	}

	var meta renderutil.MoveBadges
	if !opts.DisableAnnotations {
		meta = renderutil.BuildMoveBadges(env.Actions, hunks, filteredPairs, srcOffsets, dstOffsets, srcLines, dstLines, r, true)
	}

	var slicer renderutil.Slicer

	maxLine := max(len(srcLines), len(dstLines))
	numWidth := max(3, len(strconv.Itoa(maxLine)))

	wrapActive := opts.Wrap && opts.TerminalWidth > 0
	targetWidth := 0
	if wrapActive {
		gutterWidth := 1
		if opts.LineNumbers {
			gutterWidth = 2*numWidth + 3
			if !opts.Color {
				gutterWidth++
			}
		}
		targetWidth = opts.TerminalWidth - gutterWidth
		if targetWidth < 20 {
			targetWidth = 20
		}
	}
	tabWidth := opts.TabWidth
	if tabWidth <= 0 {
		tabWidth = 4
	}

	var out strings.Builder
	out.Grow(len(srcBytes) + len(dstBytes))

	srcHeaderPath := formatFilePath(srcFile, "a/")
	dstHeaderPath := formatFilePath(dstFile, "b/")

	if opts.Color {
		out.WriteString(color.Bold + color.TextFg + "--- " + srcHeaderPath + color.Reset + "\n")
		out.WriteString(color.Bold + color.TextFg + "+++ " + dstHeaderPath + color.Reset + "\n")
	} else {
		out.WriteString("--- " + srcHeaderPath + "\n")
		out.WriteString("+++ " + dstHeaderPath + "\n")
	}

	for hunkIdx, h := range hunks {
		var lines []hunkLine
		k := h.Start
		for k <= h.End {
			if !isPairChanged[k] {
				pair := filteredPairs[k]
				text := ""
				if pair.LeftLine >= 0 && pair.LeftLine < len(srcLines) {
					text = srcLines[pair.LeftLine]
				} else if pair.RightLine >= 0 && pair.RightLine < len(dstLines) {
					text = dstLines[pair.RightLine]
				}
				lines = append(lines, hunkLine{
					kind:       kindContext,
					srcLineIdx: pair.LeftLine,
					dstLineIdx: pair.RightLine,
					text:       text,
				})
				k++
			} else {
				bEnd := k + 1
				// Group consecutive changes into deletions followed by insertions (2-block diff).
				for bEnd <= h.End && bEnd < len(isPairChanged) && isPairChanged[bEnd] {
					bEnd++
				}

				for p := k; p < bEnd; p++ {
					pair := filteredPairs[p]
					if pair.LeftLine != -1 && pair.LeftLine < len(srcLines) {
						lines = append(lines, hunkLine{
							kind:           kindDelete,
							srcLineIdx:     pair.LeftLine,
							dstLineIdx:     -1,
							hasCounterpart: pair.RightLine != -1,
							text:           srcLines[pair.LeftLine],
						})
					}
				}

				for p := k; p < bEnd; p++ {
					pair := filteredPairs[p]
					if pair.RightLine != -1 && pair.RightLine < len(dstLines) {
						lines = append(lines, hunkLine{
							kind:           kindInsert,
							srcLineIdx:     -1,
							dstLineIdx:     pair.RightLine,
							hasCounterpart: pair.LeftLine != -1,
							text:           dstLines[pair.RightLine],
						})
					}
				}

				k = bEnd
			}
		}

		hasSrc := len(srcBytes) > 0 && srcFile != os.DevNull && srcFile != "/dev/null"
		hasDst := len(dstBytes) > 0 && dstFile != os.DevNull && dstFile != "/dev/null"
		srcStart, srcCount, dstStart, dstCount := renderutil.ComputeHunkRange(h, filteredPairs, hasSrc, hasDst)

		hunkHdrExtra := meta.HunkHeaders[hunkIdx]
		hunkHeader := renderutil.FormatHunkHeader(srcStart, srcCount, dstStart, dstCount, hunkHdrExtra)
		if opts.Color {
			out.WriteString(color.HeaderFg + hunkHeader + color.Reset + "\n")
		} else {
			out.WriteString(hunkHeader + "\n")
		}

		for _, l := range lines {
			var gutter string
			var contGutter string
			if opts.LineNumbers {
				sNum := 0
				if l.srcLineIdx >= 0 {
					sNum = l.srcLineIdx + 1
				}
				dNum := 0
				if l.dstLineIdx >= 0 {
					dNum = l.dstLineIdx + 1
				}
				gutter = formatLineGutter(sNum, dNum, numWidth, opts.Color, l.kind)
				contGutter = formatContinuationGutter(numWidth)
				if !opts.Color {
					contGutter += " "
				}
			} else {
				contGutter = " "
			}

			var badge string
			var badgeColor int
			var spans []serialize.HighlightSpan
			lineCtx := renderutil.LineContextAligned

			switch l.kind {
			case kindContext:
			case kindDelete:
				if !opts.DisableAnnotations {
					badge = meta.SrcLineBadges[l.srcLineIdx]
					badgeColor = meta.SrcLineBadgeColors[l.srcLineIdx]
				}
				spans = leftSpansByLine[l.srcLineIdx]
				if renderutil.ShouldStyleStandalone(!l.hasCounterpart, spans, l.text) {
					lineCtx = renderutil.LineContextStandaloneDelete
				}
			case kindInsert:
				if !opts.DisableAnnotations {
					badge = meta.DstLineBadges[l.dstLineIdx]
					badgeColor = meta.DstLineBadgeColors[l.dstLineIdx]
				}
				spans = rightSpansByLine[l.dstLineIdx]
				if renderutil.ShouldStyleStandalone(!l.hasCounterpart, spans, l.text) {
					lineCtx = renderutil.LineContextStandaloneInsert
				}
			}

			cfg := renderutil.SliceConfig{
				TargetWidth:      targetWidth,
				TabWidth:         tabWidth,
				ColorMode:        opts.Color,
				PadToTargetWidth: false,
				BadgeColor:       badgeColor,
				Context:          lineCtx,
			}

			chunks := slicer.SliceLineToChunks(l.text, badge, spans, cfg)
			for i, chunk := range chunks {
				if i == 0 {
					switch l.kind {
					case kindContext:
						if opts.LineNumbers {
							if opts.Color {
								out.WriteString(gutter + string(chunk) + "\n")
							} else {
								out.WriteString(gutter + " " + string(chunk) + "\n")
							}
						} else {
							out.WriteString(" " + string(chunk) + "\n")
						}
					case kindDelete:
						if opts.LineNumbers {
							if opts.Color {
								out.WriteString(gutter + string(chunk) + "\n")
							} else {
								out.WriteString(gutter + "-" + string(chunk) + "\n")
							}
						} else {
							if opts.Color {
								prefix := color.DeleteFg + "-" + color.Reset
								out.WriteString(prefix + string(chunk) + "\n")
							} else {
								out.WriteString("-" + string(chunk) + "\n")
							}
						}
					case kindInsert:
						if opts.LineNumbers {
							if opts.Color {
								out.WriteString(gutter + string(chunk) + "\n")
							} else {
								out.WriteString(gutter + "+" + string(chunk) + "\n")
							}
						} else {
							if opts.Color {
								prefix := color.InsertFg + "+" + color.Reset
								out.WriteString(prefix + string(chunk) + "\n")
							} else {
								out.WriteString("+" + string(chunk) + "\n")
							}
						}
					}
				} else {
					out.WriteString(contGutter + string(chunk) + "\n")
				}
			}

			if l.kind == kindContext && !srcEndsWithNL && !dstEndsWithNL && l.srcLineIdx == lastSrcLineIdx && l.dstLineIdx == lastDstLineIdx {
				out.WriteString("\\ No newline at end of file\n")
			} else if l.kind == kindDelete && !srcEndsWithNL && l.srcLineIdx == lastSrcLineIdx {
				out.WriteString("\\ No newline at end of file\n")
			} else if l.kind == kindInsert && !dstEndsWithNL && l.dstLineIdx == lastDstLineIdx {
				out.WriteString("\\ No newline at end of file\n")
			}
		}
	}

	return out.String()
}

func formatLineGutter(srcLine, dstLine int, numWidth int, colorMode bool, kind lineKind) string {
	srcStr := ""
	if srcLine > 0 {
		srcStr = strconv.Itoa(srcLine)
	}
	dstStr := ""
	if dstLine > 0 {
		dstStr = strconv.Itoa(dstLine)
	}

	srcPad := fmt.Sprintf("%*s", numWidth, srcStr)
	dstPad := fmt.Sprintf("%*s", numWidth, dstStr)

	if colorMode {
		switch kind {
		case kindDelete:
			return color.Bold + color.DeleteFg + srcPad + color.Reset + " " + color.OverlayFg + dstPad + color.Reset + "  "
		case kindInsert:
			return color.OverlayFg + srcPad + color.Reset + " " + color.Bold + color.InsertFg + dstPad + color.Reset + "  "
		default:
			return color.OverlayFg + srcPad + color.Reset + " " + color.OverlayFg + dstPad + color.Reset + "  "
		}
	}
	return srcPad + " " + dstPad + "  "
}

func formatContinuationGutter(numWidth int) string {
	blankPad := strings.Repeat(" ", numWidth)
	return blankPad + " " + blankPad + "  "
}

// formatFilePath builds standard diff headers like "a/foo.go" or "/dev/null".
func formatFilePath(path, prefix string) string {
	if path == os.DevNull || path == "/dev/null" || path == "" {
		return "/dev/null"
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	if strings.HasPrefix(clean, "a/") || strings.HasPrefix(clean, "b/") {
		return clean
	}
	return prefix + clean
}
