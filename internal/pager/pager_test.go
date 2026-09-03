package pager

import (
	"errors"
	"io"
	"os"
	"slices"
	"syscall"
	"testing"
)

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
	if !IsBrokenPipe(errors.New("write: broken pipe")) {
		t.Error("expected broken pipe string error to be broken pipe")
	}
	if IsBrokenPipe(errors.New("some other error")) {
		t.Error("expected other error to not be broken pipe")
	}
}
