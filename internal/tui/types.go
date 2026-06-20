package tui
import (
	"math"
	"strings"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)
// DiffFile is the TUI-facing representation of a single semantic diff.
type DiffFile struct {
	OldPath      string
	NewPath      string
	RelPath      string
	OldLines     []string
	NewLines     []string
	OldTokens    [][]chroma.Token
	NewTokens    [][]chroma.Token
	Hunks        []Hunk
	VisualHunks  []Hunk
	LeftSpans    []visualSpan
	RightSpans   []visualSpan
	NeedsCompute bool
}
func NewDiffFile(oldPath, newPath string, oldSrc, newSrc []byte, hunks []Hunk) DiffFile {
	return DiffFile{
		OldPath:     oldPath,
		NewPath:     newPath,
		OldLines:    splitSourceLines(oldSrc),
		NewLines:    splitSourceLines(newSrc),
		Hunks:       hunks,
		VisualHunks: hunks,
		OldTokens:   tokenizeFile(oldPath, oldSrc),
		NewTokens:   tokenizeFile(newPath, newSrc),
	}
}
func tokenizeFile(path string, src []byte) [][]chroma.Token {
	if len(src) == 0 {
		return nil
	}
	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)
	iterator, err := lexer.Tokenise(nil, string(src))
	if err != nil {
		return nil
	}
	var lines [][]chroma.Token
	var currentLine []chroma.Token
	for _, tok := range iterator.Tokens() {
		val := tok.Value
		for {
			idx := strings.IndexByte(val, '\n')
			if idx == -1 {
				if len(val) > 0 {
					currentLine = append(currentLine, chroma.Token{Type: tok.Type, Value: val})
				}
				break
			}
			part := val[:idx]
			if len(part) > 0 {
				currentLine = append(currentLine, chroma.Token{Type: tok.Type, Value: part})
			}
			lines = append(lines, currentLine)
			currentLine = nil
			val = val[idx+1:]
		}
	}
	if len(currentLine) > 0 || len(lines) == 0 {
		lines = append(lines, currentLine)
	}
	return lines
}
func NewDiffFileWithDetails(
	oldPath, newPath string,
	oldSrc, newSrc []byte,
	hunks []Hunk,
	actions any,
	mappings *engine.Mapping,
) DiffFile {
	file := NewDiffFile(oldPath, newPath, oldSrc, newSrc, hunks)
	file.VisualHunks = normalizeVisualHunks(hunks, mappings)
	file.LeftSpans, file.RightSpans = buildVisualSpans(actions, mappings)
	return file
}
type lineAnnotation struct {
	Kind     ChangeKind
	Priority int
	Spans    []visualSpan
}
type visualSpan struct {
	Kind      ChangeKind
	StartLine int
	EndLine   int
	StartCol  int
	EndCol    int
	Priority  int
}
func annotationPriority(kind ChangeKind) int {
	switch kind {
	case ChangeInsert, ChangeDelete:
		return 4
	case ChangeMove:
		return 3 // Move has priority over Update for full line-level backgrounds to keep them contiguous
	case ChangeUpdate:
		return 2
	default:
		return 1
	}
}
func visualSpanPriority(kind ChangeKind) int {
	switch kind {
	case ChangeInsert, ChangeDelete:
		return 4
	case ChangeUpdate:
		return 3 // Update has priority over Move for token-level highlight visibility
	case ChangeMove:
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
func (f DiffFile) displayHunks() []Hunk {
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
		case ChangeInsert:
			applyAnnotation(right, h.DstStartLine, h.DstEndLine, h.Kind)
		case ChangeDelete:
			applyAnnotation(left, h.SrcStartLine, h.SrcEndLine, h.Kind)
		case ChangeUpdate:
			applyAnnotation(left, h.SrcStartLine, h.SrcEndLine, h.Kind)
			applyAnnotation(right, h.DstStartLine, h.DstEndLine, h.Kind)
		case ChangeMove:
			applyAnnotation(left, h.SrcStartLine, h.SrcEndLine, h.Kind)
			applyAnnotation(right, h.DstStartLine, h.DstEndLine, h.Kind)
		}
	}
	applySpans(left, file.LeftSpans)
	applySpans(right, file.RightSpans)
	return left, right
}
func applyAnnotation(dst map[int]lineAnnotation, start, end int, kind ChangeKind) {
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
				ann.Priority = annotationPriority(span.Kind)
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
func buildVisualSpans(es any, mappings *engine.Mapping) ([]visualSpan, []visualSpan) {
	// TODO: fix after v0.1.0 — TUI needs to be updated to use the new actions.EditScript type instead of engine.EditScript.
	return nil, nil
}

func addLeafSpans(n *treesitter.ASTNode, kind ChangeKind, mappings *engine.Mapping, isSrc bool, checkMatches bool, spans *[]visualSpan) {
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

func isStatementOrDeclarationNode(n *treesitter.ASTNode) bool {
	if n == nil {
		return false
	}
	if isStatementOrDeclaration(n.Type) {
		return true
	}
	if n.Parent != nil && (n.Parent.Type == "block" || n.Parent.Type == "module") {
		return true
	}
	return false
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

func spanFromNode(n *treesitter.ASTNode, kind ChangeKind) (visualSpan, bool) {
	if n == nil {
		return visualSpan{}, false
	}
	return visualSpan{
		Kind:      kind,
		StartLine: int(n.StartRow) + 1,
		EndLine:   int(n.EndRow) + 1,
		StartCol:  int(n.StartCol),
		EndCol:    int(n.EndCol),
		Priority:  visualSpanPriority(kind),
	}, true
}
func resolveActionOriginal(n *treesitter.ASTNode, c2o map[*treesitter.ASTNode]*treesitter.ASTNode) *treesitter.ASTNode {
	if orig, ok := c2o[n]; ok {
		return orig
	}
	return n
}
