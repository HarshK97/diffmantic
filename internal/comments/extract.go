// Package comments extracts and diffs comments across source files.
package comments

import (
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// CommentBlock represents an extracted comment and its source position.
type CommentBlock = treesitter.CommentBlock

// CollectTrivia walks the AST and gathers all attached comment trivia.
func CollectTrivia(root *treesitter.ASTNode) []*CommentBlock {
	if root == nil {
		return nil
	}
	var list []*CommentBlock
	var walk func(n *treesitter.ASTNode)
	walk = func(n *treesitter.ASTNode) {
		if n == nil {
			return
		}
		list = append(list, n.LeadingTrivia...)
		list = append(list, n.TrailingTrivia...)
		for _, child := range n.Children {
			walk(child)
		}
		list = append(list, n.DanglingTrivia...)
	}
	walk(root)
	return list
}

// ExtractComments returns all comment blocks attached to an AST in source order.
func ExtractComments(root *treesitter.ASTNode) []CommentBlock {
	trivia := CollectTrivia(root)
	result := make([]CommentBlock, len(trivia))
	for i, t := range trivia {
		if t != nil {
			result[i] = *t
		}
	}
	return result
}
