package tui

import (
	"github.com/HarshK97/diffmantic/internal/output"
)

type DiffFile struct {
	OldPath     string
	NewPath     string
	OldLines    []string
	NewLines    []string
	Hunks       []output.Hunk
	VisualHunks []output.Hunk
	LeftSpans   []visualSpan
	RightSpans  []visualSpan
}

type visualSpan struct {
	Kind      output.ChangeKind
	StartLine int
	EndLine   int
	StartCol  int
	EndCol    int
	Priority  int
}

func NewDiffFile(oldPath, newPath string, oldSrc, newSrc []byte, hunks []output.Hunk) DiffFile {
	return DiffFile{
		OldPath:     oldPath,
		NewPath:     newPath,
		OldLines:    splitSourceLines(oldSrc),
		NewLines:    splitSourceLines(newSrc),
		Hunks:       hunks,
		VisualHunks: hunks,
	}
}

func splitSourceLines(src []byte) []string {
	if len(src) == 0 {
		return nil
	}

	lines := make([]string, 0)
	start := 0
	for i, b := range src {
		if b != '\n' {
			continue
		}
		line := string(src[start:i])
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		lines = append(lines, line)
		start = i + 1
	}
	if start < len(src) {
		line := string(src[start:])
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		lines = append(lines, line)
	}
	return lines
}
