package actions

import (
	"slices"

	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// GenerateEditScript produces the insert, delete, move, and update actions
// needed to turn src into dst given their matched nodes.
func GenerateEditScript(
	src, dst *treesitter.ASTNode,
	ms *engine.Mapping,
) *EditScript {
	if src == nil && dst == nil {
		return NewEditScript()
	}
	if ms == nil {
		ms = engine.NewMapping()
	}
	s := &chawatheState{}
	s.init(src, dst, ms)
	return s.generate()
}

// cnode is a mutable copy of an AST node for in-place tree edits.
type cnode struct {
	orig     *treesitter.ASTNode
	nodeType string
	label    string
	parent   *cnode
	children []*cnode
}

func (c *cnode) ChildIndex() int {
	if c.parent == nil {
		return -1
	}
	return slices.Index(c.parent.children, c)
}

func (c *cnode) PostOrder() []*cnode {
	if c == nil {
		return nil
	}
	capEstimate := 16
	if c.orig != nil {
		capEstimate = c.orig.Size()
	}
	nodes := make([]*cnode, 0, capEstimate)
	var traverse func(n *cnode)
	traverse = func(n *cnode) {
		for _, ch := range n.children {
			traverse(ch)
		}
		nodes = append(nodes, n)
	}
	traverse(c)
	return nodes
}

type chawatheState struct {
	cpySrc  *cnode
	origDst *treesitter.ASTNode

	cpySrcToDst map[*cnode]*treesitter.ASTNode
	cpyDstToSrc map[*treesitter.ASTNode]*cnode

	origToCopy map[*treesitter.ASTNode]*cnode

	dstInOrder map[*treesitter.ASTNode]bool
	srcInOrder map[*cnode]bool

	script *EditScript
}

func (s *chawatheState) init(
	src, dst *treesitter.ASTNode,
	ms *engine.Mapping,
) {
	s.origDst = dst

	var size int
	if src != nil {
		size = src.Size()
	}
	s.origToCopy = make(map[*treesitter.ASTNode]*cnode, size)
	if src != nil {
		s.cpySrc = s.deepCopy(src, nil)
	}

	var pairCount int
	if ms != nil {
		pairCount = len(ms.Pairs)
	}
	s.cpySrcToDst = make(map[*cnode]*treesitter.ASTNode, pairCount)
	s.cpyDstToSrc = make(map[*treesitter.ASTNode]*cnode, pairCount)

	if ms != nil {
		for _, p := range ms.Pairs {
			if cpyNode, ok := s.origToCopy[p.Src]; ok {
				s.cpySrcToDst[cpyNode] = p.Dst
				s.cpyDstToSrc[p.Dst] = cpyNode
			}
		}
	}
}

func (s *chawatheState) deepCopy(n *treesitter.ASTNode, parent *cnode) *cnode {
	if n == nil {
		return nil
	}
	cn := &cnode{
		orig:     n,
		nodeType: n.Type,
		label:    n.Label,
		parent:   parent,
	}
	s.origToCopy[n] = cn
	if len(n.Children) > 0 {
		cn.children = make([]*cnode, 0, len(n.Children))
		for _, child := range n.Children {
			if cc := s.deepCopy(child, cn); cc != nil {
				cn.children = append(cn.children, cc)
			}
		}
	}
	return cn
}

func (s *chawatheState) generate() *EditScript {
	s.script = NewEditScript()
	s.dstInOrder = make(map[*treesitter.ASTNode]bool)
	s.srcInOrder = make(map[*cnode]bool)

	if s.origDst == nil {
		if s.cpySrc != nil {
			for _, w := range s.cpySrc.PostOrder() {
				s.script.Add(Action{
					Type: Delete,
					Node: w.orig,
				})
			}
		}
		return s.script
	}

	var srcFakeChildren []*cnode
	if s.cpySrc != nil {
		srcFakeChildren = []*cnode{s.cpySrc}
	}
	srcFakeRoot := &cnode{nodeType: fakeTreeType, children: srcFakeChildren}
	if s.cpySrc != nil {
		s.cpySrc.parent = srcFakeRoot
	}
	dstFakeRoot := &treesitter.ASTNode{Type: fakeTreeType, Children: []*treesitter.ASTNode{s.origDst}}

	s.cpySrcToDst[srcFakeRoot] = dstFakeRoot
	s.cpyDstToSrc[dstFakeRoot] = srcFakeRoot

	for _, x := range s.origDst.LevelOrder() {
		var w *cnode
		y := x.Parent
		var z *cnode
		if y != nil {
			z = s.cpyDstToSrc[y]
		}
		if z == nil {
			z = srcFakeRoot
		}

		if _, hasDst := s.cpyDstToSrc[x]; !hasDst {
			k := s.findPos(x, z)
			w = &cnode{
				nodeType: x.Type,
				label:    x.Label,
				orig:     x,
			}

			var parentOrig *treesitter.ASTNode
			if z != nil && z.orig != nil {
				parentOrig = z.orig
			}

			s.script.Add(Action{
				Type:     Insert,
				Node:     x,
				Parent:   parentOrig,
				Position: k,
			})

			s.cpySrcToDst[w] = x
			s.cpyDstToSrc[x] = w
			insertChild(z, w, k)
		} else {
			w = s.cpyDstToSrc[x]
			if x != s.origDst {
				v := w.parent

				if w.label != x.Label {
					s.script.Add(Action{
						Type:  Update,
						Node:  w.orig,
						Value: x.Label,
					})
					w.label = x.Label
				}

				if z != v {
					k := s.findPos(x, z)
					var parentOrig *treesitter.ASTNode
					if z != nil && z.orig != nil {
						parentOrig = z.orig
					}
					s.script.Add(Action{
						Type:     Move,
						Node:     w.orig,
						Parent:   parentOrig,
						Position: k,
						Subtree:  len(w.orig.Children) > 0,
					})

					insertChild(z, w, k)
				}
			}
		}

		s.srcInOrder[w] = true
		s.dstInOrder[x] = true
		s.alignChildren(w, x)
	}

	if s.cpySrc != nil {
		for _, w := range s.cpySrc.PostOrder() {
			if _, hasSrc := s.cpySrcToDst[w]; !hasSrc {
				s.script.Add(Action{
					Type: Delete,
					Node: w.orig,
				})
			}
		}
	}

	return s.script
}

// findPos computes the insertion position k for node x in target parent container z
// based on Chawathe et al. (1996, Section 4.1).
// It searches for the rightmost sibling v to the left of x in the destination tree
// whose partner u in the source working tree is already in-order and belongs to z.
// If such a partner u is found, x should follow u (upos + 1). If no such sibling exists
// (i.e. x is the leftmost child or preceding siblings are not yet aligned in z),
// it defaults to 0 (baseline leftmost child position), preserving container isolation.
func (s *chawatheState) findPos(x *treesitter.ASTNode, z *cnode) int {
	if x == nil || x.Parent == nil || z == nil {
		return 0
	}
	y := x.Parent
	siblings := y.Children

	xpos := x.ChildIndex()
	if xpos <= 0 {
		return 0
	}

	for i := xpos - 1; i >= 0; i-- {
		v := siblings[i]
		if !s.dstInOrder[v] {
			continue
		}
		u := s.cpyDstToSrc[v]
		if u != nil && u.parent == z && s.srcInOrder[u] {
			upos := u.ChildIndex()
			if upos != -1 {
				return upos + 1
			}
		}
	}
	return 0
}

// alignChildren computes the longest common subsequence (LCS) of matched children
// between source container w and destination container x, and generates Move actions
// for misaligned children per Chawathe et al. (1996, Section 4.1).
func (s *chawatheState) alignChildren(w *cnode, x *treesitter.ASTNode) {
	if w == nil || x == nil {
		return
	}

	for _, c := range w.children {
		delete(s.srcInOrder, c)
	}
	for _, c := range x.Children {
		delete(s.dstInOrder, c)
	}

	s1 := make([]*cnode, 0, len(w.children))
	for _, c := range w.children {
		if dst, ok := s.cpySrcToDst[c]; ok && dst != nil {
			if dst.Parent == x {
				s1 = append(s1, c)
			}
		}
	}

	s2 := make([]*treesitter.ASTNode, 0, len(x.Children))
	for _, c := range x.Children {
		if src, ok := s.cpyDstToSrc[c]; ok && src != nil {
			if src.parent == w {
				s2 = append(s2, c)
			}
		}
	}

	if x.IsUnordered || (w.orig != nil && w.orig.IsUnordered) {
		for _, b := range s2 {
			if a, ok := s.cpyDstToSrc[b]; ok {
				s.srcInOrder[a] = true
				s.dstInOrder[b] = true
			}
		}
		return
	}

	lcsPairs := s.lcs(s1, s2)

	lcsSet := make(map[*cnode]bool, len(lcsPairs))
	for _, pair := range lcsPairs {
		s.srcInOrder[pair.src] = true
		s.dstInOrder[pair.dst] = true
		lcsSet[pair.src] = true
	}

	for _, b := range s2 {
		if a, ok := s.cpyDstToSrc[b]; ok && a != nil && a.parent == w {
			if !lcsSet[a] {
				deleteChild(a.parent, a)

				k := s.findPos(b, w)
				var parentOrig *treesitter.ASTNode
				if w != nil && w.orig != nil {
					parentOrig = w.orig
				}
				s.script.Add(Action{
					Type:     Move,
					Node:     a.orig,
					Parent:   parentOrig,
					Position: k,
					Subtree:  len(a.orig.Children) > 0,
				})

				insertChild(w, a, k)

				s.srcInOrder[a] = true
				s.dstInOrder[b] = true
			}
		}
	}
}

type lcsPair struct {
	src *cnode
	dst *treesitter.ASTNode
}

func (s *chawatheState) lcs(
	x []*cnode,
	y []*treesitter.ASTNode,
) []lcsPair {
	m := len(x)
	n := len(y)
	if m == 0 || n == 0 {
		return nil
	}

	if m == 1 && n == 1 {
		if s.cpyDstToSrc[y[0]] == x[0] {
			return []lcsPair{{x[0], y[0]}}
		}
		return nil
	}

	var idxXBuf [32]int
	var idxYBuf [32]int
	// Stack buffers cover standard containers (<=32 children); larger ones fall back to heap.
	var idxX, idxY []int
	if m <= len(idxXBuf) {
		idxX = idxXBuf[:m]
	} else {
		idxX = make([]int, m)
	}
	for i, c := range x {
		idxX[i] = c.ChildIndex()
	}
	if n <= len(idxYBuf) {
		idxY = idxYBuf[:n]
	} else {
		idxY = make([]int, n)
	}
	for j, c := range y {
		idxY[j] = c.ChildIndex()
	}

	stride := n + 1
	totalCells := (m + 1) * stride

	var opt []int
	var stackBuf [256]int
	// Stack scratch covers (M+1)*(N+1) <= 256 cells; larger tables use heap.
	if totalCells <= len(stackBuf) {
		opt = stackBuf[:totalCells]
		clear(opt)
	} else {
		opt = make([]int, totalCells)
	}

	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if s.cpyDstToSrc[y[j]] == x[i] {
				// Integer scoring preserves the 1.0/1.01 ordering exactly; +1 favors stationary siblings.
				score := 100
				if idxX[i] != -1 && idxX[i] == idxY[j] {
					score = 101
				}
				opt[i*stride+j] = opt[(i+1)*stride+(j+1)] + score
			} else {
				opt[i*stride+j] = max(opt[(i+1)*stride+j], opt[i*stride+(j+1)])
			}
		}
	}

	pairs := make([]lcsPair, 0, min(m, n))
	i, j := 0, 0
	for i < m && j < n {
		if s.cpyDstToSrc[y[j]] == x[i] {
			pairs = append(pairs, lcsPair{x[i], y[j]})
			i++
			j++
		} else if opt[(i+1)*stride+j] >= opt[i*stride+(j+1)] {
			i++
		} else {
			j++
		}
	}

	return pairs
}

const fakeTreeType = "__fake_root__"

// insertChild inserts child into parent's children at index k, maintaining tree invariants
// and container isolation per Chawathe et al. (1996, Section 4.1).
// If child already belongs to a parent (including parent itself during reordering),
// it is detached first to prevent duplicate pointer references or corrupted sibling lists.
func insertChild(parent, child *cnode, k int) {
	if child == nil {
		return
	}
	if child.parent != nil {
		deleteChild(child.parent, child)
	}
	if parent == nil {
		return
	}
	child.parent = parent
	k = max(0, min(k, len(parent.children)))
	parent.children = slices.Insert(parent.children, k, child)
}

// deleteChild safely detaches child from parent's children slice, clearing child.parent
// and maintaining container isolation invariants per Chawathe et al. (1996, Section 4.1).
func deleteChild(parent, child *cnode) {
	if parent == nil || child == nil {
		return
	}
	idx := slices.Index(parent.children, child)
	if idx != -1 {
		parent.children = slices.Delete(parent.children, idx, idx+1)
		child.parent = nil
	}
}
