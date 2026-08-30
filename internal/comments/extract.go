// Package comments extracts and diffs comments across source files.
package comments

import (
	"slices"
	"strings"

	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
	"github.com/odvcencio/gotreesitter"
)

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
}

// ExtractComments walks the AST and returns all comment blocks.
func ExtractComments(root *gotreesitter.Node, src []byte, lang *gotreesitter.Language, r *rules.Rules) []CommentBlock {
	if root == nil || lang == nil {
		return nil
	}
	var list []CommentBlock
	traverseForComments(root, src, lang, r, &list)
	return list
}

func traverseForComments(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language, r *rules.Rules, list *[]CommentBlock) {
	if n == nil {
		return
	}
	nodeType := n.Type(lang)
	if (r != nil && r.IsComment(nodeType)) || (r == nil && nodeType == "comment") {
		srcLen := uint32(len(src))
		start := min(n.StartByte(), srcLen)
		end := min(n.EndByte(), srcLen)
		if start > end {
			start = end
		}
		text := string(src[start:end])

		var parentType string
		var parentStart, parentEnd uint32
		var parentRow, parentEndRow int
		if p := n.Parent(); p != nil {
			parentType = p.Type(lang)
			parentStart = min(p.StartByte(), srcLen)
			parentEnd = min(p.EndByte(), srcLen)
			parentRow = int(p.StartPoint().Row)
			parentEndRow = int(p.EndPoint().Row)
		} else {
			parentEnd = srcLen
		}

		scopeKey := findEnclosingDeclaration(n, src, lang, r)

		*list = append(*list, CommentBlock{
			Type:         nodeType,
			Text:         text,
			StartByte:    start,
			EndByte:      end,
			StartRow:     int(n.StartPoint().Row),
			StartCol:     int(n.StartPoint().Column),
			EndRow:       int(n.EndPoint().Row),
			EndCol:       int(n.EndPoint().Column),
			ScopeKey:     scopeKey,
			ParentType:   parentType,
			ParentStart:  parentStart,
			ParentEnd:    parentEnd,
			ParentRow:    parentRow,
			ParentEndRow: parentEndRow,
		})
		return
	}
	for i := range n.ChildCount() {
		traverseForComments(n.Child(i), src, lang, r, list)
	}
}

func findEnclosingDeclaration(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language, r *rules.Rules) string {
	if r == nil || n == nil {
		return "root"
	}
	var containers []string
	curr := n.Parent()
	for curr != nil {
		t := curr.Type(lang)
		if r.IsDeclaration(t) {
			name := getDeclarationIdentifier(curr, src, lang, r)
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
		curr = curr.Parent()
	}
	if len(containers) > 0 {
		slices.Reverse(containers)
		return "root/" + strings.Join(containers, "/")
	}
	return "root"
}

func getDeclarationIdentifier(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language, r *rules.Rules) string {
	if n == nil || r == nil {
		return ""
	}
	for i := range n.ChildCount() {
		child := n.Child(i)
		if child == nil {
			continue
		}
		ct := child.Type(lang)
		if r.IsIdentifier(ct) {
			s := min(child.StartByte(), uint32(len(src)))
			e := min(child.EndByte(), uint32(len(src)))
			if s < e {
				return string(src[s:e])
			}
		}
		if r.IsScaffolding(ct) {
			for j := range child.ChildCount() {
				sub := child.Child(j)
				if sub != nil && r.IsIdentifier(sub.Type(lang)) {
					s := min(sub.StartByte(), uint32(len(src)))
					e := min(sub.EndByte(), uint32(len(src)))
					if s < e {
						return string(src[s:e])
					}
				}
			}
		}
	}
	return ""
}
