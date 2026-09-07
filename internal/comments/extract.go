// Package comments extracts and diffs comments across source files.
package comments

import (
	"slices"
	"strings"

	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

const flatSentinel = 0xFFFFFFFF

// CommentBlock represents an extracted comment and its source position.
type CommentBlock struct {
	Type         string
	Text         string
	StartByte    uint32
	EndByte      uint32
	StartRow     int // 0-indexed
	StartCol     int
	EndRow       int // 0-indexed
	EndCol       int
	ScopeKey     string
	ParentType   string
	ParentStart  uint32
	ParentEnd    uint32
	ParentRow    int
	ParentEndRow int
	Language     string

	// Structural Declaration & Scope Context
	DeclStart     uint32
	DeclEnd       uint32
	DeclType      string
	RelativePath  string
	EnclosingDecl *treesitter.ASTNode
}

// ExtractComments walks the raw flat buffer (pre-IsIgnored filtering)
// and extracts comment blocks with full CST sibling/parent context.
func ExtractComments(nodes []treesitter.FlatNode, symbols []string, src []byte, langName string) []CommentBlock {
	if len(nodes) == 0 {
		return nil
	}
	r := rules.Get(langName)
	srcLen := uint32(len(src))
	var list []CommentBlock

	for idx := range nodes {
		fn := &nodes[idx]
		rawType := flatSymbol(fn, symbols)
		if !isCommentType(rawType, r) {
			continue
		}

		// Skip child comment tokens whose ancestor is already a comment container
		// (e.g. in Rust where line_comment wraps doc_comment).
		isChildComment := false
		for pIdx := fn.ParentIdx; pIdx != flatSentinel && int(pIdx) < len(nodes); pIdx = nodes[pIdx].ParentIdx {
			if isCommentType(flatSymbol(&nodes[pIdx], symbols), r) {
				isChildComment = true
				break
			}
		}
		if isChildComment {
			continue
		}

		start := min(fn.StartByte, srcLen)
		end := min(fn.EndByte, srcLen)
		if start > end {
			start = end
		}
		text := string(src[start:end])

		var parentType string
		var parentStart, parentEnd uint32
		var parentRow, parentEndRow int
		if fn.ParentIdx != flatSentinel && int(fn.ParentIdx) < len(nodes) {
			p := &nodes[fn.ParentIdx]
			parentType = flatSymbol(p, symbols)
			parentStart = min(p.StartByte, srcLen)
			parentEnd = min(p.EndByte, srcLen)
			parentRow = int(p.StartRow)
			parentEndRow = int(p.EndRow)
		} else {
			parentEnd = srcLen
		}

		scopeKey := flatFindEnclosingDeclaration(uint32(idx), nodes, symbols, src, r)

		declStart, declEnd, declType, relPath := flatCheckLeadingDocComment(uint32(idx), nodes, symbols, src, r)
		if declType == "" {
			declStart, declEnd, declType, relPath = flatFindEnclosingDeclarationHierarchy(uint32(idx), nodes, symbols, src, r)
		}

		list = append(list, CommentBlock{
			Type:         rawType,
			Text:         text,
			StartByte:    start,
			EndByte:      end,
			StartRow:     int(fn.StartRow),
			StartCol:     int(fn.StartCol),
			EndRow:       int(fn.EndRow),
			EndCol:       int(fn.EndCol),
			ScopeKey:     scopeKey,
			ParentType:   parentType,
			ParentStart:  parentStart,
			ParentEnd:    parentEnd,
			ParentRow:    parentRow,
			ParentEndRow: parentEndRow,
			Language:     langName,
			DeclStart:    declStart,
			DeclEnd:      declEnd,
			DeclType:     declType,
			RelativePath: relPath,
		})
	}
	return list
}

func flatSymbol(fn *treesitter.FlatNode, symbols []string) string {
	if int(fn.TypeID) < len(symbols) {
		return symbols[fn.TypeID]
	}
	return ""
}

func isCommentType(nodeType string, r *rules.Rules) bool {
	if r != nil {
		return r.IsComment(nodeType)
	}
	return rules.IsComment(nodeType)
}

// flatCheckLeadingDocComment mirrors checkLeadingDocComment using flat buffer sibling walking.
func flatCheckLeadingDocComment(idx uint32, nodes []treesitter.FlatNode, symbols []string, src []byte, r *rules.Rules) (uint32, uint32, string, string) {
	if r == nil {
		return 0, 0, "", ""
	}
	fn := &nodes[idx]

	// Walk NextSiblingIdx to find the first non-comment, non-"\n" sibling
	nextIdx := fn.NextSiblingIdx
	for nextIdx != flatSentinel && int(nextIdx) < len(nodes) {
		nextType := flatSymbol(&nodes[nextIdx], symbols)
		if !isCommentType(nextType, r) && nextType != "\n" {
			break
		}
		nextIdx = nodes[nextIdx].NextSiblingIdx
	}
	if nextIdx == flatSentinel || int(nextIdx) >= len(nodes) {
		return 0, 0, "", ""
	}

	target := &nodes[nextIdx]
	t := flatSymbol(target, symbols)
	if !r.IsDeclaration(t) {
		// Check inner children for wrapped declarations (export_statement, etc.)
		childIdx := target.FirstChildIdx
		for childIdx != flatSentinel && int(childIdx) < len(nodes) {
			ct := flatSymbol(&nodes[childIdx], symbols)
			if r.IsDeclaration(ct) {
				target = &nodes[childIdx]
				t = ct
				break
			}
			childIdx = nodes[childIdx].NextSiblingIdx
		}
	}
	if !r.IsDeclaration(t) {
		return 0, 0, "", ""
	}

	// Check if comment is inside a block (interior statement comment vs exterior doc comment)
	pIdx := fn.ParentIdx
	insideBlock := false
	for pIdx != flatSentinel && int(pIdx) < len(nodes) {
		if r.IsBlock(flatSymbol(&nodes[pIdx], symbols)) {
			insideBlock = true
			break
		}
		pIdx = nodes[pIdx].ParentIdx
	}
	if insideBlock && r.IsLocalVarDeclaration(t) {
		return 0, 0, "", ""
	}

	if int(target.StartRow)-int(fn.EndRow) <= 1 {
		return target.StartByte, target.EndByte, t, "doc"
	}
	return 0, 0, "", ""
}

// flatFindEnclosingDeclarationHierarchy mirrors findEnclosingDeclarationHierarchy.
func flatFindEnclosingDeclarationHierarchy(idx uint32, nodes []treesitter.FlatNode, symbols []string, src []byte, r *rules.Rules) (uint32, uint32, string, string) {
	if r == nil {
		return 0, 0, "", "root"
	}
	var containers []string
	pIdx := nodes[idx].ParentIdx
	for pIdx != flatSentinel && int(pIdx) < len(nodes) {
		t := flatSymbol(&nodes[pIdx], symbols)
		if r.IsDeclaration(t) {
			relPath := "body"
			if len(containers) > 0 {
				slices.Reverse(containers)
				relPath = "body/" + strings.Join(containers, "/")
			}
			return nodes[pIdx].StartByte, nodes[pIdx].EndByte, t, relPath
		}
		if r.IsBlock(t) || r.IsScaffolding(t) {
			containers = append(containers, t)
		}
		pIdx = nodes[pIdx].ParentIdx
	}
	if len(containers) > 0 {
		slices.Reverse(containers)
		return 0, 0, "", "root/" + strings.Join(containers, "/")
	}
	return 0, 0, "", "root"
}

// flatFindEnclosingDeclaration mirrors findEnclosingDeclaration.
func flatFindEnclosingDeclaration(idx uint32, nodes []treesitter.FlatNode, symbols []string, src []byte, r *rules.Rules) string {
	if r == nil {
		return "root"
	}
	srcLen := uint32(len(src))
	var containers []string
	pIdx := nodes[idx].ParentIdx
	for pIdx != flatSentinel && int(pIdx) < len(nodes) {
		t := flatSymbol(&nodes[pIdx], symbols)
		if r.IsDeclaration(t) {
			name := flatGetDeclarationIdentifier(&nodes[pIdx], nodes, symbols, src, r, srcLen)
			decl := t
			if name != "" {
				decl = t + ":" + name
			}
			if len(containers) > 0 {
				slices.Reverse(containers)
				return decl + "/" + strings.Join(containers, "/")
			}
			return decl
		}
		if r.IsBlock(t) || r.IsScaffolding(t) {
			containers = append(containers, t)
		}
		pIdx = nodes[pIdx].ParentIdx
	}
	if len(containers) > 0 {
		slices.Reverse(containers)
		return "root/" + strings.Join(containers, "/")
	}
	return "root"
}

func flatGetDeclarationIdentifier(fn *treesitter.FlatNode, nodes []treesitter.FlatNode, symbols []string, src []byte, r *rules.Rules, srcLen uint32) string {
	if r == nil {
		return ""
	}
	childIdx := fn.FirstChildIdx
	for childIdx != flatSentinel && int(childIdx) < len(nodes) {
		child := &nodes[childIdx]
		ct := flatSymbol(child, symbols)
		if r.IsIdentifier(ct) {
			s := min(child.StartByte, srcLen)
			e := min(child.EndByte, srcLen)
			if s < e {
				return string(src[s:e])
			}
		}
		if r.IsScaffolding(ct) {
			subIdx := child.FirstChildIdx
			for subIdx != flatSentinel && int(subIdx) < len(nodes) {
				sub := &nodes[subIdx]
				if r.IsIdentifier(flatSymbol(sub, symbols)) {
					s := min(sub.StartByte, srcLen)
					e := min(sub.EndByte, srcLen)
					if s < e {
						return string(src[s:e])
					}
				}
				subIdx = sub.NextSiblingIdx
			}
		}
		childIdx = child.NextSiblingIdx
	}
	return ""
}

type declSpanKey struct {
	start uint32
	end   uint32
}

// BindASTNodes attaches exact AST node pointers to extracted comment blocks in O(K + N).
func BindASTNodes(comments []CommentBlock, root *treesitter.ASTNode) {
	if len(comments) == 0 || root == nil {
		return
	}
	declMap := make(map[declSpanKey]*treesitter.ASTNode)
	var indexDecls func(n *treesitter.ASTNode)
	indexDecls = func(n *treesitter.ASTNode) {
		if n == nil {
			return
		}
		key := declSpanKey{start: n.StartByte, end: n.EndByte}
		if existing, exists := declMap[key]; !exists || (existing != nil && existing.Type != n.Type) {
			declMap[key] = n
		}
		for _, child := range n.Children {
			indexDecls(child)
		}
	}
	indexDecls(root)

	for i := range comments {
		if comments[i].DeclEnd > comments[i].DeclStart || comments[i].DeclType != "" {
			if declNode, ok := declMap[declSpanKey{start: comments[i].DeclStart, end: comments[i].DeclEnd}]; ok {
				comments[i].EnclosingDecl = declNode
			}
		}
	}
}
