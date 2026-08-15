<p align="center">
  <h1 align="center">diffmantic</h1>
  <p align="center">
    <strong>Stop Diffing Text, Start Diffing Logic.</strong>
  </p>
  <p align="center">
    <a href="https://github.com/HarshK97/diffmantic/actions/workflows/ci.yml"><img src="https://github.com/HarshK97/diffmantic/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
    <a href="https://github.com/HarshK97/diffmantic/releases/latest"><img src="https://img.shields.io/github/v/release/HarshK97/diffmantic?label=release" alt="Latest Release"></a>
    <a href="https://github.com/HarshK97/diffmantic/blob/main/LICENSE"><img src="https://img.shields.io/github/license/HarshK97/diffmantic" alt="License: MIT"></a>
    <a href="https://pkg.go.dev/github.com/HarshK97/diffmantic"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white" alt="Go Version"></a>
  </p>
</p>

---

<p align="center">
  <img src="assets/demo.png" alt="diffmantic TUI demo" width="800">
</p>

---

## Why diffmantic?

Line-based diffs like `git diff` break down when you refactor code. Move a function down 50 lines, and git shows it as a full Delete and re-add. Rename a parameter, and entire lines light up red and green.

Now with AI tools generating massive PRs with Moved functions and renamed symbols everywhere, the actual Change gets buried in noise. So human reviewers end up just giving in.

diffmantic fixes this by parsing your code into ASTs using [Tree-sitter](https://tree-sitter.github.io/). It tracks structural shifts, so it knows when a function was Moved instead of deleted, and shows exact inline node edits instead of lighting up entire lines.

It works as a standalone CLI, a drop-in for `git diff`, or a backend for editor plugins via JSON output.

## Features

- **Move Detection.** When you move a function or a block, diffmantic tracks it as a Move. Not a delete + re-add. Moved functions, blocks, and statements are all first-class.
- **Update & Rename Detection.** Shows exactly what changed inside a syntax node. A variable rename, a string literal swap, a type change, you see the precise edit, not a wall of red and green.
- **Git Integration.** Run `diffm` in any Git repo and it launches an interactive TUI. Browse modified files, stage/unstage changes, commit, all without leaving the terminal.
- **Interactive TUI.** Side-by-side diff view with syntax highlighting, code folding, search, Action Inspector panel (`i`), change indicators, and jump-to-change keys (`n`, `N`, `[`, `]`). Built with [Bubbletea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss).
- **JSON Output.** Stable schema with child-index paths (e.g., `[0, 2, 1]`) instead of line numbers. So editor plugins can keep highlights intact even as you edit the file.
- **16 Core Languages.** Go, Java, JavaScript, TypeScript, Python, Rust, Zig, C, C++, PHP, Ruby, JSON, YAML, TOML, HTML, CSS, Lua. Full AST normalization and matching rules powered by Tree-sitter.
- **Line Diff Fallback.** For unsupported file types or plain text files, Diffmantic automatically falls back to line-based diffing so you can diff any file.

## Supported Languages

### Programming Languages (10)
| Language | Extensions |
|:---------|:-----------|
| Go | `.go` |
| Java | `.java` |
| JavaScript | `.js` `.jsx` `.mjs` `.cjs` |
| TypeScript | `.ts` `.tsx` `.mts` `.cts` |
| Python | `.py` |
| Rust | `.rs` |
| Zig | `.zig` |
| C | `.c` `.h` |
| C++ | `.cpp` `.cc` `.cxx` `.hpp` `.hh` |
| PHP | `.php` |
| Ruby | `.rb` |

### Markup & Data Formats (6)
| Format | Extensions |
|:-------|:-----------|
| JSON | `.json` |
| YAML | `.yaml` `.yml` |
| TOML | `.toml` |
| HTML | `.html` `.htm` |
| CSS | `.css` |
| Lua | `.lua` |

> **Note**: Fully supported languages include tailored AST normalization (stripping punctuation noise and flattening comment/string blocks). Other languages built via `make build-core` or `make build-all` fall back to raw AST matching.

## Installation

### Install Script (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/HarshK97/diffmantic/main/install.sh | sh
```

This installs the `diffm` binary to `~/.local/bin`. Make sure it's in your `$PATH`.

It auto-detects your OS and architecture, grabs the right binary from [GitHub Releases](https://github.com/HarshK97/diffmantic/releases), and verifies the SHA256 checksum.

```bash
# Install to a specific directory
curl -fsSL https://raw.githubusercontent.com/HarshK97/diffmantic/main/install.sh | sh -s -- --dir=/usr/local/bin

# Install a specific version
curl -fsSL https://raw.githubusercontent.com/HarshK97/diffmantic/main/install.sh | sh -s -- --version=v0.5.0
```

### Homebrew

```bash
brew install HarshK97/tap/diffmantic
```

### Download Binary

Prebuilt binaries for Linux, macOS, and Windows (amd64 + arm64) are on the [Releases page](https://github.com/HarshK97/diffmantic/releases).

### Build from Source

Requires Go 1.26+. No C compiler or CGo required (`CGO_ENABLED=0`).

```bash
git clone https://github.com/HarshK97/diffmantic.git
cd diffmantic

# Default build: 16 core languages (13 MB binary)
make build

# Or install directly with Go:
go install github.com/HarshK97/diffmantic/cmd/diffm@latest
```

#### Custom Grammar Build Targets

Diffmantic supports embedding different sets of Tree-sitter grammars via `Makefile` targets:

| Makefile Command | Description | Binary Size |
| :--- | :--- | :--- |
| `make build` | **Default**: Embeds [16 fully supported core languages](#supported-languages) | **~13.06 MB** |
| `make build-core` | Embeds **~100 core languages** from gotreesitter | **~22.37 MB** |
| `make build-all` | Embeds **all ~206 languages** available in gotreesitter | **~29.12 MB** |

#### Building a Custom Language Subset

If you only need a specific set of languages (for example, Go, Python, and Rust), you can compile a minimal binary using `gotreesitter` build tags:

```bash
# Pass 'grammar_subset' plus 'grammar_subset_<lang>' tags
go build -tags 'grammar_subset grammar_subset_go grammar_subset_python grammar_subset_rust' -ldflags="-s -w" -trimpath -o diffm ./cmd/diffm
```

> Built with `gotreesitter` for pure Go Tree-sitter AST parsing with zero C compiler or CGo runtime dependencies.

## Usage

### Git Status Mode (default in a repo)

```bash
# Launch the interactive TUI in any Git repository
diffm

# Show only staged changes
diffm --cached
```

### Git Revision Diffing

```bash
# Diff working tree against HEAD
diffm HEAD

# Diff between two commits, tags, or branches
diffm HEAD~1 HEAD
diffm main...feature-branch
```

### File-to-File Diff

```bash
# Interactive TUI (default when a terminal is attached)
diffm diff before.go after.go

# JSON output for editor plugins and automation
diffm diff before.go after.go -f json

# Human-readable action list
diffm diff before.go after.go -f actions

# Override language detection
diffm diff config.txt config2.txt --lang json
```

## How It Works

diffmantic matches ASTs in four phases, combining the [GumTree](https://github.com/GumTreeDiff/gumtree) algorithm, Zhang-Shasha tree edit distance, and Chawathe edit script generation:

1. **Top-Down Matching.** We look for identical subtrees by height. When we find an exact match, all nodes in the subtree get mapped together.
2. **Bottom-Up Matching.** For unmatched nodes, we look for counterparts of the same type that share already-matched children. If the Dice similarity score is high enough, we match them.
3. **Recovery.** Inside matched containers, we run LCS alignment on unmatched children (first by label, then by structural shape). For small subtrees, [Zhang-Shasha (1989)](https://doi.org/10.1137/0218082) tree edit distance is used as a precise fallback.
4. **Action Generation & Post-Processing.** We produce a raw edit script (insert, delete, update, move) using [Chawathe et al. (1996)](https://doi.org/10.1145/235968.235970) edit script generation and then refine it. Child edits get collapsed into clean subtree operations, comment changes get normalized, and related moves get grouped.

## Editor Integrations

### Neovim

[diffmantic.nvim](https://github.com/HarshK97/diffmantic.nvim) is the Neovim plugin that started this whole project. It currently ships with its own embedded alpha engine and is being migrated to use the `diffm` CLI as its backend via JSON output.

### VS Code

Planned. The JSON output is built to support editor integration, so if you want to build one, the plumbing is there.

### JSON Schema

`diffm diff -f json` outputs a stable `v1` schema with child-index paths. Take a look at `diffm diff file-a file-b -f json` to see the full structure.

## License

MIT. See [LICENSE](LICENSE).

## Acknowledgements

The engine is based on foundational research in AST differencing, tree edit distance, and edit script generation:

- [GumTree: A Complete Approach for AST Differencing](https://hal.science/hal-04855170v1/file/GumTree_simple__fine_grained__accurate_and_scalable_source_differencing.pdf), Falleri et al.
- [Beyond GumTree](https://www.researchgate.net/publication/335498580_Beyond_GumTree_A_Hybrid_Approach_to_Generate_Edit_Scripts), Huvier et al.
- [Simple Fast Algorithms for the Editing Distance Between Trees and Related Problems](https://doi.org/10.1137/0218082), Kaizhong Zhang and Dennis Shasha (SIAM Journal on Computing, 1989).
- [Change Detection in Hierarchically Structured Information](https://doi.org/10.1145/235968.235970), Sudarshan S. Chawathe, Anand Rajaraman, Hector Garcia-Molina, and Jennifer Widom (ACM SIGMOD, 1996).
- [GumTree GitHub Repository](https://github.com/GumTreeDiff/gumtree)
