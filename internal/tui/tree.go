package tui
import (
	"path/filepath"
	"sort"
	"strings"
)
type treeNode struct {
	name      string
	path      string
	fileIndex int
	parent    *treeNode
	children  []*treeNode
	collapsed bool
}
type treeRow struct {
	node  *treeNode
	depth int
}
func buildFileTree(files []DiffFile) *treeNode {
	root := &treeNode{name: "root", fileIndex: -1}
	for i, file := range files {
		path := displayPath(file)
		addTreePath(root, filepath.ToSlash(path), i)
	}
	sortTree(root)
	return root
}
func displayPath(file DiffFile) string {
	if file.RelPath != "" {
		return file.RelPath
	}
	if file.NewPath != "" {
		return file.NewPath
	}
	return file.OldPath
}
func addTreePath(root *treeNode, path string, fileIndex int) {
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' })
	if len(parts) == 0 {
		parts = []string{path}
	}
	current := root
	var full []string
	for i, part := range parts {
		if part == "" {
			continue
		}
		full = append(full, part)
		child := current.childNamed(part)
		if child == nil {
			child = &treeNode{
				name:      part,
				path:      strings.Join(full, "/"),
				fileIndex: -1,
				parent:    current,
			}
			current.children = append(current.children, child)
		}
		if i == len(parts)-1 {
			child.fileIndex = fileIndex
		}
		current = child
	}
}
func (n *treeNode) childNamed(name string) *treeNode {
	for _, child := range n.children {
		if child.name == name {
			return child
		}
	}
	return nil
}
func sortTree(n *treeNode) {
	sort.SliceStable(n.children, func(i, j int) bool {
		a, b := n.children[i], n.children[j]
		aDir, bDir := len(a.children) > 0, len(b.children) > 0
		if aDir != bDir {
			return aDir
		}
		return a.name < b.name
	})
	for _, child := range n.children {
		sortTree(child)
	}
}
func flattenTree(root *treeNode) []treeRow {
	var rows []treeRow
	for _, child := range root.children {
		flattenTreeNode(child, 0, &rows)
	}
	return rows
}
func flattenTreeNode(n *treeNode, depth int, rows *[]treeRow) {
	*rows = append(*rows, treeRow{node: n, depth: depth})
	if n.collapsed {
		return
	}
	for _, child := range n.children {
		flattenTreeNode(child, depth+1, rows)
	}
}
func firstVisibleFile(rows []treeRow) int {
	for i, row := range rows {
		if row.node.fileIndex >= 0 {
			return i
		}
	}
	return 0
}
func clampTreeCursor(cursor int, rows []treeRow) int {
	if len(rows) == 0 {
		return 0
	}
	if cursor < 0 {
		return 0
	}
	if cursor >= len(rows) {
		return len(rows) - 1
	}
	return cursor
}
