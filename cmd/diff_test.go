package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiffDirectoriesMatchedFiles(t *testing.T) {
	// Create two temp dirs with matching files.
	dirA := t.TempDir()
	dirB := t.TempDir()

	os.WriteFile(filepath.Join(dirA, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dirB, "main.go"), []byte("package main\n// changed"), 0644)

	files, err := diffDirectories(dirA, dirB)
	if err != nil {
		t.Fatalf("diffDirectories: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("want 1 diff file, got %d", len(files))
	}
	if files[0].RelPath != "main.go" {
		t.Errorf("RelPath = %q, want %q", files[0].RelPath, "main.go")
	}
}

func TestDiffDirectoriesOnlyInA(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	os.WriteFile(filepath.Join(dirA, "old.go"), []byte("package old"), 0644)

	files, err := diffDirectories(dirA, dirB)
	if err != nil {
		t.Fatalf("diffDirectories: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("want 1 file (only in A), got %d", len(files))
	}
}

func TestDiffDirectoriesOnlyInB(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	os.WriteFile(filepath.Join(dirB, "new.go"), []byte("package new"), 0644)

	files, err := diffDirectories(dirA, dirB)
	if err != nil {
		t.Fatalf("diffDirectories: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("want 1 file (only in B), got %d", len(files))
	}
}

func TestDiffDirectoriesEmpty(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	files, err := diffDirectories(dirA, dirB)
	if err != nil {
		t.Fatalf("diffDirectories: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("empty dirs should produce 0 files, got %d", len(files))
	}
}

func TestDiffDirectoriesSubdirs(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	os.MkdirAll(filepath.Join(dirA, "pkg"), 0755)
	os.MkdirAll(filepath.Join(dirB, "pkg"), 0755)
	os.WriteFile(filepath.Join(dirA, "pkg", "util.go"), []byte("package pkg"), 0644)
	os.WriteFile(filepath.Join(dirB, "pkg", "util.go"), []byte("package pkg\n// updated"), 0644)

	files, err := diffDirectories(dirA, dirB)
	if err != nil {
		t.Fatalf("diffDirectories: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("want 1 file from subdir, got %d", len(files))
	}
	if files[0].RelPath != filepath.Join("pkg", "util.go") {
		t.Errorf("RelPath = %q, want %q", files[0].RelPath, filepath.Join("pkg", "util.go"))
	}
}

func TestDiffDirectoriesSorted(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	os.WriteFile(filepath.Join(dirA, "c.go"), []byte("c"), 0644)
	os.WriteFile(filepath.Join(dirA, "a.go"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dirB, "b.go"), []byte("b"), 0644)

	files, err := diffDirectories(dirA, dirB)
	if err != nil {
		t.Fatalf("diffDirectories: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("want 3 files, got %d", len(files))
	}
	if files[0].RelPath != "a.go" || files[1].RelPath != "b.go" || files[2].RelPath != "c.go" {
		t.Errorf("files should be sorted, got %v", []string{files[0].RelPath, files[1].RelPath, files[2].RelPath})
	}
}

func TestDiffDirectoriesInvalidPath(t *testing.T) {
	_, err := diffDirectories("/nonexistent/path/abc", t.TempDir())
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestDiffCmdRegistered(t *testing.T) {
	// Verify the diff subcommand exists on rootCmd.
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "diff" {
			found = true
			break
		}
	}
	if !found {
		t.Error("diff command not registered on rootCmd")
	}
}

func TestDiffCmdFlags(t *testing.T) {
	f := diffCmd.Flags().Lookup("format")
	if f == nil {
		t.Fatal("format flag not registered")
	}
	if f.DefValue != "tui" {
		t.Errorf("format default = %q, want %q", f.DefValue, "tui")
	}

	l := diffCmd.Flags().Lookup("lang")
	if l == nil {
		t.Fatal("lang flag not registered")
	}
}
