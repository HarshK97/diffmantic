// Package e2e provides end-to-end tests for the diffm CLI binary.
// These tests compile the binary and run it as a subprocess to check output.
//
// Run with:
//
//	go test ./tests/e2e/ -v
//	go test ./tests/e2e/ -v -count=1   # no cache
package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/serialize"
)

// binaryPath is the path to the compiled diffm binary.
var binaryPath string

// TestMain compiles the binary once before running any tests.
func TestMain(m *testing.M) {
	// Build the binary into a temp folder.
	tmp, err := os.MkdirTemp("", "diffm-e2e-*")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	binaryName := "diffm"
	if runtime.GOOS == "windows" {
		binaryName = "diffm.exe"
	}
	binaryPath = filepath.Join(tmp, binaryName)
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", binaryPath, "../../cmd/diffm")
	cmd.Dir, _ = os.Getwd()
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("failed to build diffm: " + string(out) + ": " + err.Error())
	}

	os.Exit(m.Run())
}

// testdataDir returns the absolute path to tests/testdata/.
func testdataDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "testdata"))
	if err != nil {
		t.Fatalf("resolving testdata dir: %v", err)
	}
	return dir
}

// fixtureFiles returns the paths to the old and new files for a fixture.
func fixtureFiles(t *testing.T, name string) (string, string) {
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
		}
		if strings.HasPrefix(n, "new.") {
			newPath = filepath.Join(dir, n)
		}
	}
	if oldPath == "" || newPath == "" {
		t.Fatalf("fixture %s: missing old.* or new.* file", name)
	}
	return oldPath, newPath
}

// runDiffm runs the diffm binary with the given args.
func runDiffm(args ...string) (stdout, stderr string, err error) {
	cmd := exec.Command(binaryPath, args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func sampleFixture(t *testing.T) string {
	t.Helper()
	dir := testdataDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading testdata dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "go_") {
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
				return e.Name()
			}
		}
	}
	t.Fatal("no valid go_ fixture found in testdata")
	return ""
}

// --------------------------------------------------------------------------
// JSON format tests
// --------------------------------------------------------------------------

func TestCLI_JSONFormat_ValidOutput(t *testing.T) {
	oldPath, newPath := fixtureFiles(t, sampleFixture(t))

	stdout, stderr, err := runDiffm(oldPath, newPath, "-f", "json")
	if err != nil {
		t.Fatalf("diffm failed: %v\nstderr: %s", err, stderr)
	}

	var envelope serialize.Envelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("invalid JSON output: %v\nstdout: %s", err, stdout[:min(len(stdout), 500)])
	}

	if envelope.Version == "" {
		t.Error("missing version in JSON output")
	}
	if len(envelope.Actions) == 0 {
		t.Error("expected at least one action for a real diff")
	}
}

func TestCLI_NonInteractive_DefaultsToJSON(t *testing.T) {
	// Our test harness runs diffm via exec.Command, meaning stdin and stdout aren't
	// terminals (like in a CI runner or pipe). The CLI should fall back to JSON
	// output here. The TUI remains the default in actual interactive terminal sessions.
	oldPath, newPath := fixtureFiles(t, sampleFixture(t))
	stdout, stderr, err := runDiffm(oldPath, newPath)
	if err != nil {
		t.Fatalf("diffm failed: %v\nstderr: %s", err, stderr)
	}
	var envelope serialize.Envelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("non-interactive default format is not valid JSON: %v", err)
	}
}

// --------------------------------------------------------------------------
// Actions format tests
// --------------------------------------------------------------------------

func TestCLI_ActionsFormat(t *testing.T) {
	oldPath, newPath := fixtureFiles(t, sampleFixture(t))

	stdout, stderr, err := runDiffm(oldPath, newPath, "-f", "actions")
	if err != nil {
		t.Fatalf("diffm failed: %v\nstderr: %s", err, stderr)
	}

	// Verify the actions output format contains the Diffing header.
	if !strings.Contains(stdout, "Diffing") {
		t.Errorf("actions output missing 'Diffing' header:\n%s", stdout[:min(len(stdout), 500)])
	}
}

// --------------------------------------------------------------------------
// Error handling tests
// --------------------------------------------------------------------------

func TestCLI_MissingFile(t *testing.T) {
	_, stderr, err := runDiffm("/nonexistent/file.go", "/also/nonexistent.go")
	if err == nil {
		t.Fatal("expected non-zero exit for missing files")
	}
	if stderr == "" {
		t.Error("expected error message on stderr")
	}
}

func TestCLI_UnsupportedFormat(t *testing.T) {
	oldPath, newPath := fixtureFiles(t, sampleFixture(t))

	_, stderr, err := runDiffm(oldPath, newPath, "-f", "xml")
	if err == nil {
		t.Fatal("expected non-zero exit for unsupported format")
	}
	if !strings.Contains(stderr, "Unsupported output format") {
		t.Errorf("expected 'Unsupported output format' in stderr, got: %s", stderr)
	}
}

func TestCLI_DirectoryInput(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	_, stderr, err := runDiffm(dirA, dirB)
	if err == nil {
		t.Fatal("expected non-zero exit for directory input")
	}
	if !strings.Contains(stderr, "Directory diffing is not supported") {
		t.Errorf("expected directory not supported message, got: %s", stderr)
	}
}

func TestCLI_NonGitRepo_OneArg(t *testing.T) {
	oldPath, _ := fixtureFiles(t, sampleFixture(t))

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	_, stderr, err := runDiffm(oldPath)
	if err == nil {
		t.Fatal("expected non-zero exit with one arg in non-git repo")
	}
	if !strings.Contains(stderr, "Git mode requires a valid Git repository") {
		t.Errorf("expected git repo error, got: %s", stderr)
	}
}

func TestCLI_JSONFormat_UIFlags(t *testing.T) {
	oldPath, newPath := fixtureFiles(t, sampleFixture(t))

	stdout, stderr, err := runDiffm(oldPath, newPath, "-f", "json", "--ui")
	if err != nil {
		t.Fatalf("diffm failed: %v\nstderr: %s", err, stderr)
	}

	var envelope serialize.Envelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("invalid JSON output for --ui: %v", err)
	}
	if len(envelope.LineAlignment) == 0 {
		t.Error("expected line_alignment in --ui mode")
	}
	if len(envelope.LeftHighlights) == 0 && len(envelope.RightHighlights) == 0 {
		t.Error("expected highlight spans in --ui mode")
	}
	if len(envelope.Actions) != 0 {
		t.Error("expected zero actions in --ui mode")
	}
}

func TestCLI_LargeFileFallback_Exceeds400KB(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "large_old.ts")
	newPath := filepath.Join(dir, "large_new.ts")

	var oldBuilder, newBuilder strings.Builder
	for i := 0; i < 15000; i++ {
		fmt.Fprintf(&oldBuilder, "const variable_%d: number = %d;\n", i, i)
		fmt.Fprintf(&newBuilder, "const variable_%d: number = %d;\n", i, i+1)
	}

	if err := os.WriteFile(oldPath, []byte(oldBuilder.String()), 0o644); err != nil {
		t.Fatalf("writing old file: %v", err)
	}
	if err := os.WriteFile(newPath, []byte(newBuilder.String()), 0o644); err != nil {
		t.Fatalf("writing new file: %v", err)
	}

	stdout, stderr, err := runDiffm(oldPath, newPath, "-f", "json", "--ui")
	if err != nil {
		t.Fatalf("diffm failed on >400KB file: %v\nstderr: %s", err, stderr)
	}

	var envelope serialize.Envelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("invalid JSON output on >400KB file fallback: %v", err)
	}

	if envelope.Version == "" {
		t.Error("missing version in JSON output")
	}
	if len(envelope.LineAlignment) == 0 {
		t.Error("expected line alignment entries in fallback envelope")
	}
}

func TestCLI_IgnoreComments(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.go")
	newPath := filepath.Join(dir, "new.go")

	oldContent := "package main\n\n// Old Comment\nfunc main() {}\n"
	newContent := "package main\n\n// New Comment\nfunc main() {}\n"

	if err := os.WriteFile(oldPath, []byte(oldContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte(newContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// When comments aren't ignored, comment edits show up as actions.
	stdoutWith, stderr, err := runDiffm(oldPath, newPath, "-f", "json")
	if err != nil {
		t.Fatalf("diffm failed: %v\nstderr: %s", err, stderr)
	}
	var envWith serialize.Envelope
	if err := json.Unmarshal([]byte(stdoutWith), &envWith); err != nil {
		t.Fatal(err)
	}
	if len(envWith.Actions) == 0 {
		t.Errorf("expected action for comment change when not ignored")
	}

	// With --ignore-comments, comment-only changes should produce zero actions.
	stdoutIgnored, stderr, err := runDiffm(oldPath, newPath, "-f", "json", "--ignore-comments")
	if err != nil {
		t.Fatalf("diffm failed: %v\nstderr: %s", err, stderr)
	}
	var envIgnored serialize.Envelope
	if err := json.Unmarshal([]byte(stdoutIgnored), &envIgnored); err != nil {
		t.Fatal(err)
	}
	if len(envIgnored.Actions) != 0 {
		t.Errorf("expected 0 actions with --ignore-comments, got %d actions", len(envIgnored.Actions))
	}
}

func TestCLI_ThemeFlag(t *testing.T) {
	oldPath, newPath := fixtureFiles(t, sampleFixture(t))

	// Valid theme: latte
	stdout, stderr, err := runDiffm(oldPath, newPath, "-f", "json", "-t", "latte")
	if err != nil {
		t.Fatalf("diffm with -t latte failed: %v\nstderr: %s", err, stderr)
	}
	var env serialize.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("invalid JSON output with -t latte: %v", err)
	}
	if env.Version == "" {
		t.Error("missing version in JSON output with -t latte")
	}

	// Invalid theme
	_, stderr, err = runDiffm(oldPath, newPath, "-t", "unknown_theme")
	if err == nil {
		t.Fatal("expected non-zero exit for unknown theme")
	}
	if !strings.Contains(stderr, "unsupported theme") {
		t.Errorf("expected 'unsupported theme' error, got: %s", stderr)
	}
}

func TestCLI_ConfigFileDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "diffmantic")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configYAML := `
ignore_comments: true
format: actions
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yml"), []byte(configYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.go")
	newPath := filepath.Join(dir, "new.go")

	oldContent := "package main\n\n// Comment 1\nfunc main() {}\n"
	newContent := "package main\n\n// Comment 2\nfunc main() {}\n"

	_ = os.WriteFile(oldPath, []byte(oldContent), 0o644)
	_ = os.WriteFile(newPath, []byte(newContent), 0o644)

	// No CLI flags passed — should pick up format=actions and ignore_comments from config
	stdout, stderr, err := runDiffm(oldPath, newPath)
	if err != nil {
		t.Fatalf("diffm failed with custom config: %v\nstderr: %s", err, stderr)
	}

	if !strings.Contains(stdout, "Diffing") {
		t.Errorf("expected actions output with 'Diffing' header from config default, got: %s", stdout)
	}
}

func TestCLI_GitMode_TypoFailFast(t *testing.T) {
	// In the current git repo, diffm with a typo file should fail fast
	_, stderr, err := runDiffm("HEAD", "nonexistent_typo_file_12345.go")
	if err == nil {
		t.Fatal("expected error for nonexistent typo file in git mode")
	}
	if !strings.Contains(stderr, "neither a valid file nor a valid Git revision") {
		t.Errorf("expected fail-fast error, got stderr: %s", stderr)
	}
}

func TestCLI_GitMode_HeadlessSingleFileJSON(t *testing.T) {
	// Diffing a tracked file in cwd at HEAD against working tree with -f json
	stdout, stderr, err := runDiffm("HEAD", "cli_test.go", "-f", "json")
	if err != nil {
		t.Fatalf("diffm HEAD cli_test.go -f json failed: %v\nstderr: %s", err, stderr)
	}

	var env serialize.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("invalid JSON from Git headless diff: %v\noutput: %s", err, stdout)
	}
	if env.Version == "" {
		t.Error("expected valid Envelope with non-empty version")
	}
}

func TestCLI_GitMode_HeadlessRequiresPathFilter(t *testing.T) {
	// Running diffm -f json in git mode without a path filter in non-interactive mode should fail fast
	_, stderr, err := runDiffm("HEAD", "-f", "json")
	if err == nil {
		t.Fatal("expected error when running git mode without path filter in non-interactive json format")
	}
	if !strings.Contains(stderr, "requires an explicit file path") {
		t.Errorf("expected path requirement error, got: %s", stderr)
	}
}
