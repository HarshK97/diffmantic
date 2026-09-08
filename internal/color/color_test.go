package color

import (
	"regexp"
	"testing"
)

var sgrRegex = regexp.MustCompile(`^\x1b\[[0-9;]+m$`)

func TestANSIConstants_SGRFormat(t *testing.T) {
	constants := map[string]string{
		"Reset":     Reset,
		"Bold":      Bold,
		"Dim":       Dim,
		"Italic":    Italic,
		"Underline": Underline,
		"InsertFg":  InsertFg,
		"DeleteFg":  DeleteFg,
		"MoveFg":    MoveFg,
		"UpdateFg":  UpdateFg,
		"HeaderFg":  HeaderFg,
		"OverlayFg": OverlayFg,
		"SurfaceFg": SurfaceFg,
		"TextFg":    TextFg,
	}

	for name, val := range constants {
		if !sgrRegex.MatchString(val) {
			t.Errorf("constant %s = %q is not a valid ANSI SGR escape sequence", name, val)
		}
	}
}

func TestANSIConstants_Values(t *testing.T) {
	expected := map[string]string{
		"Reset":     "\x1b[0m",
		"Bold":      "\x1b[1m",
		"Dim":       "\x1b[2m",
		"Italic":    "\x1b[3m",
		"Underline": "\x1b[4m",
		"InsertFg":  "\x1b[32m",
		"DeleteFg":  "\x1b[31m",
		"MoveFg":    "\x1b[36m",
		"UpdateFg":  "\x1b[33m",
		"HeaderFg":  "\x1b[35m",
		"OverlayFg": "\x1b[90m",
		"SurfaceFg": "\x1b[90m",
		"TextFg":    "\x1b[39m",
	}

	for name, want := range expected {
		var got string
		switch name {
		case "Reset":
			got = Reset
		case "Bold":
			got = Bold
		case "Dim":
			got = Dim
		case "Italic":
			got = Italic
		case "Underline":
			got = Underline
		case "InsertFg":
			got = InsertFg
		case "DeleteFg":
			got = DeleteFg
		case "MoveFg":
			got = MoveFg
		case "UpdateFg":
			got = UpdateFg
		case "HeaderFg":
			got = HeaderFg
		case "OverlayFg":
			got = OverlayFg
		case "SurfaceFg":
			got = SurfaceFg
		case "TextFg":
			got = TextFg
		}
		if got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestActionKind_String(t *testing.T) {
	tests := []struct {
		kind ActionKind
		want string
	}{
		{ActionDelete, "delete"},
		{ActionInsert, "insert"},
		{ActionUpdate, "update"},
		{ActionMove, "move"},
		{ActionMoveUpdate, "move_update"},
		{ActionKind(-1), "unknown"},
		{ActionKind(99), "unknown"},
		{ActionKind(1000), "unknown"},
	}

	for _, tt := range tests {
		got := tt.kind.String()
		if got != tt.want {
			t.Errorf("ActionKind(%d).String() = %q, want %q", int(tt.kind), got, tt.want)
		}
	}
}
