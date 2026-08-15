# Changelog

All notable changes to Diffmantic will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.5.0] - 2026-08-15

### Added
- Completed support for all 16 target languages and formats: Go, Java, JavaScript, TypeScript, JSX, TSX, Python, Rust, Zig, C, C++, PHP, Ruby, JSON, YAML, TOML, HTML, CSS, and Lua with 100+ golden integration test fixtures.
- Added order-insensitive container node matching (`unordered`) for JSON objects, YAML mappings, TOML tables, HTML start tags, CSS selectors, Python dictionaries/sets, and Ruby hashes (PR #90).
- Added key-value pair value container and key-name affinity matching across dictionary and map entries (PR #91).
- Added native keyword container mapping (`keywords`) in parent containers, eliminating cross-scope keyword stealing (PR #85).
- Added multi-line scalar and literal flattening (`flattened`) for multi-line strings, text blocks, comments, and doc comments across languages.
- Added 400 KB file size safety limit with automatic fallback to line diffing on large source files (PR #70).
- Added dynamic worker concurrency throttling in TUI Git status precomputation to keep memory usage under 1.8 GB (PR #72).
- Added Action Inspector panel, change indicators, edge chevrons, and jump keys (`n`, `N`, `[`, `]`) to the TUI viewer (PR #79).

### Changed
- Shifted structural punctuation marks to ignored rules in language rule files so formatting changes do not pollute diff output (PR #83).
- Updated production build configuration to embed the 16 core target languages using selective build tags (`grammar_subset`), reducing binary size to 13.06 MB (-55.2%).
- Upgraded Tree-sitter core dependency from `gotreesitter` v0.48.1 to v0.50.1 (PR #97).

### Fixed
- Fixed large-distance line jumping during LineDiff DP backtracking on repeated token patterns.
- Fixed container node matching in Bottom-Up phase with container tag-name affinity for HTML and XML elements.
- Fixed unaligned line pair gaps by coalescing unaligned deletion and insertion rows in the alignment grid (PR #81).

### Performance
- Parallelized AST parsing (File A and File B) and line group partitioning concurrently using goroutines (PR #96), reducing full pipeline latency by ~20.5% suite-wide.
- Rewrote Chawathe edit script child alignment with direct O(1) parent pointer checks (PR #56), cutting edit script execution time by 19.84% (geometric mean) and reducing heap allocations by 33.89%.
- Accelerated engine performance across large 10,000-line fixtures (PR #74), achieving a 36.0% overall execution time drop (2,098 ms down to 1,333 ms geomean) and cutting heap memory by 635 MB.
- Compacted AST nodes into root-owned flat arrays in `ASTIndex` (PR #55), reducing `BenchmarkMatch` execution time by 17.34% (geometric mean) and memory by 11.12%.
- Reduced compiled binary size from 29.12 MB down to 13.06 MB, achieving a 55.2% size reduction using selective build tags (`grammar_subset`) and symbol stripping flags (`-ldflags="-s -w" -trimpath`).

### Security
- Updated build and dependency configurations to enforce trimmed paths and stripped symbols in production executables.

## [0.4.0] - 2026-08-06

### Added
- Added Universal LineDiff engine with Matsumoto line group partitioning for line-level diffing on unstructured text and fallback cases (PR #46), cutting large diff latency by 42.17% (geometric mean) and saving 43.04% RAM (~48.7 MB per diff).
- Added compact AST Arena index (`ASTIndex`) storing pre-order and post-order node IDs in flat arrays for O(1) subtree containment checks.
- Added Zhang-Shasha tree edit distance recovery for exact alignment of small unmatched subtrees.
- Added automated path-aware benchmarking workflow for pull requests.

### Changed
- Updated Tree-sitter core dependency from `gotreesitter` v0.16.0 to v0.48.1.

### Fixed
- Fixed memory thrashing during line alignment on 14,000-line files by trimming unchanged prefixes and suffixes before running LCS dynamic programming (PR #51), cutting suite execution time by 17.57% (geometric mean) and memory by 12.32%.

### Performance
- Accelerated `TopDown` candidate matching by pre-computing similarity metrics once per candidate pair and indexing height buckets with 64-bit subtree hashes. On a single 4,042-line Rust file (`rust_expr_call_precedence`), candidate sort time dropped from 9.30 seconds to 85.8 microseconds (108,000x sort speedup) and overall diff time dropped by 5.3x.
- Accelerated `BottomUp` candidate matching by walking mapped ancestor chains instead of scanning all target nodes, speeding up BottomUp execution by 6.9x.
- Accelerated leaf node matching with type and label hash maps alongside similarity memoization (PR #48), cutting suite execution time by 5.31% (geomean) and dropping peak leaf matching time on a single 500 KB file (`eval.lua`) from 1 minute 47 seconds to 65 milliseconds (2,000x peak speedup).
- Accelerated line alignment serialization by trimming unchanged prefix and suffix runs (PR #51), reducing matrix memory allocations by 20,000x and speeding up peak serialization on a single 14,000-line file from 1.9 seconds to 2.7 milliseconds (700x peak speedup).

### Security
- Updated Tree-sitter dependency to patch upstream parsing vulnerabilities.

## [0.3.0] - 2026-07-31

### Added
- Added revision diffing support so users can diff git commits directly with commands like `diffm HEAD~1 HEAD`.
- Added path filtering support in Git diff mode.
- Added automated Homebrew tap updates to the release pipeline.

### Changed
- Refactored core engine helpers to use Go standard library `slices` and `maps` packages, removing over 1,000 lines of custom helper code.

### Fixed
- Fixed noise from structural delimiters like parentheses, brackets, and braces through delimiter aliasing and post-processing punctuation filtering.
- Fixed crashes on merge conflict files by adding fallback line-diff handling.

### Performance
- Reduced engine memory overhead by eliminating custom helper slice allocations in favor of Go standard library primitives.

### Security
- Verified safe handling of untrusted diff inputs and revision specifications.

## [0.2.0] - 2026-07-26

### Added
- Added interactive terminal user interface built with Bubbletea, Lipgloss, and Bubbles.
- Added interactive Git status mode with file tree navigation, file staging (`s`), unstaging (`u`), and inline commit capability (`c`).
- Added Action Inspector panel (`i` key) displaying detailed AST edit action data for active lines.
- Added syntax highlighting powered by Tree-sitter query highlights with Catppuccin Mocha color palette.
- Added code folding for unchanged blocks while retaining surrounding context lines.
- Added side-by-side line alignment grid syncing matching AST nodes across left and right panes.
- Added navigation controls including vim movement keys, search, viewport centering (`z.`, `zt`, `zb`), and modal help window (`?`).
- Added function declaration matching by receiver type and function name.

### Changed
- Updated CLI display output to launch interactive TUI mode by default when running in a terminal.

### Fixed
- Fixed scroll alignment bugs during side-by-side rendering in narrow terminal windows.

### Performance
- Optimized TUI view rendering by caching highlighted line spans across redraw cycles.

### Security
- Verified terminal escape sequence handling to prevent terminal injection.

## [0.1.0] - 2026-07-14

### Added
- Created core structural AST diffing engine in Go using Tree-sitter.
- Added two-pass tree matching pipeline with `TopDown` height-bucket matching and `BottomUp` subtree similarity matching.
- Added Chawathe edit script generator producing atomic Insert, Delete, Move, and Update actions.
- Added post-processing step to collapse redundant child actions into single block operations.
- Added AST language parser rules for Go, Python, JavaScript, and TypeScript.
- Created `diffm` CLI entry point with versioned JSON v1 schema output support.
- Added initial release workflow configuration with GoReleaser.

### Changed
- Initial public release of Diffmantic structural diff engine.

### Fixed
- Initial release.

### Performance
- Initial baseline engine implementation.

### Security
- Initial baseline release security check.

[Unreleased]: https://github.com/HarshK97/diffmantic/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/HarshK97/diffmantic/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/HarshK97/diffmantic/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/HarshK97/diffmantic/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/HarshK97/diffmantic/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/HarshK97/diffmantic/releases/tag/v0.1.0
