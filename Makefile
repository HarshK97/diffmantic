.DEFAULT_GOAL := build

.PHONY: build test test-unit test-integration test-e2e lint fmt coverage test-update bench bench-short grammars-native clean

GRAMMAR_SRCS := native/bridge/grammars.json native/bridge/build_grammars.go $(wildcard native/bridge/src/*.c) $(wildcard native/bridge/include/*.h)

build: native/bridge/lib/libdiffmantic_grammars.a ## Build binary with native Tree-sitter flat-buffer bridge
	go build -ldflags="-s -w" -trimpath -o diffm ./cmd/diffm

native/bridge/lib/libdiffmantic_grammars.a: $(GRAMMAR_SRCS)
	@$(MAKE) grammars-native

grammars-native: ## Fetch and compile 18 native Tree-sitter grammars
	go run ./native/bridge/build_grammars.go

clean: ## Remove built binaries and coverage files
	rm -rf diffm dist coverage.out coverage.html

test: lint test-unit test-integration test-e2e ## Run everything

test-unit: ## Unit tests only
	go test ./internal/... -count=1

test-integration: ## Integration tests (golden files)
	go test ./tests/integration/ -count=1 -v

test-e2e: ## E2E CLI tests
	go test ./tests/e2e/ -count=1 -v

test-update: ## Regenerate golden files
	go test ./tests/integration/ -v -update -count=1

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
		echo "Run 'make fmt' to fix formatting errors automatically."; \
		exit 1; \
	fi
	golangci-lint run ./...

fmt: ## Format all Go code
	golangci-lint fmt ./...
