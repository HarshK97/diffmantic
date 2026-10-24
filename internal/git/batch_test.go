package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupBatchRepo(t *testing.T) (string, func(args ...string) string) {
	t.Helper()
	repoDir := t.TempDir()
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
	return repoDir, execGit
}

func TestBatchSession_Lockstep(t *testing.T) {
	repoDir, execGit := setupBatchRepo(t)

	f1Content := "hello file one"
	f2Content := "world file two with more content\nsecond line"
	f3Content := ""

	if err := os.WriteFile(filepath.Join(repoDir, "f1.txt"), []byte(f1Content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "f2.txt"), []byte(f2Content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "f3.txt"), []byte(f3Content), 0o600); err != nil {
		t.Fatal(err)
	}

	execGit("add", "f1.txt", "f2.txt", "f3.txt")
	execGit("-c", "commit.gpgsign=false", "-c", "user.email=t@test.com", "-c", "user.name=test", "commit", "-m", "init")

	oid1 := execGit("rev-parse", "HEAD:f1.txt")
	oid2 := execGit("rev-parse", "HEAD:f2.txt")
	oid3 := execGit("rev-parse", "HEAD:f3.txt")

	session, err := StartBatchSession(repoDir)
	if err != nil {
		t.Fatalf("StartBatchSession failed: %v", err)
	}
	defer func() { _ = session.Close() }()

	b1, err := session.Fetch(oid1)
	if err != nil {
		t.Fatalf("Fetch f1 failed: %v", err)
	}
	if string(b1) != f1Content {
		t.Errorf("Fetch f1 = %q, want %q", string(b1), f1Content)
	}

	b2, err := session.Fetch(oid2)
	if err != nil {
		t.Fatalf("Fetch f2 failed: %v", err)
	}
	if string(b2) != f2Content {
		t.Errorf("Fetch f2 = %q, want %q", string(b2), f2Content)
	}

	b3, err := session.Fetch(oid3)
	if err != nil {
		t.Fatalf("Fetch f3 failed: %v", err)
	}
	if string(b3) != f3Content {
		t.Errorf("Fetch f3 = %q, want %q", string(b3), f3Content)
	}

	b1Again, err := session.Fetch(oid1)
	if err != nil {
		t.Fatalf("Fetch f1 again failed: %v", err)
	}
	if string(b1Again) != f1Content {
		t.Errorf("Fetch f1 again = %q, want %q", string(b1Again), f1Content)
	}
}

func TestBatchSession_Missing(t *testing.T) {
	repoDir, _ := setupBatchRepo(t)

	session, err := StartBatchSession(repoDir)
	if err != nil {
		t.Fatalf("StartBatchSession failed: %v", err)
	}
	defer func() { _ = session.Close() }()

	missingOID := "0123456789abcdef0123456789abcdef01234567"
	b, err := session.Fetch(missingOID)
	if err != nil {
		t.Fatalf("Fetch missing object returned error: %v", err)
	}
	if b != nil {
		t.Errorf("Fetch missing object returned non-nil bytes: %q", string(b))
	}

	if session.IsPoisoned() {
		t.Errorf("Session should not be poisoned by missing object queries")
	}
}

func TestBatchSession_CircuitBreaker(t *testing.T) {
	repoDir, _ := setupBatchRepo(t)

	session, err := StartBatchSession(repoDir)
	if err != nil {
		t.Fatalf("StartBatchSession failed: %v", err)
	}
	defer func() { _ = session.Close() }()

	// Close stdin so writes fail and trip the circuit breaker.
	if session.stdin != nil {
		_ = session.stdin.Close()
	}

	for i := 1; i <= 3; i++ {
		_, err := session.Fetch("HEAD")
		if err == nil {
			t.Fatalf("Expected fetch to fail on closed stdin, succeeded on iteration %d", i)
		}
	}

	if !session.IsPoisoned() {
		t.Errorf("Expected session to be poisoned after 3 consecutive failures")
	}

	_, err = session.Fetch("HEAD")
	if !errors.Is(err, ErrBatchSessionPoisoned) {
		t.Errorf("Expected ErrBatchSessionPoisoned on subsequent fetch, got: %v", err)
	}
}

func TestBatchSession_CloseIdempotent(t *testing.T) {
	repoDir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	session, err := StartBatchSession(repoDir)
	if err != nil {
		t.Fatalf("StartBatchSession failed: %v", err)
	}

	if err := session.Close(); err != nil {
		t.Errorf("First Close() failed: %v", err)
	}

	// Second Close() should be safe no-op
	if err := session.Close(); err != nil {
		t.Errorf("Second Close() failed: %v", err)
	}
}

func TestBatchSession_ConcurrentClose(t *testing.T) {
	repoDir, _ := setupBatchRepo(t)
	session, err := StartBatchSession(repoDir)
	if err != nil {
		t.Fatalf("StartBatchSession failed: %v", err)
	}

	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			done <- session.Close()
		}()
	}

	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent Close() returned error: %v", err)
		}
	}
}
