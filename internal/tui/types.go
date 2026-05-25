package tui
import (
	"math"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/output"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)
// DiffFile is the TUI-facing representation of a single semantic diff.
type DiffFile struct {
	OldPath      string
	NewPath      string
	OldLines     []string
	NewLines     []string
	Hunks        []output.Hunk
	VisualHunks  []output.Hunk
	LeftSpans    []visualSpan
	RightSpans   []visualSpan
	RelPath      string
	NeedsCompute bool
}
func NewDiffFile(oldPath, newPath string, oldSrc, newSrc []byte, hunks []output.Hunk) DiffFile {
	return DiffFile{
		OldPath:     oldPath,
		NewPath:     newPath,
		OldLines:    splitSourceLines(oldSrc),
		NewLines:    splitSourceLines(newSrc),
		Hunks:       hunks,
		VisualHunks: hunks,
	}
}
func NewDiffFileWithDetails(
	oldPath, newPath string,
	oldSrc, newSrc []byte,
	hunks []output.Hunk,
	actions *engine.EditScript,
	mappings *engine.Mapping,
) DiffFile {
	file := NewDiffFile(oldPath, newPath, oldSrc, newSrc, hunks)
	file.VisualHunks = normalizeVisualHunks(hunks, mappings)
	file.LeftSpans, file.RightSpans = buildVisualSpans(actions, mappings)
	return file
}
type lineAnnotation struct {
	Kind     output.ChangeKind
	Priority int
	Spans    []visualSpan
}
type visualSpan struct {
	Kind      output.ChangeKind
	StartLine int
	EndLine   int
	StartCol  int
	EndCol    int
	Priority  int
}
func annotationPriority(kind output.ChangeKind) int {
	switch kind {
	case output.ChangeInsert, output.ChangeDelete:
		return 4
	case output.ChangeMove:
		return 3 // Move has priority over Update to keep move backgrounds contiguous
	case output.ChangeUpdate:
		return 2
	default:
		return 1
	}
}
func splitSourceLines(src []byte) []string {
	if len(src) == 0 {
		return nil
	}
	lines := make([]string, 0)
	start := 0
	for i, b := range src {
		if b != '\n' {
			continue
		}
		line := string(src[start:i])
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		lines = append(lines, line)
		start = i + 1
	}
	if start < len(src) {
		line := string(src[start:])
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		lines = append(lines, line)
	}
	return lines
}
func (f DiffFile) displayHunks() []output.Hunk {
	if f.VisualHunks != nil {
		return f.VisualHunks
	}
	return f.Hunks
}
func buildLineAnnotations(file DiffFile) (map[int]lineAnnotation, map[int]lineAnnotation) {
	left := make(map[int]lineAnnotation)
	right := make(map[int]lineAnnotation)
	for _, h := range file.displayHunks() {
		switch h.Kind {
		case output.ChangeInsert:
			applyAnnotation(right, h.DstStartLine, h.DstEndLine, h.Kind)
		case output.ChangeDelete:
			applyAnnotation(left, h.SrcStartLine, h.SrcEndLine, h.Kind)
		case output.ChangeUpdate:
			applyAnnotation(left, h.SrcStartLine, h.SrcEndLine, h.Kind)
			applyAnnotation(right, h.DstStartLine, h.DstEndLine, h.Kind)
		case output.ChangeMove:
			applyAnnotation(left, h.SrcStartLine, h.SrcEndLine, h.Kind)
			applyAnnotation(right, h.DstStartLine, h.DstEndLine, h.Kind)
		}
	}
	applySpans(left, file.LeftSpans)
	applySpans(right, file.RightSpans)
	return left, right
}
func applyAnnotation(dst map[int]lineAnnotation, start, end int, kind output.ChangeKind) {
	if start <= 0 || end <= 0 {
		return
	}
	if end < start {
		start, end = end, start
	}
	for line := start; line <= end; line++ {
		next := lineAnnotation{Kind: kind, Priority: annotationPriority(kind)}
		current, ok := dst[line]
		if !ok || next.Priority >= current.Priority {
			next.Spans = current.Spans
			dst[line] = next
		}
	}
}
func applySpans(dst map[int]lineAnnotation, spans []visualSpan) {
	for _, span := range spans {
		if span.StartLine <= 0 || span.EndLine <= 0 {
			continue
		}
		for line := span.StartLine; line <= span.EndLine; line++ {
			ann := dst[line]
			ann.Spans = append(ann.Spans, span.lineSpan(line))
			if ann.Priority == 0 {
				ann.Kind = span.Kind
				ann.Priority = span.Priority
			}
			dst[line] = ann
		}
	}
}
func (s visualSpan) lineSpan(line int) visualSpan {
	out := s
	out.StartLine = line
	out.EndLine = line
	if line > s.StartLine {
		out.StartCol = 0
	}
	if line < s.EndLine {
		out.EndCol = math.MaxInt
	}
	return out
}
func buildVisualSpans(es *engine.EditScript, mappings *engine.Mapping) ([]visualSpan, []visualSpan) {
	var left, right []visualSpan
	if es != nil {
		for _, action := range es.Actions {
			switch action.Kind {
			case engine.ActionInsert, engine.ActionInsertTree:
				checkMatches := !isStatementOrDeclaration(action.T2Ref.Type)
				addLeafSpans(action.T2Ref, output.ChangeInsert, mappings, false, checkMatches, &right)
			case engine.ActionDelete, engine.ActionDeleteTree:
				node := resolveActionOriginal(action.Node, es.CopyToOrig)
				checkMatches := !isStatementOrDeclaration(node.Type)
				addLeafSpans(node, output.ChangeDelete, mappings, true, checkMatches, &left)
			case engine.ActionUpdate:
				node := resolveActionOriginal(action.Node, es.CopyToOrig)
				addLeafSpans(node, output.ChangeUpdate, mappings, true, false, &left)
				addLeafSpans(action.T2Ref, output.ChangeUpdate, mappings, false, false, &right)
			}
		}
	}
	for _, pair := range normalizedVisualMovePairs(mappings) {
		if span, ok := spanFromNode(pair.Src, output.ChangeMove); ok {
			left = append(left, span)
		}
		if span, ok := spanFromNode(pair.Dst, output.ChangeMove); ok {
			right = append(right, span)
		}
	}
	return left, right
}

func addLeafSpans(n *treesitter.ASTNode, kind output.ChangeKind, mappings *engine.Mapping, isSrc bool, checkMatches bool, spans *[]visualSpan) {
	if n == nil {
		return
	}
	if len(n.Children) == 0 {
		if checkMatches && mappings != nil {
			matched := false
			if isSrc {
				matched = mappings.Has(n)
			} else {
				matched = mappings.HasDst(n)
			}
			if matched {
				return
			}
		}
		if span, ok := spanFromNode(n, kind); ok {
			*spans = append(*spans, span)
		}
		return
	}
	for _, child := range n.Children {
		addLeafSpans(child, kind, mappings, isSrc, checkMatches, spans)
	}
}

func isStatementOrDeclaration(nodeType string) bool {
	if isDeclarationLike(nodeType) {
		return true
	}
	switch nodeType {
	case "short_var_declaration", "assignment_statement", "expression_statement",
		"if_statement", "for_statement", "return_statement", "import_declaration",
		"package_clause", "comment":
		return true
	}
	return false
}

func spanFromNode(n *treesitter.ASTNode, kind output.ChangeKind) (visualSpan, bool) {
	if n == nil {
		return visualSpan{}, false
	}
	return visualSpan{
		Kind:      kind,
		StartLine: int(n.StartRow) + 1,
		EndLine:   int(n.EndRow) + 1,
		StartCol:  int(n.StartCol),
		EndCol:    int(n.EndCol),
		Priority:  annotationPriority(kind),
	}, true
}
func resolveActionOriginal(n *treesitter.ASTNode, c2o map[*treesitter.ASTNode]*treesitter.ASTNode) *treesitter.ASTNode {
	if orig, ok := c2o[n]; ok {
		return orig
	}
	return n
}
