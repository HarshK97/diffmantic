package git

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitFile represents a modified or untracked file in the repository.
type GitFile struct {
	Path     string
	OldPath  string // Non-empty if the file was renamed
	Status   string // 2-character porcelain status code (e.g. " M", "M ", "??")
	Staged   bool   // True if there are staged changes
	Unstaged bool   // True if there are unstaged changes or untracked
}

// RunGit runs a git command in the specified directory.
func RunGit(cwd string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, &GitError{
			Args:   args,
			Stderr: stderr.String(),
			Err:    err,
		}
	}
	return stdout.Bytes(), nil
}

type GitError struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *GitError) Error() string {
	return "git " + strings.Join(e.Args, " ") + " failed: " + e.Stderr + " (" + e.Err.Error() + ")"
}

// IsGitRepository checks if the directory is a Git repository.
func IsGitRepository(cwd string) bool {
	_, err := RunGit(cwd, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

// GetStatus returns the Git status of the repository.
func GetStatus(cwd string) ([]GitFile, error) {
	out, err := RunGit(cwd, "status", "--porcelain=v1")
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
		pathPart := line[3:]

		// Handle double quotes if git escaped the path.
		if strings.HasPrefix(pathPart, "\"") && strings.HasSuffix(pathPart, "\"") {
			pathPart = pathPart[1 : len(pathPart)-1]
		}

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
				oldPath := parts[0]
				newPath := parts[1]
				if strings.HasPrefix(oldPath, "\"") && strings.HasSuffix(oldPath, "\"") {
					oldPath = oldPath[1 : len(oldPath)-1]
				}
				if strings.HasPrefix(newPath, "\"") && strings.HasSuffix(newPath, "\"") {
					newPath = newPath[1 : len(newPath)-1]
				}
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
func GetChangedFiles(cwd, refA, refB string) ([]GitFile, error) {
	if refA == "" {
		return GetStatus(cwd)
	}

	var args []string
	args = append(args, "diff", "--name-status", "-M")
	args = append(args, refA)
	if refB != "" {
		args = append(args, refB)
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

		// Clean quotes if present.
		for i := range parts {
			if strings.HasPrefix(parts[i], "\"") && strings.HasSuffix(parts[i], "\"") {
				parts[i] = parts[i][1 : len(parts[i])-1]
			}
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

	var cmdArg string
	if revision == ":" {
		cmdArg = ":" + path
	} else {
		cmdArg = revision + ":" + path
	}

	out, err := RunGit(cwd, "show", cmdArg)
	if err != nil {
		// New or deleted files won't exist in HEAD/index, so treat errors as empty.
		return nil, nil
	}
	return out, nil
}
