package tui

import (
	"reflect"
	"testing"
)

func TestExpandLine(t *testing.T) {
	tests := []struct {
		name         string
		line         string
		wantExpanded string
		wantMap      []int
	}{
		{
			name:         "empty string",
			line:         "",
			wantExpanded: "",
			wantMap:      []int{0},
		},
		{
			name:         "no tabs",
			line:         "hello",
			wantExpanded: "hello",
			wantMap:      []int{0, 1, 2, 3, 4, 5},
		},
		{
			name:         "single tab at start",
			line:         "\thello",
			wantExpanded: "    hello",
			wantMap:      []int{0, 4, 5, 6, 7, 8, 9},
		},
		{
			name:         "tab in middle",
			line:         "ab\tcd",
			wantExpanded: "ab    cd",
			wantMap:      []int{0, 1, 2, 6, 7, 8},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotExpanded, gotMap := expandLine(tt.line)
			if gotExpanded != tt.wantExpanded {
				t.Errorf("expandLine(%q) string = %q, want %q", tt.line, gotExpanded, tt.wantExpanded)
			}
			if !reflect.DeepEqual(gotMap, tt.wantMap) {
				t.Errorf("expandLine(%q) map = %v, want %v", tt.line, gotMap, tt.wantMap)
			}
		})
	}
}

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{
			name:   "maxLen <= 0 returns empty",
			s:      "hello",
			maxLen: 0,
			want:   "",
		},
		{
			name:   "string shorter than maxLen returns unchanged",
			s:      "hi",
			maxLen: 5,
			want:   "hi",
		},
		{
			name:   "string equal to maxLen returns unchanged",
			s:      "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "string longer truncates with ellipsis",
			s:      "hello world",
			maxLen: 8,
			want:   "hello w…",
		},
		{
			name:   "maxLen = 1 returns ellipsis",
			s:      "hello",
			maxLen: 1,
			want:   "…",
		},
		{
			name:   "unicode string truncation",
			s:      "你好世界",
			maxLen: 3,
			want:   "你好…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateStr(tt.s, tt.maxLen); got != tt.want {
				t.Errorf("truncateStr(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{
			name:  "string shorter than width gets padded",
			s:     "hi",
			width: 5,
			want:  "hi   ",
		},
		{
			name:  "string equal to width returns unchanged",
			s:     "hello",
			width: 5,
			want:  "hello",
		},
		{
			name:  "string longer than width returns unchanged",
			s:     "hello world",
			width: 5,
			want:  "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := padRight(tt.s, tt.width); got != tt.want {
				t.Errorf("padRight(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
			}
		})
	}
}

func TestCenterPad(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{
			name:  "string shorter than width gets centered",
			s:     "hi",
			width: 6,
			want:  "  hi  ",
		},
		{
			name:  "string equal to width returns unchanged",
			s:     "hello",
			width: 5,
			want:  "hello",
		},
		{
			name:  "string longer than width gets truncated",
			s:     "hello world",
			width: 5,
			want:  "hello",
		},
		{
			name:  "odd padding distributes correctly",
			s:     "hi",
			width: 5,
			want:  " hi  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := centerPad(tt.s, tt.width); got != tt.want {
				t.Errorf("centerPad(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
			}
		})
	}
}

func TestContentHeight(t *testing.T) {
	tests := []struct {
		name          string
		height        int
		gitCommitOpen bool
		want          int
	}{
		{
			name:          "normal height (24 - 1 title - 1 status = 22)",
			height:        24,
			gitCommitOpen: false,
			want:          22,
		},
		{
			name:          "with gitCommitOpen (22 - 1 = 21)",
			height:        24,
			gitCommitOpen: true,
			want:          21,
		},
		{
			name:          "very small height clamps to 1",
			height:        2,
			gitCommitOpen: false,
			want:          1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := testModel(t, nil, nil)
			m.height = tt.height
			m.gitCommitOpen = tt.gitCommitOpen
			if got := m.contentHeight(); got != tt.want {
				t.Errorf("contentHeight() = %d, want %d", got, tt.want)
			}
		})
	}
}
