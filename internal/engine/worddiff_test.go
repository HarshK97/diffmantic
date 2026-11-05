package engine

import (
	"fmt"
	"strings"
	"testing"
)

func TestWordDiff_ZeroAlloc(t *testing.T) {
	scratch := AcquireWordDiffScratch()
	defer ReleaseWordDiffScratch(scratch)

	src := "func calculateTotal(items []Item, taxRate float64) (float64, error) {"
	dst := "func calculateTotal(items []Item, discount float64, taxRate float64) (float64, error) {"

	// Warm-up run
	ops := scratch.Diff(src, dst)
	if len(ops) == 0 {
		t.Fatalf("expected diff operations, got 0")
	}

	allocs := testing.AllocsPerRun(100, func() {
		ops = scratch.Diff(src, dst)
	})
	if allocs > 0 {
		t.Fatalf("expected 0 allocs/op, got %f", allocs)
	}
	if len(ops) == 0 {
		t.Fatalf("expected diff operations during alloc test, got 0")
	}
}

func TestWordDiff_RuneSafety(t *testing.T) {
	t.Run("Emoji", func(t *testing.T) {
		// 🚀 is 4 bytes: [0xF0, 0x9F, 0x9A, 0x80]
		// 🌟 is 4 bytes: [0xF0, 0x9F, 0x8C, 0x9F]
		src := "log.Info(\"Starting server 🚀 ...\")"
		dst := "log.Info(\"Starting server 🌟 ...\")"

		ops := DiffWords(src, dst)
		if len(ops) != 1 {
			t.Fatalf("expected 1 op, got %d: %+v", len(ops), ops)
		}
		op := ops[0]
		if op.Action != "update" {
			t.Errorf("expected action 'update', got %q", op.Action)
		}
		if src[op.SrcStart:op.SrcEnd] != "🚀" {
			t.Errorf("expected src token '🚀', got %q (offsets %d..%d)", src[op.SrcStart:op.SrcEnd], op.SrcStart, op.SrcEnd)
		}
		if dst[op.DstStart:op.DstEnd] != "🌟" {
			t.Errorf("expected dst token '🌟', got %q (offsets %d..%d)", dst[op.DstStart:op.DstEnd], op.DstStart, op.DstEnd)
		}
	})

	t.Run("MultiByteUTF8_Accented", func(t *testing.T) {
		// Non-ASCII Unicode letters like 'ü' and 'é' group into words
		src := "city := \"München\"; let résumé = \"draft\";"
		dst := "city := \"Munich\"; let résumé = \"draft\";"

		ops := DiffWords(src, dst)
		if len(ops) != 1 {
			t.Fatalf("expected 1 op, got %d: %+v", len(ops), ops)
		}
		op := ops[0]
		if op.Action != "update" {
			t.Errorf("expected action 'update', got %q", op.Action)
		}
		if src[op.SrcStart:op.SrcEnd] != "München" {
			t.Errorf("expected src 'München', got %q", src[op.SrcStart:op.SrcEnd])
		}
		if dst[op.DstStart:op.DstEnd] != "Munich" {
			t.Errorf("expected dst 'Munich', got %q", dst[op.DstStart:op.DstEnd])
		}
	})

	t.Run("CJK_ChineseCharacters", func(t *testing.T) {
		// '你好' and '世界' are Unicode word tokens
		src := "val text = \"你好 世界\";"
		dst := "val text = \"您好 世界\";"

		ops := DiffWords(src, dst)
		if len(ops) != 1 {
			t.Fatalf("expected 1 op, got %d: %+v", len(ops), ops)
		}
		op := ops[0]
		if op.Action != "update" {
			t.Errorf("expected action 'update', got %q", op.Action)
		}
		if src[op.SrcStart:op.SrcEnd] != "你好" {
			t.Errorf("expected src '你好', got %q", src[op.SrcStart:op.SrcEnd])
		}
		if dst[op.DstStart:op.DstEnd] != "您好" {
			t.Errorf("expected dst '您好', got %q", dst[op.DstStart:op.DstEnd])
		}
	})
}

func TestWordDiff_PunctuationIsolation(t *testing.T) {
	t.Run("CommaVsSemicolon", func(t *testing.T) {
		src := "foo,"
		dst := "foo;"

		ops := DiffWords(src, dst)
		if len(ops) != 1 {
			t.Fatalf("expected 1 op, got %d: %+v", len(ops), ops)
		}
		op := ops[0]
		if op.Action != "update" {
			t.Errorf("expected action 'update', got %q", op.Action)
		}
		if op.SrcStart != 3 || op.SrcEnd != 4 || src[op.SrcStart:op.SrcEnd] != "," {
			t.Errorf("expected src ',' at 3..4, got %q at %d..%d", src[op.SrcStart:op.SrcEnd], op.SrcStart, op.SrcEnd)
		}
		if op.DstStart != 3 || op.DstEnd != 4 || dst[op.DstStart:op.DstEnd] != ";" {
			t.Errorf("expected dst ';' at 3..4, got %q at %d..%d", dst[op.DstStart:op.DstEnd], op.DstStart, op.DstEnd)
		}
	})

	t.Run("OperatorSwap", func(t *testing.T) {
		src := "result := a + b"
		dst := "result := a - b"

		ops := DiffWords(src, dst)
		if len(ops) != 1 {
			t.Fatalf("expected 1 op, got %d: %+v", len(ops), ops)
		}
		op := ops[0]
		if op.Action != "update" {
			t.Errorf("expected action 'update', got %q", op.Action)
		}
		if src[op.SrcStart:op.SrcEnd] != "+" {
			t.Errorf("expected src '+', got %q", src[op.SrcStart:op.SrcEnd])
		}
		if dst[op.DstStart:op.DstEnd] != "-" {
			t.Errorf("expected dst '-', got %q", dst[op.DstStart:op.DstEnd])
		}
	})
}

func TestWordDiff_SymmetricalUpdate(t *testing.T) {
	src := "let count = 10;"
	dst := "let count = 42;"

	ops := DiffWords(src, dst)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d: %+v", len(ops), ops)
	}
	op := ops[0]
	if op.Action != "update" {
		t.Errorf("expected action 'update', got %q", op.Action)
	}
	if src[op.SrcStart:op.SrcEnd] != "10" {
		t.Errorf("expected src '10', got %q", src[op.SrcStart:op.SrcEnd])
	}
	if dst[op.DstStart:op.DstEnd] != "42" {
		t.Errorf("expected dst '42', got %q", dst[op.DstStart:op.DstEnd])
	}
}

func TestWordDiff_AsymmetricalDeleteInsert(t *testing.T) {
	t.Run("InsertArgument", func(t *testing.T) {
		src := "foo(a);"
		dst := "foo(a, b);"

		ops := DiffWords(src, dst)
		if len(ops) != 1 {
			t.Fatalf("expected 1 op, got %d: %+v", len(ops), ops)
		}
		op := ops[0]
		if op.Action != "insert" {
			t.Errorf("expected action 'insert', got %q", op.Action)
		}
		if op.SrcStart != 0 || op.SrcEnd != 0 {
			t.Errorf("expected 0 src offsets for insert, got %d..%d", op.SrcStart, op.SrcEnd)
		}
		if dst[op.DstStart:op.DstEnd] != ", b" {
			t.Errorf("expected dst ', b', got %q", dst[op.DstStart:op.DstEnd])
		}
	})

	t.Run("DeleteArgument", func(t *testing.T) {
		src := "foo(a, b);"
		dst := "foo(a);"

		ops := DiffWords(src, dst)
		if len(ops) != 1 {
			t.Fatalf("expected 1 op, got %d: %+v", len(ops), ops)
		}
		op := ops[0]
		if op.Action != "delete" {
			t.Errorf("expected action 'delete', got %q", op.Action)
		}
		if op.DstStart != 0 || op.DstEnd != 0 {
			t.Errorf("expected 0 dst offsets for delete, got %d..%d", op.DstStart, op.DstEnd)
		}
		if src[op.SrcStart:op.SrcEnd] != ", b" {
			t.Errorf("expected src ', b', got %q", src[op.SrcStart:op.SrcEnd])
		}
	})
}

func TestWordDiff_CircuitBreaker(t *testing.T) {
	// Construct a line with > 120 tokens.
	// E.g. 70 words separated by commas: "w0, w1, w2, ..., w69"
	// Each pair "wX, " is 3 tokens (word, comma, space).
	// 50 pairs = 150 tokens > MaxTokensPerLine (120).
	var parts []string
	for i := range 50 {
		parts = append(parts, fmt.Sprintf("x%d", i))
	}
	src := "data = [" + strings.Join(parts, ", ") + "]"
	dst := "data = [" + strings.Join(parts, "; ") + "]"

	ops := DiffWords(src, dst)
	if len(ops) != 2 {
		t.Fatalf("expected circuit breaker to fall back to 2 ops (delete + insert), got %d: %+v", len(ops), ops)
	}
	if ops[0].Action != "delete" || ops[0].SrcStart != 0 || ops[0].SrcEnd != len(src) {
		t.Errorf("expected full line delete, got %+v", ops[0])
	}
	if ops[1].Action != "insert" || ops[1].DstStart != 0 || ops[1].DstEnd != len(dst) {
		t.Errorf("expected full line insert, got %+v", ops[1])
	}
}

func TestWordDiff_IdenticalAndEmpty(t *testing.T) {
	t.Run("Identical", func(t *testing.T) {
		src := "func doWork(ctx context.Context) error {"
		dst := "func doWork(ctx context.Context) error {"
		ops := DiffWords(src, dst)
		if len(ops) != 0 {
			t.Errorf("expected 0 ops for identical lines, got %d: %+v", len(ops), ops)
		}
	})

	t.Run("BothEmpty", func(t *testing.T) {
		ops := DiffWords("", "")
		if len(ops) != 0 {
			t.Errorf("expected 0 ops for both empty, got %d: %+v", len(ops), ops)
		}
	})

	t.Run("SrcEmptyDstNonEmpty", func(t *testing.T) {
		dst := "println(\"hello\")"
		ops := DiffWords("", dst)
		if len(ops) != 1 || ops[0].Action != "insert" || ops[0].DstStart != 0 || ops[0].DstEnd != len(dst) {
			t.Errorf("expected full insert, got %+v", ops)
		}
	})

	t.Run("SrcNonEmptyDstEmpty", func(t *testing.T) {
		src := "println(\"hello\")"
		ops := DiffWords(src, "")
		if len(ops) != 1 || ops[0].Action != "delete" || ops[0].SrcStart != 0 || ops[0].SrcEnd != len(src) {
			t.Errorf("expected full delete, got %+v", ops)
		}
	})
}

func TestWordDiff_ZeroMatchCompleteRewrite(t *testing.T) {
	// Lines sharing zero matching tokens (K == 0) have no matching context anchor.
	// Completely rewritten lines must emit "delete" for tokensSrc and "insert" for tokensDst (red/green), NOT "update".
	src := "foo.bar"
	dst := "baz[qux]"

	ops := DiffWords(src, dst)
	if len(ops) != 2 {
		t.Fatalf("expected 2 ops (delete + insert) for zero-match rewrite, got %d: %+v", len(ops), ops)
	}
	if ops[0].Action != "delete" || ops[0].SrcStart != 0 || ops[0].SrcEnd != len(src) {
		t.Errorf("expected full delete on src, got %+v", ops[0])
	}
	if ops[1].Action != "insert" || ops[1].DstStart != 0 || ops[1].DstEnd != len(dst) {
		t.Errorf("expected full insert on dst, got %+v", ops[1])
	}
}
