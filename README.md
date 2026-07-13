# Diffmantic

A semantic diff engine using Tree-sitter.

Diffmantic is a structural diff tool written in Go. Instead of showing line-by-line differences, it parses your code into ASTs and matches nodes. This lets it track moved functions, in-place updates, and structural shifts, not just added or deleted lines.

> **Note:** The editor integrations (VS Code, Neovim) and TUI are under active development.

## Supported Languages

* Go
* JavaScript / TypeScript
* Python

## Features

* **Move Detection**: Knows when you move a block of code, instead of showing it as deleted and re-added.
* **Update Detection**: Shows when you change part of a node (like a string label or variable name).
* **Insert / Delete**: Pinpoints exactly where structure was added or removed.
* **Stable Paths**: The JSON output points to nodes using child-index paths (e.g. `[0, 2, 1]`) instead of line numbers, so editor plugins don't lose track of highlights as you edit.

## Installation

Install the CLI using the install script:

```bash
curl -fsSL https://raw.githubusercontent.com/HarshK97/diffmantic/main/install.sh | sh
```

By default, this installs the `diffm` binary to `~/.local/bin`. Make sure that folder is in your `$PATH`.

## Usage

Diff two files using:

```bash
diffm diff before.go after.go
```

By default, this outputs structured JSON (schema version `v1`) designed for editor plugins and automation.

If you want a human-readable list of edit actions, run:

```bash
diffm diff before.go after.go -f actions
```

## How It Works

Diffmantic matches ASTs in four main steps:

1. **Top-Down Matching**: We look for identical subtrees of the same height first. When we find an exact match, we map those nodes and their children recursively.
2. **Bottom-Up Matching**: For unmatched nodes, we look for counterparts of the same type that share already-matched children. We match them if their similarity score is high enough.
3. **Recovery**: Inside matched containers, we run Longest Common Subsequence (LCS) on unmatched children. We do this twice: once using exact labels, and once using structure only.
4. **Action Generation & Post-Processing**: We generate a raw edit script (insert, delete, update, move) and refine it. We collapse child edits into clean subtree edits (like a whole folder/block insert), normalize comment updates, and group moved elements together.

## License

MIT License - see [LICENSE](LICENSE).

## Acknowledgements

Our engine is based on the GumTree matching algorithm. You can check out the details here:
* [GumTree GitHub Repository](https://github.com/GumTreeDiff/gumtree)
* [GumTree Paper](https://hal.science/hal-04855170v1/file/GumTree_simple__fine_grained__accurate_and_scalable_source_differencing.pdf)
* [Beyond GumTree Paper](https://www.researchgate.net/publication/335498580_Beyond_GumTree_A_Hybrid_Approach_to_Generate_Edit_Scripts)
