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
	cmd := exec.Command("go", "build", "-o", binaryPath, "../../cmd/diffm")
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

// --------------------------------------------------------------------------
// JSON format tests
// --------------------------------------------------------------------------

func TestCLI_JSONFormat_ValidOutput(t *testing.T) {
	oldPath, newPath := fixtureFiles(t, "go_comment_update")

	stdout, stderr, err := runDiffm("diff", oldPath, newPath, "-f", "json")
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
	oldPath, newPath := fixtureFiles(t, "go_comment_update")
	stdout, stderr, err := runDiffm("diff", oldPath, newPath)
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
	oldPath, newPath := fixtureFiles(t, "go_comment_update")

	stdout, stderr, err := runDiffm("diff", oldPath, newPath, "-f", "actions")
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
	_, stderr, err := runDiffm("diff", "/nonexistent/file.go", "/also/nonexistent.go")
	if err == nil {
		t.Fatal("expected non-zero exit for missing files")
	}
	if stderr == "" {
		t.Error("expected error message on stderr")
	}
}

func TestCLI_UnsupportedFormat(t *testing.T) {
	oldPath, newPath := fixtureFiles(t, "go_comment_update")

	_, stderr, err := runDiffm("diff", oldPath, newPath, "-f", "xml")
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

	_, stderr, err := runDiffm("diff", dirA, dirB)
	if err == nil {
		t.Fatal("expected non-zero exit for directory input")
	}
	if !strings.Contains(stderr, "Directory diffing is not supported") {
		t.Errorf("expected directory not supported message, got: %s", stderr)
	}
}

func TestCLI_NoArgs(t *testing.T) {
	_, stderr, err := runDiffm("diff")
	if err == nil {
		t.Fatal("expected non-zero exit with no args")
	}
	if !strings.Contains(stderr, "accepts 2 arg(s)") {
		t.Errorf("expected arg count error, got: %s", stderr)
	}
}

func TestCLI_OneArg(t *testing.T) {
	oldPath, _ := fixtureFiles(t, "go_comment_update")

	_, stderr, err := runDiffm("diff", oldPath)
	if err == nil {
		t.Fatal("expected non-zero exit with one arg")
	}
	if !strings.Contains(stderr, "accepts 2 arg(s)") {
		t.Errorf("expected arg count error, got: %s", stderr)
	}
}
