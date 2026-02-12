SHELL := /bin/bash

IMAGE := hlcut-env:local
GOLANGCI_LINT_VERSION ?= v2.9.0
GOIMPORTS_VERSION ?= v0.42.0
GOBIN := $(shell go env GOPATH)/bin
GOLANGCI_LINT_BIN := $(or $(wildcard $(GOBIN)/golangci-lint),$(shell command -v golangci-lint 2>/dev/null))
GO_FILES := $(shell if command -v rg >/dev/null 2>&1; then rg --files -g '*.go' -g '!.cache/**' -g '!out/**'; else find . -type f -name '*.go' -not -path './.cache/*' -not -path './out/*'; fi)

.PHONY: env_up setup run_local run_docker test itest fmt lint lint_fix hooks_install

env_up:
	docker build -t $(IMAGE) -f docker/env/Dockerfile .

setup: env_up
	# Build whisper.cpp + download model into .cache (not tracked)
	docker run --rm \
		-v "$(PWD)":/work \
		-w /work \
		$(IMAGE) \
		bash -c "./scripts/setup.sh"

run_local: setup
	@if [ -z "$(INPUT)" ]; then echo 'Usage: make run_local INPUT=/path/to/input.mp4 [ARGS="--out ./out --clips 6"]'; exit 2; fi
	go run ./cmd/hlcut "$(INPUT)" $(ARGS)

run_docker: setup env_up
	@if [ -z "$(INPUT)" ]; then echo 'Usage: make run_docker INPUT=/path/to/input.mp4 [ARGS="--out ./out --clips 6"]'; exit 2; fi
	@if [ ! -f .env ]; then echo "Missing .env (copy from .env.example)"; exit 2; fi
	@if ! grep -q '^OPENROUTER_API_KEY=' .env || grep -q '^OPENROUTER_API_KEY=$$' .env; then echo "OPENROUTER_API_KEY is required in .env"; exit 2; fi
	@input="$(INPUT)"; \
		extra_mount=""; \
		container_input="$$input"; \
		if [[ "$$input" = /* ]]; then \
			input_dir="$$(dirname "$$input")"; \
			input_base="$$(basename "$$input")"; \
			extra_mount="-v $$input_dir:/input:ro"; \
			container_input="/input/$$input_base"; \
		fi; \
		docker run --rm \
			--env-file .env \
			-v "$(PWD)":/work \
			$$extra_mount \
			-w /work \
			$(IMAGE) \
			bash -c "go run ./cmd/hlcut \"$$container_input\" $(ARGS)"

# Unit tests (table-driven; mocks allowed)

test: env_up
	docker run --rm \
		-v "$(PWD)":/work \
		-w /work \
		$(IMAGE) \
		bash -c "go test ./..."

# Integration tests (real deps + internet + OpenRouter)

itest: env_up
	@if [ ! -f .env ]; then echo "Missing .env (copy from .env.example)"; exit 2; fi
	@if ! grep -q '^OPENROUTER_API_KEY=' .env || grep -q '^OPENROUTER_API_KEY=$$' .env; then echo "OPENROUTER_API_KEY is required in .env"; exit 2; fi
	docker run --rm \
		--env-file .env \
		-v "$(PWD)":/work \
		-w /work \
		$(IMAGE) \
		bash -c "go test -tags=integration ./..."

fmt: env_up
	docker run --rm \
		-v "$(PWD)":/work \
		-w /work \
		$(IMAGE) \
		bash -c "gofmt -w ."

lint:
	@if [ ! -x "$(GOLANGCI_LINT_BIN)" ]; then echo "golangci-lint is not installed (run 'make lint_fix' once)"; exit 1; fi
	$(GOLANGCI_LINT_BIN) -v run ./...

lint_fix:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)
	$(GOBIN)/goimports -w $(GO_FILES)
	go fmt ./...
	$(GOBIN)/golangci-lint -v run ./...

hooks_install:
	@mkdir -p .git/hooks
	@cp .githooks/pre-commit .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "Installed git hook: .git/hooks/pre-commit"
