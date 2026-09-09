package treesitter

import (
	"fmt"
	"io"
	"strings"
)

const maxTreeDepth = 1024

// DumpAST writes a formatted plain-text Unicode tree representation of an ASTNode to w.
func DumpAST(w io.Writer, root *ASTNode) error {
	if root == nil {
		return nil
	}
	return dumpASTNode(w, root, "", true, true, 0)
}

func dumpASTNode(w io.Writer, node *ASTNode, prefix string, isLast bool, isRoot bool, depth int) error {
	if node == nil {
		return nil
	}
	if depth > maxTreeDepth {
		return fmt.Errorf("treesitter: maximum tree depth exceeded (%d)", depth)
	}

	var line strings.Builder

	if !isRoot {
		line.WriteString(prefix)
		if isLast {
			line.WriteString("└── ")
		} else {
			line.WriteString("├── ")
		}
	}

	line.WriteString(node.Type)

	if node.Label != "" {
		fmt.Fprintf(&line, ": %q", node.Label)
	}

	if node.IsKeyword {
		line.WriteString(" [kw]")
	}
	if node.IsUnordered {
		line.WriteString(" [unordered]")
	}
	if node.Type == "ERROR" || strings.HasPrefix(node.Type, "MISSING") {
		line.WriteString(" [ERROR]")
	}

	fmt.Fprintf(&line, " [%d:%d - %d:%d] (bytes %d-%d)\n",
		node.StartRow+1, node.StartCol+1,
		node.EndRow+1, node.EndCol+1,
		node.StartByte, node.EndByte,
	)

	if _, err := io.WriteString(w, line.String()); err != nil {
		return err
	}

	newPrefix := prefix
	if !isRoot {
		if isLast {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}
	}

	childCount := len(node.Children)
	for i, child := range node.Children {
		last := i == childCount-1
		if err := dumpASTNode(w, child, newPrefix, last, false, depth+1); err != nil {
			return err
		}
	}

	return nil
}

// DumpCST writes a formatted plain-text Unicode tree representation of the raw Tree-sitter
// flat buffer nodes (Concrete Syntax Tree) to w.
func DumpCST(w io.Writer, nodes []FlatNode, symbols []string, src []byte) error {
	if len(nodes) == 0 {
		return nil
	}
	return dumpCSTNode(w, 0, nodes, symbols, src, "", true, true, 0)
}

func dumpCSTNode(w io.Writer, idx uint32, nodes []FlatNode, symbols []string, src []byte, prefix string, isLast bool, isRoot bool, depth int) error {
	if depth > maxTreeDepth {
		return fmt.Errorf("treesitter: maximum tree depth exceeded (%d)", depth)
	}
	if int(idx) >= len(nodes) {
		return nil
	}

	fn := &nodes[idx]
	var rawType string
	if (fn.Flags & FlatNodeError) != 0 {
		rawType = "ERROR"
	} else if int(fn.TypeID) < len(symbols) {
		rawType = symbols[fn.TypeID]
	}
	if (fn.Flags & FlatNodeMissing) != 0 {
		rawType = "MISSING " + rawType
	}

	var line strings.Builder

	if !isRoot {
		line.WriteString(prefix)
		if isLast {
			line.WriteString("└── ")
		} else {
			line.WriteString("├── ")
		}
	}

	line.WriteString(rawType)

	srcLen := uint32(len(src))
	if fn.ChildCount == 0 && srcLen > 0 {
		start, end := min(fn.StartByte, srcLen), min(fn.EndByte, srcLen)
		if start < end {
			snippet := strings.TrimSpace(string(src[start:end]))
			// Only show snippet if it's not identical to the type itself (e.g. for keywords or literals)
			if snippet != "" && snippet != rawType && snippet != strings.Trim(rawType, "\"") {
				snippet = strings.ReplaceAll(snippet, "\r\n", " ")
				snippet = strings.ReplaceAll(snippet, "\n", " ")
				snippet = strings.ReplaceAll(snippet, "\r", " ")
				runes := []rune(snippet)
				if len(runes) > 64 {
					snippet = string(runes[:64]) + "..."
				}
				fmt.Fprintf(&line, ": %q", snippet)
			}
		}
	}

	if (fn.Flags & FlatNodeError) != 0 {
		line.WriteString(" [ERROR]")
	}

	fmt.Fprintf(&line, " [%d:%d - %d:%d] (bytes %d-%d)\n",
		fn.StartRow+1, fn.StartCol+1,
		fn.EndRow+1, fn.EndCol+1,
		fn.StartByte, fn.EndByte,
	)

	if _, err := io.WriteString(w, line.String()); err != nil {
		return err
	}

	newPrefix := prefix
	if !isRoot {
		if isLast {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}
	}

	childIdx := fn.FirstChildIdx
	visited := 0
	maxNodes := len(nodes)
	for childIdx != FlatNodeNone && int(childIdx) < maxNodes && visited < maxNodes {
		visited++
		next := nodes[childIdx].NextSiblingIdx
		isLast := next == FlatNodeNone || int(next) >= maxNodes
		if err := dumpCSTNode(w, childIdx, nodes, symbols, src, newPrefix, isLast, false, depth+1); err != nil {
			return err
		}
		childIdx = next
	}

	return nil
}
