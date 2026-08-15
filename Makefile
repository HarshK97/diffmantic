.PHONY: build build-core build-all test test-unit test-integration test-e2e lint fmt coverage test-update bench bench-short clean

TAGS_16LANGS := grammar_subset grammar_subset_c grammar_subset_cpp grammar_subset_css grammar_subset_go grammar_subset_html grammar_subset_java grammar_subset_javascript grammar_subset_json grammar_subset_lua grammar_subset_php grammar_subset_python grammar_subset_ruby grammar_subset_rust grammar_subset_toml grammar_subset_tsx grammar_subset_typescript grammar_subset_yaml grammar_subset_zig

build: ## Build default binary with 16 core languages (13 MB)
	go build -tags '$(TAGS_16LANGS)' -ldflags="-s -w" -trimpath -o diffm ./cmd/diffm

build-core: ## Build binary with ~100 core grammars (22 MB)
	go build -tags grammar_set_core -ldflags="-s -w" -trimpath -o diffm ./cmd/diffm

build-all: ## Build binary with all ~200+ embedded grammars (29 MB)
	go build -ldflags="-s -w" -trimpath -o diffm ./cmd/diffm



clean: ## Remove built binaries and coverage files
	rm -f diffm coverage.out coverage.html

test: lint test-unit test-integration test-e2e ## Run everything

test-unit: ## Unit tests only
	go test ./internal/... -count=1

test-integration: ## Integration tests (golden files)
	go test ./tests/integration/ -count=1 -v

test-e2e: ## E2E CLI tests
	go test ./tests/e2e/ -count=1 -v

test-update: ## Regenerate golden files
	go test ./tests/integration/ -v -update

bench: ## Run all benchmarks
	go test ./tests/integration/ -bench=. -benchmem -run=^$$ -count=1

bench-short: ## Quick benchmark smoke test (single iteration)
	go test ./tests/integration/ -bench=BenchmarkPipeline -benchmem -run=^$$ -count=1 -benchtime=1x

coverage: ## Coverage report
	go test ./internal/... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | tail -1
	@echo ""
	@echo "Full report: go tool cover -html=coverage.out -o coverage.html"

lint: ## Run linter and check code formatting
	@DIFF=$$(golangci-lint fmt --diff ./...); \
	if [ -n "$$DIFF" ]; then \
		echo "Formatting errors found:"; \
		echo "$$DIFF"; \
		echo ""; \
		echo "Run 'golangci-lint fmt ./...' or 'make fmt' to fix formatting."; \
		exit 1; \
	fi
	golangci-lint run ./...

fmt: ## Format code automatically
	golangci-lint fmt ./...
