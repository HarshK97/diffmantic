package git

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestIsGitRepository(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}

	if !IsGitRepository(cwd) {
		t.Errorf("expected %s to be a Git repository", cwd)
	}

	tempDir, err := os.MkdirTemp("", "not-git")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	if IsGitRepository(tempDir) {
		t.Errorf("expected %s to NOT be a Git repository", tempDir)
	}
}

func TestGetStatus(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}

	files, err := GetStatus(cwd, "")
	if err != nil {
		t.Fatalf("failed to get git status: %v", err)
	}

	if len(files) == 0 {
		t.Log("Warning: Git status returned 0 files. This is fine if working directory is clean.")
	} else {
		for _, f := range files {
			if f.Path == "" {
				t.Error("expected non-empty path for Git status file")
			}
			t.Logf("File: %s, Status: %s, Staged: %t, Unstaged: %t", f.Path, f.Status, f.Staged, f.Unstaged)
		}
	}
}

func TestGetChangedFiles(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}

	// Test Status mode (no refs)
	files, err := GetChangedFiles(cwd, "", "", "")
	if err != nil {
		t.Fatalf("failed to get changed files (status mode): %v", err)
	}
	t.Logf("Changed files (status mode): %d", len(files))

	// Test Ref mode (HEAD~1 vs HEAD)
	// We check if HEAD~1 is valid first
	_, err = RunGit(cwd, "rev-parse", "HEAD~1")
	if err == nil {
		refFiles, err := GetChangedFiles(cwd, "HEAD~1", "HEAD", "")
		if err != nil {
			t.Fatalf("failed to get changed files (HEAD~1 vs HEAD): %v", err)
		}
		t.Logf("Changed files (HEAD~1 vs HEAD): %d", len(refFiles))
		for _, f := range refFiles {
			if f.Path == "" {
				t.Error("expected non-empty path for Git changed file")
			}
		}
	} else {
		t.Log("Skipping ref mode test because HEAD~1 is not available (e.g. shallow clone or initial commit)")
	}
}

func TestIsTrackedFile(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}

	// git.go is a tracked file in this package directory
	if !IsTrackedFile(cwd, "git.go") {
		t.Errorf("expected git.go to be tracked")
	}

	// Nonexistent file is not tracked
	if IsTrackedFile(cwd, "nonexistent_file_123456789.go") {
		t.Errorf("expected nonexistent file to NOT be tracked")
	}

	// Empty string
	if IsTrackedFile(cwd, "") {
		t.Errorf("expected empty string to NOT be tracked")
	}
}

func TestGetContent_Textconv(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	rootOut, err := RunGit(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatalf("failed to get repo root: %v", err)
	}
	repoRoot := strings.TrimSpace(string(rootOut))

	// expected_ui.json.gz has textconv configured via diff.gzip.textconv
	path := "tests/testdata/c_git_strbuf_setlen/expected_ui.json.gz"

	// 1. Test revision HEAD with textconv enabled
	dataHead, err := GetContent(repoRoot, path, "HEAD", true)
	if err != nil {
		t.Fatalf("GetContent(HEAD, true) failed: %v", err)
	}
	if len(dataHead) == 0 || dataHead[0] != '{' {
		t.Errorf("expected textconv decompressed JSON starting with '{', got: %q", string(dataHead[:min(len(dataHead), 50)]))
	}

	// 2. Test working tree on disk with textconv enabled
	dataWork, err := GetContent(repoRoot, path, "", true)
	if err != nil {
		t.Fatalf("GetContent(worktree, true) failed: %v", err)
	}
	if len(dataWork) == 0 || dataWork[0] != '{' {
		t.Errorf("expected textconv decompressed working tree JSON starting with '{', got: %q", string(dataWork[:min(len(dataWork), 50)]))
	}

	// 3. Test without textconv: should return raw binary gzip data (magic bytes 0x1f 0x8b)
	dataRaw, err := GetContent(repoRoot, path, "", false)
	if err != nil {
		t.Fatalf("GetContent(worktree, false) failed: %v", err)
	}
	if len(dataRaw) < 2 || dataRaw[0] != 0x1f || dataRaw[1] != 0x8b {
		t.Errorf("expected raw gzip magic bytes 0x1f 0x8b, got: %x", dataRaw[:min(len(dataRaw), 4)])
	}
}

func TestListRefs(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}

	refs, err := ListRefs(cwd, "")
	if err != nil {
		t.Fatalf("ListRefs failed: %v", err)
	}

	if len(refs) == 0 {
		t.Fatal("expected non-empty refs list in git repository")
	}

	// HEAD should be present when no prefix is given
	var foundHead bool
	for _, ref := range refs {
		if ref == "HEAD" {
			foundHead = true
			break
		}
	}
	if !foundHead {
		t.Error("expected 'HEAD' to be in ListRefs result")
	}

	headRefs, err := ListRefs(cwd, "HEAD")
	if err != nil {
		t.Fatalf("ListRefs(cwd, 'HEAD') failed: %v", err)
	}
	if !slices.Contains(headRefs, "HEAD") {
		t.Errorf("expected 'HEAD' in filtered refs, got: %v", headRefs)
	}

	noneRefs, err := ListRefs(cwd, "nonexistent_ref_prefix_xyz123")
	if err != nil {
		t.Fatalf("ListRefs with nonexistent prefix failed: %v", err)
	}
	if len(noneRefs) != 0 {
		t.Errorf("expected empty refs for nonexistent prefix, got: %v", noneRefs)
	}

	// Non-git directory should return error
	tempDir, err := os.MkdirTemp("", "not-git-refs")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	_, err = ListRefs(tempDir, "")
	if err == nil {
		t.Error("expected error calling ListRefs in non-git directory")
	}
}

func TestGetStatus_StagedBinary(t *testing.T) {
	tempDir := t.TempDir()
	if _, err := RunGit(tempDir, "init", "-b", "main"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	if _, err := RunGit(tempDir, "-c", "commit.gpgsign=false", "-c", "user.email=t@test.com", "-c", "user.name=test", "commit", "--allow-empty", "-m", "init"); err != nil {
		t.Fatalf("initial commit failed: %v", err)
	}

	binPath := filepath.Join(tempDir, "sample.bin")
	if err := os.WriteFile(binPath, []byte{0x00, 0x01, 0x02, 0x03}, 0o644); err != nil {
		t.Fatalf("write binary file failed: %v", err)
	}

	if _, err := RunGit(tempDir, "add", "sample.bin"); err != nil {
		t.Fatalf("git add failed: %v", err)
	}

	files, err := GetStatus(tempDir, "")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	if !files[0].IsBinary {
		t.Errorf("expected staged binary file to have IsBinary=true, got false")
	}
}
