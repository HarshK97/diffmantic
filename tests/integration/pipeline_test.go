// Package integration runs integration tests on the diffmantic engine pipeline.
// These tests exercise the full flow: parse -> match -> edit script -> postprocess -> serialize.
//
// Run with:
//
//	go test ./tests/integration/ -v
//	go test ./tests/integration/ -run TestPipeline/go_comment_update -v
//	go test ./tests/integration/ -v -update   # regenerate golden files
package integration

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/postprocess"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

var update = flag.Bool("update", false, "update the golden files")

// testdataDir returns the absolute path to tests/testdata/.
func testdataDir(t *testing.T) string {
	t.Helper()
	// pipeline_test.go is two levels below repo root; testdata is in tests/
	dir, err := filepath.Abs(filepath.Join("..", "testdata"))
	if err != nil {
		t.Fatalf("resolving testdata dir: %v", err)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatalf("testdata dir does not exist: %s", dir)
	}
	return dir
}

// fixture holds the paths and contents for a test case.
type fixture struct {
	Name    string
	OldPath string
	NewPath string
	OldSrc  []byte
	NewSrc  []byte
}

// loadFixture reads the old and new source files for a fixture.
func loadFixture(t *testing.T, name string) fixture {
	t.Helper()
	dir := filepath.Join(testdataDir(t), name)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading fixture dir %s: %v", name, err)
	}

	var oldPath, newPath string
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, "old.") {
			oldPath = filepath.Join(dir, n)
		} else if strings.HasPrefix(n, "new.") {
			newPath = filepath.Join(dir, n)
		}
	}
	if oldPath == "" || newPath == "" {
		t.Fatalf("fixture %s: missing old.* or new.* file", name)
	}

	oldSrc, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatalf("reading %s: %v", oldPath, err)
	}
	newSrc, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("reading %s: %v", newPath, err)
	}

	return fixture{
		Name:    name,
		OldPath: oldPath,
		NewPath: newPath,
		OldSrc:  oldSrc,
		NewSrc:  newSrc,
	}
}

// pipelineResult holds the output of a full engine run.
type pipelineResult struct {
	AstA     *treesitter.ASTNode
	AstB     *treesitter.ASTNode
	Mappings *engine.Mapping
	ES       *actions.EditScript
	JSON     []byte
}

// runPipeline runs the entire diffmantic pipeline on a fixture.
func runPipeline(t *testing.T, f fixture) pipelineResult {
	t.Helper()

	astA, err := treesitter.Parse(f.OldSrc, f.OldPath)
	if err != nil {
		t.Fatalf("parsing %s: %v", f.OldPath, err)
	}
	astB, err := treesitter.Parse(f.NewSrc, f.NewPath)
	if err != nil {
		t.Fatalf("parsing %s: %v", f.NewPath, err)
	}

	result := engine.Match(astA, astB)
	es := actions.GenerateEditScript(astA, astB, result.Mappings)
	es = postprocess.Run(es, result.Mappings, astA, astB)

	jsonData, err := serialize.Marshal(es, result.Mappings, astA, astB, f.OldSrc, f.NewSrc)
	if err != nil {
		t.Fatalf("serializing JSON: %v", err)
	}

	return pipelineResult{
		AstA:     astA,
		AstB:     astB,
		Mappings: result.Mappings,
		ES:       es,
		JSON:     jsonData,
	}
}

// allFixtures returns the names of all fixture directories in testdata/.
func allFixtures(t *testing.T) []string {
	t.Helper()
	dir := testdataDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading testdata dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			// Make sure the directory contains both old and new files.
			subEntries, err := os.ReadDir(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			hasOld, hasNew := false, false
			for _, se := range subEntries {
				if strings.HasPrefix(se.Name(), "old.") {
					hasOld = true
				}
				if strings.HasPrefix(se.Name(), "new.") {
					hasNew = true
				}
			}
			if hasOld && hasNew {
				names = append(names, e.Name())
			}
		}
	}
	return names
}

// --------------------------------------------------------------------------
// Integration tests
// --------------------------------------------------------------------------

// TestPipeline runs the full engine pipeline on every fixture and validates structural invariants.
func TestPipeline(t *testing.T) {
	fixtures := allFixtures(t)
	if len(fixtures) == 0 {
		t.Fatal("no fixtures found in testdata/")
	}

	for _, name := range fixtures {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := loadFixture(t, name)
			result := runPipeline(t, f)

			// Make sure the output is valid JSON.
			var envelope serialize.Envelope
			if err := json.Unmarshal(result.JSON, &envelope); err != nil {
				t.Fatalf("output is not valid JSON: %v", err)
			}

			// Make sure the version field is set.
			if envelope.Version == "" {
				t.Error("JSON envelope missing version field")
			}

			// Make sure every action type is valid.
			validActions := map[string]bool{
				"insert": true, "delete": true,
				"update": true, "move": true,
			}
			for i, a := range envelope.Actions {
				if !validActions[a.Action] {
					t.Errorf("action[%d] has invalid type %q", i, a.Action)
				}
			}

			// Make sure node references point to a valid tree.
			for i, a := range envelope.Actions {
				if a.Node != nil {
					if a.Node.Tree != "before" && a.Node.Tree != "after" {
						t.Errorf("action[%d].node.tree = %q, want before|after", i, a.Node.Tree)
					}
				}
			}

			// Compare results. output against the golden file.
			goldenPath := filepath.Join(testdataDir(t), name, "expected.json")
			if *update {
				// Pretty-print the JSON for readable diffs.
				pretty, err := json.MarshalIndent(envelope, "", "  ")
				if err != nil {
					t.Fatalf("pretty-printing JSON: %v", err)
				}
				if err := os.WriteFile(goldenPath, pretty, 0644); err != nil {
					t.Fatalf("writing golden file: %v", err)
				}
				t.Logf("updated golden file: %s", goldenPath)
				return
			}

			golden, err := os.ReadFile(goldenPath)
			if os.IsNotExist(err) {
				t.Logf("golden file not found: %s (run with -update to create)", goldenPath)
				return
			}
			if err != nil {
				t.Fatalf("reading golden file: %v", err)
			}

			// Parse both to normalize whitespace for comparison.
			var expected serialize.Envelope
			if err := json.Unmarshal(golden, &expected); err != nil {
				t.Fatalf("parsing golden file: %v", err)
			}

			gotPretty, _ := json.MarshalIndent(envelope, "", "  ")
			expPretty, _ := json.MarshalIndent(expected, "", "  ")

			if string(gotPretty) != string(expPretty) {
				t.Errorf("output differs from golden file.\nRun with -update to regenerate.\nGot %d actions, expected %d actions",
					len(envelope.Actions), len(expected.Actions))
			}
		})
	}
}

// TestPipelineIdenticalFiles checks that diffing a file against itself returns zero actions.
func TestPipelineIdenticalFiles(t *testing.T) {
	fixtures := allFixtures(t)
	if len(fixtures) == 0 {
		t.Skip("no fixtures")
	}

	// Use the first fixture's old file on both sides.
	f := loadFixture(t, fixtures[0])
	f.NewPath = f.OldPath
	f.NewSrc = f.OldSrc

	result := runPipeline(t, f)

	var envelope serialize.Envelope
	if err := json.Unmarshal(result.JSON, &envelope); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(envelope.Actions) != 0 {
		t.Errorf("identical files should produce 0 actions, got %d", len(envelope.Actions))
		for i, a := range envelope.Actions {
			t.Logf("  action[%d]: %s %s", i, a.Action, a.Node.Type)
		}
	}
}

// TestPipelineEmptyOld checks that diffing an empty file against a real file returns only insert actions.
func TestPipelineEmptyOld(t *testing.T) {
	fixtures := allFixtures(t)
	if len(fixtures) == 0 {
		t.Skip("no fixtures")
	}

	f := loadFixture(t, fixtures[0])
	// Create a minimal empty file for the old side.
	ext := filepath.Ext(f.OldPath)
	emptyFile := filepath.Join(t.TempDir(), "empty"+ext)
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatalf("failed to write empty file: %v", err)
	}

	f.OldPath = emptyFile
	f.OldSrc = []byte{}

	result := runPipeline(t, f)

	var envelope serialize.Envelope
	if err := json.Unmarshal(result.JSON, &envelope); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// With an empty old file, all actions should be inserts.
	for i, a := range envelope.Actions {
		if a.Action != "insert" {
			t.Errorf("action[%d] = %q, want insert (old file is empty)", i, a.Action)
		}
	}
}

// TestPipelineEmptyNew checks that diffing a real file against an empty file returns only delete actions.
func TestPipelineEmptyNew(t *testing.T) {
	fixtures := allFixtures(t)
	if len(fixtures) == 0 {
		t.Skip("no fixtures")
	}

	f := loadFixture(t, fixtures[0])
	ext := filepath.Ext(f.NewPath)
	emptyFile := filepath.Join(t.TempDir(), "empty"+ext)
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatalf("failed to write empty file: %v", err)
	}

	f.NewPath = emptyFile
	f.NewSrc = []byte{}

	result := runPipeline(t, f)

	var envelope serialize.Envelope
	if err := json.Unmarshal(result.JSON, &envelope); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	for i, a := range envelope.Actions {
		if a.Action != "delete" {
			t.Errorf("action[%d] = %q, want delete (new file is empty)", i, a.Action)
		}
	}
}

// TestRoundTripMarshalUnmarshal checks that Marshal followed by Unmarshal is lossless.
func TestRoundTripMarshalUnmarshal(t *testing.T) {
	for _, name := range allFixtures(t) {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := loadFixture(t, name)
			result := runPipeline(t, f)

			// Marshal first.
			var original serialize.Envelope
			if err := json.Unmarshal(result.JSON, &original); err != nil {
				t.Fatalf("unmarshal original: %v", err)
			}

			// Re-marshal the parsed envelope.
			reMarshalled, err := json.MarshalIndent(original, "", "  ")
			if err != nil {
				t.Fatalf("re-marshal: %v", err)
			}

			// Unmarshal the re-marshalled JSON.
			var roundTripped serialize.Envelope
			if err := json.Unmarshal(reMarshalled, &roundTripped); err != nil {
				t.Fatalf("unmarshal round-tripped: %v", err)
			}

			// Compare results.
			if len(original.Actions) != len(roundTripped.Actions) {
				t.Errorf("action count: original=%d, round-tripped=%d",
					len(original.Actions), len(roundTripped.Actions))
			}
			if original.Version != roundTripped.Version {
				t.Errorf("version: original=%q, round-tripped=%q",
					original.Version, roundTripped.Version)
			}
		})
	}
}

// TestCrossLanguageParsing checks that the pipeline runs on all supported languages without errors.
func TestCrossLanguageParsing(t *testing.T) {
	langSeen := make(map[string]bool)

	for _, name := range allFixtures(t) {
		t.Run(name, func(t *testing.T) {
			f := loadFixture(t, name)
			ext := filepath.Ext(f.OldPath)
			langSeen[ext] = true

			// Assert that the pipeline does not panic or return an error.
			result := runPipeline(t, f)

			if result.AstA == nil || result.AstB == nil {
				t.Fatal("pipeline returned nil AST")
			}
			if result.ES == nil {
				t.Fatal("pipeline returned nil edit script")
			}
			if len(result.JSON) == 0 {
				t.Fatal("pipeline returned empty JSON")
			}
		})
	}

	t.Logf("Languages tested: %v", langSeen)
}

// TestActionSemantics checks action-specific invariants on real diffs.
func TestActionSemantics(t *testing.T) {
	for _, name := range allFixtures(t) {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := loadFixture(t, name)
			result := runPipeline(t, f)

			var envelope serialize.Envelope
			if err := json.Unmarshal(result.JSON, &envelope); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}

			for i, a := range envelope.Actions {
				switch a.Action {
				case "insert":
					// Inserts must point to the new tree.
					if a.Node != nil && a.Node.Tree != "after" {
						t.Errorf("action[%d] insert: node.tree=%q, want after", i, a.Node.Tree)
					}
					// Non-root inserts must have a parent.
					if a.Parent == nil && a.Node != nil && len(a.Node.Path) > 0 {
						t.Errorf("action[%d] insert: missing parent for non-root node", i)
					}
				case "delete":
					// Deletes must point to the old tree.
					if a.Node != nil && a.Node.Tree != "before" {
						t.Errorf("action[%d] delete: node.tree=%q, want before", i, a.Node.Tree)
					}
				case "update":
					// Updates must point to the old tree.
					if a.Node != nil && a.Node.Tree != "before" {
						t.Errorf("action[%d] update: node.tree=%q, want before", i, a.Node.Tree)
					}
					// Updates must have both old and new values.
					if a.OldValue == "" && a.NewValue == "" {
						t.Errorf("action[%d] update: missing both old_value and new_value", i)
					}
				case "move":
					// Moves must point to the old tree.
					if a.Node != nil && a.Node.Tree != "before" {
						t.Errorf("action[%d] move: node.tree=%q, want before", i, a.Node.Tree)
					}
				}
			}
		})
	}
}

// TestActionCountNonZero checks that real bug-fix diffs produce at least one action.
func TestActionCountNonZero(t *testing.T) {
	for _, name := range allFixtures(t) {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := loadFixture(t, name)
			result := runPipeline(t, f)

			var envelope serialize.Envelope
			if err := json.Unmarshal(result.JSON, &envelope); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}

			if len(envelope.Actions) == 0 {
				t.Error("real bug-fix diff produced zero actions: something is wrong")
			}

			// Log a summary for visibility.
			counts := make(map[string]int)
			for _, a := range envelope.Actions {
				counts[a.Action]++
			}
			summary := fmt.Sprintf("total=%d", len(envelope.Actions))
			for k, v := range counts {
				summary += fmt.Sprintf(" %s=%d", k, v)
			}
			t.Logf("%s: %s", name, summary)
		})
	}
}
