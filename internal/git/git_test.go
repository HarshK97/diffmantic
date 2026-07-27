package git

import (
	"os"
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

	files, err := GetStatus(cwd)
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
