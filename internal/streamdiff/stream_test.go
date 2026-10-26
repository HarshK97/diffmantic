package streamdiff

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/unified"
)

func TestParseDiffGitPaths(t *testing.T) {
	tests := []struct {
		line    string
		wantOld string
		wantNew string
	}{
		{
			line:    "diff --git a/internal/streamdiff/stream.go b/internal/streamdiff/stream.go",
			wantOld: "internal/streamdiff/stream.go",
			wantNew: "internal/streamdiff/stream.go",
		},
		{
			line:    "diff --git a/old.go b/new.go",
			wantOld: "old.go",
			wantNew: "new.go",
		},
		{
			line:    "diff --git a/path with spaces.go b/path with spaces.go",
			wantOld: "path with spaces.go",
			wantNew: "path with spaces.go",
		},
		{
			line:    "diff --git path with spaces.go path with spaces.go",
			wantOld: "path with spaces.go",
			wantNew: "path with spaces.go",
		},
		{
			line:    `diff --git "a/path with spaces.go" "b/path with spaces.go"`,
			wantOld: "path with spaces.go",
			wantNew: "path with spaces.go",
		},
		{
			line:    `diff --git "a/pkg/test b/foo.go" "b/pkg/test b/foo.go"`,
			wantOld: "pkg/test b/foo.go",
			wantNew: "pkg/test b/foo.go",
		},
		{
			line:    `diff --git a/old.go "b/path with spaces.go"`,
			wantOld: "old.go",
			wantNew: "path with spaces.go",
		},
		{
			line:    `diff --git "a/path with spaces.go" b/new.go`,
			wantOld: "path with spaces.go",
			wantNew: "new.go",
		},
		{
			line:    `diff --git a/foo.go "b/foo\tbar.go"`,
			wantOld: "foo.go",
			wantNew: "foo\tbar.go",
		},
		{
			line:    `diff --git "a/foo\"bar.go" "b/foo\"bar.go"`,
			wantOld: "foo\"bar.go",
			wantNew: "foo\"bar.go",
		},
		{
			line:    "diff --git old.go new.go",
			wantOld: "old.go",
			wantNew: "new.go",
		},
		{
			line:    "not a diff line",
			wantOld: "",
			wantNew: "",
		},
	}

	for _, tt := range tests {
		gotOld, gotNew := parseDiffGitPaths(tt.line)
		if gotOld != tt.wantOld || gotNew != tt.wantNew {
			t.Errorf("parseDiffGitPaths(%q) = (%q, %q), want (%q, %q)", tt.line, gotOld, gotNew, tt.wantOld, tt.wantNew)
		}
	}
}

func TestParseHeaderPath(t *testing.T) {
	tests := []struct {
		line   string
		prefix string
		want   string
	}{
		{
			line:   "--- a/path with spaces.go\t2026-10-26 12:00:00",
			prefix: "--- ",
			want:   "path with spaces.go",
		},
		{
			line:   "+++ b/path with spaces.go",
			prefix: "+++ ",
			want:   "path with spaces.go",
		},
		{
			line:   "--- /dev/null",
			prefix: "--- ",
			want:   "",
		},
		{
			line:   `rename from "old with \"quote\".go"`,
			prefix: "rename from ",
			want:   `old with "quote".go`,
		},
		{
			line:   "rename to new with space.go",
			prefix: "rename to ",
			want:   "new with space.go",
		},
	}

	for _, tt := range tests {
		got := parseHeaderPath(tt.line, tt.prefix)
		if got != tt.want {
			t.Errorf("parseHeaderPath(%q, %q) = %q, want %q", tt.line, tt.prefix, got, tt.want)
		}
	}
}

func TestParseIndexOIDs(t *testing.T) {
	tests := []struct {
		line    string
		wantOld string
		wantNew string
	}{
		{
			line:    "index 7d68fcded..c661b4c7a 100644",
			wantOld: "7d68fcded",
			wantNew: "c661b4c7a",
		},
		{
			line:    "index 0000000..abcdef1 100644",
			wantOld: "0000000",
			wantNew: "abcdef1",
		},
		{
			line:    "index 1234567,89abcde..fedcba9",
			wantOld: "",
			wantNew: "",
		},
	}

	for _, tt := range tests {
		gotOld, gotNew := parseIndexOIDs(tt.line)
		if gotOld != tt.wantOld || gotNew != tt.wantNew {
			t.Errorf("parseIndexOIDs(%q) = (%q, %q), want (%q, %q)", tt.line, gotOld, gotNew, tt.wantOld, tt.wantNew)
		}
	}
}

func TestIsAllZeros(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"0", true},
		{"0000000", true},
		{"0000000000000000000000000000000000000000", true},
		{"0000001", false},
		{"1000000", false},
		{"abcdef", false},
	}
	for _, tt := range tests {
		if got := isAllZeros(tt.input); got != tt.want {
			t.Errorf("isAllZeros(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestPassthroughNonGitLines(t *testing.T) {
	input := `commit 1482c9cdd9d9cf878191d1b04f81422ccd02967e
Author: Developer <dev@example.com>
Date:   Tue Oct 6 09:24:01 2026 +0530

    feat: some commit message

diff --cc conflict.go
index 1111111,2222222..3333333
@@@ -1,2 -1,2 +1,3 @@@
`
	var out bytes.Buffer
	cfg := StreamConfig{Format: "unified"}
	err := Process(strings.NewReader(input), &out, ".", cfg)
	if err != nil {
		t.Fatalf("Process returned unexpected error: %v", err)
	}

	if out.String() != input {
		t.Errorf("Expected passthrough to preserve exact input.\nGot:\n%s\nWant:\n%s", out.String(), input)
	}
}

func setupStreamRepo(t *testing.T) (repoDir, oldOID, newOID string) {
	t.Helper()
	repoDir = t.TempDir()
	execGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	execGit("init")
	execGit("-c", "commit.gpgsign=false", "-c", "user.email=t@test.com", "-c", "user.name=test", "checkout", "-b", "main")

	srcContent := "package main\n\nfunc calculate(x int) int {\n\treturn x * 2\n}\n"
	dstContent := "package main\n\nfunc calculate(x int) int {\n\tmultiplier := 2\n\treturn x * multiplier\n}\n"

	filePath := filepath.Join(repoDir, "calc.go")
	if err := os.WriteFile(filePath, []byte(srcContent), 0o600); err != nil {
		t.Fatal(err)
	}
	execGit("add", "calc.go")
	execGit("-c", "commit.gpgsign=false", "-c", "user.email=t@test.com", "-c", "user.name=test", "commit", "-m", "init calc")
	oldOID = execGit("rev-parse", "HEAD:calc.go")

	if err := os.WriteFile(filePath, []byte(dstContent), 0o600); err != nil {
		t.Fatal(err)
	}
	execGit("add", "calc.go")
	execGit("-c", "commit.gpgsign=false", "-c", "user.email=t@test.com", "-c", "user.name=test", "commit", "-m", "update calc")
	newOID = execGit("rev-parse", "HEAD:calc.go")

	return repoDir, oldOID, newOID
}

func TestRealGitCommitStream(t *testing.T) {
	repoDir, _, _ := setupStreamRepo(t)
	cmd := exec.Command("git", "log", "-p", "-n", "1")
	cmd.Dir = repoDir
	logBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log -p failed: %v", err)
	}

	var out bytes.Buffer
	cfg := StreamConfig{
		Format: "unified",
		UnifiedOpts: unified.RenderOptions{
			ContextLines: 3,
			Color:        false,
		},
	}

	err = Process(bytes.NewReader(logBytes), &out, repoDir, cfg)
	if err != nil {
		t.Fatalf("Process returned unexpected error: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, "commit ") {
		t.Errorf("Expected output to retain commit header, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "calc.go") {
		t.Errorf("Expected output to contain diff for calc.go, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "multiplier := 2") {
		t.Errorf("Expected output to contain new code, got:\n%s", outStr)
	}
}

func TestMultiCommitStream(t *testing.T) {
	repoDir, _, _ := setupStreamRepo(t)
	cmd := exec.Command("git", "log", "-p", "-n", "2")
	cmd.Dir = repoDir
	logBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log -p failed: %v", err)
	}

	var out bytes.Buffer
	cfg := StreamConfig{
		Format: "unified",
	}

	err = Process(bytes.NewReader(logBytes), &out, repoDir, cfg)
	if err != nil {
		t.Fatalf("Process returned unexpected error: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, "init calc") {
		t.Errorf("Expected commit 1 header to be preserved in stream, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "update calc") {
		t.Errorf("Expected commit 2 header to be preserved in stream, got:\n%s", outStr)
	}
}

func TestCleanStreamLine(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "\x1b[1mdiff --git a/foo.go b/foo.go\x1b[m\r\n",
			want:  "diff --git a/foo.go b/foo.go",
		},
		{
			input: "\x1b[33mcommit 1482c9cdd\x1b[m (\x1b[1;36mHEAD\x1b[m)\n",
			want:  "commit 1482c9cdd (HEAD)",
		},
		{
			input: "plain text line\n",
			want:  "plain text line",
		},
		{
			input: "\x1b[1mindex 7d68fcded..c661b4c7a 100644\x1b[m\r\n",
			want:  "index 7d68fcded..c661b4c7a 100644",
		},
	}

	for _, tt := range tests {
		got := cleanStreamLine(tt.input)
		if got != tt.want {
			t.Errorf("cleanStreamLine(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestAnsiColoredGitCommitStream(t *testing.T) {
	repoDir, _, _ := setupStreamRepo(t)
	cmd := exec.Command("git", "-c", "color.diff=always", "-c", "color.ui=always", "log", "-p", "-n", "1")
	cmd.Dir = repoDir
	logBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log -p with color failed: %v", err)
	}

	var out bytes.Buffer
	cfg := StreamConfig{
		Format: "unified",
		UnifiedOpts: unified.RenderOptions{
			ContextLines: 3,
			Color:        false,
		},
	}

	err = Process(bytes.NewReader(logBytes), &out, repoDir, cfg)
	if err != nil {
		t.Fatalf("Process returned unexpected error on ANSI stream: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, "\x1b[") {
		t.Errorf("Expected output to retain colorized commit header, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "calc.go") {
		t.Errorf("Expected output to contain AST diff for calc.go, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "multiplier := 2") {
		t.Errorf("Expected output to contain AST modified lines, got:\n%s", outStr)
	}
}

func TestFileAdditionAndDeletionStreams(t *testing.T) {
	repoDir, oldOID, _ := setupStreamRepo(t)
	input := fmt.Sprintf(`diff --git a/added.go b/added.go
new file mode 100644
index 0000000..%s
--- /dev/null
+++ b/added.go
@@ -0,0 +1,5 @@
+package main
diff --git a/deleted.go b/deleted.go
deleted file mode 100644
index %s..0000000
--- a/deleted.go
+++ /dev/null
@@ -1,5 +0,0 @@
-package main
`, oldOID[:7], oldOID[:7])

	var out bytes.Buffer
	cfg := StreamConfig{
		Format: "unified",
		UnifiedOpts: unified.RenderOptions{
			ContextLines: 3,
			Color:        false,
		},
	}

	err := Process(strings.NewReader(input), &out, repoDir, cfg)
	if err != nil {
		t.Fatalf("Process failed on addition/deletion stream: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, "added.go") {
		t.Errorf("Expected output to contain added.go diff, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "deleted.go") {
		t.Errorf("Expected output to contain deleted.go diff, got:\n%s", outStr)
	}
}

func TestStreamFormatDispatch_JsonAndActions(t *testing.T) {
	repoDir, oldOID, newOID := setupStreamRepo(t)
	input := fmt.Sprintf(`diff --git a/calc.go b/calc.go
index %s..%s 100644
--- a/calc.go
+++ b/calc.go
@@ -1 +1 @@
-old
+new
`, oldOID[:7], newOID[:7])

	// 1. JSON format
	{
		var out bytes.Buffer
		cfg := StreamConfig{Format: "json"}
		err := Process(strings.NewReader(input), &out, repoDir, cfg)
		if err != nil {
			t.Fatalf("Process format=json failed: %v", err)
		}
		if !strings.Contains(out.String(), `"actions":`) && !strings.Contains(out.String(), `"alignment":`) {
			t.Errorf("Expected JSON envelope output, got:\n%s", out.String())
		}
	}

	// 2. Actions format
	{
		var out bytes.Buffer
		cfg := StreamConfig{Format: "actions"}
		err := Process(strings.NewReader(input), &out, repoDir, cfg)
		if err != nil {
			t.Fatalf("Process format=actions failed: %v", err)
		}
		outStr := out.String()
		if !strings.Contains(outStr, "Diffing  calc.go") {
			t.Errorf("Expected actions header, got:\n%s", outStr)
		}
	}
}

func TestStreamBinaryDiff(t *testing.T) {
	repoDir, _, _ := setupStreamRepo(t)
	input := `diff --git a/image.png b/image.png
index 0123456..789abcd 100644
Binary files a/image.png and b/image.png differ
`
	var out bytes.Buffer
	cfg := StreamConfig{Format: "unified"}
	err := Process(strings.NewReader(input), &out, repoDir, cfg)
	if err != nil {
		t.Fatalf("Process failed on binary diff: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, "Binary files a/image.png and b/image.png differ") {
		t.Errorf("Expected binary notice to be preserved, got:\n%s", outStr)
	}
}

func TestStreamFormatDispatch_SideBySideBanner(t *testing.T) {
	repoDir, oldOID, newOID := setupStreamRepo(t)
	input := fmt.Sprintf(`diff --git a/calc.go b/calc.go
index %s..%s 100644
--- a/calc.go
+++ b/calc.go
@@ -1 +1 @@
-old
+new
`, oldOID[:7], newOID[:7])

	var out bytes.Buffer
	cfg := StreamConfig{
		Format: "side-by-side",
	}
	err := Process(strings.NewReader(input), &out, repoDir, cfg)
	if err != nil {
		t.Fatalf("Process format=side-by-side failed: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, "calc.go") {
		t.Errorf("Expected side-by-side output to contain file path banner, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "[+") || !strings.Contains(outStr, "-") {
		t.Errorf("Expected side-by-side banner to contain diff stats [+... -...], got:\n%s", outStr)
	}
}

func TestStreamShallowCloneFallback(t *testing.T) {
	repoDir, _, _ := setupStreamRepo(t)
	// Completely unknown hashes that do not exist in repoDir
	input := `diff --git a/calc.go b/calc.go
index 1111111..2222222 100644
--- a/calc.go
+++ b/calc.go
@@ -1 +1 @@
-old
+new
`
	var out bytes.Buffer
	cfg := StreamConfig{Format: "side-by-side"}
	err := Process(strings.NewReader(input), &out, repoDir, cfg)
	if err != nil {
		t.Fatalf("Process returned unexpected error: %v", err)
	}

	if out.String() != input {
		t.Errorf("Expected raw patch fallback on missing blobs in shallow clone.\nGot:\n%s\nWant:\n%s", out.String(), input)
	}
}

func TestStreamFormatPathHandling(t *testing.T) {
	// Renamed file
	hRename := &fileHunk{
		oldPath: "old_service.go",
		newPath: "new_service.go",
		oldOID:  "1234567",
		newOID:  "7654321",
	}
	if p := formatBannerPath(hRename); p != "old_service.go → new_service.go" {
		t.Errorf("formatBannerPath for rename = %q, want %q", p, "old_service.go → new_service.go")
	}

	// Added file
	hAdd := &fileHunk{
		oldPath: "added.go",
		newPath: "added.go",
		oldOID:  "0000000",
		newOID:  "1234567",
	}
	if p := formatBannerPath(hAdd); p != "added.go" {
		t.Errorf("formatBannerPath for addition = %q, want %q", p, "added.go")
	}

	// Deleted file
	hDel := &fileHunk{
		oldPath: "deleted.go",
		newPath: "deleted.go",
		oldOID:  "1234567",
		newOID:  "0000000",
	}
	if p := formatBannerPath(hDel); p != "deleted.go" {
		t.Errorf("formatBannerPath for deletion = %q, want %q", p, "deleted.go")
	}
}

func TestStreamUnstagedFileFallback(t *testing.T) {
	repoDir := t.TempDir()
	execGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
		}
		return string(out)
	}

	execGit("init")
	execGit("-c", "commit.gpgsign=false", "-c", "user.email=t@test.com", "-c", "user.name=test", "checkout", "-b", "main")

	srcContent := "package main\n\nfunc greet() string {\n\treturn \"hi\"\n}\n"
	dstContent := "package main\n\nfunc greet() string {\n\treturn \"hello world\"\n}\n"

	filePath := filepath.Join(repoDir, "greet.go")
	if err := os.WriteFile(filePath, []byte(srcContent), 0o600); err != nil {
		t.Fatal(err)
	}
	execGit("add", "greet.go")
	execGit("-c", "commit.gpgsign=false", "-c", "user.email=t@test.com", "-c", "user.name=test", "commit", "-m", "init")

	// Write unstaged changes to disk
	if err := os.WriteFile(filePath, []byte(dstContent), 0o600); err != nil {
		t.Fatal(err)
	}

	diffText := execGit("diff")
	if !strings.Contains(diffText, "diff --git a/greet.go b/greet.go") {
		t.Fatalf("unexpected git diff output: %s", diffText)
	}

	var out bytes.Buffer
	cfg := StreamConfig{
		Format: "unified",
		UnifiedOpts: unified.RenderOptions{
			ContextLines: 3,
			Color:        false,
		},
	}

	err := Process(strings.NewReader(diffText), &out, repoDir, cfg)
	if err != nil {
		t.Fatalf("Process returned unexpected error: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, "greet.go") {
		t.Errorf("Expected output to contain greet.go banner, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "hello world") {
		t.Errorf("Expected output to contain unstaged modification 'hello world', got:\n%s", outStr)
	}
}

func TestStreamUnquotedSpacesAndRename(t *testing.T) {
	repoDir := t.TempDir()
	execGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
		}
		return string(out)
	}

	execGit("init")
	execGit("-c", "commit.gpgsign=false", "-c", "user.email=t@test.com", "-c", "user.name=test", "checkout", "-b", "main")

	srcContent := "package main\n\nfunc hello() string {\n\treturn \"hi\"\n}\n"
	dstContent := "package main\n\nfunc hello() string {\n\treturn \"spaces worked\"\n}\n"

	filePath := filepath.Join(repoDir, "my test file.go")
	if err := os.WriteFile(filePath, []byte(srcContent), 0o600); err != nil {
		t.Fatal(err)
	}
	execGit("add", "my test file.go")
	execGit("-c", "commit.gpgsign=false", "-c", "user.email=t@test.com", "-c", "user.name=test", "commit", "-m", "init spaces")

	if err := os.WriteFile(filePath, []byte(dstContent), 0o600); err != nil {
		t.Fatal(err)
	}

	diffText := execGit("diff")
	if !strings.Contains(diffText, "diff --git a/my test file.go b/my test file.go") {
		t.Fatalf("expected git diff to contain unquoted spaces, got:\n%s", diffText)
	}

	var out bytes.Buffer
	cfg := StreamConfig{
		Format: "unified",
		UnifiedOpts: unified.RenderOptions{
			ContextLines: 3,
			Color:        false,
		},
	}

	err := Process(strings.NewReader(diffText), &out, repoDir, cfg)
	if err != nil {
		t.Fatalf("Process returned unexpected error: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, "my test file.go") {
		t.Errorf("Expected output to contain 'my test file.go' banner, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "spaces worked") {
		t.Errorf("Expected output to contain AST modified lines, got:\n%s", outStr)
	}
}

func TestStreamGraphLinesPassthrough(t *testing.T) {
	// Simulated git log --graph -p stream with graph prefixes
	input := `* commit deadbeef1234567890abcdef (HEAD -> main)
| Author: Developer <dev@example.com>
| Date:   Mon Oct 12 12:00:00 2026 +0000
| 
|     feat: add graph commit
| 
| diff --git a/graph.go b/graph.go
| index 1234567..7654321 100644
| --- a/graph.go
| +++ b/graph.go
| @@ -1,2 +1,2 @@
| -old
| +new
`
	var out bytes.Buffer
	cfg := StreamConfig{Format: "unified"}
	err := Process(strings.NewReader(input), &out, ".", cfg)
	if err != nil {
		t.Fatalf("Process returned unexpected error: %v", err)
	}

	// Graph lines must pass through verbatim per Delta's precedent
	if out.String() != input {
		t.Errorf("Expected verbatim passthrough of git graph stream.\nGot:\n%s\nWant:\n%s", out.String(), input)
	}
}

func TestStreamNonGitDirectoryFallback(t *testing.T) {
	nonGitDir := t.TempDir()
	input := `diff --git a/unknown.go b/unknown.go
index 1234567..7654321 100644
--- a/unknown.go
+++ b/unknown.go
@@ -1 +1 @@
-old
+new
`
	var out bytes.Buffer
	cfg := StreamConfig{Format: "unified"}
	// In a directory with no git repository, Process must not crash, hang, or error;
	// it must fall back to dumping raw patch lines verbatim.
	err := Process(strings.NewReader(input), &out, nonGitDir, cfg)
	if err != nil {
		t.Fatalf("Process returned unexpected error in non-git dir: %v", err)
	}
	if out.String() != input {
		t.Errorf("Expected raw fallback in non-git directory.\nGot:\n%s\nWant:\n%s", out.String(), input)
	}
}

func TestStreamLargeHunkFallback(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("diff --git a/big.sql b/big.sql\nindex 1111111..2222222 100644\n--- a/big.sql\n+++ b/big.sql\n")
	for i := 0; i < 50005; i++ {
		sb.WriteString("+INSERT INTO table VALUES (1);\n")
	}

	var out bytes.Buffer
	cfg := StreamConfig{Format: "unified"}
	err := Process(strings.NewReader(sb.String()), &out, t.TempDir(), cfg)
	if err != nil {
		t.Fatalf("Process returned unexpected error on large hunk: %v", err)
	}

	if out.Len() != sb.Len() {
		t.Errorf("Expected large hunk to be completely passed through verbatim. Got length %d, want %d", out.Len(), sb.Len())
	}
}
