package actions

import (
	"slices"

	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func GenerateEditScript(
	src, dst *treesitter.ASTNode,
	ms *engine.Mapping,
) *EditScript {
	s := &chawatheState{}
	s.init(src, dst, ms)
	return s.generate()
}

// Chawathe Node, contain only necesssary attributes of ASTNode
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
	var nodes []*cnode
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

	size := src.Size()
	s.origToCopy = make(map[*treesitter.ASTNode]*cnode, size)
	s.cpySrc = s.deepCopy(src, nil)

	s.cpySrcToDst = make(map[*cnode]*treesitter.ASTNode, len(ms.Pairs))
	s.cpyDstToSrc = make(map[*treesitter.ASTNode]*cnode, len(ms.Pairs))

	for _, p := range ms.Pairs {
		if cpyNode, ok := s.origToCopy[p.Src]; ok {
			s.cpySrcToDst[cpyNode] = p.Dst
			s.cpyDstToSrc[p.Dst] = cpyNode
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
	srcFakeRoot := &cnode{nodeType: fakeTreeType, children: []*cnode{s.cpySrc}}
	s.cpySrc.parent = srcFakeRoot
	dstFakeRoot := &treesitter.ASTNode{Type: fakeTreeType, Children: []*treesitter.ASTNode{s.origDst}}

	s.script = NewEditScript()
	s.dstInOrder = make(map[*treesitter.ASTNode]bool)
	s.srcInOrder = make(map[*cnode]bool)

	s.cpySrcToDst[srcFakeRoot] = dstFakeRoot
	s.cpyDstToSrc[dstFakeRoot] = srcFakeRoot

	for _, x := range s.origDst.LevelOrder() {
		var w *cnode
		y := x.Parent
		z := s.cpyDstToSrc[y]

		if _, hasDst := s.cpyDstToSrc[x]; !hasDst {
			k := s.findPos(x)
			w = &cnode{nodeType: fakeTreeType, orig: x}

			s.script.Add(Action{
				Type:     Insert,
				Node:     x,
				Parent:   z.orig,
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
					k := s.findPos(x)
					s.script.Add(Action{
						Type:     Move,
						Node:     w.orig,
						Parent:   z.orig,
						Position: k,
					})

					s.addDescendantMoves(w)

					oldk := w.ChildIndex()
					if oldk >= 0 {
						w.parent.children = slices.Delete(w.parent.children, oldk, oldk+1)
					}
					insertChild(z, w, k)
				}
			}
		}

		s.srcInOrder[w] = true
		s.dstInOrder[x] = true
		s.alignChildren(w, x)
	}

	for _, w := range s.cpySrc.PostOrder() {
		if _, hasSrc := s.cpySrcToDst[w]; !hasSrc {
			s.script.Add(Action{
				Type: Delete,
				Node: w.orig,
			})
		}
	}

	return s.script
}

func (s *chawatheState) findPos(x *treesitter.ASTNode) int {
	y := x.Parent
	siblings := y.Children

	for _, c := range siblings {
		if s.dstInOrder[c] {
			if c == x {
				return 0
			}
			break
		}
	}

	xpos := x.ChildIndex()
	var v *treesitter.ASTNode
	for i := range xpos {
		c := siblings[i]
		if s.dstInOrder[c] {
			v = c
		}
	}

	if v == nil {
		return 0
	}

	u := s.cpyDstToSrc[v]
	upos := u.ChildIndex()
	return upos + 1
}

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

	var s1 []*cnode
	for _, c := range w.children {
		if dst, ok := s.cpySrcToDst[c]; ok {
			if dst.Parent == x {
				s1 = append(s1, c)
			}
		}
	}

	var s2 []*treesitter.ASTNode
	for _, c := range x.Children {
		if src, ok := s.cpyDstToSrc[c]; ok {
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

	lcsSet := make(map[*cnode]bool)
	for _, pair := range lcsPairs {
		s.srcInOrder[pair.src] = true
		s.dstInOrder[pair.dst] = true
		lcsSet[pair.src] = true
	}

	for _, b := range s2 {
		for _, a := range s1 {
			if src, ok := s.cpySrcToDst[a]; ok && src == b {
				if !lcsSet[a] {
					if idx := a.ChildIndex(); idx >= 0 {
						a.parent.children = slices.Delete(a.parent.children, idx, idx+1)
					}

					k := s.findPos(b)
					s.script.Add(Action{
						Type:     Move,
						Node:     a.orig,
						Parent:   w.orig,
						Position: k,
					})

					s.addDescendantMoves(a)

					insertChild(w, a, k)

					s.srcInOrder[a] = true
					s.dstInOrder[b] = true
				}
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

	stride := n + 1
	opt := make([]float64, (m+1)*stride)

	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if s.cpyDstToSrc[y[j]] == x[i] {
				score := 1.0
				if x[i].parent != nil && y[j].Parent != nil {
					idxX := slices.Index(x[i].parent.children, x[i])
					idxY := slices.Index(y[j].Parent.Children, y[j])
					if idxX == idxY && idxX != -1 {
						score += 0.01
					}
				}
				opt[i*stride+j] = opt[(i+1)*stride+(j+1)] + score
			} else {
				opt[i*stride+j] = max(opt[(i+1)*stride+j], opt[i*stride+(j+1)])
			}
		}
	}

	var pairs []lcsPair
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

func insertChild(parent, child *cnode, k int) {
	child.parent = parent
	k = max(0, min(k, len(parent.children)))
	parent.children = slices.Insert(parent.children, k, child)
}

func (s *chawatheState) addDescendantMoves(n *cnode) {
	var traverse func(curr *cnode)
	traverse = func(curr *cnode) {
		for _, child := range curr.children {
			if dst, ok := s.cpySrcToDst[child]; ok {
				pos := max(0, dst.ChildIndex())
				s.script.Add(Action{
					Type:     Move,
					Node:     child.orig,
					Parent:   dst.Parent,
					Position: pos,
				})
			}
			traverse(child)
		}
	}
	traverse(n)
}
