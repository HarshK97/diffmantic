package integration

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/postprocess"
	"github.com/HarshK97/diffmantic/internal/serialize"
)

// Run the full pipeline (parse -> match -> edit script -> postprocess -> serialize) across all fixtures.
func BenchmarkPipeline(b *testing.B) {
	for _, name := range allFixtures(b) {
		f := loadFixture(b, name)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				astA := mustParse(b, f.OldSrc, f.OldPath)
				astB := mustParse(b, f.NewSrc, f.NewPath)

				result := engine.Match(astA, astB)
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

// Time AST matching. We re-parse on each iteration because Match mutates internal state.
func BenchmarkMatch(b *testing.B) {
	for _, name := range allFixtures(b) {
		f := loadFixture(b, name)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				astA := mustParse(b, f.OldSrc, f.OldPath)
				astB := mustParse(b, f.NewSrc, f.NewPath)
				engine.Match(astA, astB)
			}
		})
	}
}

// Time edit script generation (Chawathe algorithm).
func BenchmarkEditScript(b *testing.B) {
	for _, name := range allFixtures(b) {
		f := loadFixture(b, name)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				astA := mustParse(b, f.OldSrc, f.OldPath)
				astB := mustParse(b, f.NewSrc, f.NewPath)
				result := engine.Match(astA, astB)
				actions.GenerateEditScript(astA, astB, result.Mappings)
			}
		})
	}
}

// Time postprocessing (collapsing, punctuation filtering, and action grouping).
func BenchmarkPostprocess(b *testing.B) {
	for _, name := range allFixtures(b) {
		f := loadFixture(b, name)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				astA := mustParse(b, f.OldSrc, f.OldPath)
				astB := mustParse(b, f.NewSrc, f.NewPath)
				result := engine.Match(astA, astB)
				es := actions.GenerateEditScript(astA, astB, result.Mappings)
				postprocess.Run(es, result.Mappings, astA, astB)
			}
		})
	}
}

// Time JSON envelope creation and marshalling.
func BenchmarkSerialize(b *testing.B) {
	for _, name := range allFixtures(b) {
		f := loadFixture(b, name)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				astA := mustParse(b, f.OldSrc, f.OldPath)
				astB := mustParse(b, f.NewSrc, f.NewPath)
				result := engine.Match(astA, astB)
				es := actions.GenerateEditScript(astA, astB, result.Mappings)
				es = postprocess.Run(es, result.Mappings, astA, astB)
				if _, err := serialize.Marshal(es, result.Mappings, astA, astB, f.OldSrc, f.NewSrc); err != nil {
					b.Fatalf("serializing: %v", err)
				}
			}
		})
	}
}
