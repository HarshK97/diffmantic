package pager

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"syscall"
	"testing"
)

// TestMain clears pager env vars so host settings don't change test expectations.
func TestMain(m *testing.M) {
	for _, k := range []string{"DIFFM_NO_PAGER", "DIFFM_PAGER", "GIT_PAGER", "PAGER"} {
		_ = os.Unsetenv(k)
	}
	os.Exit(m.Run())
}

func TestPager_Disabled(t *testing.T) {
	p, w := Start(true)
	if p != nil {
		t.Fatalf("expected nil pager when disabled, got %v", p)
	}
	if w != os.Stdout {
		t.Fatalf("expected os.Stdout writer when disabled, got %v", w)
	}
	p.Close() // Safe no-op on nil
}

func TestPager_EnvDisabled(t *testing.T) {
	t.Setenv("DIFFM_NO_PAGER", "1")
	p, w := Start(false)
	if p != nil {
		t.Fatalf("expected nil pager when DIFFM_NO_PAGER=1, got %v", p)
	}
	if w != os.Stdout {
		t.Fatalf("expected os.Stdout writer when DIFFM_NO_PAGER=1, got %v", w)
	}
}

func TestResolvePagerCmd(t *testing.T) {
	tests := []struct {
		env      string
		wantCmd  string
		wantArgs []string
	}{
		{"", "less", []string{"-RFX"}},
		{"less", "less", []string{"-RFX"}},
		{"more", "more", nil},
		{"less -R", "less", []string{"-R"}},
		{"bat --paging=always", "bat", []string{"--paging=always"}},
		{"less -p 'foo bar'", "less", []string{"-p", "foo bar"}},
		{`pager --opt="with spaces"`, "pager", []string{`--opt=with spaces`}},
	}

	for _, tt := range tests {
		gotCmd, gotArgs := resolvePagerCmd(tt.env)
		if gotCmd != tt.wantCmd {
			t.Errorf("resolvePagerCmd(%q) gotCmd = %q, want %q", tt.env, gotCmd, tt.wantCmd)
		}
		if !slices.Equal(gotArgs, tt.wantArgs) {
			t.Errorf("resolvePagerCmd(%q) gotArgs = %v, want %v", tt.env, gotArgs, tt.wantArgs)
		}
	}
}

func TestIsBrokenPipe(t *testing.T) {
	if IsBrokenPipe(nil) {
		t.Error("expected nil error to not be broken pipe")
	}
	if !IsBrokenPipe(io.ErrClosedPipe) {
		t.Error("expected io.ErrClosedPipe to be broken pipe")
	}
	if !IsBrokenPipe(syscall.EPIPE) {
		t.Error("expected syscall.EPIPE to be broken pipe")
	}
	if !IsBrokenPipe(os.ErrClosed) {
		t.Error("expected os.ErrClosed to be broken pipe")
	}
	if !IsBrokenPipe(errors.New("write: broken pipe")) {
		t.Error("expected broken pipe string error to be broken pipe")
	}
	if !IsBrokenPipe(errors.New("write |1: file already closed")) {
		t.Error("expected file already closed string error to be broken pipe")
	}
	if !IsBrokenPipe(errors.New("write |1: The pipe has been ended.")) {
		t.Error("expected windows pipe has been ended error to be broken pipe")
	}
	if !IsBrokenPipe(errors.New("write |1: The pipe is being closed.")) {
		t.Error("expected windows pipe is being closed error to be broken pipe")
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create os.Pipe: %v", err)
	}
	_ = r.Close()
	_ = w.Close()
	_, writeErr := w.Write([]byte("test"))
	if !IsBrokenPipe(writeErr) {
		t.Errorf("expected write on closed pipe (%v) to be broken pipe", writeErr)
	}

	if IsBrokenPipe(errors.New("some other error")) {
		t.Error("expected other error to not be broken pipe")
	}
}

func TestIsBrokenPipe_OSKernelPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create os.Pipe: %v", err)
	}
	_ = r.Close()
	defer func() { _ = w.Close() }()

	var writeErr error
	for i := 0; i < 1000; i++ {
		_, writeErr = w.Write([]byte("test data to trigger kernel broken pipe\n"))
		if writeErr != nil {
			break
		}
	}
	if writeErr == nil {
		t.Fatal("expected write to pipe with closed reader to fail, got nil")
	}
	if !IsBrokenPipe(writeErr) {
		t.Errorf("expected kernel write error (%v) to be broken pipe on %s", writeErr, runtime.GOOS)
	}
}

func TestIsBrokenPipe_SubprocessExit(t *testing.T) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/c", "exit 0")
	} else {
		cmd = exec.Command("true")
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe failed: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start failed: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("cmd.Wait failed: %v", err)
	}

	_, writeErr := stdin.Write([]byte("payload after subprocess exit\n"))
	if writeErr == nil {
		t.Fatal("expected write after child exit to fail, got nil")
	}
	if !IsBrokenPipe(writeErr) {
		t.Errorf("expected write error after child exit (%v) to be broken pipe on %s", writeErr, runtime.GOOS)
	}
}

func TestPager_IsActive(t *testing.T) {
	var nilPager *Pager
	if nilPager.IsActive() {
		t.Error("expected nil pager to not be active")
	}

	emptyPager := &Pager{}
	if emptyPager.IsActive() {
		t.Error("expected pager with nil process to not be active")
	}

	activePager := &Pager{doneChan: make(chan struct{})}
	if !activePager.IsActive() {
		t.Error("expected pager with open doneChan to be active")
	}
	close(activePager.doneChan)
	if activePager.IsActive() {
		t.Error("expected pager with closed doneChan to not be active")
	}
}

func TestPager_Done(t *testing.T) {
	var nilPager *Pager
	if nilPager.Done() != nil {
		t.Error("expected nil pager Done() to be nil channel")
	}

	doneChan := make(chan struct{})
	activePager := &Pager{doneChan: doneChan}
	if activePager.Done() != doneChan {
		t.Error("expected active pager Done() to return doneChan")
	}
	close(doneChan)
	select {
	case <-activePager.Done():
		// ok
	default:
		t.Error("expected closed doneChan to be receivable")
	}
}

func TestResolvePagerPreference_Hierarchy(t *testing.T) {
	// Isolate host git config so core.pager from ~/.gitconfig doesn't leak into tests
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	// 1. DIFFM_PAGER takes highest precedence
	t.Setenv("DIFFM_PAGER", "diffm_custom")
	t.Setenv("GIT_PAGER", "git_custom")
	t.Setenv("PAGER", "pager_custom")
	if pref := resolvePagerPreference(); pref != "diffm_custom" {
		t.Errorf("Expected DIFFM_PAGER to win, got: %s", pref)
	}

	// 2. If DIFFM_PAGER is self-pager, falls back to GIT_PAGER
	t.Setenv("DIFFM_PAGER", "diffm")
	if pref := resolvePagerPreference(); pref != "git_custom" {
		t.Errorf("Expected self DIFFM_PAGER to fall back to GIT_PAGER, got: %s", pref)
	}

	// 3. If GIT_PAGER is self-pager, falls back past it
	t.Setenv("GIT_PAGER", "diffmantic")
	if pref := resolvePagerPreference(); pref != "pager_custom" {
		t.Errorf("Expected self GIT_PAGER to fall back to PAGER, got: %s", pref)
	}

	// 4. Default fallback when no env vars set
	t.Setenv("DIFFM_PAGER", "")
	t.Setenv("GIT_PAGER", "")
	t.Setenv("PAGER", "")
	if pref := resolvePagerPreference(); pref != "less -RFX" {
		t.Errorf("Expected default less -RFX, got: %s", pref)
	}
}

func TestIsSelfPager(t *testing.T) {
	tests := []struct {
		val  string
		want bool
	}{
		{"", false},
		{"less -RFX", false},
		{"bat --plain", false},
		{"diffm", true},
		{"diffmantic", true},
		{"diffm -f side-by-side", true},
		{"diffm.exe", true},
		{"diffmantic.exe", true},
		{"/usr/local/bin/diffm", true},
		{`C:\bin\diffm.exe`, true},
	}

	for _, tt := range tests {
		t.Run(tt.val, func(t *testing.T) {
			if got := isSelfPager(tt.val); got != tt.want {
				t.Errorf("isSelfPager(%q) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}
