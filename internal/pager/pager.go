// Package pager manages launching and streaming output to terminal pagers.
package pager

import (
	"cmp"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/HarshK97/diffmantic/internal/git"
)

// Pager manages an active terminal pager subprocess.
type Pager struct {
	cmd      *exec.Cmd
	writer   io.WriteCloser
	doneChan chan struct{}
	waitErr  error
}

// IsBrokenPipe reports whether an error indicates a broken pipe or closed pipe.
func IsBrokenPipe(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.ErrClosedPipe) || errors.Is(err, syscall.EPIPE) || errors.Is(err, os.ErrClosed) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "closed pipe") ||
		strings.Contains(msg, "file already closed") ||
		strings.Contains(msg, "pipe has been ended") ||
		strings.Contains(msg, "pipe is being closed")
}

// Start launches a pager (evaluating $DIFFM_PAGER -> $GIT_PAGER -> core.pager -> $PAGER -> less -RFX)
// when stdout is a terminal and disabled is false.
// Returns a Pager handle (or nil if no pager was started) and the io.Writer to write to.
func Start(disabled bool) (*Pager, io.Writer) {
	fi, err := os.Stdout.Stat()
	isTerm := err == nil && (fi.Mode()&os.ModeCharDevice) != 0
	if disabled || !isTerm || os.Getenv("TERM") == "dumb" || os.Getenv("DIFFM_NO_PAGER") != "" {
		return nil, os.Stdout
	}

	pref := resolvePagerPreference()
	if pref == "cat" || pref == "" {
		return nil, os.Stdout
	}

	pagerCmd, args := resolvePagerCmd(pref)

	cmd := exec.Command(pagerCmd, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, os.Stdout
	}

	if err := cmd.Start(); err != nil {
		return nil, os.Stdout
	}

	p := &Pager{
		cmd:      cmd,
		writer:   stdin,
		doneChan: make(chan struct{}),
	}

	go func() {
		p.waitErr = cmd.Wait()
		close(p.doneChan)
	}()

	return p, stdin
}

// Close closes the pager input stream and waits for the subprocess to finish.
func (p *Pager) Close() {
	if p == nil {
		return
	}
	if p.writer != nil {
		_ = p.writer.Close()
		p.writer = nil
	}
	if p.doneChan != nil {
		<-p.doneChan
	}
}

// IsActive reports whether the pager subprocess is still alive.
func (p *Pager) IsActive() bool {
	if p == nil || p.doneChan == nil {
		return false
	}
	select {
	case <-p.doneChan:
		return false
	default:
		return true
	}
}

// Done returns a channel that is closed when the pager subprocess exits.
// If p is nil, Done returns nil so it never fires in select statements.
// If the pager subprocess has already exited, the returned channel is closed and unblocks immediately.
func (p *Pager) Done() <-chan struct{} {
	if p == nil {
		return nil
	}
	return p.doneChan
}

func isSelfPager(val string) bool {
	parts := splitArgs(val)
	if len(parts) == 0 {
		return false
	}
	raw := parts[0]
	if idx := strings.LastIndexAny(raw, `/\`); idx != -1 {
		raw = raw[idx+1:]
	}
	base := strings.TrimSuffix(raw, ".exe")

	var self string
	if len(os.Args) > 0 {
		rawSelf := os.Args[0]
		if idx := strings.LastIndexAny(rawSelf, `/\`); idx != -1 {
			rawSelf = rawSelf[idx+1:]
		}
		self = strings.TrimSuffix(rawSelf, ".exe")
	}
	return base == "diffm" || base == "diffmantic" || (self != "" && base == self)
}

func resolvePagerPreference() string {
	candidate := func(raw string) string {
		val := strings.TrimSpace(raw)
		if val != "" && !isSelfPager(val) {
			return val
		}
		return ""
	}

	if p := cmp.Or(candidate(os.Getenv("DIFFM_PAGER")), candidate(os.Getenv("GIT_PAGER"))); p != "" {
		return p
	}
	if out, err := git.RunGit(".", "config", "core.pager"); err == nil {
		if p := candidate(string(out)); p != "" {
			return p
		}
	}
	return cmp.Or(candidate(os.Getenv("PAGER")), "less -RFX")
}

func resolvePagerCmd(pagerEnv string) (string, []string) {
	pagerEnv = strings.TrimSpace(pagerEnv)
	if pagerEnv == "" {
		return "less", []string{"-RFX"}
	}
	parts := splitArgs(pagerEnv)
	if len(parts) == 0 {
		return "less", []string{"-RFX"}
	}
	if len(parts) == 1 && parts[0] == "less" {
		return "less", []string{"-RFX"}
	}
	return parts[0], parts[1:]
}

// splitArgs splits a command string into words, respecting single and double quotes and backslash escapes.
func splitArgs(s string) []string {
	var args []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\\' && !inSingle {
			if i+1 < len(runes) {
				next := runes[i+1]
				if next == ' ' || next == '\t' || next == '"' || next == '\'' || next == '\\' {
					current.WriteRune(next)
					i++
					continue
				}
			}
			current.WriteRune(r)
			continue
		}

		if r == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}

		if r == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}

		if (r == ' ' || r == '\t' || r == '\n') && !inSingle && !inDouble {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}
