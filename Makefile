.PHONY: test test-unit test-integration test-e2e lint fmt coverage test-update bench bench-short

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
