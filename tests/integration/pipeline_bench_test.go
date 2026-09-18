package integration

import (
	"encoding/json"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/pipeline"
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
				res, err := pipeline.Run(f.OldSrc, f.NewSrc, f.OldPath, f.NewPath, pipeline.DiffOptions{
					DisableErrorFallback: true,
					DisableSizeLimit:     true,
					EnvelopeOpts: serialize.EnvelopeOptions{
						IncludeActions:    true,
						IncludeAlignment:  true,
						IncludeHighlights: true,
					},
				})
				if err != nil {
					b.Fatalf("pipeline run failed: %v", err)
				}
				if _, err := json.Marshal(res.Envelope); err != nil {
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

// Time AST matching on parsed AST pairs.
func BenchmarkMatch(b *testing.B) {
	for _, name := range allFixtures(b) {
		f := loadFixture(b, name)
		b.Run(name, func(b *testing.B) {
			baseA := mustParse(b, f.OldSrc, f.OldPath)
			baseB := mustParse(b, f.NewSrc, f.NewPath)

			b.ReportAllocs()
			for b.Loop() {
				engine.Match(baseA, baseB, f.OldSrc, f.NewSrc, nil)
			}
		})
	}
}

// Time edit script generation (Chawathe algorithm) on pre-matched AST pairs.
func BenchmarkEditScript(b *testing.B) {
	for _, name := range allFixtures(b) {
		f := loadFixture(b, name)
		b.Run(name, func(b *testing.B) {
			baseA := mustParse(b, f.OldSrc, f.OldPath)
			baseB := mustParse(b, f.NewSrc, f.NewPath)
			baseRes := engine.Match(baseA, baseB, f.OldSrc, f.NewSrc, nil)

			b.ReportAllocs()
			for b.Loop() {
				actions.GenerateEditScript(baseA, baseB, baseRes.Mappings)
			}
		})
	}
}

// Time postprocessing on pre-generated edit scripts and isolated mappings.
func BenchmarkPostprocess(b *testing.B) {
	for _, name := range allFixtures(b) {
		f := loadFixture(b, name)
		b.Run(name, func(b *testing.B) {
			baseA := mustParse(b, f.OldSrc, f.OldPath)
			baseB := mustParse(b, f.NewSrc, f.NewPath)
			baseRes := engine.Match(baseA, baseB, f.OldSrc, f.NewSrc, nil)
			baseES := actions.GenerateEditScript(baseA, baseB, baseRes.Mappings)

			b.ReportAllocs()
			for b.Loop() {
				ms := baseRes.Mappings.Clone()
				postprocess.Run(baseES, ms, baseA, baseB)
			}
		})
	}
}

// Time JSON envelope creation and marshalling on pre-postprocessed edit scripts.
func BenchmarkSerialize(b *testing.B) {
	modes := []struct {
		name string
		opts serialize.EnvelopeOptions
	}{
		{"ActionsOnly", serialize.EnvelopeOptions{IncludeActions: true}},
		{"UIMode", serialize.EnvelopeOptions{IncludeAlignment: true, IncludeHighlights: true}},
		{"FullMode", serialize.EnvelopeOptions{IncludeActions: true, IncludeAlignment: true, IncludeHighlights: true}},
	}

	for _, name := range allFixtures(b) {
		f := loadFixture(b, name)
		b.Run(name, func(b *testing.B) {
			astA := mustParse(b, f.OldSrc, f.OldPath)
			astB := mustParse(b, f.NewSrc, f.NewPath)
			res := engine.Match(astA, astB, f.OldSrc, f.NewSrc, nil)
			es := actions.GenerateEditScript(astA, astB, res.Mappings)
			es = postprocess.Run(es, res.Mappings, astA, astB)

			for _, m := range modes {
				b.Run(m.name, func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						if _, err := serialize.MarshalWithOptions(es, res.Mappings, astA, astB, f.OldSrc, f.NewSrc, m.opts); err != nil {
							b.Fatalf("serializing: %v", err)
						}
					}
				})
			}
		})
	}
}
