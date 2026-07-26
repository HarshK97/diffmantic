.PHONY: test test-unit test-integration test-e2e lint fmt coverage test-update

test: fmt lint test-unit test-integration test-e2e ## Run everything

test-unit: ## Unit tests only
	go test ./internal/... -count=1

test-integration: ## Integration tests (golden files)
	go test ./tests/integration/ -count=1 -v

test-e2e: ## E2E CLI tests
	go test ./tests/e2e/ -count=1 -v

test-update: ## Regenerate golden files
	go test ./tests/integration/ -v -update

coverage: ## Coverage report
	go test ./internal/... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | tail -1
	@echo ""
	@echo "Full report: go tool cover -html=coverage.out -o coverage.html"

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...
