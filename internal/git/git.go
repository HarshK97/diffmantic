package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	IsBinary bool   // True if the file was identified as binary
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

func getBinaryFiles(cwd string, args ...string) map[string]bool {
	numstatArgs := append([]string{"diff", "--numstat"}, args...)
	out, err := RunGit(cwd, numstatArgs...)
	if err != nil {
		return nil
	}
	binaryMap := make(map[string]bool)
	for line := range strings.SplitSeq(string(out), "\n") {
		if rawPath, ok := strings.CutPrefix(line, "-\t-\t"); ok {
			rawPath = strings.Trim(strings.TrimSpace(rawPath), "\"")
			binaryMap[rawPath] = true
		}
	}
	return binaryMap
}

// GetStatus returns the Git status of the repository.
func GetStatus(cwd string, pathFilter string) ([]GitFile, error) {
	args := []string{"status", "--porcelain=v1", "-uall"}
	if pathFilter != "" {
		args = append(args, "--", pathFilter)
	}
	out, err := RunGit(cwd, args...)
	if err != nil {
		return nil, err
	}

	binMap := make(map[string]bool)
	var binFilter []string
	if pathFilter != "" {
		binFilter = []string{"--", pathFilter}
	}
	for _, extra := range [][]string{nil, {"--cached"}} {
		diffArgs := append(append([]string{"diff", "--numstat"}, extra...), binFilter...)
		if binOut, binErr := RunGit(cwd, diffArgs...); binErr == nil {
			for line := range strings.SplitSeq(string(binOut), "\n") {
				if rawPath, ok := strings.CutPrefix(line, "-\t-\t"); ok {
					rawPath = strings.Trim(strings.TrimSpace(rawPath), "\"")
					binMap[rawPath] = true
				}
			}
		}
	}

	lines := strings.Split(string(out), "\n")
	var files []GitFile

	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		status := line[0:2]
		pathPart := strings.Trim(line[3:], "\"")
		if strings.HasSuffix(pathPart, "/") {
			continue
		}
		if info, err := os.Stat(filepath.Join(cwd, pathPart)); err == nil && info.IsDir() {
			continue
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

		if binMap != nil {
			gitFile.IsBinary = binMap[gitFile.Path] || binMap[pathPart]
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

	numstatArgs := []string{"-M", refA}
	if refB != "" {
		numstatArgs = append(numstatArgs, refB)
	}
	if pathFilter != "" {
		numstatArgs = append(numstatArgs, "--", pathFilter)
	}
	binMap := getBinaryFiles(cwd, numstatArgs...)

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

		if binMap != nil {
			gitFile.IsBinary = binMap[gitFile.Path] || binMap[pathPart]
		}

		files = append(files, gitFile)
	}

	return files, nil
}

// GetRepoPrefix returns the path prefix of cwd relative to the Git repository worktree root.
func GetRepoPrefix(cwd string) (string, error) {
	out, err := RunGit(cwd, "rev-parse", "--show-prefix")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

var (
	textconvCacheMu sync.RWMutex
	textconvCache   = make(map[string]string) // cacheKey (cleanCwd + "\x00" + driver) -> cmdStr
)

func getDriverTextconv(cwd, driver string) string {
	cacheKey := filepath.Clean(cwd) + "\x00" + driver
	textconvCacheMu.RLock()
	cmdStr, found := textconvCache[cacheKey]
	textconvCacheMu.RUnlock()
	if found {
		return cmdStr
	}

	textconvCacheMu.Lock()
	defer textconvCacheMu.Unlock()
	if cmdStr, found := textconvCache[cacheKey]; found {
		return cmdStr
	}

	configOut, err := RunGit(cwd, "config", fmt.Sprintf("diff.%s.textconv", driver))
	if err != nil {
		textconvCache[cacheKey] = ""
		return ""
	}
	cmdStr = strings.TrimSpace(string(configOut))
	textconvCache[cacheKey] = cmdStr
	return cmdStr
}

// splitCommandWords splits a command into words, preserving quoted tokens and escapes.
func splitCommandWords(s string) []string {
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

		if r == '\\' && runtime.GOOS != "windows" && !inSingle {
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

// applyWorkingTreeTextconv runs Git's textconv filter on a working tree file if configured.
func applyWorkingTreeTextconv(cwd, path, cleanPath string) ([]byte, bool, error) {
	attrOut, err := RunGit(cwd, "check-attr", "diff", cleanPath)
	if err != nil {
		return nil, false, err
	}
	line := strings.TrimSpace(string(attrOut))
	_, driver, ok := strings.Cut(line, ": diff: ")
	if !ok {
		return nil, false, nil
	}
	driver = strings.TrimSpace(driver)
	if driver == "" || driver == "unspecified" || driver == "unset" || driver == "set" {
		return nil, false, nil
	}

	cmdStr := getDriverTextconv(cwd, driver)
	if cmdStr == "" {
		return nil, false, nil
	}

	fullFilePath := filepath.Join(cwd, path)
	if _, statErr := os.Stat(fullFilePath); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, false, nil
		}
		return nil, false, statErr
	}

	var c *exec.Cmd
	if strings.ContainsAny(cmdStr, "|><&;") {
		c = exec.Command("sh", "-c", cmdStr+" \"$1\"", "sh", fullFilePath)
	} else {
		cmdParts := splitCommandWords(cmdStr)
		if len(cmdParts) == 0 {
			return nil, false, nil
		}
		c = exec.Command(cmdParts[0], append(cmdParts[1:], fullFilePath)...)
	}
	c.Dir = cwd
	out, err := c.Output()
	if err != nil {
		return nil, true, fmt.Errorf("running textconv filter for %s: %w", path, err)
	}
	return out, true, nil
}

// GetContent reads a file at a given revision (e.g. "HEAD", a hash, or ":" for index).
// If revision is empty, it reads the local file on disk (applying textconv filter only if useTextconv is true).
func GetContent(cwd, path, revision string, useTextconv bool) ([]byte, error) {
	cleanPath := filepath.ToSlash(filepath.Clean(path))
	if prefix, err := GetRepoPrefix(cwd); err == nil && prefix != "" {
		if !filepath.IsAbs(path) && !strings.HasPrefix(cleanPath, prefix) {
			cleanPath = filepath.ToSlash(filepath.Join(prefix, cleanPath))
		}
	}

	if revision == "" {
		if useTextconv {
			data, applicable, err := applyWorkingTreeTextconv(cwd, path, cleanPath)
			if applicable {
				if err != nil {
					return nil, err
				}
				return data, nil
			}
		}
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
		cmdArg = ":" + cleanPath
	} else {
		cmdArg = revision + ":" + cleanPath
	}

	showArgs := []string{"show"}
	if useTextconv {
		showArgs = append(showArgs, "--textconv")
	}
	showArgs = append(showArgs, cmdArg)

	out, err := RunGit(cwd, showArgs...)
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

// ListRefs returns Git revisions for shell completion, optionally filtered by prefix.
func ListRefs(cwd string, toComplete string) ([]string, error) {
	gitDirOut, err := RunGit(cwd, "rev-parse", "--git-dir")
	if err != nil {
		return nil, err
	}
	gitDir := strings.TrimSpace(string(gitDirOut))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(cwd, gitDir)
	}

	var refs []string
	seen := make(map[string]bool)

	// Include HEAD and common symbolic refs if they match the filter and exist on disk
	for _, sym := range []string{"HEAD", "FETCH_HEAD", "ORIG_HEAD"} {
		if toComplete != "" && !strings.HasPrefix(sym, toComplete) {
			continue
		}
		symPath := filepath.Join(gitDir, sym)
		if fi, err := os.Stat(symPath); err == nil && !fi.IsDir() {
			if IsValidRevision(cwd, sym) {
				refs = append(refs, sym)
				seen[sym] = true
			}
		}
	}

	args := []string{"for-each-ref", "--format=%(refname)\t%(refname:short)"}
	if toComplete != "" {
		if strings.HasPrefix(toComplete, "refs/") {
			args = append(args, toComplete+"*")
		} else {
			pattern := toComplete + "*"
			args = append(args, "refs/heads/"+pattern, "refs/tags/"+pattern, "refs/remotes/"+pattern)
		}
	} else {
		args = append(args, "refs/heads/", "refs/tags/", "refs/remotes/")
	}

	out, err := RunGit(cwd, args...)
	if err != nil {
		return nil, err
	}

	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fullName, shortName, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		if strings.HasSuffix(fullName, "/HEAD") {
			continue
		}
		if seen[shortName] {
			continue
		}
		seen[shortName] = true
		refs = append(refs, shortName)
	}

	return refs, nil
}
