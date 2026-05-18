# Diffmantic

Semantic Diff Engine Using Tree-sitter.

Diffmantic is a standalone semantic diff engine written in Go. It understands code structure to detect moved functions, updated blocks, and real changes—not just line differences.

> **Note:** The UI (TUI + editor integrations) is still in progress.

## Features

- **Move detection** — Knows when code blocks are moved, not deleted and re-added  
- **Update detection** — Highlights modified code in place  
- **Insert/Delete detection** — Shows new and removed code  
- **Language agnostic** — Works with languages that have Tree-sitter parsers and Diffmantic query support  

## Coming Soon (Work in Progress)

- **Terminal TUI** — Side-by-side diff viewer built with Bubble Tea  
- **git difftool** — Drop-in replacement: `git difftool -t diffmantic`  
- **Editor backends** — JSON output for Neovim and VS Code clients  
- **CI / scripts** — Unified diff format for automation  

## Installation

> CLI packaging and install instructions are in progress.  
> This section will be updated once the binary release flow is finalized.

## Usage

> CLI interface is under active development.  
> Examples will be added once the core API stabilizes.

## How It Works

Diffmantic follows a multi-phase AST matching pipeline:

1. **Pre-match** — Seeds stable mappings from unchanged lines  
2. **Top-down matching** — Finds identical/high-confidence subtree pairs  
3. **Bottom-up matching** — Expands matches using mapped descendants  
4. **Recovery matching** — Iteratively recovers remaining valid mappings  
5. **Action generation + analysis** — Produces move/update/insert/delete actions and refined hunks  

## Requirements

- Go (for building the engine)
- Tree-sitter parser for the language you’re diffing  

## License

MIT License — see [LICENSE](LICENSE).

## Acknowledgements

- GumTree repository: https://github.com/GumTreeDiff/gumtree  
- GumTree paper: https://hal.science/hal-04855170v1/file/GumTree_simple__fine_grained__accurate_and_scalable_source_differencing.pdf  
- Beyond GumTree paper: https://www.researchgate.net/publication/335498580_Beyond_GumTree_A_Hybrid_Approach_to_Generate_Edit_Scripts  
