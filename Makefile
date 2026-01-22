# golangci-lint version must match CI (see .github/workflows/ci.yml)
GOLANGCI_LINT_VERSION := v2.8.0

.PHONY: proto
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/choo/v1/daemon.proto
	mv proto/choo/v1/*.go pkg/api/v1/

.PHONY: lint
lint: lint-install
	golangci-lint run ./...

.PHONY: lint-install
lint-install:
	@if ! command -v golangci-lint &> /dev/null || \
		[ "$$(golangci-lint version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')" != "$(patsubst v%,%,$(GOLANGCI_LINT_VERSION))" ]; then \
		echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi

.PHONY: test
test:
	go test -race ./...

.PHONY: skills
skills:
	@mkdir -p internal/skills/skills
	@cp .claude/skills/spec.md .claude/skills/spec-validate.md .claude/skills/ralph-prep.md internal/skills/skills/

.PHONY: build
build: skills
	go build ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: check
check: build vet lint test
	@echo "All checks passed!"

.PHONY: install-hooks
install-hooks: lint-install
	@echo "Installing git hooks..."
	@git config core.hooksPath scripts
	@echo "Git hooks installed (using core.hooksPath). Run 'make check' to test locally."
