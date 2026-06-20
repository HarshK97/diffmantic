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

type chawatheState struct {
	origSrc *treesitter.ASTNode
	cpySrc  *treesitter.ASTNode
	origDst *treesitter.ASTNode

	origMappings *engine.Mapping
	cpyMappings  *engine.Mapping

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
}

func (s *chawatheState) generate() *EditScript {
	srcFakeRoot := newFakeTree(s.cpySrc)
	dstFakeRoot := newFakeTree(s.origDst)

	s.script = NewEditScript()
	s.dstInOrder = make(map[*treesitter.ASTNode]bool)
	s.srcInOrder = make(map[*treesitter.ASTNode]bool)

	s.cpyMappings.Add(srcFakeRoot, dstFakeRoot)

	for _, x := range bfs(s.origDst) {
		var w *treesitter.ASTNode
		y := x.Parent
		z := s.cpyMappings.Dst()[y]

		if !s.cpyMappings.HasDst(x) {
			k := s.findPos(x)
			w = newEmptyFakeTree()

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
			w = s.cpyMappings.Dst()[x]
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

					oldk := slices.Index(w.Parent.Children, w)
					if oldk >= 0 {
						w.Parent.Children = append(
							w.Parent.Children[:oldk],
							w.Parent.Children[oldk+1:]...,
						)
					}
					insertChild(z, w, k)
				}
			}
		}

		s.srcInOrder[w] = true
		s.dstInOrder[x] = true
		s.alignChildren(w, x)
	}

	for _, w := range engine.PostOrder(s.cpySrc) {
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

	xpos := slices.Index(siblings, x)
	var v *treesitter.ASTNode
	for i := 0; i < xpos; i++ {
		c := siblings[i]
		if s.dstInOrder[c] {
			v = c
		}
	}

	if v == nil {
		return 0
	}

	u := s.cpyMappings.Dst()[v]
	upos := slices.Index(u.Parent.Children, u)
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
		if s.cpyMappings.Has(c) {
			dst := s.cpyMappings.Src()[c]
			if slices.Contains(x.Children, dst) {
				s1 = append(s1, c)
			}
		}
	}

	var s2 []*treesitter.ASTNode
	for _, c := range x.Children {
		if s.cpyMappings.HasDst(c) {
			src := s.cpyMappings.Dst()[c]
			if slices.Contains(w.Children, src) {
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
			if s.cpyMappings.Has(a) && s.cpyMappings.Src()[a] == b {
				if !lcsSet[a] {
					if idx := slices.Index(a.Parent.Children, a); idx != -1 {
						a.Parent.Children = slices.Delete(a.Parent.Children, idx, idx+1)
					}

					k := s.findPos(b)
					s.script.Add(Action{
						Type:     Move,
						Node:     s.copyToOrig[a],
						Parent:   s.copyToOrig[w],
						Position: k,
					})
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

	opt := make([][]int, m+1)
	for i := range opt {
		opt[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if s.cpyMappings.Dst()[y[j]] == x[i] {
				opt[i][j] = opt[i+1][j+1] + 1
			} else {
				opt[i][j] = max(opt[i+1][j], opt[i][j+1])
			}
		}
	}

	var pairs [][2]*treesitter.ASTNode
	i, j := 0, 0
	for i < m && j < n {
		if s.cpyMappings.Dst()[y[j]] == x[i] {
			pairs = append(pairs, [2]*treesitter.ASTNode{x[i], y[j]})
			i++
			j++
		} else if opt[i+1][j] >= opt[i][j+1] {
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
	o2c := make(map[*treesitter.ASTNode]*treesitter.ASTNode)
	c2o := make(map[*treesitter.ASTNode]*treesitter.ASTNode)
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

func newEmptyFakeTree() *treesitter.ASTNode {
	return &treesitter.ASTNode{Type: fakeTreeType}
}

func insertChild(parent, child *treesitter.ASTNode, k int) {
	child.Parent = parent
	k = max(0, min(k, len(parent.Children)))
	parent.Children = slices.Insert(parent.Children, k, child)
}

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
