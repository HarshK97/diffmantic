package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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
	sandboxDetected  bool
	sandboxMu        sync.RWMutex
	cacheMu          sync.RWMutex
	isGitRepoCache   = make(map[string]bool)
	validRevCache    = make(map[string]bool)
	trackedFileCache = make(map[string]bool)
	repoPrefixCache  = make(map[string]string)
)

// ClearCache resets cached repo metadata. Mainly used to isolate test runs.
func ClearCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	clear(isGitRepoCache)
	clear(validRevCache)
	clear(trackedFileCache)
	clear(repoPrefixCache)

	textconvCacheMu.Lock()
	clear(textconvCache)
	textconvCacheMu.Unlock()

	diffAttrCacheMu.Lock()
	clear(diffAttrCache)
	diffAttrCacheMu.Unlock()
}

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
	cleanCwd := filepath.Clean(cwd)
	cacheMu.RLock()
	val, ok := isGitRepoCache[cleanCwd]
	cacheMu.RUnlock()
	if ok {
		return val
	}

	_, err := RunGit(cwd, "rev-parse", "--is-inside-work-tree")
	isRepo := (err == nil)

	cacheMu.Lock()
	isGitRepoCache[cleanCwd] = isRepo
	cacheMu.Unlock()
	return isRepo
}

// IsNullOID reports whether s is non-empty and consists entirely of '0' characters (a Git null OID).
func IsNullOID(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := range len(s) {
		if s[i] != '0' {
			return false
		}
	}
	return true
}

// IsValidRevision checks if a string is a valid Git revision.
func IsValidRevision(cwd, ref string) bool {
	if ref == "" || IsNullOID(ref) {
		return false
	}
	cleanKey := filepath.Clean(cwd) + "\x00" + ref
	cacheMu.RLock()
	val, ok := validRevCache[cleanKey]
	cacheMu.RUnlock()
	if ok {
		return val
	}

	verifyArg := ref
	if !strings.Contains(ref, ":") {
		verifyArg = ref + "^{object}"
	}
	_, err := RunGit(cwd, "rev-parse", "--verify", verifyArg)
	isValid := (err == nil)

	cacheMu.Lock()
	validRevCache[cleanKey] = isValid
	cacheMu.Unlock()
	return isValid
}

// IsTrackedFile checks if a path is tracked in the Git repository index or tree.
func IsTrackedFile(cwd, path string) bool {
	if path == "" {
		return false
	}
	cleanKey := filepath.Clean(cwd) + "\x00" + filepath.Clean(path)
	cacheMu.RLock()
	val, ok := trackedFileCache[cleanKey]
	cacheMu.RUnlock()
	if ok {
		return val
	}

	_, err := RunGit(cwd, "ls-files", "--error-unmatch", "--", path)
	isTracked := (err == nil)

	cacheMu.Lock()
	trackedFileCache[cleanKey] = isTracked
	cacheMu.Unlock()
	return isTracked
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
	if len(bytes.TrimSpace(out)) == 0 {
		return nil, nil
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
	if len(bytes.TrimSpace(out)) == 0 {
		return nil, nil
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
	cleanCwd := filepath.Clean(cwd)
	cacheMu.RLock()
	val, ok := repoPrefixCache[cleanCwd]
	cacheMu.RUnlock()
	if ok {
		return val, nil
	}

	out, err := RunGit(cwd, "rev-parse", "--show-prefix")
	if err != nil {
		return "", err
	}
	prefix := strings.TrimSpace(string(out))

	cacheMu.Lock()
	repoPrefixCache[cleanCwd] = prefix
	cacheMu.Unlock()
	return prefix, nil
}

// FindRepoRoot walks up from cwd looking for a .git directory or file
// (for worktrees and submodules). Returns an empty string if not in a git repo.
func FindRepoRoot(cwd string) string {
	dir := filepath.Clean(cwd)
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}

	for {
		gitPath := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
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

// DiffAttr represents the diff behavior specified in .gitattributes for a file.
type DiffAttr int

const (
	// DiffAttrUnspecified means no diff attribute was set.
	DiffAttrUnspecified DiffAttr = iota
	// DiffAttrText forces text diffing (attribute 'diff').
	DiffAttrText
	// DiffAttrBinary forces binary treatment (attribute '-diff' or 'binary').
	DiffAttrBinary
)

// FileDiffAttr holds gitattributes diff settings for a file.
type FileDiffAttr struct {
	Attr   DiffAttr // Configured diff behavior (unspecified, text, or binary).
	Driver string   // Custom diff driver name, if set (e.g. "pdfconv").
}

var (
	diffAttrCacheMu sync.RWMutex
	diffAttrCache   = make(map[string]FileDiffAttr) // cacheKey (cleanRepoRoot + "\x00" + cleanPath) -> FileDiffAttr
)

func diffAttrCacheKey(cleanRepoRoot, cwd, p string) string {
	absPath := p
	if !filepath.IsAbs(p) {
		absPath = filepath.Join(cwd, p)
	}
	return cleanRepoRoot + "\x00" + filepath.Clean(absPath)
}

// CheckDiffAttr queries Git for the diff and binary attributes of the given paths.
// If neither cwd nor any path is inside a Git repository, it returns an empty map without running git.
func CheckDiffAttr(cwd string, paths ...string) map[string]FileDiffAttr {
	if len(paths) == 0 {
		return make(map[string]FileDiffAttr)
	}

	repoRoot := FindRepoRoot(cwd)
	execDir := cwd
	if repoRoot == "" {
		for _, p := range paths {
			if p != "" && p != os.DevNull && p != "/dev/null" {
				targetPath := p
				if !filepath.IsAbs(p) {
					targetPath = filepath.Join(cwd, p)
				}
				dir := filepath.Dir(targetPath)
				if r := FindRepoRoot(dir); r != "" {
					repoRoot = r
					execDir = dir
					break
				}
			}
		}
	}
	if repoRoot == "" {
		return make(map[string]FileDiffAttr)
	}

	result := make(map[string]FileDiffAttr, len(paths))
	var missingPaths []string
	cleanRepoRoot := filepath.Clean(repoRoot)

	diffAttrCacheMu.RLock()
	for _, p := range paths {
		if p == "" || p == os.DevNull || p == "/dev/null" {
			continue
		}
		cacheKey := diffAttrCacheKey(cleanRepoRoot, cwd, p)
		if val, found := diffAttrCache[cacheKey]; found {
			result[p] = val
		} else {
			missingPaths = append(missingPaths, p)
		}
	}
	diffAttrCacheMu.RUnlock()

	if len(missingPaths) == 0 {
		return result
	}

	args := append([]string{"check-attr", "diff", "binary", "-z", "--"}, missingPaths...)
	out, err := RunGit(execDir, args...)
	if err != nil {
		return result
	}

	tokens := bytes.Split(out, []byte{0})
	if len(tokens) > 0 && len(tokens[len(tokens)-1]) == 0 {
		tokens = tokens[:len(tokens)-1]
	}

	rawMap := make(map[string]FileDiffAttr, len(missingPaths))
	for i := 0; i+2 < len(tokens); i += 3 {
		cleanPath := filepath.Clean(string(tokens[i]))
		attrName := string(tokens[i+1])
		attrInfo := string(tokens[i+2])

		entry := rawMap[cleanPath]
		switch {
		case attrName == "binary" && attrInfo == "set",
			attrName == "diff" && attrInfo == "unset":
			entry.Attr = DiffAttrBinary
		case attrName == "diff" && attrInfo == "set":
			entry.Attr = DiffAttrText
		case attrName == "diff" && attrInfo != "" && attrInfo != "unspecified":
			entry.Driver = attrInfo
		}
		rawMap[cleanPath] = entry
	}

	diffAttrCacheMu.Lock()
	defer diffAttrCacheMu.Unlock()

	for _, p := range missingPaths {
		fileAttr := rawMap[filepath.Clean(p)]
		cacheKey := diffAttrCacheKey(cleanRepoRoot, cwd, p)
		diffAttrCache[cacheKey] = fileAttr
		result[p] = fileAttr
	}

	return result
}

// ApplyTextconvFilter runs a configured textconv filter for driver on filePath.
// If driver has no configured textconv command in Git config or filePath does not exist,
// it returns nil, false, nil.
func ApplyTextconvFilter(cwd, driver, filePath string) ([]byte, bool, error) {
	cmdStr := getDriverTextconv(cwd, driver)
	if cmdStr == "" {
		return nil, false, nil
	}

	fullFilePath := filePath
	if !filepath.IsAbs(fullFilePath) {
		fullFilePath = filepath.Join(cwd, filePath)
	}
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
		return nil, true, fmt.Errorf("running textconv filter for %s: %w", filePath, err)
	}
	return out, true, nil
}

// applyWorkingTreeTextconv runs Git's textconv filter on a working tree file if configured.
func applyWorkingTreeTextconv(cwd, path, cleanPath string) ([]byte, bool, error) {
	attrs := CheckDiffAttr(cwd, cleanPath)
	driver := attrs[cleanPath].Driver
	if driver == "" {
		return nil, false, nil
	}
	return ApplyTextconvFilter(cwd, driver, path)
}

func cleanRevisionPath(cwd, path string) string {
	cleanPath := filepath.ToSlash(filepath.Clean(path))
	if prefix, err := GetRepoPrefix(cwd); err == nil && prefix != "" {
		if !filepath.IsAbs(path) && !strings.HasPrefix(cleanPath, prefix) {
			cleanPath = filepath.ToSlash(filepath.Join(prefix, cleanPath))
		}
	}
	return cleanPath
}

// BlobRequest pairs a repository-relative path with a Git revision.
type BlobRequest struct {
	Path     string
	Revision string
}

// GetBatchContent fetches blobs in one git cat-file --batch pass.
// When Revision is empty, it reads the working-tree file straight from disk.
func GetBatchContent(cwd string, reqs []BlobRequest) (map[BlobRequest][]byte, error) {
	result := make(map[BlobRequest][]byte, len(reqs))
	if len(reqs) == 0 {
		return result, nil
	}

	var catSpecs []string
	specToReqs := make(map[string][]BlobRequest)

	for _, req := range reqs {
		if req.Revision == "" {
			fullPath := filepath.Join(cwd, req.Path)
			if fi, err := os.Stat(fullPath); err == nil && fi.IsDir() {
				result[req] = nil
				continue
			}
			data, err := os.ReadFile(fullPath)
			if err != nil {
				if os.IsNotExist(err) {
					result[req] = nil
					continue
				}
				return nil, err
			}
			result[req] = data
			continue
		}

		cleanPath := cleanRevisionPath(cwd, req.Path)
		var spec string
		if req.Revision == ":" {
			spec = ":" + cleanPath
		} else {
			spec = req.Revision + ":" + cleanPath
		}

		if existing, ok := specToReqs[spec]; ok {
			specToReqs[spec] = append(existing, req)
		} else {
			catSpecs = append(catSpecs, spec)
			specToReqs[spec] = []BlobRequest{req}
		}
	}

	if len(catSpecs) == 0 {
		return result, nil
	}

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
		return fallbackGetContent(cwd, reqs)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fallbackGetContent(cwd, reqs)
	}

	if err := cmd.Start(); err != nil {
		return fallbackGetContent(cwd, reqs)
	}

	go func() {
		defer func() { _ = stdin.Close() }()
		for _, spec := range catSpecs {
			if _, err := io.WriteString(stdin, spec+"\n"); err != nil {
				break
			}
		}
	}()

	r := bufio.NewReader(stdout)
	for _, spec := range catSpecs {
		header, err := r.ReadString('\n')
		if err != nil {
			_ = stdin.Close()
			_ = cmd.Wait()
			return fallbackGetContent(cwd, reqs)
		}
		header = strings.TrimRight(header, "\r\n")

		matchingReqs := specToReqs[spec]
		if strings.HasSuffix(header, " missing") || strings.HasSuffix(header, " ambiguous") {
			for _, req := range matchingReqs {
				result[req] = nil
			}
			continue
		}

		_, rest, ok1 := strings.Cut(header, " ")
		_, sizeStr, ok2 := strings.Cut(rest, " ")
		if !ok1 || !ok2 {
			_ = stdin.Close()
			_ = cmd.Wait()
			return fallbackGetContent(cwd, reqs)
		}

		size, err := strconv.Atoi(sizeStr)
		if err != nil {
			_ = stdin.Close()
			_ = cmd.Wait()
			return fallbackGetContent(cwd, reqs)
		}

		buf := make([]byte, size)
		if _, err := io.ReadFull(r, buf); err != nil {
			_ = stdin.Close()
			_ = cmd.Wait()
			return fallbackGetContent(cwd, reqs)
		}
		_, _ = r.ReadByte() // git cat-file appends a trailing newline after each object payload

		for _, req := range matchingReqs {
			result[req] = buf
		}
	}

	_ = cmd.Wait()
	return result, nil
}

func fallbackGetContent(cwd string, reqs []BlobRequest) (map[BlobRequest][]byte, error) {
	result := make(map[BlobRequest][]byte, len(reqs))
	for _, req := range reqs {
		data, err := GetContent(cwd, req.Path, req.Revision, false)
		if err != nil {
			return nil, err
		}
		result[req] = data
	}
	return result, nil
}

// GetContent reads a file at a given revision (e.g. "HEAD", a hash, or ":" for index).
// If revision is empty, it reads the local file on disk (applying textconv filter only if useTextconv is true).
func GetContent(cwd, path, revision string, useTextconv bool) ([]byte, error) {
	cleanPath := cleanRevisionPath(cwd, path)

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
		fullPath := filepath.Join(cwd, path)
		if fi, err := os.Stat(fullPath); err == nil && fi.IsDir() {
			return nil, nil
		}
		data, err := os.ReadFile(fullPath)
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
