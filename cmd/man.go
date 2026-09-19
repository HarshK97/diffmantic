package cmd

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// GenerateManPage renders a Unix roff man page from the CLI command tree and flag definitions.
func GenerateManPage(cmd *cobra.Command, releaseDate string) string {
	if releaseDate == "" {
		releaseDate = "2026-09-19"
	}

	var buf bytes.Buffer

	version := cmd.Version
	if version == "" {
		version = "0.8.0"
	}
	fmt.Fprintf(&buf, ".TH DIFFM 1 %q %q \"Diffmantic Manual\"\n", releaseDate, "Diffmantic "+version)

	buf.WriteString(".SH NAME\n")
	buf.WriteString("diffm \\- structural, semantic diff engine powered by Tree-sitter\n")

	buf.WriteString(".SH SYNOPSIS\n")
	buf.WriteString(".B diffm\n")
	buf.WriteString("[\\fIOPTIONS\\fR]\n")
	buf.WriteString("\\fIFILE_A\\fR \\fIFILE_B\\fR\n")
	buf.WriteString(".br\n")
	buf.WriteString(".B diffm\n")
	buf.WriteString("[\\fIOPTIONS\\fR]\n")
	buf.WriteString("[\\fIREVISION\\fR]\n")
	buf.WriteString("[\\fIPATH\\fR]\n")
	buf.WriteString(".br\n")
	buf.WriteString(".B diffm\n")
	buf.WriteString("[\\fIOPTIONS\\fR]\n")
	buf.WriteString("\\fIREV_A\\fR \\fIREV_B\\fR\n")
	buf.WriteString(".br\n")
	buf.WriteString(".B diffm\n")
	buf.WriteString("\\fIPATH\\fR \\fIOLD_FILE\\fR \\fIOLD_HEX\\fR \\fIOLD_MODE\\fR\n")
	buf.WriteString("\\fINEW_FILE\\fR \\fINEW_HEX\\fR \\fINEW_MODE\\fR\n")
	buf.WriteString(".br\n")
	buf.WriteString(".B diffm\n")
	buf.WriteString("[\\fB\\-\\-parse\\-tree\\fR | \\fB\\-\\-cst\\fR]\n")
	buf.WriteString("\\fIFILE\\fR...\n")

	buf.WriteString(".SH DESCRIPTION\n")
	buf.WriteString(".B diffmantic\n")
	buf.WriteString("is a structural source code diff engine.\n")
	buf.WriteString("Instead of diffing files line-by-line, it parses source code into Abstract\n")
	buf.WriteString("Syntax Trees (ASTs) using Tree-sitter and tracks how language constructs\n")
	buf.WriteString("shift across revisions.\n")
	buf.WriteString(".PP\n")
	buf.WriteString("Diffmantic recognizes when entire functions, classes, methods, or blocks are\n")
	buf.WriteString("moved or reordered, when identifiers or expressions are updated in-place,\n")
	buf.WriteString("and when statements are inserted or deleted.\n")
	buf.WriteString(".PP\n")
	buf.WriteString("If a file exceeds size limits or contains parse errors beyond configured\n")
	buf.WriteString("thresholds, diffmantic falls back to line diffing so you always get a\n")
	buf.WriteString("usable diff.\n")

	cmd.InitDefaultHelpFlag()
	cmd.InitDefaultVersionFlag()

	buf.WriteString(".SH OPTIONS\n")
	var flags []*pflag.Flag
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if !f.Hidden && len(f.Deprecated) == 0 {
			flags = append(flags, f)
		}
	})

	sort.Slice(flags, func(i, j int) bool {
		return flags[i].Name < flags[j].Name
	})

	for _, f := range flags {
		formatFlagOption(&buf, f)
	}

	buf.WriteString(".SH GIT INTEGRATION\n")
	buf.WriteString("You can use\n")
	buf.WriteString(".B diffm\n")
	buf.WriteString("as a standalone command, or wire it directly into your Git workflow.\n")
	buf.WriteString(".SS Git Difftool\n")
	buf.WriteString("To configure diffmantic as your default Git difftool:\n")
	buf.WriteString(".PP\n")
	buf.WriteString(".nf\n")
	buf.WriteString("git config --global diff.tool diffm\n")
	buf.WriteString("git config --global difftool.diffm.cmd 'diffm \"$LOCAL\" \"$REMOTE\"'\n")
	buf.WriteString("git config --global difftool.prompt false\n")
	buf.WriteString(".fi\n")
	buf.WriteString(".PP\n")
	buf.WriteString("Then run:\n")
	buf.WriteString(".PP\n")
	buf.WriteString(".nf\n")
	buf.WriteString("git difftool\n")
	buf.WriteString("git difftool HEAD~1\n")
	buf.WriteString(".fi\n")
	buf.WriteString(".SS Git External Diff Driver\n")
	buf.WriteString("To use diffmantic whenever you run \\fBgit diff\\fR:\n")
	buf.WriteString(".PP\n")
	buf.WriteString(".nf\n")
	buf.WriteString("git config --global diff.external diffm\n")
	buf.WriteString(".fi\n")
	buf.WriteString(".PP\n")
	buf.WriteString("Diffmantic implements Git's 7-argument external diff driver protocol:\n")
	buf.WriteString("\\fIpath old-file old-hex old-mode new-file new-hex new-mode\\fR.\n")
	buf.WriteString(".SS File-Specific Driver via .gitattributes\n")
	buf.WriteString("To use diffmantic only for specific file types in a repository, add this to\n")
	buf.WriteString("your \\fB.gitconfig\\fR:\n")
	buf.WriteString(".PP\n")
	buf.WriteString(".nf\n")
	buf.WriteString("[diff \"diffmantic\"]\n")
	buf.WriteString("    command = diffm\n")
	buf.WriteString(".fi\n")
	buf.WriteString(".PP\n")
	buf.WriteString("And add this line to your repository's \\fB.gitattributes\\fR:\n")
	buf.WriteString(".PP\n")
	buf.WriteString(".nf\n")
	buf.WriteString("*.go diff=diffmantic\n")
	buf.WriteString("*.rs diff=diffmantic\n")
	buf.WriteString(".fi\n")

	buf.WriteString(".SH ENVIRONMENT\n")
	buf.WriteString("Diffmantic supports configuration via the following environment variables.\n")
	buf.WriteString("Command-line flags always take precedence over environment variables.\n")
	buf.WriteString(".TP\n")
	buf.WriteString(".B DIFFM_FORMAT\n")
	buf.WriteString(".br\n")
	buf.WriteString("Default diff output format (\\fIside-by-side\\fR, \\fIinline\\fR, \\fIjson\\fR,\n")
	buf.WriteString("\\fIactions\\fR).\n")
	buf.WriteString(".TP\n")
	buf.WriteString(".B DIFFM_TAB_WIDTH\n")
	buf.WriteString(".br\n")
	buf.WriteString("Number of spaces per tab stop (default: 4).\n")
	buf.WriteString(".TP\n")
	buf.WriteString(".B DIFFM_IGNORE_COMMENTS\n")
	buf.WriteString(".br\n")
	buf.WriteString("Boolean (\\fI1\\fR, \\fItrue\\fR, \\fIyes\\fR) to ignore comments during AST\n")
	buf.WriteString("diffing (default: 0).\n")
	buf.WriteString(".TP\n")
	buf.WriteString(".B DIFFM_PARSE_ERROR_LIMIT\n")
	buf.WriteString(".br\n")
	buf.WriteString("Maximum parse errors permitted before falling back to line diffing\n")
	buf.WriteString("(default: 0).\n")
	buf.WriteString(".TP\n")
	buf.WriteString(".B DIFFM_SIZE_LIMIT\n")
	buf.WriteString(".br\n")
	buf.WriteString("Maximum file size in KB for AST parsing before fallback (0 to disable,\n")
	buf.WriteString("default: 1024).\n")
	buf.WriteString(".TP\n")
	buf.WriteString(".B DIFFM_LINE_LIMIT\n")
	buf.WriteString(".br\n")
	buf.WriteString("Maximum line count for AST parsing before fallback (0 to disable,\n")
	buf.WriteString("default: 10000).\n")
	buf.WriteString(".TP\n")
	buf.WriteString(".B DIFFM_NO_PAGER\n")
	buf.WriteString(".br\n")
	buf.WriteString("If set to any non-empty value, disables the interactive terminal pager.\n")

	buf.WriteString(".SH SUPPORTED LANGUAGES\n")
	buf.WriteString("Diffmantic includes native Tree-sitter parsers for 10 programming languages:\n")
	buf.WriteString(".IP \\(bu 2\n")
	buf.WriteString("\\fBGo\\fR (\\fI.go\\fR)\n")
	buf.WriteString(".IP \\(bu 2\n")
	buf.WriteString("\\fBJava\\fR (\\fI.java\\fR)\n")
	buf.WriteString(".IP \\(bu 2\n")
	buf.WriteString("\\fBJavaScript\\fR (\\fI.js\\fR, \\fI.jsx\\fR)\n")
	buf.WriteString(".IP \\(bu 2\n")
	buf.WriteString("\\fBTypeScript\\fR (\\fI.ts\\fR)\n")
	buf.WriteString(".IP \\(bu 2\n")
	buf.WriteString("\\fBTSX\\fR (\\fI.tsx\\fR)\n")
	buf.WriteString(".IP \\(bu 2\n")
	buf.WriteString("\\fBPython\\fR (\\fI.py\\fR)\n")
	buf.WriteString(".IP \\(bu 2\n")
	buf.WriteString("\\fBRust\\fR (\\fI.rs\\fR)\n")
	buf.WriteString(".IP \\(bu 2\n")
	buf.WriteString("\\fBZig\\fR (\\fI.zig\\fR)\n")
	buf.WriteString(".IP \\(bu 2\n")
	buf.WriteString("\\fBC\\fR (\\fI.c\\fR, \\fI.h\\fR)\n")
	buf.WriteString(".IP \\(bu 2\n")
	buf.WriteString("\\fBC++\\fR (\\fI.cpp\\fR, \\fI.cc\\fR, \\fI.hpp\\fR)\n")
	buf.WriteString(".IP \\(bu 2\n")
	buf.WriteString("\\fBLua\\fR (\\fI.lua\\fR)\n")
	buf.WriteString(".PP\n")
	buf.WriteString("Files in other languages fall back automatically to standard line diffing.\n")

	buf.WriteString(".SH EXIT STATUS\n")
	buf.WriteString(".TP\n")
	buf.WriteString(".B 0\n")
	buf.WriteString(".br\n")
	buf.WriteString("Success. Files were diffed, syntax trees were dumped, or files are identical.\n")
	buf.WriteString(".TP\n")
	buf.WriteString(".B 1\n")
	buf.WriteString(".br\n")
	buf.WriteString("Differences detected or an error occurred (such as invalid arguments,\n")
	buf.WriteString("unreadable files, or internal parsing failures).\n")

	buf.WriteString(".SH EXAMPLES\n")
	buf.WriteString("Compare two local Go files side-by-side with an interactive pager:\n")
	buf.WriteString(".PP\n")
	buf.WriteString(".RS\n")
	buf.WriteString("diffm old.go new.go\n")
	buf.WriteString(".RE\n")
	buf.WriteString(".PP\n")
	buf.WriteString("Print an AST-aware inline diff:\n")
	buf.WriteString(".PP\n")
	buf.WriteString(".RS\n")
	buf.WriteString("diffm -f inline old.rs new.rs\n")
	buf.WriteString(".RE\n")
	buf.WriteString(".PP\n")
	buf.WriteString("Generate an inline patch suitable for patch tools:\n")
	buf.WriteString(".PP\n")
	buf.WriteString(".RS\n")
	buf.WriteString("diffm -p old.py new.py\n")
	buf.WriteString(".RE\n")
	buf.WriteString(".PP\n")
	buf.WriteString("Inspect unstaged working directory changes in a Git repository:\n")
	buf.WriteString(".PP\n")
	buf.WriteString(".RS\n")
	buf.WriteString("diffm\n")
	buf.WriteString(".RE\n")
	buf.WriteString(".PP\n")
	buf.WriteString("Inspect staged changes only:\n")
	buf.WriteString(".PP\n")
	buf.WriteString(".RS\n")
	buf.WriteString("diffm --cached\n")
	buf.WriteString(".RE\n")
	buf.WriteString(".PP\n")
	buf.WriteString("Compare two Git branches or commits:\n")
	buf.WriteString(".PP\n")
	buf.WriteString(".RS\n")
	buf.WriteString("diffm main feature-branch\n")
	buf.WriteString(".RE\n")
	buf.WriteString(".PP\n")
	buf.WriteString("Compare a single file between commits:\n")
	buf.WriteString(".PP\n")
	buf.WriteString(".RS\n")
	buf.WriteString("diffm HEAD~1 src/server.go\n")
	buf.WriteString(".RE\n")
	buf.WriteString(".PP\n")
	buf.WriteString("Export structured JSON for editor integration (Neovim, VS Code):\n")
	buf.WriteString(".PP\n")
	buf.WriteString(".RS\n")
	buf.WriteString("diffm --ui -f json old.go new.go\n")
	buf.WriteString(".RE\n")
	buf.WriteString(".PP\n")
	buf.WriteString("Dump the simplified AST for debugging parser output:\n")
	buf.WriteString(".PP\n")
	buf.WriteString(".RS\n")
	buf.WriteString("diffm --parse-tree main.go\n")
	buf.WriteString(".RE\n")

	buf.WriteString(".SH SEE ALSO\n")
	buf.WriteString(".BR git (1),\n")
	buf.WriteString(".BR git-difftool (1),\n")
	buf.WriteString(".BR diff (1)\n")

	buf.WriteString(".SH AUTHORS\n")
	buf.WriteString("Harsh Kapse <harshkapse.dev@gmail.com>\n")
	buf.WriteString(".PP\n")
	buf.WriteString("Source code and issue tracker: https://github.com/HarshK97/diffmantic\n")

	return buf.String()
}

func formatFlagOption(buf *bytes.Buffer, f *pflag.Flag) {
	buf.WriteString(".TP\n")

	escapedName := strings.ReplaceAll(f.Name, "-", "\\-")
	argPlaceholder := flagPlaceholder(f)

	if f.Shorthand != "" {
		if argPlaceholder != "" {
			fmt.Fprintf(buf, "\\fB\\-%s\\fR, \\fB\\-\\-%s\\fR=%s\n", f.Shorthand, escapedName, argPlaceholder)
		} else {
			fmt.Fprintf(buf, "\\fB\\-%s\\fR, \\fB\\-\\-%s\\fR\n", f.Shorthand, escapedName)
		}
	} else {
		if argPlaceholder != "" {
			fmt.Fprintf(buf, "\\fB\\-\\-%s\\fR=%s\n", escapedName, argPlaceholder)
		} else {
			fmt.Fprintf(buf, "\\fB\\-\\-%s\\fR\n", escapedName)
		}
	}

	buf.WriteString(".br\n")

	usage := f.Usage
	if usage != "" {
		if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" && !strings.Contains(usage, "default") {
			usage = fmt.Sprintf("%s (default: %s)", usage, f.DefValue)
		}
		buf.WriteString(wrapManText(usage, 72))
		buf.WriteString("\n")
	}
}

func wrapManText(text string, limit int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	var sb strings.Builder
	lineLen := 0
	for i, w := range words {
		if i > 0 {
			if lineLen+1+len(w) > limit {
				sb.WriteByte('\n')
				lineLen = 0
			} else {
				sb.WriteByte(' ')
				lineLen++
			}
		}
		sb.WriteString(w)
		lineLen += len(w)
	}
	return sb.String()
}

func flagPlaceholder(f *pflag.Flag) string {
	if f.Value.Type() == "bool" {
		return ""
	}
	switch f.Name {
	case "format":
		return "\\fIFORMAT\\fR"
	case "color":
		return "\\fIWHEN\\fR"
	case "context":
		return "\\fILINES\\fR"
	case "parse-error-limit":
		return "\\fIN\\fR"
	case "size-limit":
		return "\\fIKB\\fR"
	case "line-limit":
		return "\\fILINES\\fR"
	case "wrap-width":
		return "\\fICOLS\\fR"
	case "tab-width":
		return "\\fIN\\fR"
	default:
		return "\\fI" + strings.ToUpper(f.Value.Type()) + "\\fR"
	}
}
