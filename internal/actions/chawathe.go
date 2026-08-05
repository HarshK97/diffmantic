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
	defer func() {
		if dst != nil {
			dst.Parent = nil
		}
	}()
	return s.generate()
}

type chawatheState struct {
	origSrc *treesitter.ASTNode
	cpySrc  *treesitter.ASTNode
	origDst *treesitter.ASTNode

	origMappings *engine.Mapping
	cpyMappings  *engine.Mapping

	cpySrcToDst map[*treesitter.ASTNode]*treesitter.ASTNode
	cpyDstToSrc map[*treesitter.ASTNode]*treesitter.ASTNode

	origToCopy map[*treesitter.ASTNode]*treesitter.ASTNode
	copyToOrig map[*treesitter.ASTNode]*treesitter.ASTNode

	dstInOrder map[*treesitter.ASTNode]bool
	srcInOrder map[*treesitter.ASTNode]bool

	script *EditScript
}

func (s *chawatheState) init(
	src, dst *treesitter.ASTNode,
	ms *engine.Mapping,
) {
	s.origSrc = src
	s.origDst = dst
	s.origMappings = ms

	cr := deepCopyTree(src)
	s.cpySrc = cr.root
	s.origToCopy = cr.origToCopy
	s.copyToOrig = cr.copyToOrig

	s.cpyMappings = engine.NewMapping()
	for _, p := range ms.Pairs {
		if cpyNode, ok := s.origToCopy[p.Src]; ok {
			s.cpyMappings.Add(cpyNode, p.Dst)
		}
	}
	s.cpySrcToDst = s.cpyMappings.Src()
	s.cpyDstToSrc = s.cpyMappings.Dst()
}

func (s *chawatheState) generate() *EditScript {
	srcFakeRoot := newFakeTree(s.cpySrc)
	dstFakeRoot := newFakeTree(s.origDst)

	s.script = NewEditScript()
	s.dstInOrder = make(map[*treesitter.ASTNode]bool)
	s.srcInOrder = make(map[*treesitter.ASTNode]bool)

	s.cpyMappings.Add(srcFakeRoot, dstFakeRoot)

	for _, x := range s.origDst.LevelOrder() {
		var w *treesitter.ASTNode
		y := x.Parent
		z := s.cpyDstToSrc[y]

		if !s.cpyMappings.HasDst(x) {
			k := s.findPos(x)
			w = &treesitter.ASTNode{Type: fakeTreeType}

			s.script.Add(Action{
				Type:     Insert,
				Node:     x,
				Parent:   s.copyToOrig[z],
				Position: k,
			})

			s.copyToOrig[w] = x
			s.cpyMappings.Add(w, x)
			insertChild(z, w, k)
		} else {
			w = s.cpyDstToSrc[x]
			if x != s.origDst {
				v := w.Parent

				if w.Label != x.Label {
					s.script.Add(Action{
						Type:  Update,
						Node:  s.copyToOrig[w],
						Value: x.Label,
					})
					w.Label = x.Label
				}

				if z != v {
					k := s.findPos(x)
					s.script.Add(Action{
						Type:     Move,
						Node:     s.copyToOrig[w],
						Parent:   s.copyToOrig[z],
						Position: k,
					})

					s.addDescendantMoves(w)

					oldk := w.ChildIndex()
					if oldk >= 0 {
						w.Parent.Children = slices.Delete(w.Parent.Children, oldk, oldk+1)
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
		if !s.cpyMappings.Has(w) {
			s.script.Add(Action{
				Type: Delete,
				Node: s.copyToOrig[w],
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

func (s *chawatheState) alignChildren(w, x *treesitter.ASTNode) {
	for _, c := range w.Children {
		delete(s.srcInOrder, c)
	}
	for _, c := range x.Children {
		delete(s.dstInOrder, c)
	}

	var s1 []*treesitter.ASTNode
	for _, c := range w.Children {
		if dst, ok := s.cpySrcToDst[c]; ok {
			if dst.Parent == x {
				s1 = append(s1, c)
			}
		}
	}

	var s2 []*treesitter.ASTNode
	for _, c := range x.Children {
		if src, ok := s.cpyDstToSrc[c]; ok {
			if src.Parent == w {
				s2 = append(s2, c)
			}
		}
	}

	lcsPairs := s.lcs(s1, s2)

	lcsSet := make(map[*treesitter.ASTNode]bool)
	for _, pair := range lcsPairs {
		s.srcInOrder[pair[0]] = true
		s.dstInOrder[pair[1]] = true
		lcsSet[pair[0]] = true
	}

	for _, b := range s2 {
		for _, a := range s1 {
			if src, ok := s.cpySrcToDst[a]; ok && src == b {
				if !lcsSet[a] {
					if idx := a.ChildIndex(); idx >= 0 {
						a.Parent.Children = slices.Delete(a.Parent.Children, idx, idx+1)
					}

					k := s.findPos(b)
					s.script.Add(Action{
						Type:     Move,
						Node:     s.copyToOrig[a],
						Parent:   s.copyToOrig[w],
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

func (s *chawatheState) lcs(
	x []*treesitter.ASTNode,
	y []*treesitter.ASTNode,
) [][2]*treesitter.ASTNode {
	m := len(x)
	n := len(y)
	if m == 0 || n == 0 {
		return nil
	}

	if m == 1 && n == 1 {
		if s.cpyDstToSrc[y[0]] == x[0] {
			return [][2]*treesitter.ASTNode{{x[0], y[0]}}
		}
		return nil
	}

	stride := n + 1
	opt := make([]int, (m+1)*stride)

	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if s.cpyDstToSrc[y[j]] == x[i] {
				opt[i*stride+j] = opt[(i+1)*stride+(j+1)] + 1
			} else {
				opt[i*stride+j] = max(opt[(i+1)*stride+j], opt[i*stride+(j+1)])
			}
		}
	}

	var pairs [][2]*treesitter.ASTNode
	i, j := 0, 0
	for i < m && j < n {
		if s.cpyDstToSrc[y[j]] == x[i] {
			pairs = append(pairs, [2]*treesitter.ASTNode{x[i], y[j]})
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

type copyResult struct {
	root       *treesitter.ASTNode
	origToCopy map[*treesitter.ASTNode]*treesitter.ASTNode
	copyToOrig map[*treesitter.ASTNode]*treesitter.ASTNode
}

func deepCopyTree(n *treesitter.ASTNode) *copyResult {
	size := n.Size()
	o2c := make(map[*treesitter.ASTNode]*treesitter.ASTNode, size)
	c2o := make(map[*treesitter.ASTNode]*treesitter.ASTNode, size)
	root := deepCopyNode(n, nil, o2c, c2o)
	return &copyResult{root: root, origToCopy: o2c, copyToOrig: c2o}
}

func deepCopyNode(
	n, parent *treesitter.ASTNode,
	o2c, c2o map[*treesitter.ASTNode]*treesitter.ASTNode,
) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	cp := &treesitter.ASTNode{
		Type:      n.Type,
		Label:     n.Label,
		Parent:    parent,
		StartByte: n.StartByte,
		EndByte:   n.EndByte,
		StartRow:  n.StartRow,
		StartCol:  n.StartCol,
		EndRow:    n.EndRow,
		EndCol:    n.EndCol,
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

func newFakeTree(child *treesitter.ASTNode) *treesitter.ASTNode {
	fake := &treesitter.ASTNode{Type: fakeTreeType}
	if child != nil {
		fake.Children = []*treesitter.ASTNode{child}
		child.Parent = fake
	}
	return fake
}

func insertChild(parent, child *treesitter.ASTNode, k int) {
	child.Parent = parent
	k = max(0, min(k, len(parent.Children)))
	parent.Children = slices.Insert(parent.Children, k, child)
}

func (s *chawatheState) addDescendantMoves(n *treesitter.ASTNode) {
	var traverse func(curr *treesitter.ASTNode)
	traverse = func(curr *treesitter.ASTNode) {
		for _, child := range curr.Children {
			if dst, ok := s.cpySrcToDst[child]; ok {
				pos := max(0, dst.ChildIndex())
				s.script.Add(Action{
					Type:     Move,
					Node:     s.copyToOrig[child],
					Parent:   dst.Parent,
					Position: pos,
				})
			}
			traverse(child)
		}
	}
	traverse(n)
}
