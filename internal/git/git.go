package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// GitFile represents a modified or untracked file in the repository.
type GitFile struct {
	Path     string
	OldPath  string // Non-empty if the file was renamed
	Status   string // 2-character porcelain status code (e.g. " M", "M ", "??")
	Staged   bool   // True if there are staged changes
	Unstaged bool   // True if there are unstaged changes or untracked
}

var (
	sandboxDetected bool
	sandboxMu       sync.RWMutex
)

func isSandboxPermissionError(stderr string) bool {
	return strings.Contains(stderr, "unable to access") ||
		strings.Contains(stderr, "Operation not permitted") ||
		strings.Contains(stderr, "Permission denied") ||
		strings.Contains(stderr, ".gitconfig")
}

// RunGit runs a git command in the specified directory.
func RunGit(cwd string, args ...string) ([]byte, error) {
	sandboxMu.RLock()
	isSandboxed := sandboxDetected
	sandboxMu.RUnlock()

	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	if isSandboxed {
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	} else {
		cmd.Env = os.Environ()
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		errOutput := strings.TrimSpace(stderr.String())
		// If we are not yet marked as sandboxed and the failure is due to gitconfig/sandbox permission restrictions, retry once and cache.
		if !isSandboxed && isSandboxPermissionError(errOutput) {
			retryCmd := exec.Command("git", args...)
			retryCmd.Dir = cwd
			retryCmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
			var retryStdout, retryStderr bytes.Buffer
			retryCmd.Stdout = &retryStdout
			retryCmd.Stderr = &retryStderr
			if retryErr := retryCmd.Run(); retryErr == nil {
				sandboxMu.Lock()
				sandboxDetected = true
				sandboxMu.Unlock()
				return retryStdout.Bytes(), nil
			}
			retryErrOutput := strings.TrimSpace(retryStderr.String())
			if retryErrOutput != "" {
				errOutput = retryErrOutput
			}
		}
		return nil, fmt.Errorf("git %s failed: %s (%w)", strings.Join(args, " "), errOutput, err)
	}
	return stdout.Bytes(), nil
}

// IsGitRepository checks if the directory is a Git repository.
func IsGitRepository(cwd string) bool {
	_, err := RunGit(cwd, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

// IsValidRevision checks if a string is a valid Git revision.
func IsValidRevision(cwd, ref string) bool {
	if ref == "" {
		return false
	}
	_, err := RunGit(cwd, "rev-parse", "--verify", ref)
	return err == nil
}

// IsTrackedFile checks if a path is tracked in the Git repository index or tree.
func IsTrackedFile(cwd, path string) bool {
	if path == "" {
		return false
	}
	_, err := RunGit(cwd, "ls-files", "--error-unmatch", "--", path)
	return err == nil
}

// GetStatus returns the Git status of the repository.
func GetStatus(cwd string, pathFilter string) ([]GitFile, error) {
	args := []string{"status", "--porcelain=v1"}
	if pathFilter != "" {
		args = append(args, "--", pathFilter)
	}
	out, err := RunGit(cwd, args...)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(out), "\n")
	var files []GitFile

	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		status := line[0:2]
		pathPart := strings.Trim(line[3:], "\"")

		gitFile := GitFile{
			Status: status,
		}

		x := status[0]
		y := status[1]

		if x != ' ' && x != '?' && x != '!' {
			gitFile.Staged = true
		}
		if y != ' ' {
			gitFile.Unstaged = true
		}

		// Handle renames (e.g. R old -> new).
		if x == 'R' || x == 'C' {
			parts := strings.Split(pathPart, " -> ")
			if len(parts) == 2 {
				oldPath := strings.Trim(parts[0], "\"")
				newPath := strings.Trim(parts[1], "\"")
				gitFile.OldPath = oldPath
				gitFile.Path = newPath
			} else {
				gitFile.Path = pathPart
			}
		} else {
			gitFile.Path = pathPart
		}

		files = append(files, gitFile)
	}

	return files, nil
}

// GetChangedFiles lists files changed between refA and refB.
// If refA is empty, it returns current working directory status.
// If refB is empty, it compares refA against the working tree.
func GetChangedFiles(cwd, refA, refB string, pathFilter string) ([]GitFile, error) {
	if refA == "" {
		return GetStatus(cwd, pathFilter)
	}

	var args []string
	args = append(args, "diff", "--name-status", "-M")
	args = append(args, refA)
	if refB != "" {
		args = append(args, refB)
	}
	if pathFilter != "" {
		args = append(args, "--", pathFilter)
	}

	out, err := RunGit(cwd, args...)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(out), "\n")
	var files []GitFile

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}

		for i := range parts {
			parts[i] = strings.Trim(parts[i], "\"")
		}

		status := parts[0]
		pathPart := parts[1]

		gitFile := GitFile{
			Status:   status,
			Staged:   false,
			Unstaged: false,
		}

		if strings.HasPrefix(status, "R") && len(parts) >= 3 {
			gitFile.OldPath = parts[1]
			gitFile.Path = parts[2]
		} else {
			gitFile.Path = pathPart
		}

		files = append(files, gitFile)
	}

	return files, nil
}

// StageFile stages a file (git add).
func StageFile(cwd, path string) error {
	_, err := RunGit(cwd, "add", path)
	return err
}

// UnstageFile unstages a file (git reset HEAD).
func UnstageFile(cwd, path string) error {
	_, err := RunGit(cwd, "reset", "HEAD", "--", path)
	return err
}

// Commit commits staged changes.
func Commit(cwd, msg string) error {
	_, err := RunGit(cwd, "commit", "-m", msg)
	return err
}

// GetRepoPrefix returns the path prefix of cwd relative to the Git repository worktree root.
func GetRepoPrefix(cwd string) (string, error) {
	out, err := RunGit(cwd, "rev-parse", "--show-prefix")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// GetContent reads a file at a given revision (e.g. "HEAD", a hash, or ":" for index).
// If revision is empty, it reads the local file on disk.
func GetContent(cwd, path, revision string) ([]byte, error) {
	if revision == "" {
		data, err := os.ReadFile(filepath.Join(cwd, path))
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		return data, nil
	}

	cleanPath := filepath.ToSlash(filepath.Clean(path))
	if prefix, err := GetRepoPrefix(cwd); err == nil && prefix != "" {
		if !filepath.IsAbs(path) && !strings.HasPrefix(cleanPath, prefix) {
			cleanPath = filepath.ToSlash(filepath.Join(prefix, cleanPath))
		}
	}

	var cmdArg string
	if revision == ":" {
		cmdArg = ":" + cleanPath
	} else {
		cmdArg = revision + ":" + cleanPath
	}

	out, err := RunGit(cwd, "show", cmdArg)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "does not exist in") ||
			strings.Contains(errStr, "exists on disk, but not in") ||
			strings.Contains(errStr, "fatal: Path") ||
			strings.Contains(errStr, "fatal: path") ||
			strings.Contains(errStr, "fatal: invalid object name") {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}
