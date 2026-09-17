package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
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
	tempDir := t.TempDir()

	if _, err := RunGit(tempDir, "init", "-b", "main"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	if _, err := RunGit(tempDir, "-c", "commit.gpgsign=false", "-c", "user.email=t@test.com", "-c", "user.name=test", "commit", "--allow-empty", "-m", "init"); err != nil {
		t.Fatalf("initial commit failed: %v", err)
	}

	// Build a lightweight standalone textconv converter binary.
	// It reads the file passed in os.Args[1] and outputs "CONVERTED: " + content.
	helperSrc := filepath.Join(tempDir, "conv_main.go")
	helperExe := filepath.Join(tempDir, "conv")
	if runtime.GOOS == "windows" {
		helperExe += ".exe"
	}

	const helperCode = `package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		return
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		os.Exit(1)
	}
	fmt.Print("CONVERTED: " + string(data))
}
`
	if err := os.WriteFile(helperSrc, []byte(helperCode), 0o644); err != nil {
		t.Fatalf("write helper src failed: %v", err)
	}

	goBin, err := exec.LookPath("go")
	if err != nil {
		goBin = "go"
	}

	buildCmd := exec.Command(goBin, "build", "-o", helperExe, helperSrc)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile textconv helper: %v, output: %s", err, string(out))
	}

	// Configure .gitattributes and diff.customconv.textconv in the test repository
	gitattrPath := filepath.Join(tempDir, ".gitattributes")
	if err := os.WriteFile(gitattrPath, []byte("*.custom diff=customconv\n"), 0o644); err != nil {
		t.Fatalf("write .gitattributes failed: %v", err)
	}

	convCmd := filepath.ToSlash(helperExe)
	if _, err := RunGit(tempDir, "config", "diff.customconv.textconv", convCmd); err != nil {
		t.Fatalf("git config diff.customconv.textconv failed: %v", err)
	}

	// Create and commit test file
	filePath := "sample.custom"
	rawContent := "raw file content"
	fullPath := filepath.Join(tempDir, filePath)
	if err := os.WriteFile(fullPath, []byte(rawContent), 0o644); err != nil {
		t.Fatalf("write sample file failed: %v", err)
	}

	if _, err := RunGit(tempDir, "add", ".gitattributes", filePath); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if _, err := RunGit(tempDir, "-c", "commit.gpgsign=false", "-c", "user.email=t@test.com", "-c", "user.name=test", "commit", "-m", "add custom file"); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// 1. Test revision HEAD with textconv enabled: should invoke Git's textconv filter
	dataHead, err := GetContent(tempDir, filePath, "HEAD", true)
	if err != nil {
		t.Fatalf("GetContent(HEAD, true) failed: %v", err)
	}
	expectedConverted := "CONVERTED: " + rawContent
	if string(dataHead) != expectedConverted {
		t.Errorf("GetContent(HEAD, true) = %q, want %q", string(dataHead), expectedConverted)
	}

	// Modify working tree on disk to verify working tree reads current disk state
	modifiedContent := "modified on disk"
	if err := os.WriteFile(fullPath, []byte(modifiedContent), 0o644); err != nil {
		t.Fatalf("write modified file failed: %v", err)
	}

	// 2. Test working tree on disk with textconv enabled: should invoke working tree textconv filter
	dataWork, err := GetContent(tempDir, filePath, "", true)
	if err != nil {
		t.Fatalf("GetContent(worktree, true) failed: %v", err)
	}
	expectedWorkConverted := "CONVERTED: " + modifiedContent
	if string(dataWork) != expectedWorkConverted {
		t.Errorf("GetContent(worktree, true) = %q, want %q", string(dataWork), expectedWorkConverted)
	}

	// 3. Test without textconv: should return raw unconverted content from disk
	dataRaw, err := GetContent(tempDir, filePath, "", false)
	if err != nil {
		t.Fatalf("GetContent(worktree, false) failed: %v", err)
	}
	if string(dataRaw) != modifiedContent {
		t.Errorf("GetContent(worktree, false) = %q, want %q", string(dataRaw), modifiedContent)
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
