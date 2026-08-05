package serialize

import "testing"

func TestBuildLineIndex(t *testing.T) {
	data := []byte("hello\nworld\nfoo")
	idx := BuildLineIndex(data)
	want := []int{0, 6, 12}
	if len(idx) != len(want) {
		t.Fatalf("got len %d, want %d", len(idx), len(want))
	}
	for i := range want {
		if idx[i] != want[i] {
			t.Errorf("idx[%d] = %d, want %d", i, idx[i], want[i])
		}
	}
}

func TestByteToLineCol(t *testing.T) {
	data := []byte("hello\nworld\nfoo")
	idx := BuildLineIndex(data)

	line, col := ByteToLineCol(idx, 0)
	if line != 0 || col != 0 {
		t.Errorf("ByteToLineCol(0) = (%d, %d), want (0, 0)", line, col)
	}

	line, col = ByteToLineCol(idx, 7)
	if line != 1 || col != 1 {
		t.Errorf("ByteToLineCol(7) = (%d, %d), want (1, 1)", line, col)
	}

	line, col = ByteToLineCol(idx, 12)
	if line != 2 || col != 0 {
		t.Errorf("ByteToLineCol(12) = (%d, %d), want (2, 0)", line, col)
	}
}

func TestForEachLineSpan(t *testing.T) {
	data := []byte("hello\nworld\nfoo")
	idx := BuildLineIndex(data)

	type span struct {
		line, sc, ec int
	}
	var spans []span
	ForEachLineSpan(idx, data, 3, 14, func(line, sc, ec int) {
		spans = append(spans, span{line, sc, ec})
	})

	want := []span{
		{0, 3, 5},
		{1, 0, 5},
		{2, 0, 2},
	}

	if len(spans) != len(want) {
		t.Fatalf("got %d spans, want %d", len(spans), len(want))
	}
	for i, w := range want {
		if spans[i] != w {
			t.Errorf("span[%d] = %+v, want %+v", i, spans[i], w)
		}
	}
}
