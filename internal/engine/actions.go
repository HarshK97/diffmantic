package engine

import (
	"fmt"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// ActionKind enumerates the four Chawathe edit operations.
type ActionKind int

const (
	ActionInsert ActionKind = iota
	ActionDelete
	ActionUpdate
	ActionMove
)

func (k ActionKind) String() string {
	switch k {
	case ActionInsert:
		return "INS"
	case ActionDelete:
		return "DEL"
	case ActionUpdate:
		return "UPD"
	case ActionMove:
		return "MOV"
	default:
		return "???"
	}
}

// Action represents a single edit operation in the edit script.
type Action struct {
	Kind     ActionKind
	Node     *treesitter.ASTNode
	Parent   *treesitter.ASTNode
	Position int
	Value    string
	T2Ref    *treesitter.ASTNode
}

// copyResult holds the copied tree root and bidirectional maps between
// original and copied nodes.
type copyResult struct {
	Root       *treesitter.ASTNode
	origToCopy map[*treesitter.ASTNode]*treesitter.ASTNode
	copyToOrig map[*treesitter.ASTNode]*treesitter.ASTNode
}

// deepCopyTree creates a deep copy of the tree rooted at n.
func deepCopyTree(n *treesitter.ASTNode) *copyResult {
	o2c := make(map[*treesitter.ASTNode]*treesitter.ASTNode)
	c2o := make(map[*treesitter.ASTNode]*treesitter.ASTNode)
	root := deepCopyNode(n, nil, o2c, c2o)
	return &copyResult{Root: root, origToCopy: o2c, copyToOrig: c2o}
}

func deepCopyNode(
	n, parent *treesitter.ASTNode,
	o2c, c2o map[*treesitter.ASTNode]*treesitter.ASTNode,
) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	cp := &treesitter.ASTNode{
		Type:   n.Type,
		Label:  n.Label,
		Parent: parent,
	}
	o2c[n] = cp
	c2o[cp] = n
	for _, child := range n.Children {
		cc := deepCopyNode(child, cp, o2c, c2o)
		if cc != nil {
			cp.Children = append(cp.Children, cc)
		}
	}
	return cp
}

// rebuildMapping translates the original matching M (which maps
// original-T1 nodes → T2 nodes) into M' that maps copied-T1 → T2.
func rebuildMapping(m *Mapping, cr *copyResult) *Mapping {
	mPrime := NewMapping()
	for _, p := range m.Pairs {
		if cp, ok := cr.origToCopy[p.Src]; ok {
			mPrime.Add(cp, p.Dst)
		}
	}
	return mPrime
}

// inOrder tracks which nodes are currently marked "in order".
type inOrderSet struct {
	s map[*treesitter.ASTNode]bool
}

func newInOrderSet() *inOrderSet {
	return &inOrderSet{s: make(map[*treesitter.ASTNode]bool)}
}

func (io *inOrderSet) mark(n *treesitter.ASTNode)           { io.s[n] = true }
func (io *inOrderSet) unmark(n *treesitter.ASTNode)         { delete(io.s, n) }
func (io *inOrderSet) isInOrder(n *treesitter.ASTNode) bool { return io.s[n] }

// insertChild inserts child as the k-th child (1-based) of parent,
// and sets child.Parent.
func insertChild(parent, child *treesitter.ASTNode, k int) {
	child.Parent = parent
	idx := max(k-1, 0) // convert to 0-based
	if idx >= len(parent.Children) {
		parent.Children = append(parent.Children, child)
	} else {
		parent.Children = append(parent.Children, nil)
		copy(parent.Children[idx+1:], parent.Children[idx:])
		parent.Children[idx] = child
	}
}

// removeChild removes child from its parent's Children slice.
func removeChild(child *treesitter.ASTNode) {
	p := child.Parent
	if p == nil {
		return
	}
	for i, c := range p.Children {
		if c == child {
			p.Children = append(p.Children[:i], p.Children[i+1:]...)
			break
		}
	}
	child.Parent = nil
}

// childIndex returns the 1-based position of child among its parent's children.
func childIndex(child *treesitter.ASTNode) int {
	if child.Parent == nil {
		return 1
	}
	for i, c := range child.Parent.Children {
		if c == child {
			return i + 1
		}
	}
	return 1
}

// x is a node in T2. Returns the 1-based position among the children
// of the partner of p(x) in T1 where x's partner should be placed.
func findPos(x *treesitter.ASTNode, mPrime *Mapping, io *inOrderSet) int {
	y := x.Parent
	if y == nil {
		return 1
	}

	var leftInOrder *treesitter.ASTNode
	for _, sib := range y.Children {
		if sib == x {
			break
		}
		if io.isInOrder(sib) {
			leftInOrder = sib
		}
	}
	if leftInOrder == nil {
		return 1
	}

	u, ok := mPrime.Dst()[leftInOrder]
	if !ok {
		return 1
	}

	return childIndex(u) + 1
}

// w is a node in the copied T1, x is a node in T2.
// Aligns the children of w to match the order of x's children.
func alignChildren(
	w, x *treesitter.ASTNode,
	mPrime *Mapping,
	io *inOrderSet,
	script *[]Action,
) {
	// 1. Mark all children of w and x "out of order".
	for _, c := range w.Children {
		io.unmark(c)
	}
	for _, c := range x.Children {
		io.unmark(c)
	}

	// 2. Build S1: children of w whose partners are children of x.
	//    Build S2: children of x whose partners are children of w.
	var s1 []*treesitter.ASTNode
	for _, c := range w.Children {
		if partner, ok := mPrime.Src()[c]; ok {
			if partner.Parent == x {
				s1 = append(s1, c)
			}
		}
	}

	var s2 []*treesitter.ASTNode
	for _, c := range x.Children {
		if partner, ok := mPrime.Dst()[c]; ok {
			if partner.Parent == w {
				s2 = append(s2, c)
			}
		}
	}

	// 3. equal(a, b) = true iff (a, b) ∈ M'
	equal := func(a, b *treesitter.ASTNode) bool {
		if mapped, ok := mPrime.Src()[a]; ok {
			return mapped == b
		}
		return false
	}

	// 4. S ← LCS(S1, S2, equal)
	s := lcs(s1, s2, equal)

	// 5. For each (a, b) ∈ S, mark a and b "in order".
	inLCS := make(map[*treesitter.ASTNode]bool)
	for _, pair := range s {
		io.mark(pair[0])
		io.mark(pair[1])
		inLCS[pair[0]] = true
	}

	// 6. For each a ∈ S1 matched to b ∈ S2 via M', but (a,b) ∉ S:
	//    move a to w at position FindPos(b), mark a and b "in order".
	for _, a := range s1 {
		if inLCS[a] {
			continue
		}
		b, ok := mPrime.Src()[a]
		if !ok {
			continue
		}

		k := findPos(b, mPrime, io)

		*script = append(*script, Action{
			Kind:     ActionMove,
			Node:     a,
			Parent:   w,
			Position: k,
			T2Ref:    b,
		})

		// Apply MOV(a, w, k) to T1
		removeChild(a)
		insertChild(w, a, k)

		io.mark(a)
		io.mark(b)
	}
}

// BFS helper
func bfs(root *treesitter.ASTNode) []*treesitter.ASTNode {
	if root == nil {
		return nil
	}
	queue := []*treesitter.ASTNode{root}
	var out []*treesitter.ASTNode
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		out = append(out, n)
		queue = append(queue, n.Children...)
	}
	return out
}

// Given the original trees T1, T2 and the matching M produced by Match,
// compute the Chawathe minimum conforming edit script.
//
// The original T1 is deep-copied; the copy is mutated to become
// isomorphic to T2.  The returned actions reference nodes in the copy.
func GenerateActions(
	t1Root, t2Root *treesitter.ASTNode,
	m *Mapping,
) []Action {
	// 0. Deep-copy T1 so we never mutate the original.
	cr := deepCopyTree(t1Root)
	t1Copy := cr.Root

	// 1. E ← ∅, M' ← M (re-keyed to use copied nodes)
	var script []Action
	mPrime := rebuildMapping(m, cr)
	io := newInOrderSet()

	// 2. Visit nodes of T2 in breadth-first order.
	for _, x := range bfs(t2Root) {
		y := x.Parent // p(x) in T2

		// The root of T2 must be matched; skip it for insert/move logic.
		if y == nil {
			// x is the root of T2 – handle update if needed.
			if w, ok := mPrime.Dst()[x]; ok {
				if len(x.Children) == 0 && w.Label != x.Label {
					script = append(script, Action{
						Kind:  ActionUpdate,
						Node:  w,
						Value: x.Label,
						T2Ref: x,
					})
					w.Label = x.Label
				}
			}
			continue
		}

		// z = partner of y in M' (y's partner in T1-copy)
		z, hasZ := mPrime.Dst()[y]

		// (b) If x has no partner in M' → INSERT
		if !mPrime.HasDst(x) {
			if !hasZ {
				continue
			}

			k := findPos(x, mPrime, io)
			w := &treesitter.ASTNode{
				Type:  x.Type,
				Label: x.Label,
			}

			script = append(script, Action{
				Kind:     ActionInsert,
				Node:     w,
				Parent:   z,
				Position: k,
				Value:    x.Label,
				T2Ref:    x,
			})

			// Apply INS to T1-copy
			insertChild(z, w, k)

			// Add (w, x) to M'
			mPrime.Add(w, x)

			continue
		}

		// (c) x is not the root and has a partner
		w := mPrime.Dst()[x]
		v := w.Parent

		// (c.ii) If v(w) ≠ v(x) → UPDATE (only for leaf nodes)
		if len(x.Children) == 0 && w.Label != x.Label {
			script = append(script, Action{
				Kind:  ActionUpdate,
				Node:  w,
				Value: x.Label,
				T2Ref: x,
			})
			w.Label = x.Label
		}

		// (c.iii) If (y, v) ∉ M' → MOVE
		if hasZ && v != z {
			// The parent relationship disagrees with the matching.
			k := findPos(x, mPrime, io)

			script = append(script, Action{
				Kind:     ActionMove,
				Node:     w,
				Parent:   z,
				Position: k,
				T2Ref:    x,
			})

			// Apply MOV(w, z, k) to T1-copy
			removeChild(w)
			insertChild(z, w, k)
		}

		// (d) AlignChildren(w, x)
		alignChildren(w, x, mPrime, io, &script)
	}

	// 3. Post-order traversal of T1-copy – DELETE phase.
	// We snapshot the post-order before deleting so we don't
	// iterate over a mutating tree.
	for _, w := range PostOrder(t1Copy) {
		if !mPrime.Has(w) {
			script = append(script, Action{
				Kind: ActionDelete,
				Node: w,
			})
			removeChild(w)
		}
	}

	return script
}

// PrintActions prints the edit script in a human-readable table.
func PrintActions(actions []Action) {
	if len(actions) == 0 {
		fmt.Println("(no edit actions)")
		return
	}

	fmt.Printf("\n%-4s  %-4s  %-25s %-20s  %-25s  %s\n",
		"#", "Op", "Node Type", "Node Label", "Parent Type", "Details")
	fmt.Println("──────────────────────────────────────────────────────────────────────────────────────────────────────")

	for i, a := range actions {
		nodeType := a.Node.Type
		nodeLabel := a.Node.Label
		if nodeLabel == "" {
			nodeLabel = "-"
		}

		parentType := "-"
		detail := ""

		switch a.Kind {
		case ActionInsert:
			if a.Parent != nil {
				parentType = a.Parent.Type
			}
			detail = fmt.Sprintf("pos=%d val=%q", a.Position, a.Value)
		case ActionDelete:
			// no extra detail
		case ActionUpdate:
			detail = fmt.Sprintf("val=%q", a.Value)
		case ActionMove:
			if a.Parent != nil {
				parentType = a.Parent.Type
			}
			detail = fmt.Sprintf("pos=%d", a.Position)
		}

		fmt.Printf("%-4d  %-4s  %-25s %-20s  %-25s  %s\n",
			i+1, a.Kind, nodeType, nodeLabel, parentType, detail)
	}
	fmt.Printf("\nTotal actions: %d\n", len(actions))
}
