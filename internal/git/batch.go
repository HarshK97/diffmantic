package git

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// ErrBatchSessionPoisoned is returned when repeated errors poison the batch session.
var ErrBatchSessionPoisoned = errors.New("git batch session is poisoned after repeated errors")

// BatchSession keeps a git cat-file --batch process open to fetch blobs
// sequentially without spawning a new process per object.
type BatchSession struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   *bufio.Reader
	doneChan chan struct{}
	waitErr  error

	mu                  sync.Mutex
	consecutiveFailures int
	poisoned            bool
	closed              bool
}

// StartBatchSession starts a git cat-file --batch process in cwd.
func StartBatchSession(cwd string) (*BatchSession, error) {
	sandboxMu.RLock()
	isSandboxed := sandboxDetected
	sandboxMu.RUnlock()

	cmd := exec.Command("git", "cat-file", "--batch")
	cmd.Dir = cwd
	if isSandboxed {
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	} else {
		cmd.Env = os.Environ()
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe for git cat-file: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("creating stdout pipe for git cat-file: %w", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("starting git cat-file --batch: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	session := &BatchSession{
		cmd:      cmd,
		stdin:    stdin,
		stdout:   bufio.NewReader(stdout),
		doneChan: make(chan struct{}),
	}

	go func() {
		session.waitErr = cmd.Wait()
		close(session.doneChan)
	}()

	return session, nil
}

// Fetch gets the raw blob for an object ID. Returns (nil, nil) if the
// object is missing so the caller can fall back to disk.
func (s *BatchSession) Fetch(oid string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, errors.New("batch session is closed")
	}
	if s.poisoned {
		return nil, ErrBatchSessionPoisoned
	}

	oid = strings.TrimSpace(oid)
	if oid == "" {
		return nil, nil
	}
	if strings.ContainsAny(oid, "\r\n") {
		return nil, errors.New("invalid oid containing newline")
	}

	recordFailure := func(err error) error {
		s.consecutiveFailures++
		if s.consecutiveFailures >= 3 {
			s.poisoned = true
			if s.cmd.Process != nil {
				_ = s.cmd.Process.Kill()
			}
		}
		return err
	}

	if _, err := io.WriteString(s.stdin, oid+"\n"); err != nil {
		return nil, recordFailure(fmt.Errorf("sending oid %s to batch session: %w", oid, err))
	}

	header, err := s.stdout.ReadString('\n')
	if err != nil {
		return nil, recordFailure(fmt.Errorf("reading batch header for %s: %w", oid, err))
	}

	header = strings.TrimRight(header, "\r\n")

	if strings.HasSuffix(header, " missing") || strings.HasSuffix(header, " ambiguous") {
		s.consecutiveFailures = 0
		return nil, nil
	}

	fields := strings.Fields(header)
	if len(fields) < 3 {
		return nil, recordFailure(fmt.Errorf("malformed batch header %q for %s", header, oid))
	}

	size, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || size < 0 {
		return nil, recordFailure(fmt.Errorf("invalid size %q in batch header for %s: %w", fields[2], oid, err))
	}

	buf := make([]byte, size)
	if _, err := io.ReadFull(s.stdout, buf); err != nil {
		return nil, recordFailure(fmt.Errorf("reading payload (%d bytes) for %s: %w", size, oid, err))
	}

	// git cat-file appends a trailing newline after each object payload.
	trailing, err := s.stdout.ReadByte()
	if err != nil {
		return nil, recordFailure(fmt.Errorf("reading trailing newline for %s: %w", oid, err))
	}
	if trailing != '\n' {
		return nil, recordFailure(fmt.Errorf("expected newline after payload for %s, got byte %q", oid, trailing))
	}

	s.consecutiveFailures = 0
	return buf, nil
}

// IsPoisoned reports whether the session was shut down after repeated errors.
func (s *BatchSession) IsPoisoned() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.poisoned
}

// Close stops the batch process and waits for it to exit.
func (s *BatchSession) Close() error {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		if s.poisoned && s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
		if s.stdin != nil {
			_ = s.stdin.Close()
		}
	}
	s.mu.Unlock()

	<-s.doneChan
	return s.waitErr
}
