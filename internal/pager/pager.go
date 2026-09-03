// Package pager manages launching and streaming output to terminal pagers.
package pager

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/HarshK97/diffmantic/internal/git"
	"github.com/mattn/go-isatty"
)

// Pager manages an active terminal pager subprocess.
type Pager struct {
	cmd    *exec.Cmd
	writer io.WriteCloser
}

// IsBrokenPipe reports whether an error indicates a broken pipe or closed pipe.
func IsBrokenPipe(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.ErrClosedPipe) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "broken pipe") || strings.Contains(msg, "closed pipe")
}

// Start launches a pager (evaluating $GIT_PAGER -> core.pager -> $PAGER -> less -RFX)
// when stdout is a terminal and disabled is false.
// Returns a Pager handle (or nil if no pager was started) and the io.Writer to write to.
func Start(disabled bool) (*Pager, io.Writer) {
	if disabled || !isatty.IsTerminal(os.Stdout.Fd()) || os.Getenv("TERM") == "dumb" || os.Getenv("DIFFM_NO_PAGER") != "" {
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

	return &Pager{cmd: cmd, writer: stdin}, stdin
}

// Close closes the pager input stream and waits for the subprocess to finish.
func (p *Pager) Close() {
	if p == nil {
		return
	}
	if p.writer != nil {
		_ = p.writer.Close()
	}
	if p.cmd != nil {
		_ = p.cmd.Wait()
	}
}

func resolvePagerPreference() string {
	if val := strings.TrimSpace(os.Getenv("GIT_PAGER")); val != "" {
		return val
	}
	if out, err := git.RunGit(".", "config", "core.pager"); err == nil {
		if val := strings.TrimSpace(string(out)); val != "" {
			return val
		}
	}
	if val := strings.TrimSpace(os.Getenv("PAGER")); val != "" {
		return val
	}
	return "less -RFX"
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
	escaped := false

	for _, r := range s {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		if r == '\\' && !inSingle {
			escaped = true
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
