package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func TestRenderShortHelp(t *testing.T) {
	output := RenderShortHelp(rootCmd)

	if !strings.Contains(output, "diffmantic is a structural source code diff engine") {
		t.Errorf("expected header intro in short help, got:\n%s", output)
	}

	if !strings.Contains(output, "Usage:") {
		t.Errorf("expected Usage section in short help")
	}

	if !strings.Contains(output, "Examples:") {
		t.Errorf("expected Examples section in short help")
	}

	if !strings.Contains(output, "Flags:") {
		t.Errorf("expected Flags section in short help")
	}

	if !strings.Contains(output, "Use 'diffm --help' for the complete reference manual") {
		t.Errorf("expected --help reference footer in short help")
	}

	rootCmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden || len(f.Deprecated) > 0 {
			return
		}
		flagName := "--" + f.Name
		if !strings.Contains(output, flagName) {
			t.Errorf("flag %q missing from short help output", flagName)
		}
	})
}

func TestRenderLongHelp(t *testing.T) {
	output := RenderLongHelp(rootCmd, false)

	sections := []string{
		"NAME",
		"SYNOPSIS",
		"DESCRIPTION",
		"OPTIONS",
		"GIT INTEGRATION",
		"ENVIRONMENT VARIABLES",
		"SUPPORTED LANGUAGES",
		"EXIT STATUS",
		"EXAMPLES",
		"AUTHORS & REPOSITORY",
	}

	for _, sec := range sections {
		if !strings.Contains(output, sec) {
			t.Errorf("expected section %q in long help manual", sec)
		}
	}

	gitSnippets := []string{
		"Inside a Git Repo (Recommended)",
		"git config --global diff.external diffm",
		"*.go diff=diffmantic",
		"7-argument external diff driver protocol",
	}
	for _, snip := range gitSnippets {
		if !strings.Contains(output, snip) {
			t.Errorf("expected git snippet %q in long help", snip)
		}
	}

	envVars := []string{
		"DIFFM_FORMAT",
		"DIFFM_COLOR",
		"DIFFM_TAB_WIDTH",
		"DIFFM_IGNORE_COMMENTS",
		"DIFFM_PARSE_ERROR_LIMIT",
		"DIFFM_SIZE_LIMIT",
		"DIFFM_LINE_LIMIT",
		"DIFFM_NO_PAGER",
	}
	for _, ev := range envVars {
		if !strings.Contains(output, ev) {
			t.Errorf("expected environment variable %q in long help", ev)
		}
	}

	languages := []string{
		"Go (.go)",
		"Java (.java)",
		"JavaScript (.js, .jsx",
		"TypeScript (.ts",
		"TSX (.tsx)",
		"Python (.py",
		"Rust (.rs)",
		"Zig (.zig)",
		"C (.c, .h)",
		"C++ (.cpp",
		"Lua (.lua)",
	}
	for _, lang := range languages {
		if !strings.Contains(output, lang) {
			t.Errorf("expected language %q in long help", lang)
		}
	}

	rootCmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden || len(f.Deprecated) > 0 {
			return
		}
		flagName := "--" + f.Name
		if !strings.Contains(output, flagName) {
			t.Errorf("flag %q missing from long help OPTIONS", flagName)
		}
	})
}

func TestRenderLongHelpColorize(t *testing.T) {
	plain := RenderLongHelp(rootCmd, false)
	if strings.Contains(plain, "\x1b[") {
		t.Errorf("plain long help should not contain ANSI escape sequences")
	}

	colored := RenderLongHelp(rootCmd, true)
	if !strings.Contains(colored, "\x1b[1mNAME\x1b[0m") {
		t.Errorf("colored long help should contain bold section headers")
	}
}

func TestUsageHandler(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetErr(&buf)
	t.Cleanup(func() {
		rootCmd.SetErr(nil)
	})
	err := UsageHandler(rootCmd)
	if err != nil {
		t.Fatalf("UsageHandler returned unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Usage:\n  diffm [flags]") {
		t.Errorf("UsageHandler missing Usage: block, got:\n%s", out)
	}
	if !strings.Contains(out, "Run 'diffm -h' for short help, or 'diffm --help' for the complete reference manual.") {
		t.Errorf("UsageHandler missing help reference footer, got:\n%s", out)
	}
}

func TestRenderHelpNilCommand(t *testing.T) {
	short := RenderShortHelp(nil)
	if !strings.Contains(short, "diffmantic is a structural source code diff engine") {
		t.Errorf("RenderShortHelp(nil) missing expected header")
	}

	long := RenderLongHelp(nil, false)
	if !strings.Contains(long, "NAME") {
		t.Errorf("RenderLongHelp(nil, false) missing NAME section")
	}
}

func TestShouldColorizeHelp(t *testing.T) {
	t.Setenv("DIFFM_COLOR", "always")
	if !shouldColorizeHelp(rootCmd) {
		t.Errorf("expected colorize to be true when DIFFM_COLOR=always")
	}

	t.Setenv("DIFFM_COLOR", "never")
	if shouldColorizeHelp(rootCmd) {
		t.Errorf("expected colorize to be false when DIFFM_COLOR=never")
	}
}

func TestHelpHandler(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })

	t.Run("ShortHelp", func(t *testing.T) {
		os.Args = []string{"diffm", "-h"}
		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		t.Cleanup(func() { rootCmd.SetOut(nil) })

		HelpHandler(rootCmd, nil)
		out := buf.String()
		if !strings.Contains(out, "Usage:\n  diffm [flags]") {
			t.Errorf("expected Usage block in short help, got:\n%s", out)
		}
		if !strings.Contains(out, "Use 'diffm --help' for the complete reference manual") {
			t.Errorf("expected --help reference footer in short help, got:\n%s", out)
		}
	})

	t.Run("LongHelp", func(t *testing.T) {
		os.Args = []string{"diffm", "--help"}
		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		t.Cleanup(func() { rootCmd.SetOut(nil) })

		HelpHandler(rootCmd, nil)
		out := buf.String()
		if !strings.Contains(out, "NAME\n") {
			t.Errorf("expected NAME section in long help, got:\n%s", out)
		}
		if !strings.Contains(out, "GIT INTEGRATION\n") {
			t.Errorf("expected GIT INTEGRATION section in long help, got:\n%s", out)
		}
	})

	t.Run("HelpCommandArgument", func(t *testing.T) {
		os.Args = []string{"diffm", "help"}
		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		t.Cleanup(func() { rootCmd.SetOut(nil) })

		HelpHandler(rootCmd, []string{"help"})
		out := buf.String()
		if !strings.Contains(out, "NAME\n") {
			t.Errorf("expected NAME section for 'diffm help', got:\n%s", out)
		}
	})
}

func TestPrintLongHelpRedirected(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	t.Cleanup(func() { rootCmd.SetOut(nil) })

	PrintLongHelp(rootCmd)
	out := buf.String()
	if !strings.Contains(out, "NAME\n") {
		t.Errorf("expected NAME section from PrintLongHelp, got:\n%s", out)
	}
	if !strings.Contains(out, "OPTIONS\n") {
		t.Errorf("expected OPTIONS section from PrintLongHelp, got:\n%s", out)
	}
}

func TestRootCmdHelpSubcommand(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"diffm", "help"}

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"help"})
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetArgs(nil)
	})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing 'diffm help': %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "NAME\n") {
		t.Errorf("expected manual output from 'diffm help', got:\n%s", out)
	}
}
