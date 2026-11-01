package cmd

import (
	"cmp"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/HarshK97/diffmantic/internal/color"
	"github.com/HarshK97/diffmantic/internal/pager"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func formatFlagTitle(f *pflag.Flag) string {
	var s string
	if f.Shorthand != "" {
		s = fmt.Sprintf("-%s, --%s", f.Shorthand, f.Name)
	} else {
		s = fmt.Sprintf("--%s", f.Name)
	}
	if ph := flagPlaceholder(f); ph != "" {
		s += " " + ph
	}
	return s
}

func formatFlagUsage(f *pflag.Flag) string {
	usage := f.Usage
	if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" && !strings.Contains(usage, "default") {
		return fmt.Sprintf("%s (default: %s)", usage, f.DefValue)
	}
	return usage
}

// RenderShortHelp renders a concise, single-screen usage overview and flag table.
func RenderShortHelp(cmd *cobra.Command) string {
	if cmd == nil {
		cmd = rootCmd
	}
	var buf strings.Builder
	buf.WriteString("diffmantic is a structural source code diff engine powered by Tree-sitter.\n\n")

	buf.WriteString("Usage:\n")
	buf.WriteString("  diffm [flags] <fileA> <fileB>\n")
	buf.WriteString("  diffm [flags] <dirA> <dirB>\n")
	buf.WriteString("  diffm [flags] [revision] [path]\n")
	buf.WriteString("  diffm [flags] <revA> <revB>\n\n")

	buf.WriteString("Examples:\n")
	buf.WriteString("  diffm before.go after.go                     Side-by-side diff with pager (default in TTY)\n")
	buf.WriteString("  diffm dir_a dir_b                            Recursive directory diff\n")
	buf.WriteString("  diffm -u before.go after.go                  AST-aware unified diff\n")
	buf.WriteString("  git diff | diffm                             Stream Git diff with AST highlighting\n")
	buf.WriteString("  git log -p | diffm                           Stream Git commit log with AST highlighting\n")
	buf.WriteString("  diffm                                        Git mode on unstaged changes\n")
	buf.WriteString("  diffm --cached                               Git staged changes diff\n")
	buf.WriteString("  diffm HEAD~1 HEAD                            Git revision comparison\n")
	buf.WriteString("  diffm -f json --ui before.go after.go        Structured JSON for editor plugins\n\n")

	buf.WriteString("Flags:\n")

	type flagItem struct {
		prefix string
		usage  string
	}
	var items []flagItem
	maxPrefixLen := 0

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Deprecated != "" {
			return
		}
		prefix := "      " + formatFlagTitle(f)
		if f.Shorthand != "" {
			prefix = "  " + formatFlagTitle(f)
		}
		maxPrefixLen = max(maxPrefixLen, len(prefix))
		items = append(items, flagItem{prefix: prefix, usage: formatFlagUsage(f)})
	})

	for _, item := range items {
		padding := max(2, maxPrefixLen+2-len(item.prefix))
		buf.WriteString(item.prefix)
		buf.WriteString(strings.Repeat(" ", padding))
		buf.WriteString(item.usage)
		buf.WriteByte('\n')
	}

	buf.WriteString("\nUse 'diffm --help' for the complete reference manual and Git integration guide.\n")
	return buf.String()
}

// RenderLongHelp renders the complete reference manual with optional ANSI section styling.
func RenderLongHelp(cmd *cobra.Command, colorize bool) string {
	if cmd == nil {
		cmd = rootCmd
	}
	var buf strings.Builder

	bold := func(s string) string {
		if colorize {
			return color.Bold + s + color.Reset
		}
		return s
	}

	buf.WriteString(bold("NAME") + "\n")
	if cmd != nil && cmd.Version != "" {
		fmt.Fprintf(&buf, "    diffm %s - structural, semantic diff engine powered by Tree-sitter\n\n", cmd.Version)
	} else {
		buf.WriteString("    diffm - structural, semantic diff engine powered by Tree-sitter\n\n")
	}

	buf.WriteString(bold("SYNOPSIS") + "\n")
	buf.WriteString("    diffm [OPTIONS] <fileA> <fileB>\n")
	buf.WriteString("    diffm [OPTIONS] <dirA> <dirB>\n")
	buf.WriteString("    diffm [OPTIONS] [REVISION] [PATH]\n")
	buf.WriteString("    diffm [OPTIONS] <revA> <revB>\n")
	buf.WriteString("    diffm <path> <old-file> <old-hex> <old-mode> <new-file> <new-hex> <new-mode>\n")
	buf.WriteString("    diffm [--parse-tree | --cst] <file>...\n\n")

	buf.WriteString(bold("DESCRIPTION") + "\n")
	buf.WriteString("    diffmantic is a structural source code diff engine.\n")
	buf.WriteString("    Instead of diffing files line-by-line, it parses source code into\n")
	buf.WriteString("    Abstract Syntax Trees (ASTs) using Tree-sitter and tracks how language\n")
	buf.WriteString("    constructs shift across revisions.\n\n")
	buf.WriteString("    Diffmantic recognizes when entire functions, classes, methods, or blocks\n")
	buf.WriteString("    are moved or reordered, when identifiers or expressions are updated\n")
	buf.WriteString("    in-place, and when statements are inserted or deleted.\n\n")
	buf.WriteString("    If a file exceeds size limits or contains parse errors beyond configured\n")
	buf.WriteString("    thresholds, diffmantic falls back to line diffing so you always get a\n")
	buf.WriteString("    usable diff.\n\n")

	buf.WriteString(bold("OPTIONS") + "\n")
	var flags []*pflag.Flag
	if cmd != nil {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if !f.Hidden && f.Deprecated == "" {
				flags = append(flags, f)
			}
		})
	}

	slices.SortFunc(flags, func(a, b *pflag.Flag) int {
		return cmp.Compare(a.Name, b.Name)
	})

	for _, f := range flags {
		flagTitle := formatFlagTitle(f)
		buf.WriteString("    " + bold(flagTitle) + "\n")
		buf.WriteString(wrapText(formatFlagUsage(f), 8, 76))
		buf.WriteString("\n\n")
	}

	buf.WriteString(bold("GIT INTEGRATION") + "\n")
	buf.WriteString("    Diffmantic can act as your default Git pager, providing AST-aware diffs\n")
	buf.WriteString("    and structural move tracking across your entire daily Git workflow.\n\n")
	buf.WriteString("    " + bold("Git Pager Integration (Recommended)") + "\n")
	buf.WriteString("        Add this to your ~/.gitconfig so git diff, git show, and git log -p\n")
	buf.WriteString("        use diffmantic automatically:\n\n")
	buf.WriteString("            [pager]\n")
	buf.WriteString("                diff = diffm\n")
	buf.WriteString("                show = diffm\n")
	buf.WriteString("                log  = diffm\n\n")
	buf.WriteString("        Or configure via Git CLI:\n\n")
	buf.WriteString("            git config --global pager.diff diffm\n")
	buf.WriteString("            git config --global pager.show diffm\n")
	buf.WriteString("            git config --global pager.log  diffm\n\n")
	buf.WriteString("        Note: Pass '-u' (e.g. pager.diff = diffm -u) if you prefer unified diffs.\n")
	buf.WriteString("        Non-interactive environments (scripts, pipes, CI) automatically bypass\n")
	buf.WriteString("        the pager and receive standard Git diffs.\n\n")
	buf.WriteString("    " + bold("Piped Git Streams") + "\n")
	buf.WriteString("        You can pipe any Git patch or commit log into diffmantic:\n\n")
	buf.WriteString("            git diff | diffm\n")
	buf.WriteString("            git log -p -n 5 | diffm\n")
	buf.WriteString("            git show HEAD | diffm\n\n")
	buf.WriteString("    " + bold("Direct Git Inspection") + "\n")
	buf.WriteString("        Run diffm directly inside any Git repository without arguments:\n\n")
	buf.WriteString("            diffm                # Unstaged changes\n")
	buf.WriteString("            diffm --cached       # Staged changes\n")
	buf.WriteString("            diffm HEAD~1         # Changes in last commit\n")
	buf.WriteString("            diffm HEAD~1 HEAD    # Compare two commits\n")
	buf.WriteString("            diffm main feature   # Compare branches\n")
	buf.WriteString("            diffm -u             # Unified AST diff\n\n")
	buf.WriteString("    " + bold("Git External Diff Driver (Legacy)") + "\n")
	buf.WriteString("        Diffmantic also implements Git's 7-argument external diff driver protocol:\n")
	buf.WriteString("            <path> <old-file> <old-hex> <old-mode> <new-file> <new-hex> <new-mode>\n\n")
	buf.WriteString("        To use diffmantic whenever you run 'git diff':\n\n")
	buf.WriteString("            git config --global diff.external diffm\n\n")
	buf.WriteString("        Note: diff.external is legacy. It spawns a new diffm process for every\n")
	buf.WriteString("        single file and cannot format 'git log -p'. Pager integration above\n")
	buf.WriteString("        is strongly recommended instead.\n\n")
	buf.WriteString("    " + bold("File-Specific Driver via .gitattributes") + "\n")
	buf.WriteString("        To use diffmantic only for specific file types in a repo, add this\n")
	buf.WriteString("        to your ~/.gitconfig:\n\n")
	buf.WriteString("            [diff \"diffmantic\"]\n")
	buf.WriteString("                command = diffm\n\n")
	buf.WriteString("        And add this to your repository's .gitattributes:\n\n")
	buf.WriteString("            *.go diff=diffmantic\n")
	buf.WriteString("            *.rs diff=diffmantic\n")
	buf.WriteString("            *.py diff=diffmantic\n")
	buf.WriteString("            *.ts diff=diffmantic\n\n")

	buf.WriteString(bold("ENVIRONMENT VARIABLES") + "\n")
	buf.WriteString("    Diffmantic supports configuration via the following environment\n")
	buf.WriteString("    variables. Command-line flags always take precedence over environment\n")
	buf.WriteString("    variables.\n\n")
	buf.WriteString("    " + bold("DIFFM_FORMAT") + "\n")
	buf.WriteString("        Default diff output format (side-by-side, unified, json, actions, auto).\n\n")
	buf.WriteString("    " + bold("DIFFM_COLOR") + "\n")
	buf.WriteString("        Color output mode (always, never, auto).\n\n")
	buf.WriteString("    " + bold("DIFFM_TAB_WIDTH") + "\n")
	buf.WriteString("        Number of spaces per tab stop (default: 4).\n\n")
	buf.WriteString("    " + bold("DIFFM_IGNORE_COMMENTS") + "\n")
	buf.WriteString("        Boolean (1, true, yes) to ignore comments during AST diffing (default: 0).\n\n")
	buf.WriteString("    " + bold("DIFFM_PARSE_ERROR_LIMIT") + "\n")
	buf.WriteString("        Maximum parse errors permitted before falling back to line diffing\n")
	buf.WriteString("        (default: 0).\n\n")
	buf.WriteString("    " + bold("DIFFM_SIZE_LIMIT") + "\n")
	buf.WriteString("        Maximum file size in KB for AST parsing before fallback (0 to disable,\n")
	buf.WriteString("        default: 1024).\n\n")
	buf.WriteString("    " + bold("DIFFM_LINE_LIMIT") + "\n")
	buf.WriteString("        Maximum line count for AST parsing before fallback (0 to disable,\n")
	buf.WriteString("        default: 10000).\n\n")
	buf.WriteString("    " + bold("DIFFM_NO_PAGER") + "\n")
	buf.WriteString("        If set to any non-empty value, disables the interactive terminal pager.\n\n")

	buf.WriteString(bold("SUPPORTED LANGUAGES") + "\n")
	buf.WriteString("    Diffmantic includes native Tree-sitter parsers for 10 programming languages:\n")
	buf.WriteString("      • Go (.go)\n")
	buf.WriteString("      • Java (.java)\n")
	buf.WriteString("      • JavaScript (.js, .jsx, .mjs, .cjs)\n")
	buf.WriteString("      • TypeScript (.ts, .mts, .cts)\n")
	buf.WriteString("      • TSX (.tsx)\n")
	buf.WriteString("      • Python (.py, .pyi)\n")
	buf.WriteString("      • Rust (.rs)\n")
	buf.WriteString("      • Zig (.zig)\n")
	buf.WriteString("      • C (.c, .h)\n")
	buf.WriteString("      • C++ (.cpp, .cc, .cxx, .hpp, .hh, .hxx)\n")
	buf.WriteString("      • Lua (.lua)\n\n")
	buf.WriteString("    Files in other languages fall back automatically to standard line diffing.\n\n")

	buf.WriteString(bold("EXIT STATUS") + "\n")
	buf.WriteString("    0   Success. Files were diffed, syntax trees were dumped, or files are\n")
	buf.WriteString("        identical.\n")
	buf.WriteString("    1   Differences detected or an error occurred (such as invalid arguments,\n")
	buf.WriteString("        unreadable files, or internal parsing failures).\n\n")

	buf.WriteString(bold("EXAMPLES") + "\n")
	buf.WriteString("    Compare two local Go files side-by-side with an interactive pager:\n")
	buf.WriteString("        diffm old.go new.go\n\n")
	buf.WriteString("    Compare two directories recursively:\n")
	buf.WriteString("        diffm dir_a dir_b\n\n")
	buf.WriteString("    Print an AST-aware unified diff:\n")
	buf.WriteString("        diffm -u old.rs new.rs\n\n")
	buf.WriteString("    Inspect unstaged working directory changes in a Git repository:\n")
	buf.WriteString("        diffm\n\n")
	buf.WriteString("    Inspect staged changes only:\n")
	buf.WriteString("        diffm --cached\n\n")
	buf.WriteString("    Compare two Git branches or commits:\n")
	buf.WriteString("        diffm main feature-branch\n\n")
	buf.WriteString("    Compare a single file between commits:\n")
	buf.WriteString("        diffm HEAD~1 src/server.go\n\n")
	buf.WriteString("    Export structured JSON for editor integration (Neovim, VS Code):\n")
	buf.WriteString("        diffm --ui -f json old.go new.go\n\n")
	buf.WriteString("    Dump the simplified AST for debugging parser output:\n")
	buf.WriteString("        diffm --parse-tree main.go\n\n")
	buf.WriteString("    Dump the raw Tree-sitter concrete syntax tree:\n")
	buf.WriteString("        diffm --cst main.go\n\n")

	buf.WriteString(bold("AUTHORS & REPOSITORY") + "\n")
	buf.WriteString("    Harsh Kapse <harshkapse.dev@gmail.com>\n")
	buf.WriteString("    Source code and issue tracker: https://github.com/HarshK97/diffmantic\n")

	return buf.String()
}

// HelpHandler handles Cobra help requests. Most CLI tools blur -h and --help together
// into one giant wall of text. We deliberately split them: -h gives a quick cheatsheet
// that fits on one screen, while --help launches the full reference manual in a pager.
func HelpHandler(cmd *cobra.Command, args []string) {
	if isShortHelp() {
		_, _ = fmt.Fprint(cmd.OutOrStdout(), RenderShortHelp(cmd))
		return
	}
	PrintLongHelp(cmd)
}

// RenderUsage renders a concise synopsis for syntax or argument errors.
func RenderUsage(cmd *cobra.Command) string {
	var buf strings.Builder
	buf.WriteString("Usage:\n")
	buf.WriteString("  diffm [flags] <fileA> <fileB>\n")
	buf.WriteString("  diffm [flags] <dirA> <dirB>\n")
	buf.WriteString("  diffm [flags] [revision] [path]\n")
	buf.WriteString("  diffm [flags] <revA> <revB>\n\n")
	buf.WriteString("Run 'diffm -h' for short help, or 'diffm --help' for the complete reference manual.\n")
	return buf.String()
}

// UsageHandler renders a compact synopsis on flag syntax or argument errors.
func UsageHandler(cmd *cobra.Command) error {
	_, err := fmt.Fprint(cmd.ErrOrStderr(), RenderUsage(cmd))
	return err
}

// PrintLongHelp renders and pages the full reference manual.
func PrintLongHelp(cmd *cobra.Command) {
	colorize := shouldColorizeHelp(cmd)
	manual := RenderLongHelp(cmd, colorize)

	// If output is redirected to a buffer (e.g. in tests) or stdout is not a TTY, write directly.
	if cmd.OutOrStdout() != os.Stdout || !isTerminal(os.Stdout) {
		_, _ = fmt.Fprint(cmd.OutOrStdout(), manual)
		return
	}

	noPager, _ := cmd.Flags().GetBool("no-pager")
	if !cmd.Flags().Changed("no-pager") {
		noPager = getEnvBool("DIFFM_NO_PAGER", false)
	}
	if slices.Contains(os.Args[1:], "--no-pager") {
		noPager = true
	}

	p, writer := pager.Start(noPager)
	defer p.Close()

	if _, err := io.WriteString(writer, manual); err != nil {
		if !pager.IsBrokenPipe(err) {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error writing help: %v\n", err)
		}
		return
	}
}

func isShortHelp() bool {
	return !slices.ContainsFunc(os.Args[1:], func(arg string) bool {
		return arg == "--help" || strings.HasPrefix(arg, "--help=") || arg == "help"
	})
}

// Cobra fires HelpHandler the moment it encounters a help flag during argument
// parsing, which means flags specified after --help haven't been parsed yet.
// We scan os.Args directly so flags like --color or --no-pager take effect
// regardless of where they sit in the command line.
func shouldColorizeHelp(cmd *cobra.Command) bool {
	colorMode := "auto"
	if cmd != nil && cmd.Flags().Changed("color") {
		if val, err := cmd.Flags().GetString("color"); err == nil && val != "" {
			colorMode = val
		}
	}
	for i, arg := range os.Args[1:] {
		if arg == "--color" && i+2 < len(os.Args) {
			colorMode = os.Args[i+2]
		} else if val, ok := strings.CutPrefix(arg, "--color="); ok {
			colorMode = val
		}
	}
	if colorMode == "auto" {
		colorMode = getEnvString("DIFFM_COLOR", "auto")
	}
	switch colorMode {
	case "always":
		return true
	case "never":
		return false
	default:
		if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
			return false
		}
		return isTerminal(os.Stdout)
	}
}

func flagPlaceholder(f *pflag.Flag) string {
	if f.Value.Type() == "bool" {
		return ""
	}
	switch f.Name {
	case "format":
		return "<format>"
	case "color":
		return "<when>"
	case "context":
		return "<lines>"
	case "parse-error-limit":
		return "<n>"
	case "size-limit":
		return "<kb>"
	case "line-limit":
		return "<lines>"
	case "wrap-width":
		return "<cols>"
	case "tab-width":
		return "<n>"
	default:
		return "<" + strings.ToLower(f.Value.Type()) + ">"
	}
}

func wrapText(text string, indent int, limit int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	indentStr := strings.Repeat(" ", indent)
	var sb strings.Builder
	sb.WriteString(indentStr)
	lineLen := indent

	for _, w := range words {
		if lineLen > indent && lineLen+1+len(w) > limit {
			sb.WriteByte('\n')
			sb.WriteString(indentStr)
			lineLen = indent
		} else if lineLen > indent {
			sb.WriteByte(' ')
			lineLen++
		}
		sb.WriteString(w)
		lineLen += len(w)
	}
	return sb.String()
}
