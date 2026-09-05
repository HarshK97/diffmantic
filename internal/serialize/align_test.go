package serialize

import (
	"reflect"
	"testing"
)

func TestAlignLines(t *testing.T) {
	tests := []struct {
		name     string
		srcBytes []byte
		dstBytes []byte
		want     []LineAlignmentPair
	}{
		{
			name:     "Identical single-line files",
			srcBytes: []byte("hello"),
			dstBytes: []byte("hello"),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
			},
		},
		{
			name:     "Identical multi-line files",
			srcBytes: []byte("hello\nworld"),
			dstBytes: []byte("hello\nworld"),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: 1, RightLine: 1},
			},
		},
		{
			name:     "Empty old file",
			srcBytes: []byte(""),
			dstBytes: []byte("new\nlines"),
			want: []LineAlignmentPair{
				{LeftLine: -1, RightLine: 0},
				{LeftLine: -1, RightLine: 1},
			},
		},
		{
			name:     "Empty new file",
			srcBytes: []byte("old\nlines"),
			dstBytes: []byte(""),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: -1},
				{LeftLine: 1, RightLine: -1},
			},
		},
		{
			name:     "Both empty",
			srcBytes: []byte(""),
			dstBytes: []byte(""),
			want: []LineAlignmentPair{
				{LeftLine: -1, RightLine: 0},
			},
		},
		{
			name:     "Deleted lines",
			srcBytes: []byte("line1\nline2\nline3"),
			dstBytes: []byte("line1\nline3"),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: 1, RightLine: -1},
				{LeftLine: 2, RightLine: 1},
			},
		},
		{
			name:     "Inserted lines",
			srcBytes: []byte("line1\nline3"),
			dstBytes: []byte("line1\nline2\nline3"),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: -1, RightLine: 1},
				{LeftLine: 1, RightLine: 2},
			},
		},
		{
			name:     "Single line change in middle",
			srcBytes: []byte("line1\nline2\nline3"),
			dstBytes: []byte("line1\nline2_changed\nline3"),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: 1, RightLine: 1},
				{LeftLine: 2, RightLine: 2},
			},
		},
		{
			name:     "Unequal length modified block (more insertions)",
			srcBytes: []byte("header\nold1\nfooter"),
			dstBytes: []byte("header\nnew1\nnew2\nfooter"),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: 1, RightLine: 1},
				{LeftLine: -1, RightLine: 2},
				{LeftLine: 2, RightLine: 3},
			},
		},
		{
			name:     "Unequal length modified block (more deletions)",
			srcBytes: []byte("header\nold1\nold2\nfooter"),
			dstBytes: []byte("header\nnew1\nfooter"),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: 1, RightLine: 1},
				{LeftLine: 2, RightLine: -1},
				{LeftLine: 3, RightLine: 2},
			},
		},
		{
			name:     "Moved block rendered cleanly via line diff",
			srcBytes: []byte("func foo() {\n  a()\n}\nfunc bar() {\n}"),
			dstBytes: []byte("func foo() {\n}\nfunc bar() {\n  a()\n}"),
			want: []LineAlignmentPair{
				{LeftLine: 0, RightLine: 0},
				{LeftLine: 1, RightLine: -1},
				{LeftLine: 2, RightLine: 1},
				{LeftLine: 3, RightLine: 2},
				{LeftLine: -1, RightLine: 3},
				{LeftLine: 4, RightLine: 4},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AlignLines(tt.srcBytes, tt.dstBytes)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AlignLines() got = %v, want = %v", got, tt.want)
			}
		})
	}
}


