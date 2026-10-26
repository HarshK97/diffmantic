// Package streamdiff intercepts streaming Git patches on stdin (git diff, git log -p,
// git show), fetches the blobs, and renders AST diffs in place of standard patch text.
package streamdiff

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/git"
	"github.com/HarshK97/diffmantic/internal/pager"
	"github.com/HarshK97/diffmantic/internal/pipeline"
	"github.com/HarshK97/diffmantic/internal/sidebyside"
	"github.com/HarshK97/diffmantic/internal/unified"
)

// StreamConfig configures the streaming diff processor and renderers.
type StreamConfig struct {
	Format         string
	DiffOpts       pipeline.DiffOptions
	UnifiedOpts    unified.RenderOptions
	SideBySideOpts sidebyside.RenderOptions
}

// fileHunk buffers raw lines and tracks object IDs for a single file in the stream.
type fileHunk struct {
	oldPath      string
	newPath      string
	oldOID       string
	newOID       string
	lines        []string
	streamingRaw bool
}

// Process parses a Git patch stream from in, runs 2-way diffs through the AST pipeline,
// and passes everything else (commit logs, raw patches, merge conflicts) straight to out.
func Process(in io.Reader, out io.Writer, cwd string, cfg StreamConfig) error {
	reader := bufio.NewReader(in)
	var current *fileHunk
	var session *git.BatchSession
	gitAvailable := true

	defer func() {
		if session != nil {
			_ = session.Close()
		}
	}()

	flushCurrent := func() error {
		if current == nil {
			return nil
		}
		defer func() { current = nil }()

		if current.streamingRaw {
			return nil
		}

		// Need both paths and object IDs to run an AST diff — fall back to raw Git text.
		if current.oldPath == "" || current.newPath == "" || current.oldOID == "" || current.newOID == "" {
			return dumpLines(out, current.lines)
		}

		oldBytes, newBytes, err := fetchBuffers(cwd, current, session, gitAvailable)
		if err != nil {
			// Blob fetch failed (e.g. shallow clone, untracked file) — fall back to raw text.
			return dumpLines(out, current.lines)
		}

		dr, err := pipeline.Run(oldBytes, newBytes, current.oldPath, current.newPath, cfg.DiffOpts)
		if err != nil || dr == nil {
			return dumpLines(out, current.lines)
		}

		return renderDiff(out, dr, current, cfg)
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			if err == io.EOF {
				break
			}
			return err
		}

		cleanLine := cleanStreamLine(line)

		// Flush the previous file diff before printing the next commit header.
		if strings.HasPrefix(cleanLine, "commit ") || strings.HasPrefix(cleanLine, "From ") {
			if flushErr := flushCurrent(); flushErr != nil {
				if pager.IsBrokenPipe(flushErr) {
					return nil
				}
				return flushErr
			}
			if _, wErr := io.WriteString(out, line); wErr != nil {
				if pager.IsBrokenPipe(wErr) {
					return nil
				}
				return wErr
			}
			continue
		}

		// Start of a new file diff. Graph lines (*, |) and combined diffs pass straight
		// through to keep column alignment and multi-parent merge output intact (Case 77).
		if strings.HasPrefix(cleanLine, "diff --git ") {
			if flushErr := flushCurrent(); flushErr != nil {
				if pager.IsBrokenPipe(flushErr) {
					return nil
				}
				return flushErr
			}

			if session == nil && gitAvailable {
				if s, err := git.StartBatchSession(cwd); err == nil {
					session = s
				} else {
					gitAvailable = false
				}
			}

			oldPath, newPath := parseDiffGitPaths(cleanLine)
			current = &fileHunk{
				oldPath: oldPath,
				newPath: newPath,
				lines:   []string{line},
			}
			continue
		}

		// Flush in-flight diffs and pass combined merge diffs through untouched.
		if strings.HasPrefix(cleanLine, "diff --combined ") || strings.HasPrefix(cleanLine, "diff --cc ") {
			if flushErr := flushCurrent(); flushErr != nil {
				if pager.IsBrokenPipe(flushErr) {
					return nil
				}
				return flushErr
			}
			if _, wErr := io.WriteString(out, line); wErr != nil {
				if pager.IsBrokenPipe(wErr) {
					return nil
				}
				return wErr
			}
			continue
		}

		// Buffer raw lines so we can replay Git's original output if AST parsing fails.
		if current != nil {
			if current.streamingRaw {
				if _, wErr := io.WriteString(out, line); wErr != nil {
					if pager.IsBrokenPipe(wErr) {
						return nil
					}
					return wErr
				}
				continue
			}

			current.lines = append(current.lines, line)
			if len(current.lines) > 50000 {
				if err := dumpLines(out, current.lines); err != nil {
					if pager.IsBrokenPipe(err) {
						return nil
					}
					return err
				}
				current.lines = nil
				current.streamingRaw = true
				continue
			}

			if strings.HasPrefix(cleanLine, "index ") && current.oldOID == "" {
				oldOID, newOID := parseIndexOIDs(cleanLine)
				current.oldOID = oldOID
				current.newOID = newOID
			} else if strings.HasPrefix(cleanLine, "rename from ") {
				if p := parseHeaderPath(cleanLine, "rename from "); p != "" {
					current.oldPath = p
				}
			} else if strings.HasPrefix(cleanLine, "rename to ") {
				if p := parseHeaderPath(cleanLine, "rename to "); p != "" {
					current.newPath = p
				}
			} else if strings.HasPrefix(cleanLine, "--- ") {
				if p := parseHeaderPath(cleanLine, "--- "); p != "" {
					current.oldPath = p
				}
			} else if strings.HasPrefix(cleanLine, "+++ ") {
				if p := parseHeaderPath(cleanLine, "+++ "); p != "" {
					current.newPath = p
				}
			}
			continue
		}

		if _, wErr := io.WriteString(out, line); wErr != nil {
			if pager.IsBrokenPipe(wErr) {
				return nil
			}
			return wErr
		}

		if err == io.EOF {
			break
		}
	}

	if err := flushCurrent(); err != nil && !pager.IsBrokenPipe(err) {
		return err
	}
	return nil
}

// parseDiffGitPaths extracts the old and new file paths from "diff --git a/foo b/bar".
// It handles standard prefixes, unquoted, quoted, and asymmetrically quoted paths.
func parseDiffGitPaths(line string) (string, string) {
	rest, ok := strings.CutPrefix(line, "diff --git ")
	if !ok {
		return "", ""
	}
	rest = strings.TrimSpace(rest)

	// Symmetric unquoted path with spaces: "a/path with spaces.go b/path with spaces.go"
	if strings.HasPrefix(rest, "a/") {
		if rem := len(rest) - 5; rem > 0 && rem%2 == 0 {
			xLen := rem / 2
			first := rest[2 : 2+xLen]
			sep := rest[2+xLen : 2+xLen+3]
			second := rest[2+xLen+3:]
			if sep == " b/" && first == second {
				return first, second
			}
		}
	} else if rem := len(rest) - 1; rem > 0 && rem%2 == 0 {
		xLen := rem / 2
		if rest[xLen] == ' ' && rest[:xLen] == rest[xLen+1:] {
			return rest[:xLen], rest[xLen+1:]
		}
	}

	pA, rest := parseGitPathToken(rest)
	pB, _ := parseGitPathToken(rest)
	if pA != "" && pB != "" {
		return pA, pB
	}
	return "", ""
}

// parseGitPathToken extracts a single path token (quoted or unquoted) from s.
// It trims Git's "a/" or "b/" prefix and unquotes C-style escape sequences when present.
func parseGitPathToken(s string) (string, string) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return "", ""
	}
	if s[0] == '"' {
		// Quoted path: find unescaped closing quote
		for i := 1; i < len(s); i++ {
			if s[i] == '\\' {
				i++
				continue
			}
			if s[i] == '"' {
				raw := s[:i+1]
				unquoted, err := strconv.Unquote(raw)
				if err != nil {
					unquoted = s[1:i]
				}
				rem := strings.TrimSpace(s[i+1:])
				return cleanGitPathPrefix(unquoted), rem
			}
		}
		// Fallback for unclosed quote
		return cleanGitPathPrefix(strings.Trim(s, "\"")), ""
	}

	if before, after, ok := strings.Cut(s, " "); ok {
		return cleanGitPathPrefix(before), strings.TrimSpace(after)
	}
	return cleanGitPathPrefix(s), ""
}

// parseHeaderPath extracts the target path from hunk headers like "--- a/path",
// "+++ b/path", "rename from path", and "rename to path".
func parseHeaderPath(line, prefix string) string {
	after, ok := strings.CutPrefix(line, prefix)
	if !ok {
		return ""
	}
	after = strings.TrimSpace(after)
	pathPart, _, _ := strings.Cut(after, "\t")
	pathPart = strings.TrimSpace(pathPart)
	if pathPart == "" || pathPart == os.DevNull || pathPart == "/dev/null" {
		return ""
	}
	if strings.HasPrefix(pathPart, "\"") {
		p, _ := parseGitPathToken(pathPart)
		return p
	}
	return cleanGitPathPrefix(pathPart)
}

func cleanGitPathPrefix(p string) string {
	if after, ok := strings.CutPrefix(p, "a/"); ok {
		return after
	}
	if after, ok := strings.CutPrefix(p, "b/"); ok {
		return after
	}
	return p
}

// parseIndexOIDs extracts old and new object IDs from "index 1234567..89abcdef 100644".
func parseIndexOIDs(line string) (string, string) {
	rest, ok := strings.CutPrefix(line, "index ")
	if !ok {
		return "", ""
	}
	rest = strings.TrimSpace(rest)

	hashes, _, _ := strings.Cut(rest, " ")
	oldOID, newOID, found := strings.Cut(hashes, "..")
	if !found || strings.Contains(oldOID, ",") || strings.Contains(newOID, ",") {
		return "", ""
	}
	return oldOID, newOID
}

func isAllZeros(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] != '0' {
			return false
		}
	}
	return true
}

// fetchBlob loads object content from the batch coprocess, falling back
// to git cat-file or working-tree disk files on missing hashes.
func fetchBlob(cwd, oid, path string, session *git.BatchSession, gitAvailable bool) ([]byte, bool, error) {
	if isAllZeros(oid) {
		return nil, true, nil
	}
	var data []byte
	var err error
	if session != nil && !session.IsPoisoned() {
		data, err = session.Fetch(oid)
		if err != nil {
			return nil, false, fmt.Errorf("reading blob %s: %w", oid, err)
		}
	} else if gitAvailable {
		data, err = git.RunGit(cwd, "cat-file", "-p", oid)
		if err != nil {
			data = nil
		}
	}

	if data != nil {
		return data, true, nil
	}

	diskPath := filepath.Join(cwd, path)
	diskData, diskErr := os.ReadFile(diskPath)
	if diskErr != nil {
		return nil, false, fmt.Errorf("reading file %s: %w", path, diskErr)
	}
	return diskData, false, nil
}

// fetchBuffers retrieves before and after file contents from Git objects or disk.
func fetchBuffers(cwd string, h *fileHunk, session *git.BatchSession, gitAvailable bool) ([]byte, []byte, error) {
	oldBytes, oldFromGit, err := fetchBlob(cwd, h.oldOID, h.oldPath, session, gitAvailable)
	if err != nil {
		return nil, nil, err
	}
	newBytes, newFromGit, err := fetchBlob(cwd, h.newOID, h.newPath, session, gitAvailable)
	if err != nil {
		return nil, nil, err
	}

	// If neither blob existed in Git and both read the exact same working-tree file,
	// the commit objects are missing from the local repo (e.g. a shallow clone).
	if !oldFromGit && !newFromGit && !isAllZeros(h.oldOID) && !isAllZeros(h.newOID) && h.oldOID != h.newOID && h.oldPath == h.newPath {
		return nil, nil, fmt.Errorf("git blobs %s and %s missing from repository", h.oldOID, h.newOID)
	}

	return oldBytes, newBytes, nil
}

func formatBannerPath(h *fileHunk) string {
	if h.oldPath != "" && h.newPath != "" && h.oldPath != h.newPath {
		if isAllZeros(h.oldOID) || h.oldPath == os.DevNull || h.oldPath == "/dev/null" {
			return h.newPath
		}
		if isAllZeros(h.newOID) || h.newPath == os.DevNull || h.newPath == "/dev/null" {
			return h.oldPath
		}
		return fmt.Sprintf("%s → %s", h.oldPath, h.newPath)
	}
	if h.newPath != "" && h.newPath != os.DevNull && h.newPath != "/dev/null" {
		return h.newPath
	}
	return h.oldPath
}

// renderDiff writes the AST diff in the requested format.
func renderDiff(out io.Writer, dr *pipeline.DiffResult, h *fileHunk, cfg StreamConfig) error {
	path := formatBannerPath(h)

	if dr.IsBinary {
		if cfg.Format == "side-by-side" || cfg.Format == "" {
			if err := sidebyside.RenderFileBanner(path, 0, 0, 0, cfg.SideBySideOpts.Color, out); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintf(out, "Binary files %s and %s differ\n", h.oldPath, h.newPath)
		return err
	}

	switch cfg.Format {
	case "unified":
		output := unified.Render(h.oldPath, h.newPath, dr.SrcBytes, dr.DstBytes, dr.Envelope, cfg.UnifiedOpts)
		if output != "" {
			if _, err := io.WriteString(out, output); err != nil {
				return err
			}
			if !strings.HasSuffix(output, "\n") {
				if _, err := io.WriteString(out, "\n"); err != nil {
					return err
				}
			}
		}
		return nil
	case "json":
		jsonData, err := json.MarshalIndent(dr.Envelope, "", "  ")
		if err != nil {
			return err
		}
		if _, err := out.Write(jsonData); err != nil {
			return err
		}
		_, err = out.Write([]byte("\n"))
		return err
	case "actions":
		if _, err := fmt.Fprintf(out, "Diffing  %s  →  %s\n\n", h.oldPath, h.newPath); err != nil {
			return err
		}
		if err := engine.FprintMappings(out, dr.MatchResult); err != nil {
			return err
		}
		return actions.FprintActions(out, dr.EditScript)
	default:
		ins, del, upd := sidebyside.CountLineStats(dr.SrcBytes, dr.DstBytes, dr.Envelope)
		if err := sidebyside.RenderFileBanner(path, ins, del, upd, cfg.SideBySideOpts.Color, out); err != nil {
			return err
		}
		return sidebyside.Render(h.oldPath, h.newPath, dr.SrcBytes, dr.DstBytes, dr.Envelope, cfg.SideBySideOpts, out)
	}
}

// dumpLines replays buffered raw Git lines to out.
func dumpLines(out io.Writer, lines []string) error {
	for _, l := range lines {
		if _, err := io.WriteString(out, l); err != nil {
			return err
		}
	}
	return nil
}

var ansiRegex = regexp.MustCompile(`\x1b(?:\[[0-9;?]*[a-zA-Z]|\([a-zA-Z])`)

// cleanStreamLine strips ANSI escape codes and trailing newlines for header matching.
// Skips regex stripping when no escape character is present to avoid extra allocations.
func cleanStreamLine(line string) string {
	cleaned := line
	if strings.Contains(line, "\x1b") {
		cleaned = ansiRegex.ReplaceAllString(line, "")
	}
	return strings.TrimRight(cleaned, "\r\n")
}
