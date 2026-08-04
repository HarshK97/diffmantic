package integration

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/postprocess"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func cloneAST(n *treesitter.ASTNode) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	cp := *n
	if len(n.Children) > 0 {
		cp.Children = make([]*treesitter.ASTNode, len(n.Children))
		for i, child := range n.Children {
			cpChild := cloneAST(child)
			cpChild.Parent = &cp
			cp.Children[i] = cpChild
		}
	}
	return &cp
}

func cloneASTPairAndMapping(baseA, baseB *treesitter.ASTNode, baseMs *engine.Mapping) (*treesitter.ASTNode, *treesitter.ASTNode, *engine.Mapping, map[*treesitter.ASTNode]*treesitter.ASTNode, map[*treesitter.ASTNode]*treesitter.ASTNode) {
	srcMap := make(map[*treesitter.ASTNode]*treesitter.ASTNode)
	dstMap := make(map[*treesitter.ASTNode]*treesitter.ASTNode)

	var cloneWithMap func(n *treesitter.ASTNode, nodeMap map[*treesitter.ASTNode]*treesitter.ASTNode) *treesitter.ASTNode
	cloneWithMap = func(n *treesitter.ASTNode, nodeMap map[*treesitter.ASTNode]*treesitter.ASTNode) *treesitter.ASTNode {
		if n == nil {
			return nil
		}
		cp := *n
		nodeMap[n] = &cp
		if len(n.Children) > 0 {
			cp.Children = make([]*treesitter.ASTNode, len(n.Children))
			for i, child := range n.Children {
				cpChild := cloneWithMap(child, nodeMap)
				cpChild.Parent = &cp
				cp.Children[i] = cpChild
			}
		}
		return &cp
	}

	astA := cloneWithMap(baseA, srcMap)
	astB := cloneWithMap(baseB, dstMap)

	ms := engine.NewMapping()
	if baseMs != nil {
		for _, pair := range baseMs.Pairs {
			newSrc := srcMap[pair.Src]
			newDst := dstMap[pair.Dst]
			if newSrc != nil && newDst != nil {
				ms.Add(newSrc, newDst)
			}
		}
	}
	return astA, astB, ms, srcMap, dstMap
}

func cloneEditScript(es *actions.EditScript, srcMap, dstMap map[*treesitter.ASTNode]*treesitter.ASTNode) *actions.EditScript {
	if es == nil {
		return nil
	}
	cp := actions.NewEditScript()
	for _, a := range es.Actions() {
		node := a.Node
		if mapped, ok := srcMap[node]; ok {
			node = mapped
		} else if mapped, ok := dstMap[node]; ok {
			node = mapped
		}

		parent := a.Parent
		if mapped, ok := srcMap[parent]; ok {
			parent = mapped
		} else if mapped, ok := dstMap[parent]; ok {
			parent = mapped
		}

		ca := a
		ca.Node = node
		ca.Parent = parent
		cp.Add(ca)
	}
	return cp
}

// Run the full pipeline (parse -> match -> edit script -> postprocess -> serialize) across all fixtures.
func BenchmarkPipeline(b *testing.B) {
	for _, name := range allFixtures(b) {
		f := loadFixture(b, name)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				astA := mustParse(b, f.OldSrc, f.OldPath)
				astB := mustParse(b, f.NewSrc, f.NewPath)

				result := engine.Match(astA, astB, f.OldSrc, f.NewSrc)
				es := actions.GenerateEditScript(astA, astB, result.Mappings)
				es = postprocess.Run(es, result.Mappings, astA, astB)

				if _, err := serialize.Marshal(es, result.Mappings, astA, astB, f.OldSrc, f.NewSrc); err != nil {
					b.Fatalf("serializing: %v", err)
				}
			}
		})
	}
}

// Time tree-sitter parsing for old and new files.
func BenchmarkParse(b *testing.B) {
	for _, name := range allFixtures(b) {
		f := loadFixture(b, name)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				mustParse(b, f.OldSrc, f.OldPath)
				mustParse(b, f.NewSrc, f.NewPath)
			}
		})
	}
}

// Time AST matching on fresh, unmutated AST pairs (StopTimer pauses during lightweight AST cloning).
func BenchmarkMatch(b *testing.B) {
	for _, name := range allFixtures(b) {
		f := loadFixture(b, name)
		b.Run(name, func(b *testing.B) {
			baseA := mustParse(b, f.OldSrc, f.OldPath)
			baseB := mustParse(b, f.NewSrc, f.NewPath)

			b.ReportAllocs()
			for b.Loop() {
				b.StopTimer()
				astA := cloneAST(baseA)
				astB := cloneAST(baseB)
				b.StartTimer()

				engine.Match(astA, astB, f.OldSrc, f.NewSrc)
			}
		})
	}
}

// Time edit script generation (Chawathe algorithm) on fresh pre-matched AST pairs and mapped node pointers.
func BenchmarkEditScript(b *testing.B) {
	for _, name := range allFixtures(b) {
		f := loadFixture(b, name)
		b.Run(name, func(b *testing.B) {
			baseA := mustParse(b, f.OldSrc, f.OldPath)
			baseB := mustParse(b, f.NewSrc, f.NewPath)
			baseRes := engine.Match(baseA, baseB, f.OldSrc, f.NewSrc)

			b.ReportAllocs()
			for b.Loop() {
				b.StopTimer()
				astA, astB, ms, _, _ := cloneASTPairAndMapping(baseA, baseB, baseRes.Mappings)
				b.StartTimer()

				actions.GenerateEditScript(astA, astB, ms)
			}
		})
	}
}

// Time postprocessing on fresh pre-generated edit scripts and mapped node pointers.
func BenchmarkPostprocess(b *testing.B) {
	for _, name := range allFixtures(b) {
		f := loadFixture(b, name)
		b.Run(name, func(b *testing.B) {
			baseA := mustParse(b, f.OldSrc, f.OldPath)
			baseB := mustParse(b, f.NewSrc, f.NewPath)
			baseRes := engine.Match(baseA, baseB, f.OldSrc, f.NewSrc)
			baseES := actions.GenerateEditScript(baseA, baseB, baseRes.Mappings)

			b.ReportAllocs()
			for b.Loop() {
				b.StopTimer()
				astA, astB, ms, srcMap, dstMap := cloneASTPairAndMapping(baseA, baseB, baseRes.Mappings)
				es := cloneEditScript(baseES, srcMap, dstMap)
				b.StartTimer()

				postprocess.Run(es, ms, astA, astB)
			}
		})
	}
}

// Time JSON envelope creation and marshalling on pre-postprocessed edit scripts.
func BenchmarkSerialize(b *testing.B) {
	for _, name := range allFixtures(b) {
		f := loadFixture(b, name)
		b.Run(name, func(b *testing.B) {
			astA := mustParse(b, f.OldSrc, f.OldPath)
			astB := mustParse(b, f.NewSrc, f.NewPath)
			res := engine.Match(astA, astB, f.OldSrc, f.NewSrc)
			es := actions.GenerateEditScript(astA, astB, res.Mappings)
			es = postprocess.Run(es, res.Mappings, astA, astB)

			b.ReportAllocs()
			for b.Loop() {
				if _, err := serialize.Marshal(es, res.Mappings, astA, astB, f.OldSrc, f.NewSrc); err != nil {
					b.Fatalf("serializing: %v", err)
				}
			}
		})
	}
}
